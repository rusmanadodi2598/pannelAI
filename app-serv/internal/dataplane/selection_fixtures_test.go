// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_fixtures_test.go
// @for       The in-memory doubles and fixtures the selection and resolution tests build on.
// @uses      context, sync, testing, time, internal/domain, internal/repository
// @reason    SPEC-API-001 §7.5 fixes the selection rule (endpoint by priority, then a healthy key, circuit-broken keys skipped), and a branch of it can only be exercised with a repository double: a storage failure, a key that is not active, and a credential the opener cannot read are all states a live database would make awkward to stage. The doubles live here so each test file reads as the rule it pins (AGENTS.md §2.1).
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-17
package dataplane

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// now is the fixed instant every selection test uses, so the circuit window is
// decided by the test rather than by the wall clock.
var now = time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

// memEndpointRepo is an in-memory EndpointRepository, keyed by provider so the
// filter the selector sends is what the double answers.
//
// The mutex guards the double against the fan-out's concurrency: a fusion panel
// records every member's outcome in parallel, so a bare map write here would be
// a data race the production repository (a database) never has. List also clones
// each endpoint it hands back, because a stored endpoint's key slice would
// otherwise be shared by every concurrent member that mutates its health. Tests
// read health directly once Relay has returned, which the fan-out's join orders.
type memEndpointRepo struct {
	mu         sync.Mutex
	byProvider map[string][]domain.UpstreamEndpoint
	// health records the last health write per key id, so a test can assert the
	// circuit the domain owns is what changed.
	health map[string]domain.UpstreamKey
	// outcome records the last parity write per endpoint id, so a test can
	// assert the endpoint's own connection state is what the data plane
	// reported.
	outcome map[string]domain.UpstreamEndpoint
	err     error
	// healthErr fails every health write, so a test can drive the bookkeeping
	// path that runs after an upstream already answered.
	healthErr error
}

func newMemEndpointRepo() *memEndpointRepo {
	return &memEndpointRepo{
		byProvider: map[string][]domain.UpstreamEndpoint{},
		health:     map[string]domain.UpstreamKey{},
		outcome:    map[string]domain.UpstreamEndpoint{},
	}
}

func (r *memEndpointRepo) List(_ context.Context, filter repository.EndpointFilter, _ repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, 0, r.err
	}
	stored := r.byProvider[filter.ProviderID]
	found := make([]domain.UpstreamEndpoint, 0, len(stored))
	for _, endpoint := range stored {
		if filter.Status != "" && string(endpoint.Status()) != filter.Status {
			continue
		}
		found = append(found, endpoint.Clone())
	}
	return found, int64(len(found)), nil
}

func (r *memEndpointRepo) RecordKeyHealth(_ context.Context, key domain.UpstreamKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.healthErr != nil {
		return r.healthErr
	}
	r.health[key.ID()] = key
	return nil
}

func (r *memEndpointRepo) RecordUpstreamOutcome(_ context.Context, endpoint domain.UpstreamEndpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.healthErr != nil {
		return r.healthErr
	}
	r.outcome[endpoint.ID()] = endpoint
	// Write the served endpoint back into its stored row, the way a database
	// persists the row a call touched: List hands out clones, so without this the
	// health a relay recorded (a rotated key, a parity error) would never reach the
	// next read. Storing a clone keeps the row's own key array independent of any
	// caller's live memory.
	rows := r.byProvider[endpoint.ProviderID()]
	for i := range rows {
		if rows[i].ID() == endpoint.ID() {
			rows[i] = endpoint.Clone()
			break
		}
	}
	return nil
}

func (r *memEndpointRepo) Create(context.Context, domain.UpstreamEndpoint) error { return nil }
func (r *memEndpointRepo) GetByID(context.Context, string) (domain.UpstreamEndpoint, error) {
	return domain.UpstreamEndpoint{}, domain.ErrEndpointNotFound
}
func (r *memEndpointRepo) Update(context.Context, domain.UpstreamEndpoint) error { return nil }
func (r *memEndpointRepo) UpdateIfUnchanged(context.Context, domain.UpstreamEndpoint, time.Time) error {
	return nil
}
func (r *memEndpointRepo) Delete(context.Context, string) error { return nil }
func (r *memEndpointRepo) AddKey(context.Context, string, domain.UpstreamKey) error {
	return nil
}
func (r *memEndpointRepo) UpdateKey(context.Context, domain.UpstreamKey) error { return nil }
func (r *memEndpointRepo) DeleteKey(context.Context, string, string) error     { return nil }
func (r *memEndpointRepo) Reorder(context.Context, string, []string) error     { return nil }

// endpointFixture builds one endpoint with a key per priority.
type keyFixture struct {
	id       string
	priority int
	status   domain.UpstreamKeyStatus
	failures int
	// failuresAgo is how long before the fixture's instant the recorded failures
	// happened. A trip recorded at the fixture instant is still inside its
	// backoff window and is therefore correctly skipped by selection, so a test
	// that needs a trip the router may retry must place it in the past.
	failuresAgo time.Duration
}

// buildEndpoint assembles an endpoint from a fixture, using the domain
// constructors so the aggregate's own rules are what the test exercises.
//
// A zero status on a key fixture means active, matching how the table omits the
// field: every fixture that does not care about key state would otherwise have to
// repeat the default, and the parse of an empty string is a validation error, not
// a silent "active".
func buildEndpoint(t *testing.T, id string, priority int, status domain.UpstreamEndpointStatus, keys []keyFixture) domain.UpstreamEndpoint {
	t.Helper()
	endpoint, err := domain.NewUpstreamEndpoint(id, "provider-a", "label-"+id, domain.UpstreamAuthAPIKey, priority, now)
	if err != nil {
		t.Fatalf("building endpoint %s: %v", id, err)
	}
	if status != domain.UpstreamEndpointActive {
		if updateErr := endpoint.Update("label-"+id, priority, string(status), now); updateErr != nil {
			t.Fatalf("disabling endpoint %s: %v", id, updateErr)
		}
	}
	for _, fixture := range keys {
		key := domain.RehydrateUpstreamKey(
			fixture.id, id, "key-"+fixture.id, "v1:nonce:cipher", "abc...wxyz",
			fixture.priority, domain.UpstreamKeyActive, nil, "", 0, nil, now, now,
		)
		if fixture.status != "" && fixture.status != domain.UpstreamKeyActive {
			// reason: the transition is the domain's, so a disabled key is built
			// through it rather than by writing the field.
			disabled, transitionErr := domain.ParseUpstreamKeyStatus(string(fixture.status))
			if transitionErr != nil {
				t.Fatalf("parsing key status: %v", transitionErr)
			}
			if transitionErr = key.Transition(disabled); transitionErr != nil {
				t.Fatalf("transitioning key %s: %v", fixture.id, transitionErr)
			}
		}
		for i := 0; i < fixture.failures; i++ {
			key.RecordFailure("upstream rejected", domain.KeyFailureAuth, now.Add(-fixture.failuresAgo))
		}
		endpoint.AttachKey(key)
	}
	return endpoint
}

// opener is a deterministic SecretOpener: the sealed value's inverse, so a test
// can assert the plaintext only reaches the credential and never the selection.
type opener struct {
	err error
	// failFor refuses to decrypt exactly these sealed values, which is how a test
	// stages one stale ciphertext beside healthy accounts.
	failFor map[string]error
}

func (o opener) Open(sealed string) (string, error) {
	if err, ok := o.failFor[sealed]; ok {
		return "", err
	}
	if o.err != nil {
		return "", o.err
	}
	return "plain-" + sealed, nil
}

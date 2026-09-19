// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/selection_test.go
// @for       Tests for the selection ordering, the model-string resolution order,
//
//	and the error codes each failure answers with.
//
// @uses      testing, internal/domain, internal/repository, internal/schema.
// @reason    SPEC-API-001 §7.5 fixes the selection rule (endpoint by priority, then
//
//	a healthy key, circuit-broken keys skipped) and §7.15 fixes the
//	resolution order and its failure code. Both decide whether a request
//	is served and which account pays for it, so every branch is pinned here
//	against in-memory doubles (AGENTS.md §2.1).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
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
// a data race the production repository (a database) never has. Tests read
// health directly once Relay has returned, which the fan-out's join orders.
type memEndpointRepo struct {
	mu         sync.Mutex
	byProvider map[string][]domain.UpstreamEndpoint
	// health records the last health write per key id, so a test can assert the
	// circuit the domain owns is what changed.
	health map[string]domain.UpstreamKey
	err    error
}

func newMemEndpointRepo() *memEndpointRepo {
	return &memEndpointRepo{
		byProvider: map[string][]domain.UpstreamEndpoint{},
		health:     map[string]domain.UpstreamKey{},
	}
}

func (r *memEndpointRepo) List(_ context.Context, filter repository.EndpointFilter, _ repository.PageQuery) ([]domain.UpstreamEndpoint, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, 0, r.err
	}
	found := r.byProvider[filter.ProviderID]
	if filter.Status != "" {
		kept := make([]domain.UpstreamEndpoint, 0, len(found))
		for _, endpoint := range found {
			if string(endpoint.Status()) == filter.Status {
				kept = append(kept, endpoint)
			}
		}
		found = kept
	}
	return found, int64(len(found)), nil
}

func (r *memEndpointRepo) RecordKeyHealth(_ context.Context, key domain.UpstreamKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.health[key.ID()] = key
	return nil
}

func (r *memEndpointRepo) Create(context.Context, domain.UpstreamEndpoint) error { return nil }
func (r *memEndpointRepo) GetByID(context.Context, string) (domain.UpstreamEndpoint, error) {
	return domain.UpstreamEndpoint{}, domain.ErrEndpointNotFound
}
func (r *memEndpointRepo) Update(context.Context, domain.UpstreamEndpoint) error { return nil }
func (r *memEndpointRepo) Delete(context.Context, string) error                  { return nil }
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
			key.RecordFailure("upstream rejected", now.Add(-fixture.failuresAgo))
		}
		endpoint.AttachKey(key)
	}
	return endpoint
}

// opener is a deterministic SecretOpener: the sealed value's inverse, so a test
// can assert the plaintext only reaches the credential and never the selection.
type opener struct{ err error }

func (o opener) Open(sealed string) (string, error) {
	if o.err != nil {
		return "", o.err
	}
	return "plain-" + sealed, nil
}

// TestSelector_Ordering pins SPEC-API-001 §7.5: endpoints are tried in priority
// order, a disabled endpoint is skipped, and a tripped key is skipped in favour of
// the next healthy one.
func TestSelector_Ordering(t *testing.T) {
	cases := []struct {
		name         string
		endpoints    []domain.UpstreamEndpoint
		wantEndpoint string
		wantKey      string
		wantErr      bool
	}{
		{
			name: "the highest-priority endpoint answers",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_b", priority: 1}}),
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_a",
		},
		{
			name: "a disabled endpoint is skipped for the next one",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointDisabled, []keyFixture{{id: "uky_a", priority: 1}}),
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_b", priority: 1}}),
			},
			wantEndpoint: "ep_b", wantKey: "uky_b",
		},
		{
			name: "the highest-priority healthy key inside the endpoint answers",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_second", priority: 2},
					{id: "uky_first", priority: 1},
				}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_first",
		},
		{
			name: "a key with a disabled status is skipped",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_off", priority: 1, status: domain.UpstreamKeyDisabled},
					{id: "uky_on", priority: 2},
				}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_on",
		},
		{
			name: "a tripped key is skipped for the next one",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_tripped", priority: 1, failures: domain.CircuitThreshold()},
					{id: "uky_healthy", priority: 2},
				}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_healthy",
		},
		{
			name: "a tripped key in the first endpoint fails over to the second",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_tripped", priority: 1, failures: domain.CircuitThreshold()},
				}),
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_b", priority: 1}}),
			},
			wantEndpoint: "ep_b", wantKey: "uky_b",
		},
		{
			name: "an endpoint below the circuit threshold is still used",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_warm", priority: 1, failures: domain.CircuitThreshold() - 1},
				}),
			},
			wantEndpoint: "ep_a", wantKey: "uky_warm",
		},
		{
			name: "every endpoint unavailable yields NO_PROVIDER_AVAILABLE",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointDisabled, []keyFixture{{id: "uky_a", priority: 1}}),
				buildEndpoint(t, "ep_b", 2, domain.UpstreamEndpointDisabled, []keyFixture{{id: "uky_b", priority: 1}}),
			},
			wantErr: true,
		},
		{
			name: "an endpoint whose every key is tripped yields NO_PROVIDER_AVAILABLE",
			endpoints: []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_1", priority: 1, failures: domain.CircuitThreshold()},
					{id: "uky_2", priority: 2, failures: domain.CircuitThreshold()},
				}),
			},
			wantErr: true,
		},
		{
			name:    "a provider with no endpoint yields NO_PROVIDER_AVAILABLE",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = tc.endpoints

			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selector.clock = func() time.Time { return now }

			got, err := selector.Select(context.Background(), "provider-a")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want NO_PROVIDER_AVAILABLE (got %s/%s)", got.Endpoint.ID(), got.Key.ID())
				}
				if code := domain.AsAppError(err).Code; code != "NO_PROVIDER_AVAILABLE" {
					t.Fatalf("code = %q, want NO_PROVIDER_AVAILABLE", code)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want none", err)
			}
			if got.Endpoint.ID() != tc.wantEndpoint {
				t.Fatalf("endpoint = %q, want %q", got.Endpoint.ID(), tc.wantEndpoint)
			}
			if got.Key.ID() != tc.wantKey {
				t.Fatalf("key = %q, want %q", got.Key.ID(), tc.wantKey)
			}
			if got.Credential.APIKey == "" {
				t.Fatal("Credential.APIKey is empty, want the opened plaintext")
			}
			if got.Credential.KeyID != tc.wantKey {
				t.Fatalf("credential key id = %q, want %q", got.Credential.KeyID, tc.wantKey)
			}
		})
	}
}

// TestSelector_HealthAccounting pins that an upstream outcome changes the domain's
// own circuit state and persists it through RecordKeyHealth, so no second health
// model exists.
func TestSelector_HealthAccounting(t *testing.T) {
	cases := []struct {
		name          string
		failures      int
		failuresAgo   time.Duration
		outcome       string
		wantStatus    domain.UpstreamKeyStatus
		wantCounterAt int
	}{
		{name: "a success clears the counter", failures: 1, outcome: "success", wantStatus: domain.UpstreamKeyActive, wantCounterAt: 0},
		{name: "a first failure leaves the key usable", failures: 0, outcome: "failure", wantStatus: domain.UpstreamKeyActive, wantCounterAt: 1},
		{name: "the threshold failure trips the key", failures: domain.CircuitThreshold() - 1, outcome: "failure", wantStatus: domain.UpstreamKeyError, wantCounterAt: domain.CircuitThreshold()},
		{name: "a success after a trip restores the key", failures: domain.CircuitThreshold(), failuresAgo: domain.CircuitBackoff() + time.Minute, outcome: "success", wantStatus: domain.UpstreamKeyActive, wantCounterAt: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{
					{id: "uky_a", priority: 1, failures: tc.failures, failuresAgo: tc.failuresAgo},
				}),
			}
			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selector.clock = func() time.Time { return now }

			selection, err := selector.Select(context.Background(), "provider-a")
			if err != nil {
				t.Fatalf("selecting: %v", err)
			}
			if tc.outcome == "success" {
				err = selector.RecordSuccess(context.Background(), selection)
			} else {
				err = selector.RecordFailure(context.Background(), selection, "upstream rejected")
			}
			if err != nil {
				t.Fatalf("recording %s: %v", tc.outcome, err)
			}

			stored, ok := repo.health["uky_a"]
			if !ok {
				t.Fatal("RecordKeyHealth was not called, so the circuit change was not persisted")
			}
			if stored.Status() != tc.wantStatus {
				t.Fatalf("stored status = %q, want %q", stored.Status(), tc.wantStatus)
			}
			if stored.ConsecutiveErrors() != tc.wantCounterAt {
				t.Fatalf("stored consecutive errors = %d, want %d", stored.ConsecutiveErrors(), tc.wantCounterAt)
			}
		})
	}
}

// TestSelector_RepositoryFailureIsReported pins that a storage failure surfaces
// rather than being swallowed into an empty candidate list, which would be
// reported to the client as "no provider available" and hide a database outage.
func TestSelector_RepositoryFailureIsReported(t *testing.T) {
	repo := newMemEndpointRepo()
	repo.err = errors.New("connection reset")
	selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: opener{}})
	if err != nil {
		t.Fatalf("building selector: %v", err)
	}
	if _, err := selector.Select(context.Background(), "provider-a"); err == nil {
		t.Fatal("err = nil, want the storage failure")
	} else if !strings.Contains(err.Error(), "connection reset") {
		t.Fatalf("err = %v, want it to carry the storage cause", err)
	}
}

// TestSelector_UnreadableCredentialIsInternal pins that an undecryptable stored
// key is an internal failure, not a selection of a different account: sending the
// sealed value upstream would authenticate with ciphertext.
func TestSelector_UnreadableCredentialIsInternal(t *testing.T) {
	cases := []struct {
		name   string
		opener SecretOpener
	}{
		{name: "an opener that fails", opener: opener{err: errors.New("bad ciphertext")}},
		{name: "no opener at all", opener: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemEndpointRepo()
			repo.byProvider["provider-a"] = []domain.UpstreamEndpoint{
				buildEndpoint(t, "ep_a", 1, domain.UpstreamEndpointActive, []keyFixture{{id: "uky_a", priority: 1}}),
			}
			selector, err := NewSelector(SelectorDeps{Endpoints: repo, Opener: tc.opener})
			if err != nil {
				t.Fatalf("building selector: %v", err)
			}
			selection, err := selector.Select(context.Background(), "provider-a")
			if err == nil {
				t.Fatalf("err = nil, want an internal failure (selected %+v)", selection)
			}
			if code := domain.AsAppError(err).Code; code != "INTERNAL_ERROR" {
				t.Fatalf("code = %q, want INTERNAL_ERROR", code)
			}
		})
	}
}

// fakeLookup answers the catalog questions from fixed sets.
type fakeLookup struct {
	combos   map[string]domain.Combo
	aliases  map[string]string
	disabled []domain.ModelRef
	err      error
}

func (l fakeLookup) Combo(_ context.Context, name string) (domain.Combo, bool, error) {
	if l.err != nil {
		return domain.Combo{}, false, l.err
	}
	combo, ok := l.combos[name]
	return combo, ok, nil
}

func (l fakeLookup) Alias(_ context.Context, name string) (string, bool, error) {
	if l.err != nil {
		return "", false, l.err
	}
	target, ok := l.aliases[name]
	return target, ok, nil
}

func (l fakeLookup) Disabled(_ context.Context, providerID, modelID string) (bool, error) {
	if l.err != nil {
		return false, l.err
	}
	for _, ref := range l.disabled {
		if ref.ProviderID() == providerID && ref.ModelID() == modelID {
			return true, nil
		}
	}
	return false, nil
}

func (l fakeLookup) DisabledPairs(context.Context) ([]domain.ModelRef, error) {
	return l.disabled, l.err
}

func (l fakeLookup) ComboNames(context.Context) ([]string, error) {
	if l.err != nil {
		return nil, l.err
	}
	names := make([]string, 0, len(l.combos))
	for name := range l.combos {
		names = append(names, name)
	}
	return names, nil
}

// testIndex builds a registry index covering the routable and non-routable cases
// from one YAML document, so the resolution tests run against the loader the router
// uses rather than a hand-built struct.
func testIndex(t *testing.T) *registry.Index {
	t.Helper()
	index, err := registry.NewIndex(registry.Document{Providers: []registry.Provider{
		{
			ID: "provider-a", Category: "apikey", Alias: "pa", PassthroughModels: true,
			Transport: registry.Transport{Format: registry.DefaultFormat, BaseURL: "https://a.test/v1"},
		},
		{
			ID: "claude-only", Category: "apikey", PassthroughModels: true,
			Transport: registry.Transport{Format: "claude", BaseURL: "https://c.test/v1/messages"},
		},
		{
			ID: "gated", Category: "apikey", Hidden: true,
			Transport: registry.Transport{Format: "claude", BaseURL: "https://g.test/v1"},
			Models:    []registry.Model{{ID: "secret-model"}},
		},
		{
			ID: "connector-only", Category: "oauth", AuthType: registry.AuthOAuth,
			Transport: registry.Transport{Format: "kiro", BaseURL: "https://k.test"},
		},
		{
			ID: "declared", Category: "apikey",
			Transport: registry.Transport{Format: registry.DefaultFormat, BaseURL: "https://d.test/v1"},
			Models: []registry.Model{
				{ID: "known-model"},
				{ID: "exposed-model", UpstreamModelID: "real-model"},
				{ID: "claude-native", TargetFormat: "claude"},
				{ID: "an-image", Kind: "image"},
			},
		},
	}})
	if err != nil {
		t.Fatalf("building index: %v", err)
	}
	return index
}

// TestResolver_Order pins the documented resolution order (SPEC-API-001 §7.15):
// combo name, then alias, then provider/model, then MODEL_NOT_FOUND — including
// each failure and the provider-not-routable case §8 adds.
func TestResolver_Order(t *testing.T) {
	lookup := fakeLookup{
		combos: map[string]domain.Combo{
			"my-combo":   comboRow("my-combo", "provider-a/fast", "claude-only/slow"),
			"nested":     comboRow("nested", "my-combo"),
			"empty":      comboRow("empty"),
			"claude-com": comboRow("claude-com", "claude-only/x"),
		},
		aliases: map[string]string{
			"fast":     "provider-a/gpt-fast",
			"shortcut": "my-combo",
			"to-bad":   "connector-only/model",
			"chain":    "fast",
		},
	}

	cases := []struct {
		name         string
		model        string
		wantProvider string
		wantModel    string
		wantUpstream string
		wantTarget   string
		wantCombo    string
		wantErrCode  string
	}{
		{
			name:  "a combo name resolves to its first member and keeps the combo",
			model: "my-combo", wantProvider: "provider-a", wantModel: "fast",
			wantUpstream: "fast", wantTarget: TargetOpenAI, wantCombo: "my-combo",
		},
		{
			name:  "a combo whose first member is a claude provider keeps that target",
			model: "claude-com", wantProvider: "claude-only", wantModel: "x",
			wantUpstream: "x", wantTarget: TargetClaude, wantCombo: "claude-com",
		},
		{
			name:  "an alias resolves to its provider and model",
			model: "fast", wantProvider: "provider-a", wantModel: "gpt-fast", wantUpstream: "gpt-fast",
			wantTarget: TargetOpenAI,
		},
		{
			name:  "an alias pointing at a combo resolves through it",
			model: "shortcut", wantProvider: "provider-a", wantModel: "fast",
			wantUpstream: "fast", wantTarget: TargetOpenAI, wantCombo: "my-combo",
		},
		{
			name:  "a provider alias resolves as the provider",
			model: "pa/thing", wantProvider: "provider-a", wantModel: "thing",
			wantUpstream: "thing", wantTarget: TargetOpenAI,
		},
		{
			name:  "a declared model uses its upstream override",
			model: "declared/exposed-model", wantProvider: "declared", wantModel: "exposed-model",
			wantUpstream: "real-model", wantTarget: TargetOpenAI,
		},
		{
			name:  "a declared model may override the target format",
			model: "declared/claude-native", wantProvider: "declared", wantModel: "claude-native",
			wantUpstream: "claude-native", wantTarget: TargetClaude,
		},
		{
			name:  "a passthrough provider accepts an undeclared model",
			model: "provider-a/anything", wantProvider: "provider-a", wantModel: "anything",
			wantUpstream: "anything", wantTarget: TargetOpenAI,
		},
		{
			name:  "a model a non-passthrough provider does not declare is MODEL_NOT_FOUND",
			model: "declared/missing", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "an unknown provider is MODEL_NOT_FOUND",
			model: "nope/model", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "an unknown bare name is MODEL_NOT_FOUND",
			model: "nothing-like-this", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "a model string with no provider segment is MODEL_NOT_FOUND",
			model: "justaname/", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "an empty model string is MODEL_NOT_FOUND",
			model: "", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "a provider whose protocol has no translator is PROVIDER_NOT_ROUTABLE",
			model: "connector-only/model", wantErrCode: CodeProviderNotRoutable,
		},
		{
			name:  "an alias pointing at an unroutable provider is PROVIDER_NOT_ROUTABLE",
			model: "to-bad", wantErrCode: CodeProviderNotRoutable,
		},
		{
			name:  "an empty combo falls through to the alias path",
			model: "empty", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "an alias chain terminates at the model",
			model: "chain", wantProvider: "provider-a", wantModel: "gpt-fast",
			wantUpstream: "gpt-fast", wantTarget: TargetOpenAI,
		},
		{
			name:  "a nested combo is not expanded, so the outer name is refused",
			model: "nested", wantErrCode: CodeModelNotFound,
		},
		{
			name:  "a hidden provider's model is still resolvable by id",
			model: "gated/secret-model", wantProvider: "gated", wantModel: "secret-model",
			wantUpstream: "secret-model", wantTarget: TargetClaude,
		},
	}

	resolver, err := NewResolver(testIndex(t), lookup)
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolver.Resolve(context.Background(), tc.model)
			if tc.wantErrCode != "" {
				if err == nil {
					t.Fatalf("err = nil, want %s (got %+v)", tc.wantErrCode, got)
				}
				if code := AsError(err).Code; code != tc.wantErrCode {
					t.Fatalf("code = %q, want %q (message: %s)", code, tc.wantErrCode, AsError(err).Message)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want none", err)
			}
			if got.Provider.ID != tc.wantProvider {
				t.Fatalf("provider = %q, want %q", got.Provider.ID, tc.wantProvider)
			}
			if got.ModelID != tc.wantModel {
				t.Fatalf("model = %q, want %q", got.ModelID, tc.wantModel)
			}
			if got.UpstreamID != tc.wantUpstream {
				t.Fatalf("upstream id = %q, want %q", got.UpstreamID, tc.wantUpstream)
			}
			if got.Target != tc.wantTarget {
				t.Fatalf("target = %q, want %q", got.Target, tc.wantTarget)
			}
			if got.Combo != tc.wantCombo {
				t.Fatalf("combo = %q, want %q", got.Combo, tc.wantCombo)
			}
		})
	}
}

// TestResolver_ModelList pins the models list: routable models only, disabled
// pairs hidden, combos owned by "combo", and a stable order.
func TestResolver_ModelList(t *testing.T) {
	lookup := fakeLookup{
		combos:  map[string]domain.Combo{"my-combo": comboRow("my-combo", "provider-a/x")},
		aliases: map[string]string{},
		disabled: func() []domain.ModelRef {
			ref, err := domain.NewModelRef("declared", "known-model")
			if err != nil {
				t.Fatalf("building disabled ref: %v", err)
			}
			return []domain.ModelRef{ref}
		}(),
	}
	resolver, err := NewResolver(testIndex(t), lookup)
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}

	list, err := resolver.ModelList(context.Background())
	if err != nil {
		t.Fatalf("ModelList: %v", err)
	}
	if list.Object != "list" {
		t.Fatalf("Object = %q, want list", list.Object)
	}

	for _, entry := range list.Data {
		if entry.Object != "model" {
			t.Fatalf("entry %q object = %q, want model", entry.ID, entry.Object)
		}
	}
	// The list is exactly the chat-capable declared models of translatable
	// providers, plus every combo. A passthrough provider enumerates nothing, an
	// image model is not chat, and a disabled pair is hidden.
	want := []string{"declared/claude-native", "declared/exposed-model", "gated/secret-model", "my-combo"}
	got := make([]string, 0, len(list.Data))
	for _, entry := range list.Data {
		got = append(got, entry.ID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("models list = %v, want %v", got, want)
	}
	for i := 1; i < len(list.Data); i++ {
		if list.Data[i-1].ID > list.Data[i].ID {
			t.Fatalf("models list is not sorted: %q before %q", list.Data[i-1].ID, list.Data[i].ID)
		}
	}
}

// TestTargetFormat pins the translator check: only a format with a translator
// decodes as routable, so a Gemini or Responses provider is refused by name rather
// than sent as a 502.
func TestTargetFormat(t *testing.T) {
	cases := []struct {
		format string
		want   string
	}{
		{format: registry.DefaultFormat, want: TargetOpenAI},
		{format: "claude", want: TargetClaude},
		{format: "gemini", want: ""},
		{format: "gemini-cli", want: ""},
		{format: registry.FormatOpenAIResponses, want: ""},
		{format: "kiro", want: ""},
		{format: "", want: ""},
	}
	for _, tc := range cases {
		t.Run("format="+tc.format, func(t *testing.T) {
			if got := targetFormat(tc.format); got != tc.want {
				t.Fatalf("targetFormat(%q) = %q, want %q", tc.format, got, tc.want)
			}
		})
	}
}

// TestDataPlaneErrorStatus pins the OpenAI envelope mapping (SPEC-API-001 §4, §8):
// MODEL_NOT_FOUND and PROVIDER_NOT_ROUTABLE are client errors, and an upstream
// failure keeps its own class.
func TestDataPlaneErrorStatus(t *testing.T) {
	cases := []struct {
		name       string
		code       string
		wantStatus int
		wantType   string
	}{
		{name: "validation", code: CodeValidation, wantStatus: 400, wantType: "invalid_request_error"},
		{name: "model not found", code: CodeModelNotFound, wantStatus: 400, wantType: "invalid_request_error"},
		{name: "provider not routable", code: CodeProviderNotRoutable, wantStatus: 400, wantType: "invalid_request_error"},
		{name: "unauthorized", code: CodeUnauthorized, wantStatus: 401, wantType: "authentication_error"},
		{name: "rate limited", code: CodeRateLimited, wantStatus: 429, wantType: "rate_limit_error"},
		{name: "no provider", code: CodeNoProvider, wantStatus: 503, wantType: "server_error"},
		{name: "upstream error", code: CodeUpstreamError, wantStatus: 502, wantType: "server_error"},
		{name: "upstream timeout", code: CodeUpstreamTimeout, wantStatus: 504, wantType: "server_error"},
		{name: "internal", code: CodeInternal, wantStatus: 500, wantType: "server_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failure := dataPlaneError(tc.code, "a message")
			if failure.OpenAIStatus() != tc.wantStatus {
				t.Fatalf("status = %d, want %d", failure.OpenAIStatus(), tc.wantStatus)
			}
			if failure.Type != tc.wantType {
				t.Fatalf("type = %q, want %q", failure.Type, tc.wantType)
			}
			if failure.Message != "a message" {
				t.Fatalf("message = %q, want the English message passed in", failure.Message)
			}
		})
	}
}

// TestAsError_ManagementMapping pins that a management error from a shared
// dependency does not leak a management code to a CLI tool.
func TestAsError_ManagementMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{name: "a not-found maps to MODEL_NOT_FOUND", err: domain.NewNotFoundError("gone"), wantCode: CodeModelNotFound},
		{name: "an unauthorized maps to UNAUTHORIZED", err: domain.NewUnauthorizedError("no"), wantCode: CodeUnauthorized},
		{name: "a conflict maps to INTERNAL_ERROR", err: domain.NewConflictError("dup"), wantCode: CodeInternal},
		{name: "a plain error maps to INTERNAL_ERROR", err: errors.New("boom"), wantCode: CodeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AsError(tc.err).Code; got != tc.wantCode {
				t.Fatalf("code = %q, want %q", got, tc.wantCode)
			}
		})
	}
	if got := AsError(nil); got != nil {
		t.Fatalf("AsError(nil) = %+v, want nil", got)
	}
}

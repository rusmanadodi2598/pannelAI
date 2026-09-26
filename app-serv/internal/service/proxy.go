// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy.go
// @for       The proxy pool use cases: sealed candidates, CRUD, and the two
//
//	connectivity tests (SPEC-API-001 §7.11).
//
// @uses      internal/domain, internal/repository, context, time.
// @reason    §7.11 makes the password write-only and the test a diagnostic; both
//
//	are service rules rather than request-shape rules. The prober is a
//	one-method seam because AGENTS.md §1.5 keeps net/http out of this
//	layer — and because the SSRF guard must sit with the dialer that
//	enforces it, so this layer only reports what the test found.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/repository"
)

// ProxyTarget is one connection a test dials, with the password unsealed for
// the duration of the call. The plaintext never reaches a log or a response.
type ProxyTarget struct {
	Protocol domain.ProxyProtocol
	Host     string
	Port     int
	Username string
	Password string
}

// ProxyProbeResult is what the adapter found.
type ProxyProbeResult struct {
	State     string
	LatencyMS int
	Message   string
}

// ProxyProber runs one connectivity test. A refused destination is a result
// with State fail, not an error: the operator asked whether the candidate
// works, and "no, because the address is private" is the answer.
type ProxyProber interface {
	ProbeProxy(ctx context.Context, target ProxyTarget) (ProxyProbeResult, error)
}

// ProxyDraft is the create/update input.
type ProxyDraft struct {
	Label    string
	Protocol domain.ProxyProtocol
	Host     string
	Port     int
	Username string
	// Password is the plaintext the operator typed. Empty on an update means
	// "keep the stored secret", which is what makes it write-only.
	Password string
	// Enabled is nil when the caller said nothing, which keeps the stored state.
	Enabled *bool
}

// ProxyPatch is a partial change to a stored candidate. A nil field means
// "leave unchanged"; an empty non-nil password means "keep the secret".
type ProxyPatch struct {
	Label    *string
	Protocol *string
	Host     *string
	Port     *int
	Username *string
	Password *string
	Enabled  *bool
}

// ProxyServiceDeps holds the collaborators the service needs.
type ProxyServiceDeps struct {
	Repo   repository.ProxyRepository
	Sealer CredentialSealer
	Prober ProxyProber
}

// ProxyService implements §7.11.
type ProxyService struct {
	repo   repository.ProxyRepository
	sealer CredentialSealer
	prober ProxyProber
	clock  func() time.Time
}

// NewProxyService validates deps and returns a ready service.
func NewProxyService(deps ProxyServiceDeps) (*ProxyService, error) {
	if deps.Repo == nil {
		return nil, domain.NewValidationError("proxy service requires a repository")
	}
	if deps.Sealer == nil {
		return nil, domain.NewValidationError("proxy service requires a credential sealer")
	}
	if deps.Prober == nil {
		return nil, domain.NewValidationError("proxy service requires a connectivity prober")
	}
	return &ProxyService{repo: deps.Repo, sealer: deps.Sealer, prober: deps.Prober, clock: time.Now}, nil
}

// List returns the pool, ordered by label.
func (s *ProxyService) List(ctx context.Context) ([]domain.Proxy, error) {
	return s.repo.List(ctx)
}

// Create seals the password and stores a new candidate.
func (s *ProxyService) Create(ctx context.Context, draft ProxyDraft) (domain.Proxy, error) {
	sealed, err := s.seal(draft.Password)
	if err != nil {
		return domain.Proxy{}, err
	}
	proxy, err := domain.NewProxy("", draft.Label, draft.Protocol, draft.Host, draft.Port,
		draft.Username, sealed, s.clock().UTC())
	if err != nil {
		return domain.Proxy{}, err
	}
	if draft.Enabled != nil && !*draft.Enabled {
		proxy.SetEnabled(false, s.clock().UTC())
	}
	if err := s.repo.Create(ctx, proxy); err != nil {
		return domain.Proxy{}, err
	}
	return proxy, nil
}

// Update applies the partial patch of §7.11. An omitted field keeps its
// stored value, which is what makes a label-only save possible.
//
// A repoint or a credential change clears the stored test result, so each is
// applied only when the value actually changed: saving an unchanged form must
// not erase the last proof the candidate was tested. An empty password keeps
// the stored secret, because the panel renders has_password and never the value.
func (s *ProxyService) Update(ctx context.Context, id string, patch ProxyPatch) (domain.Proxy, error) {
	if patch.Label == nil && patch.Protocol == nil && patch.Host == nil &&
		patch.Port == nil && patch.Username == nil && patch.Password == nil &&
		patch.Enabled == nil {
		return domain.Proxy{}, domain.NewValidationError("nothing to update")
	}
	proxy, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return domain.Proxy{}, err
	}
	now := s.clock().UTC()
	if patch.Label != nil {
		if err := proxy.Relabel(*patch.Label, now); err != nil {
			return domain.Proxy{}, err
		}
	}
	protocol := proxy.Protocol()
	if patch.Protocol != nil {
		parsed, err := domain.ParseProxyProtocol(*patch.Protocol)
		if err != nil {
			return domain.Proxy{}, err
		}
		protocol = parsed
	}
	host := proxy.Host()
	if patch.Host != nil {
		host = *patch.Host
	}
	port := proxy.Port()
	if patch.Port != nil {
		port = *patch.Port
	}
	if protocol != proxy.Protocol() || host != proxy.Host() || port != proxy.Port() {
		if err := proxy.Repoint(protocol, host, port, now); err != nil {
			return domain.Proxy{}, err
		}
	}
	username := proxy.Username()
	if patch.Username != nil {
		username = *patch.Username
	}
	if err := s.applyCredentials(&proxy, patch, username, now); err != nil {
		return domain.Proxy{}, err
	}
	if patch.Enabled != nil && *patch.Enabled != proxy.Enabled() {
		proxy.SetEnabled(*patch.Enabled, now)
	}
	if err := s.repo.Update(ctx, proxy); err != nil {
		return domain.Proxy{}, err
	}
	return proxy, nil
}

// applyCredentials seals a retyped password or reuses the stored secret for a
// username-only change. A client that resends the same secret — rather than
// the panel, which never holds it — does not invalidate the last test result.
func (s *ProxyService) applyCredentials(proxy *domain.Proxy, patch ProxyPatch, username string, now time.Time) error {
	switch {
	case patch.Password != nil && *patch.Password != "":
		if s.sameSecret(*proxy, *patch.Password) && proxy.Username() == username {
			return nil
		}
		sealed, err := s.seal(*patch.Password)
		if err != nil {
			return err
		}
		return proxy.SetCredentials(username, sealed, now)
	case proxy.Username() != username:
		return proxy.SetCredentials(username, proxy.PasswordEncrypted(), now)
	default:
		return nil
	}
}

// Delete removes one candidate.
func (s *ProxyService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

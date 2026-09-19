// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/proxy.go
// @for       The proxy pool contracts: list, create, patch, the two test routes,
//
//	and the status both routes report (SPEC-API-001 §7.11).
//
// @uses      go-playground/validator/v10 through shared validation, internal/domain.
// @reason    AGENTS.md §2.4 requires the typed contract before the handler, and
//
//	§7.11 makes the password write-only: it appears in a request and never
//	in a response, which is why the response carries has_password rather
//	than a value. The create and patch bodies are one struct for the same
//	reason §7.7's are — the panel edits the whole form and saves it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-19
package schema

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// ProxyRequest is the body of POST /api/v1/proxies and PATCH /api/v1/proxies/{id}.
//
// Password is write-only: on a patch an empty value keeps the stored secret, so
// a panel that renders has_password without the value cannot erase it by saving.
// Enabled is a pointer so an omitted field keeps the stored state rather than
// silently disabling the candidate.
type ProxyRequest struct {
	Label    string `json:"label"    validate:"required,min=1,max=120"`
	Protocol string `json:"protocol" validate:"required,oneof=http https socks5"`
	Host     string `json:"host"     validate:"required,max=253"`
	Port     int    `json:"port"     validate:"required,min=1,max=65535"`
	Username string `json:"username" validate:"omitempty,max=200"`
	Password string `json:"password" validate:"omitempty,max=1024"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

// ProxyCandidateRequest is the body of POST /api/v1/proxies/test: an unsaved
// candidate, tested before it is stored.
type ProxyCandidateRequest struct {
	Protocol string `json:"protocol" validate:"required,oneof=http https socks5"`
	Host     string `json:"host"     validate:"required,max=253"`
	Port     int    `json:"port"     validate:"required,min=1,max=65535"`
	Username string `json:"username" validate:"omitempty,max=200"`
	Password string `json:"password" validate:"omitempty,max=1024"`
}

// ProxyTestStatusResponse is the last connectivity test, as the panel reads it.
type ProxyTestStatusResponse struct {
	State     string `json:"state"`
	LatencyMS int    `json:"latency_ms"`
	CheckedAt string `json:"checked_at,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ProxyResponse is one candidate on the wire. The password is never echoed;
// HasPassword is what the panel renders.
type ProxyResponse struct {
	ID          string                   `json:"id"`
	Label       string                   `json:"label"`
	Protocol    string                   `json:"protocol"`
	Host        string                   `json:"host"`
	Port        int                      `json:"port"`
	Username    string                   `json:"username"`
	HasPassword bool                     `json:"has_password"`
	Enabled     bool                     `json:"enabled"`
	Status      *ProxyTestStatusResponse `json:"status,omitempty"`
	CreatedAt   string                   `json:"created_at"`
	UpdatedAt   string                   `json:"updated_at"`
}

// ProxyList wraps the pool. §7.11 lists it without pagination: an operator
// keeps a handful of candidates, and the list is read whole by the panel.
type ProxyList struct {
	Data []ProxyResponse `json:"data"`
}

// ProxyTestResponse is the answer of both test routes (§7.11).
//
// State, latency_ms, and checked_at are the spec's fields. Message is additive
// and omitted when empty: a failed test that says only "fail" sends an operator
// looking for a cause the gateway already knows.
type ProxyTestResponse struct {
	State     string `json:"state"`
	LatencyMS int    `json:"latency_ms"`
	CheckedAt string `json:"checked_at"`
	Message   string `json:"message,omitempty"`
}

// ToProxyResponse maps a candidate onto the wire shape.
func ToProxyResponse(proxy domain.Proxy) ProxyResponse {
	return ProxyResponse{
		ID:          proxy.ID(),
		Label:       proxy.Label(),
		Protocol:    string(proxy.Protocol()),
		Host:        proxy.Host(),
		Port:        proxy.Port(),
		Username:    proxy.Username(),
		HasPassword: proxy.HasPassword(),
		Enabled:     proxy.Enabled(),
		Status:      toProxyTestStatus(proxy.Status()),
		CreatedAt:   Timestamp(proxy.CreatedAt()),
		UpdatedAt:   Timestamp(proxy.UpdatedAt()),
	}
}

// ToProxyResponses maps the pool onto the wire shape.
func ToProxyResponses(proxies []domain.Proxy) []ProxyResponse {
	out := make([]ProxyResponse, 0, len(proxies))
	for _, proxy := range proxies {
		out = append(out, ToProxyResponse(proxy))
	}
	return out
}

// toProxyTestStatus renders a stored status, or nil when the candidate was
// never tested — an untested candidate has no state to report, which is not the
// same as a failed one.
func toProxyTestStatus(status domain.ProxyTestStatus) *ProxyTestStatusResponse {
	if status.State == "" && status.CheckedAt == nil {
		return nil
	}
	checkedAt := ""
	if status.CheckedAt != nil {
		checkedAt = Timestamp(*status.CheckedAt)
	}
	return &ProxyTestStatusResponse{
		State:     status.State,
		LatencyMS: status.LatencyMS,
		CheckedAt: checkedAt,
		Message:   status.Message,
	}
}

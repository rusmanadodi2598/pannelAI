// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/usage_record.go
// @for       The raw usage record, quota window, and quota cap contracts.
// @uses      internal/domain (UsageRecord, QuotaWindow, QuotaCap, ParseDecimal).
// @reason    SPEC-API-001 §7.12 returns raw records, quota windows, and budget
//
//	caps; §4 requires cost as a decimal string and §2.4 requires the
//	cap body typed and validated, so the money rule is enforced here
//	as well as in the domain where the value object lives.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability experimental
// @since     2026-09-18
package schema

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// UsageRecordResponse is one raw usage row.
type UsageRecordResponse struct {
	ID               string `json:"id"`
	RequestID        string `json:"request_id"`
	TS               string `json:"ts"`
	EndpointID       string `json:"endpoint_id,omitempty"`
	ProviderID       string `json:"provider_id"`
	GatewayKeyID     string `json:"gateway_key_id,omitempty"`
	Model            string `json:"model"`
	Combo            string `json:"combo,omitempty"`
	TokensIn         int64  `json:"tokens_in"`
	TokensOut        int64  `json:"tokens_out"`
	TokensCacheRead  int64  `json:"tokens_cache_read"`
	TokensCacheWrite int64  `json:"tokens_cache_write"`
	CostUSD          string `json:"cost_usd"`
	LatencyMS        int64  `json:"latency_ms"`
	Status           string `json:"status"`
	ErrorCode        string `json:"error_code,omitempty"`
}

// UsageRecordList wraps a page of records with the pagination meta block.
type UsageRecordList struct {
	Data []UsageRecordResponse `json:"data"`
	Meta Page                  `json:"meta"`
}

// UsageRecordDetailResponse is one record joined with its captured log when
// capture is on (SPEC-API-001 §7.12). CaptureEnabled is always present so the
// panel can say "capture is off" instead of rendering an empty body area.
type UsageRecordDetailResponse struct {
	Usage          UsageRecordResponse `json:"usage"`
	CaptureEnabled bool                `json:"capture_enabled"`
	Log            *LogDetailResponse  `json:"log,omitempty"`
}

// QuotaWindowResponse is one quota window (SPEC-API-001 §7.12). Limit is a
// pointer because a window with no configured ceiling has none, which is not
// the same as a ceiling of zero.
type QuotaWindowResponse struct {
	EndpointID string  `json:"endpoint_id"`
	ProviderID string  `json:"provider_id"`
	Window     string  `json:"window"`
	Used       int64   `json:"used"`
	Limit      *int64  `json:"limit,omitempty"`
	ResetsAt   *string `json:"resets_at,omitempty"`
	Source     string  `json:"source"`
}

// QuotaWindowList is the body of GET /api/v1/quotas, which reads every
// endpoint's windows. The collection route carries no cap: a cap belongs to one
// endpoint, and the per-endpoint route is where a client reads it back.
type QuotaWindowList struct {
	Data []QuotaWindowResponse `json:"data"`
}

// QuotaEndpointDetail is the body of GET /api/v1/quotas/{endpoint_id}: that
// endpoint's windows plus its stored budget cap, so a client that just wrote a
// cap reads the same values back (SPEC-API-001 §7.12, owner decision D5 = b).
// Cap is null when no cap is stored, a state the panel renders as an empty
// form, which a missing field could not be told apart from.
type QuotaEndpointDetail struct {
	EndpointID string                `json:"endpoint_id"`
	Cap        *QuotaCapResponse     `json:"cap"`
	Data       []QuotaWindowResponse `json:"data"`
}

// QuotaCapResponse is the body of PUT /api/v1/quotas/{endpoint_id}. An omitted
// field means no cap is set, which is distinct from a cap of zero.
type QuotaCapResponse struct {
	EndpointID     string  `json:"endpoint_id"`
	MonthlyCostUSD *string `json:"monthly_cost_usd,omitempty"`
	MonthlyTokens  *int64  `json:"monthly_tokens,omitempty"`
	UpdatedAt      string  `json:"updated_at"`
}

// QuotaCapRequest is the body of PUT /api/v1/quotas/{endpoint_id}. Cost is a
// decimal string so the wire never carries a float (SPEC-API-001 §4). An
// omitted field clears that cap: the route replaces the cap set, matching the
// reference's whole-object write, so a client removing a budget sends the body
// without that field rather than leaving a value it cannot see. The value rules
// (sign, ceiling, degenerate zero cost) live in ValidateQuotaCap and the domain
// rule set it shares, not in struct tags, so they are stated exactly once.
type QuotaCapRequest struct {
	MonthlyCostUSD *string `json:"monthly_cost_usd,omitempty"`
	MonthlyTokens  *int64  `json:"monthly_tokens,omitempty"`
}

// ValidateQuotaCap enforces what the wire must check before the service sees
// the body: the cost has to be a parseable decimal string, and both amounts
// have to satisfy the one cap rule set the domain constructor also applies
// (domain.ValidateQuotaCapValues), so this boundary and the domain cannot
// disagree about what a legal cap is.
func ValidateQuotaCap(req QuotaCapRequest) error {
	var cost *domain.Decimal
	if req.MonthlyCostUSD != nil {
		amount, err := domain.ParseDecimal(*req.MonthlyCostUSD)
		if err != nil {
			return err
		}
		cost = &amount
	}
	return domain.ValidateQuotaCapValues(cost, req.MonthlyTokens)
}

// UsageRecordResponseFrom maps one stored record onto the wire shape.
func UsageRecordResponseFrom(rec domain.UsageRecord) UsageRecordResponse {
	return UsageRecordResponse{
		ID:               rec.ID(),
		RequestID:        rec.RequestID(),
		TS:               Timestamp(rec.TS()),
		EndpointID:       rec.EndpointID(),
		ProviderID:       rec.ProviderID(),
		GatewayKeyID:     rec.GatewayKeyID(),
		Model:            rec.Model(),
		Combo:            rec.Combo(),
		TokensIn:         rec.TokensIn(),
		TokensOut:        rec.TokensOut(),
		TokensCacheRead:  rec.TokensCacheRead(),
		TokensCacheWrite: rec.TokensCacheWrite(),
		CostUSD:          rec.CostUSD(),
		LatencyMS:        rec.LatencyMS(),
		Status:           rec.Status(),
		ErrorCode:        rec.ErrorCode(),
	}
}

// QuotaWindowResponseFrom maps one stored window onto the wire shape.
func QuotaWindowResponseFrom(w domain.QuotaWindow) QuotaWindowResponse {
	resp := QuotaWindowResponse{
		EndpointID: w.EndpointID(),
		ProviderID: w.ProviderID(),
		Window:     string(w.Window()),
		Used:       w.Used(),
		Source:     string(w.Source()),
	}
	if limit, ok := w.Limit(); ok {
		resp.Limit = &limit
	}
	if resets := w.ResetsAt(); resets != nil {
		formatted := Timestamp(*resets)
		resp.ResetsAt = &formatted
	}
	return resp
}

// QuotaCapResponseFrom maps the stored cap onto the wire shape.
func QuotaCapResponseFrom(cap domain.QuotaCap) QuotaCapResponse {
	resp := QuotaCapResponse{EndpointID: cap.EndpointID(), UpdatedAt: Timestamp(cap.UpdatedAt())}
	if cost, ok := cap.MonthlyCostUSD(); ok {
		resp.MonthlyCostUSD = &cost
	}
	if tokens, ok := cap.MonthlyTokens(); ok {
		resp.MonthlyTokens = &tokens
	}
	return resp
}

// UsageRecordDetailFrom assembles the detail response from the usage record and
// the optional captured log. It takes the parts rather than the service type so
// the schema layer never imports the service layer (AGENTS.md §1.5 layer flow).
func UsageRecordDetailFrom(usage UsageRecordResponse, captureEnabled bool, maxBytes int, log *LogDetailResponse) UsageRecordDetailResponse {
	detail := UsageRecordDetailResponse{
		Usage:          usage,
		CaptureEnabled: captureEnabled,
	}
	if captureEnabled && log != nil {
		detail.Log = log
	}
	return detail
}

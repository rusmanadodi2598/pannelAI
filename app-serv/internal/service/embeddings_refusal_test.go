// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/embeddings_refusal_test.go
// @for       The log row POST /embeddings leaves when a call is refused before
//
//	any upstream attempt, and the identity such a row can still name.
//
// @uses      internal/dataplane, internal/domain, internal/schema, context,
//
//	testing, time.
//
// @reason    Register G20 names embeddings alongside the media routes: the
//
//	service wrote rows only after the call, so an unresolvable model, a
//	combo, a provider without an embeddings block, or no usable account
//	never appeared in the Logs screen. The resolution phase returns the
//	identity resolved so far with its error, which is what these cases
//	pin, together with the benign control that a served call still
//	writes the accounting pair.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"testing"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// stubModelResolver answers resolution with one canned resolution or failure.
type stubModelResolver struct {
	resolution dataplane.Resolution
	failure    error
}

func (r stubModelResolver) Resolve(context.Context, string) (dataplane.Resolution, error) {
	if r.failure != nil {
		return dataplane.Resolution{}, r.failure
	}
	return r.resolution, nil
}

// ResolveForSystemOne answers the decision route's question with the same canned
// resolution, so one stub serves both planes' tests. The context it was given is
// the one it passes on, so a caller's cancellation reaches the answer.
func (r stubModelResolver) ResolveForSystemOne(ctx context.Context, _ string) (dataplane.Resolution, error) {
	return r.Resolve(ctx, "")
}

// embeddingsRefusalFixture builds the embeddings service over the collecting
// doubles with a fixed request id, so one refusal's row can be read field by
// field. No engine is built: the resolver and the router are the two ports the
// use case asks its questions through.
func embeddingsRefusalFixture(resolver ModelResolver, router MediaRouter) (*EmbeddingsService, *stubUsageRecorder, *stubLogRecorder) {
	usage, logs := &stubUsageRecorder{}, &stubLogRecorder{}
	svc := &EmbeddingsService{
		resolver: resolver,
		router:   router,
		caller:   &stubMediaCaller{answer: dataplane.MediaResponse{Status: 200, Body: []byte(`{"data":[{"embedding":[0.1],"index":0}]}`)}},
		recorder: newDataPlaneRecorder(usage, logs, nil, func(context.Context) string { return "req_embed_refused" }),
	}
	return svc, usage, logs
}

// embeddingsProvider is the provider a refusal case resolves to: it declares an
// OpenAI-shaped embeddings block, so the case's failure is the one under test.
func embeddingsProvider() registry.Provider {
	return registry.Provider{
		ID: "plain", Display: registry.Display{Name: "Plain"}, Category: "apikey",
		Media: registry.MediaConfigs{
			registry.MediaEmbedding: {
				BaseURL: "https://api.example.com/v1/embeddings", Format: "openai",
			},
		},
	}
}

// embeddingsCombo builds the combo a resolution can carry, so the combo refusal
// is reached with the identity resolution had established.
func embeddingsCombo(t *testing.T) domain.Combo {
	t.Helper()
	model, err := domain.NewComboModel("plain/embed-model", 1)
	if err != nil {
		t.Fatalf("NewComboModel() error = %v", err)
	}
	combo, err := domain.NewCombo("cmb_embed", "daily", domain.ComboFallback, 0, "", []domain.ComboModel{model}, time.Now().UTC())
	if err != nil {
		t.Fatalf("NewCombo() error = %v", err)
	}
	return combo
}

// TestEmbeddingsService_RecordsRefusalsBeforeTheCall pins that a call refused
// before any dial leaves exactly one request log row — no usage row — carrying
// the refusal's code and the identity resolution had reached, under the
// router's request id.
func TestEmbeddingsService_RecordsRefusalsBeforeTheCall(t *testing.T) {
	plain := embeddingsProvider()
	noBlock := registry.Provider{ID: "chatonly", Display: registry.Display{Name: "Chat Only"}, Category: "apikey"}

	cases := []struct {
		name         string
		resolver     ModelResolver
		router       MediaRouter
		wantCode     string
		wantProvider string
		wantModel    string
	}{
		{
			name:     "an unresolvable model",
			resolver: stubModelResolver{failure: dataplane.ErrNotFound("model nope/embed is not routable")},
			router:   &stubMediaRouter{}, wantCode: dataplane.CodeModelNotFound,
		},
		{
			name: "a combo",
			resolver: stubModelResolver{resolution: dataplane.Resolution{
				ModelID: "daily", Combo: embeddingsCombo(t),
			}},
			router: &stubMediaRouter{}, wantCode: dataplane.CodeValidation, wantModel: "daily",
		},
		{
			name: "a provider without an embeddings block",
			resolver: stubModelResolver{resolution: dataplane.Resolution{
				Provider: noBlock, ModelID: "chat-model",
			}},
			router: &stubMediaRouter{}, wantCode: dataplane.CodeProviderNotRoutable,
			wantProvider: "chatonly", wantModel: "chat-model",
		},
		{
			name: "no usable account",
			resolver: stubModelResolver{resolution: dataplane.Resolution{
				Provider: plain, ModelID: "embed-model",
			}},
			router:       &stubMediaRouter{failure: domain.NewNoProviderAvailableError("no account for provider plain")},
			wantCode:     dataplane.CodeNoProvider,
			wantProvider: "plain", wantModel: "embed-model",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, usage, logs := embeddingsRefusalFixture(tc.resolver, tc.router)

			if _, _, err := svc.Embed(context.Background(), schema.EmbeddingsRequest{
				Model: "plain/embed-model", Input: schema.EmbeddingInput{Texts: []string{"ping"}},
			}, "gky_refused"); err == nil {
				t.Fatalf("Embed() error = nil, want a refusal")
			}

			if len(usage.rows) != 0 {
				t.Fatalf("recorded %d usage rows, want none for a call that never dialed", len(usage.rows))
			}
			if len(logs.rows) != 1 {
				t.Fatalf("recorded %d log rows, want the refusal logged", len(logs.rows))
			}
			entry := logs.rows[0]
			if entry.RequestID != "req_embed_refused" || entry.GatewayKeyID != "gky_refused" {
				t.Fatalf("ids = %q/%q, want the request's own", entry.RequestID, entry.GatewayKeyID)
			}
			if entry.Status != domain.RequestLogError || entry.Error != tc.wantCode {
				t.Fatalf("log = %s/%q, want error/%s alone", entry.Status, entry.Error, tc.wantCode)
			}
			if entry.ProviderID != tc.wantProvider || entry.Model != tc.wantModel {
				t.Fatalf("identity = %q/%q, want %q/%q", entry.ProviderID, entry.Model, tc.wantProvider, tc.wantModel)
			}
			if entry.EndpointID != "" || entry.LatencyMS != 0 || entry.RequestBody != "" || entry.ResponseBody != "" {
				t.Fatalf("row = endpoint %q latency %d bodies %q/%q, want none for a call that never dialed",
					entry.EndpointID, entry.LatencyMS, entry.RequestBody, entry.ResponseBody)
			}
		})
	}
}

// TestEmbeddingsService_ServedCallStillWritesThePair is the benign control: a
// call that reaches its account writes the §7.12/§7.13 pair exactly as before
// the refusal row existed, and writes no refusal row on top of it.
func TestEmbeddingsService_ServedCallStillWritesThePair(t *testing.T) {
	svc, usage, logs := embeddingsRefusalFixture(
		stubModelResolver{resolution: dataplane.Resolution{Provider: embeddingsProvider(), ModelID: "embed-model"}},
		&stubMediaRouter{},
	)

	if _, _, err := svc.Embed(context.Background(), schema.EmbeddingsRequest{
		Model: "plain/embed-model", Input: schema.EmbeddingInput{Texts: []string{"ping"}},
	}, "gky_served"); err != nil {
		t.Fatalf("Embed() error = %v, want the served answer", err)
	}
	if len(usage.rows) != 1 || len(logs.rows) != 1 {
		t.Fatalf("recorded %d usage / %d log rows, want exactly one of each",
			len(usage.rows), len(logs.rows))
	}
	if logs.rows[0].Status != domain.RequestLogSuccess || logs.rows[0].Error != "" {
		t.Fatalf("log = %s/%q, want a clean success", logs.rows[0].Status, logs.rows[0].Error)
	}
}

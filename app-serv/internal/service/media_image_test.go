// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_image_test.go
// @for       The image and video generation payloads and normalization.
// @uses      internal/dataplane, internal/domain, internal/schema, context,
//
//	encoding/json, strings, testing.
//
// @reason    The normalization is the part a client depends on: whichever
//
//	provider answered, `data[0].url` must be where the asset is. The
//	cases pin that, plus the refusals §7.10 owes a client naming a
//	provider the gateway cannot serve.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// TestMediaCallService_GenerateImage pins the upstream payload and the
// normalized answer.
func TestMediaCallService_GenerateImage(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(
		`{"created":1700000000,"data":[{"url":"https://img.example.com/a.png","revised_prompt":"a cat"}]}`)}

	n, size := 2, "1024x1024"
	response, outcome, err := svc.GenerateImage(context.Background(), schema.ImageRequest{
		Model: "openai/gpt-image-1", Prompt: "a cat", N: &n, Size: size, Quality: "hd",
	}, "")
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if response.Created != 1700000000 || len(response.Data) != 1 ||
		response.Data[0].URL != "https://img.example.com/a.png" {
		t.Fatalf("response = %+v, want the normalized envelope", response)
	}
	// The outcome is the accounting identity: the upstream model, not the
	// client's `provider/model` string.
	if outcome.ProviderID != "openai" || outcome.Model != "gpt-image-1" {
		t.Fatalf("outcome = %+v, want the provider and the upstream model", outcome)
	}

	var body schema.ImageBody
	if err := json.Unmarshal(caller.requests[0].Body, &body); err != nil {
		t.Fatalf("decoding the upstream body: %v", err)
	}
	if body.Model != "gpt-image-1" || body.Prompt != "a cat" || body.N == nil || *body.N != 2 || body.Quality != "hd" {
		t.Fatalf("body = %+v, want the OpenAI image payload", body)
	}
}

// TestMediaCallService_GenerateImageRefusesANonJSONAnswer pins that a 200 that
// is not a generation answer is reported as a gateway failure rather than
// forwarded: a proxy's error page would otherwise reach the client as a result.
func TestMediaCallService_GenerateImageRefusesANonJSONAnswer(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte("<html>gateway</html>")}

	_, _, err := svc.GenerateImage(context.Background(), schema.ImageRequest{
		Model: "openai/gpt-image-1", Prompt: "a cat",
	}, "")
	if err == nil || !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("error = %v, want the unreadable-answer refusal", err)
	}
}

// TestMediaCallService_GenerateImageEmptyData pins that an answer with no
// assets normalizes to an empty list rather than a null one, so a client
// iterating `data` never sees a nil.
func TestMediaCallService_GenerateImageEmptyData(t *testing.T) {
	svc, caller, _ := mediaCallFixture(t)
	caller.answer = dataplane.MediaResponse{Status: 200, Body: []byte(`{"created":1}`)}

	response, _, err := svc.GenerateImage(context.Background(), schema.ImageRequest{
		Model: "openai/gpt-image-1", Prompt: "a cat",
	}, "")
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if response.Data == nil || len(response.Data) != 0 {
		t.Fatalf("data = %v, want an empty list", response.Data)
	}
	if !strings.Contains(mustJSON(t, response), `"data":[]`) {
		t.Fatalf("response = %s, want an empty data array on the wire", mustJSON(t, response))
	}
}

// TestMediaCallService_GenerateVideoRefusesWithoutADeclaration pins §7.10's
// video route: no registry provider declares the kind yet, and the refusal
// names the provider rather than looking like an upstream outage.
func TestMediaCallService_GenerateVideoRefusesWithoutADeclaration(t *testing.T) {
	svc, _, _ := mediaCallFixture(t)
	_, _, err := svc.GenerateVideo(context.Background(), schema.VideoRequest{
		Model: "openai/sora", Prompt: "a cat",
	}, "")
	if err == nil || !strings.Contains(err.Error(), "does not offer video") {
		t.Fatalf("error = %v, want the not-routable refusal", err)
	}
}

// mustJSON renders a value as JSON for an assertion message.
func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshalling for the message: %v", err)
	}
	return string(encoded)
}

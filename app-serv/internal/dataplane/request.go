// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/request.go
// @for       One client request's resolved form: the route it arrived on, the
//
//	model it named, and the parameters routing needs.
//
// @uses      internal/schema.
// @reason    SPEC-API-001 §7.15 serves two client wire formats over three routes,
//
//	so "what the client speaks" and "what the upstream speaks" are two
//	independent facts. Declaring the request once here is what keeps the
//	service from taking eight positional parameters and mixing them up.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-17
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// Route names the client-facing endpoint a request arrived on.
type Route string

const (
	// RouteChatCompletions is POST /api/v1/chat/completions (OpenAI wire).
	RouteChatCompletions Route = "chat_completions"
	// RouteMessages is POST /api/v1/messages (Anthropic wire).
	RouteMessages Route = "messages"
	// RouteResponses is POST /api/v1/responses (Responses API wire).
	RouteResponses Route = "responses"
	// RouteEmbeddings is POST /api/v1/embeddings (OpenAI wire).
	RouteEmbeddings Route = "embeddings"
)

// Request is one data plane call in the form routing needs.
type Request struct {
	// Route is the endpoint the client called, which decides the response shape.
	Route Route
	// ClientFormat is the wire format the client sent and therefore expects back.
	ClientFormat schema.DataPlaneFormat
	// Model is the model string the client named, before resolution.
	Model string
	// Stream reports whether the caller asked for SSE.
	Stream bool
	// IncludeUsage reports whether a streamed answer must carry a usage chunk.
	IncludeUsage bool
	// Chat is the decoded OpenAI body, set for the OpenAI routes.
	Chat *schema.ChatRequest
	// Messages is the decoded Anthropic body, set for the Anthropic route.
	Messages *schema.MessagesRequest
	// Responses is the decoded Responses body, set for the Responses route.
	Responses *schema.ResponsesRequest
	// Embeddings is the decoded embeddings body.
	Embeddings *schema.EmbeddingsRequest
	// Raw is the body as it arrived, which a same-format target is forwarded
	// with so no unmodelled field is lost.
	Raw []byte
}

// ProviderID reports the provider the request resolved to, or "" before
// resolution.
type ResolvedCall struct {
	// Resolution is the provider and model the request resolved to.
	Resolution Resolution
	// Selection is the endpoint and key the call will spend.
	Selection Selection
	// Body is the upstream-shaped request payload.
	Body []byte
	// Upstream is the wire format Body is written in.
	Upstream string
	// ResponseFormat is the wire format the client expects back, which is the
	// request's ClientFormat unless a combo member changed the provider.
	ResponseFormat schema.DataPlaneFormat
}

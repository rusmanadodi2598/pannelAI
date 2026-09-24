// Package dataplane routes a client request through the gateway.
//
// @file      internal/dataplane/engine_opencode_free_streams_test.go
// @for       The SSE bodies the free-tier stand-in answers with, one per wire.
// @uses      nothing outside the package.
// @reason    The bodies are shared by the fixture and the acceptance tests, and
//
//	keeping them out of both is what lets each file stay inside the
//	AGENTS.md section 1.1 budget. They are constants rather than a builder
//	because the framing is the point: a chat model answers chat frames and
//	a Responses model answers Responses events, and a test that could not
//	tell them apart would not prove the per-model wire rule.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

// chatFreeStreamBody is the SSE a chat-wire free model answers with.
func chatFreeStreamBody(model string) string {
	return `data: {"id":"free-1","object":"chat.completion.chunk","created":1,"model":"` + model + `",` +
		`"choices":[{"index":0,"delta":{"role":"assistant","content":"pong"},"finish_reason":null}]}` + "\n\n" +
		`data: {"id":"free-1","object":"chat.completion.chunk","created":1,"model":"` + model + `",` +
		`"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":2,"total_tokens":9}}` + "\n\n" +
		"data: [DONE]\n\n"
}

// responsesFreeStreamBody is the SSE a Responses-wire free model answers with,
// ending in the terminal event that carries the complete response.
func responsesFreeStreamBody(model string) string {
	return "event: response.created\n" +
		`data: {"type":"response.created","response":{"id":"resp_free","model":"` + model + `"}}` + "\n\n" +
		"event: response.output_text.delta\n" +
		`data: {"type":"response.output_text.delta","delta":"pong"}` + "\n\n" +
		"event: response.completed\n" +
		`data: {"type":"response.completed","response":{"id":"resp_free","object":"response","status":"completed",` +
		`"model":"` + model + `","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"pong"}]}],` +
		`"usage":{"input_tokens":7,"output_tokens":2,"total_tokens":9}}}` + "\n\n" +
		"data: [DONE]\n\n"
}

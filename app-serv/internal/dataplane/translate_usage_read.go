// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_usage_read.go
// @for       Reading an upstream usage object into the OpenAI accounting block,
//
//	in whichever format the upstream wrote it.
//
// @uses      internal/schema.
// @reason    Both the non-streamed reader and the two stream states fold usage the
//
//	same way, and a private copy per caller is how the two directions
//	start disagreeing. They live in one file so a new format adds one
//	reader rather than one per caller.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// claudeUsageFromObject reads an Anthropic usage object from a decoded chunk.
func claudeUsageFromObject(usage object) schema.MessagesUsage {
	return schema.MessagesUsage{
		InputTokens:              intField(usage, "input_tokens"),
		OutputTokens:             intField(usage, "output_tokens"),
		CacheReadInputTokens:     intField(usage, "cache_read_input_tokens"),
		CacheCreationInputTokens: intField(usage, "cache_creation_input_tokens"),
	}
}

// openAIUsageFromObject reads an OpenAI usage object from a decoded chunk. A
// null member decodes to a nil object, which is the upstream saying it has no
// numbers rather than reporting zeros, so it yields nil: publishing a zero
// usage would send a 0/0 chunk a client reads as measured (draft 021 F2).
func openAIUsageFromObject(usage object) *schema.Usage {
	if usage == nil {
		return nil
	}
	parsed := schema.Usage{
		PromptTokens:     intField(usage, "prompt_tokens"),
		CompletionTokens: intField(usage, "completion_tokens"),
		TotalTokens:      intField(usage, "total_tokens"),
	}
	if details, ok := objectField(usage, "prompt_tokens_details"); ok {
		cached := intField(details, "cached_tokens")
		creation := intField(details, "cache_creation_tokens")
		if cached > 0 || creation > 0 {
			parsed.PromptTokensDetails = &schema.PromptTokensDetails{
				CachedTokens:         cached,
				CacheCreationTokens:  creation,
				CacheReadInputTokens: cached,
			}
		}
	}
	if details, ok := objectField(usage, "completion_tokens_details"); ok {
		if reasoning := intField(details, "reasoning_tokens"); reasoning > 0 {
			parsed.CompletionTokensDetail = &schema.CompletionDetail{ReasoningTokens: reasoning}
		}
	}
	return &parsed
}

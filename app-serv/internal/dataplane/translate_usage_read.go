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
	// The details block is read as possibly absent rather than as a gate, because a
	// vendor may state its cache split only at the usage top level — a call that
	// reports `cache_creation_input_tokens` with no details block at all still owns
	// a cache write the panel has to show.
	details, _ := objectField(usage, "prompt_tokens_details")
	cached := openAICacheReadCount(usage, details)
	creation := openAICacheWriteCount(usage, details)
	if cached > 0 || creation > 0 {
		parsed.PromptTokensDetails = &schema.PromptTokensDetails{
			CachedTokens:         cached,
			CacheCreationTokens:  creation,
			CacheReadInputTokens: cached,
		}
	}
	if details, ok := objectField(usage, "completion_tokens_details"); ok {
		if reasoning := intField(details, "reasoning_tokens"); reasoning > 0 {
			parsed.CompletionTokensDetail = &schema.CompletionDetail{ReasoningTokens: reasoning}
		}
	}
	return &parsed
}

// openAICacheReadCount and openAICacheWriteCount read the two cache splits from
// every place the OpenAI wire is seen to state them, and answer the one quantity
// those places describe.
//
// The candidate spellings are alternative reports of a single number, never parts
// of it: codebuddy-intl states its read as `prompt_tokens_details.cached_tokens`
// while its top-level `cached_tokens` and `cache_read_input_tokens` stay 0
// (measured 2026-09-30), and the write sits at the top level as
// `cache_creation_input_tokens` or `prompt_cache_write_tokens` rather than in the
// details block. Summing the spellings would bill the same token twice, so the
// first one that carries a number wins, in the order OpenAI documents its own
// field first.
func openAICacheReadCount(usage, details object) int {
	return firstCounted(
		usageMember{details, "cached_tokens"},
		usageMember{details, "cache_read_input_tokens"},
		usageMember{usage, "cache_read_input_tokens"},
		usageMember{usage, "cached_tokens"},
	)
}

func openAICacheWriteCount(usage, details object) int {
	return firstCounted(
		usageMember{details, "cache_creation_tokens"},
		usageMember{usage, "cache_creation_input_tokens"},
		usageMember{usage, "prompt_cache_write_tokens"},
	)
}

// usageMember is one place a usage object may state a count: the object to read
// and the member name it uses there.
type usageMember struct {
	from object
	key  string
}

// firstCounted returns the first of the candidates that is above zero. A nil
// object reads as absent, so a vendor that sends no details block still answers
// from its top level.
func firstCounted(candidates ...usageMember) int {
	for _, candidate := range candidates {
		if count := intField(candidate.from, candidate.key); count > 0 {
			return count
		}
	}
	return 0
}

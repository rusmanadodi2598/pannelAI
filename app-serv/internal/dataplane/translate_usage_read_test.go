// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/translate_usage_read_test.go
// @for       Reading the cache split out of an upstream usage object, in every spelling the OpenAI wire is seen to use.
// @uses      testing, internal/schema.
// @reason    This reader is the single funnel for six call sites, and the panel's cache tiles are its only product. Measured live on 2026-09-30, codebuddy-intl states its read as `prompt_tokens_details.cached_tokens` while its top-level `cached_tokens` and `cache_read_input_tokens` stay 0, and would state a write at the top level as `cache_creation_input_tokens` or `prompt_cache_write_tokens` rather than in the details block. The spellings are alternative reports of ONE number, so both halves are pinned here: a write that still reads as zero when it only exists at the top level, and a count never taken twice.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-30
package dataplane

import (
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// cacheSplit reads one usage object the way the six call sites do and returns the
// two cache counts the panel renders.
func cacheSplit(t *testing.T, usage string) (read int, write int) {
	t.Helper()
	object, ok := decodeObject([]byte(usage))
	if !ok {
		t.Fatalf("usage object does not decode: %s", usage)
	}
	parsed := openAIUsageFromObject(object)
	if parsed == nil {
		t.Fatal("openAIUsageFromObject() = nil, want the usage read")
	}
	if parsed.PromptTokensDetails == nil {
		return 0, 0
	}
	return parsed.PromptTokensDetails.CachedTokens, parsed.PromptTokensDetails.CacheCreationTokens
}

func TestOpenAIUsage_ReadsACacheWriteStatedOnlyAtTheTopLevel(t *testing.T) {
	// The shape codebuddy-intl answers with, minus the zeros: a cache write lives at
	// the usage top level, and the details block carries no creation member at all.
	cases := []struct {
		name      string
		usage     string
		wantWrite int
	}{
		{
			name:      "cache_creation_input_tokens",
			usage:     `{"prompt_tokens":1501,"cache_creation_input_tokens":1501,"prompt_tokens_details":{"cached_tokens":0}}`,
			wantWrite: 1501,
		},
		{
			name:      "prompt_cache_write_tokens",
			usage:     `{"prompt_tokens":900,"prompt_cache_write_tokens":220}`,
			wantWrite: 220,
		},
		{
			name:      "cache_creation_tokens in the details block, the OpenAI spelling",
			usage:     `{"prompt_tokens":900,"prompt_tokens_details":{"cache_creation_tokens":120}}`,
			wantWrite: 120,
		},
		{
			name:      "no details block at all",
			usage:     `{"prompt_tokens":900,"cache_creation_input_tokens":75}`,
			wantWrite: 75,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, write := cacheSplit(t, tc.usage)
			if write != tc.wantWrite {
				t.Fatalf("cache write = %d, want %d", write, tc.wantWrite)
			}
		})
	}
}

func TestOpenAIUsage_CacheSpellingsAreOneNumberNeverTwo(t *testing.T) {
	// Every spelling agrees at 1280. A reader that summed them would report 3840
	// cached tokens against a 1501-token prompt, and price the difference.
	usage := `{"prompt_tokens":1501,"cached_tokens":1280,"cache_read_input_tokens":1280,` +
		`"prompt_tokens_details":{"cached_tokens":1280,"cache_read_input_tokens":1280}}`
	read, write := cacheSplit(t, usage)
	if read != 1280 {
		t.Fatalf("cache read = %d, want 1280: the spellings are alternatives, not parts", read)
	}
	if write != 0 {
		t.Fatalf("cache write = %d, want 0", write)
	}
}

func TestOpenAIUsage_PrefersTheDocumentedHomeOverATopLevelAlias(t *testing.T) {
	// Where two spellings disagree, OpenAI's own documented member wins, so a
	// vendor's leftover top-level counter cannot override the block it also filled.
	usage := `{"prompt_tokens":100,"cache_creation_input_tokens":40,` +
		`"prompt_tokens_details":{"cache_creation_tokens":10}}`
	_, write := cacheSplit(t, usage)
	if write != 10 {
		t.Fatalf("cache write = %d, want the details block's 10", write)
	}
}

func TestOpenAIUsage_ReadIsTakenFromTheTopLevelWhenDetailsSayNothing(t *testing.T) {
	usage := `{"prompt_tokens":1000,"cache_read_input_tokens":640,"prompt_tokens_details":{"cached_tokens":0}}`
	read, _ := cacheSplit(t, usage)
	if read != 640 {
		t.Fatalf("cache read = %d, want 640 from the top-level spelling", read)
	}
}

func TestOpenAIUsage_AVendorReportingNoCacheSplitsStaysZero(t *testing.T) {
	// The cold-call shape measured live: every cache member present and every one of
	// them zero. Nothing is invented, so the panel reads a real zero.
	usage := `{"prompt_tokens":1501,"cached_tokens":0,"cache_read_input_tokens":0,` +
		`"cache_creation_input_tokens":0,"prompt_cache_write_tokens":0,` +
		`"prompt_tokens_details":{"cached_tokens":0}}`
	read, write := cacheSplit(t, usage)
	if read != 0 || write != 0 {
		t.Fatalf("cache split = (%d, %d), want (0, 0)", read, write)
	}
}

func TestClaudeUsage_CacheCreationReachesTheDetailNotJustThePrompt(t *testing.T) {
	// The one upstream shape that genuinely meters a cache write. The prompt total
	// folding it in was already pinned; this is the member the usage row reads, and
	// without it the panel's cache-write tile has no source at all.
	parsed := ClaudeUsageToOpenAI(schema.MessagesUsage{
		InputTokens: 10, OutputTokens: 5, CacheCreationInputTokens: 90,
	})
	if parsed.PromptTokens != 100 {
		t.Fatalf("prompt = %d, want cache creation folded in", parsed.PromptTokens)
	}
	if parsed.PromptTokensDetails == nil {
		t.Fatal("PromptTokensDetails = nil, want the cache split carried")
	}
	if got := parsed.PromptTokensDetails.CacheCreationTokens; got != 90 {
		t.Fatalf("cache creation = %d, want 90", got)
	}
}

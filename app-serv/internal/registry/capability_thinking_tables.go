// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_thinking_tables.go
// @for       The reasoning tables the thinking resolver reads: the ordered
//
//	pattern walk, the per-provider overrides, and the exact-id
//	exceptions.
//
// @uses      (none; the tables are data read by capability_thinking.go).
// @reason    Each table is a transcription of one layer in the reference's
//
//	capabilities.js, in the reference's own order, so a reader diffing
//	this file against the reference sees one row per pattern. Keeping
//	them apart from the resolver holds capability_thinking.go inside the
//	AGENTS.md §1.1 budget.
//
//	The pattern table carries two kinds of row. A reasoning row answers
//	true and names the wire format; a stop row answers false and exists
//	because the reference's walk stops at it, so dropping it would let a
//	later reasoning row claim a model the reference denies (the gemini
//	image models are the case the corpus caught). Rows for families no
//	kept model reaches are still carried, because a custom node's model
//	id can match any pattern.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-26
package registry

// thinkingRules is the reference's PATTERN_CAPABILITIES reduced to its reasoning
// decision, in its order: the first match wins. The `ref #N` comment on each row
// is the pattern's index in the reference's array at revision 39e36d3d, which is
// what a maintainer re-checks when the reference moves.
var thinkingRules = []thinkingRule{
	{pattern: "*claude*opus-5*", format: "claude-adaptive", reasons: true},     // ref #0
	{pattern: "*claude*opus-4.6*", format: "claude-adaptive", reasons: true},   // ref #1
	{pattern: "*claude*opus-4.7*", format: "claude-adaptive", reasons: true},   // ref #2
	{pattern: "*claude*opus-4.8*", format: "claude-adaptive", reasons: true},   // ref #3
	{pattern: "*claude*sonnet-4.6*", format: "claude-adaptive", reasons: true}, // ref #4
	{pattern: "*claude*sonnet-4.7*", format: "claude-adaptive", reasons: true}, // ref #5
	{pattern: "*claude*haiku*", format: "claude-budget", reasons: true},        // ref #6
	{pattern: "*claude*opus*", format: "claude-budget", reasons: true},         // ref #7
	{pattern: "*claude*sonnet*", format: "claude-budget", reasons: true},       // ref #8
	{pattern: "*claude*fable*", format: "claude-budget", reasons: true},        // ref #9
	{pattern: "*claude*mythos*", format: "claude-budget", reasons: true},       // ref #10
	{pattern: "*claude*", format: "claude-budget", reasons: true},              // ref #12
	{pattern: "*gemini*image*"}, // ref #13, stop
	{pattern: "*gemini-3.8*", format: "gemini-level", cannotDisable: true, reasons: true},   // ref #14
	{pattern: "*gemini-3.7*", format: "gemini-level", cannotDisable: true, reasons: true},   // ref #15
	{pattern: "*gemini-3*pro*", format: "gemini-level", cannotDisable: true, reasons: true}, // ref #16
	{pattern: "*gemini-3*", format: "gemini-level", cannotDisable: true, reasons: true},     // ref #17
	{pattern: "*gemini-2.5*", format: "gemini-budget", reasons: true},                       // ref #18
	{pattern: "*gemini-2*"},                                                            // ref #19, stop
	{pattern: "*gemini*"},                                                              // ref #20, stop
	{pattern: "*gemma*"},                                                               // ref #21, stop
	{pattern: "*gpt-6*", format: "openai", reasons: true},                              // ref #23
	{pattern: "*gpt-5*codex*", format: "openai", reasons: true},                        // ref #25
	{pattern: "*gpt-5*", format: "openai", reasons: true},                              // ref #26
	{pattern: "*gpt-4o*"},                                                              // ref #27, stop
	{pattern: "*gpt-4.1*"},                                                             // ref #28, stop
	{pattern: "*gpt-4-turbo*"},                                                         // ref #29, stop
	{pattern: "*gpt-oss*", format: "openai", reasons: true},                            // ref #32
	{pattern: "*o1-mini*", format: "openai", reasons: true},                            // ref #33
	{pattern: "*o1*", format: "openai", reasons: true},                                 // ref #34
	{pattern: "*o3*", format: "openai", reasons: true},                                 // ref #35
	{pattern: "*o4*", format: "openai", reasons: true},                                 // ref #36
	{pattern: "*grok*image*"},                                                          // ref #37, stop
	{pattern: "*grok-code*", format: "openai", reasons: true},                          // ref #38
	{pattern: "*grok-4.6*", format: "openai", reasons: true},                           // ref #39
	{pattern: "*grok-4.5*", format: "openai", reasons: true},                           // ref #40
	{pattern: "*grok-4*", format: "openai", reasons: true},                             // ref #41
	{pattern: "*grok-3*", format: "openai", reasons: true},                             // ref #42
	{pattern: "*grok*", format: "openai", reasons: true},                               // ref #43
	{pattern: "*qwen*vl*", format: "qwen", reasons: true},                              // ref #44
	{pattern: "*qwen*omni*", format: "qwen", reasons: true},                            // ref #45
	{pattern: "*qwen*coder*", format: "qwen", reasons: true},                           // ref #46
	{pattern: "*qwen*max*", format: "qwen", reasons: true},                             // ref #47
	{pattern: "*qwen3.5*", format: "qwen", reasons: true},                              // ref #48
	{pattern: "*qwen3.6*", format: "qwen", reasons: true},                              // ref #49
	{pattern: "*qwen3.7*", format: "qwen", reasons: true},                              // ref #50
	{pattern: "*qwen*plus*", format: "qwen", reasons: true},                            // ref #51
	{pattern: "*qwen*235b*", format: "qwen", reasons: true},                            // ref #52
	{pattern: "*qwq*", format: "qwen", cannotDisable: true, reasons: true},             // ref #53
	{pattern: "*qwen*", format: "qwen", reasons: true},                                 // ref #54
	{pattern: "*kimi*k3*", format: "kimi", cannotDisable: true, reasons: true},         // ref #55
	{pattern: "*kimi*for-coding*", format: "kimi", cannotDisable: true, reasons: true}, // ref #56
	{pattern: "*kimi*k2.7*code*", format: "kimi", cannotDisable: true, reasons: true},  // ref #57
	{pattern: "*kimi*k2*", format: "kimi", reasons: true},                              // ref #58
	{pattern: "*kimi*", format: "kimi", reasons: true},                                 // ref #59
	{pattern: "*glm-5.3*", format: "zai", reasons: true},                               // ref #60
	{pattern: "*glm-5.2*", format: "zai", reasons: true},                               // ref #61
	{pattern: "*glm-5*", format: "zai", reasons: true},                                 // ref #62
	{pattern: "*glm-4.7*", format: "zai", reasons: true},                               // ref #63
	{pattern: "*glm-4*", format: "zai", reasons: true},                                 // ref #64
	{pattern: "*glm*", format: "zai", reasons: true},                                   // ref #65
	{pattern: "*deepseek-v4.*", format: "deepseek", reasons: true},                     // ref #66
	{pattern: "*deepseek-v4*", format: "deepseek", reasons: true},                      // ref #67
	{pattern: "*reasoner*", format: "deepseek", cannotDisable: true, reasons: true},    // ref #68
	{pattern: "*deepseek-r*", format: "deepseek", cannotDisable: true, reasons: true},  // ref #69
	{pattern: "*deepseek-chat*"},                                                       // ref #70, stop
	{pattern: "*deepseek*", format: "deepseek", reasons: true},                         // ref #71
	{pattern: "*minimax*image*"},                                                       // ref #72, stop
	{pattern: "*minimax-m3*", format: "minimax", reasons: true},                        // ref #73
	{pattern: "*minimax-m2.7*", format: "minimax", cannotDisable: true, reasons: true}, // ref #74
	{pattern: "*minimax-m2.5*", format: "minimax", cannotDisable: true, reasons: true}, // ref #75
	{pattern: "*minimax*", format: "minimax", cannotDisable: true, reasons: true},      // ref #76
	{pattern: "*mimo*v2.6*", format: "deepseek", cannotDisable: true, reasons: true},   // ref #77
	{pattern: "*mimo*v2.5*", format: "deepseek", cannotDisable: true, reasons: true},   // ref #78
	{pattern: "*mimo*omni*", format: "deepseek", cannotDisable: true, reasons: true},   // ref #79
	{pattern: "*mimo*", format: "deepseek", cannotDisable: true, reasons: true},        // ref #80
	{pattern: "*llama-4*"},                                                             // ref #81, stop
	{pattern: "*llama*"},                                                               // ref #82, stop
	{pattern: "*sonar*"},                                                               // ref #88, stop
	{pattern: "*pplx*"},                                                                // ref #89, stop
	{pattern: "*perplexity*"},                                                          // ref #90, stop
	{pattern: "*laguna-s-2.1*free*", format: "openai", reasons: true},                  // ref #91
	{pattern: "*laguna-s-2.1*", format: "openai", reasons: true},                       // ref #92
	{pattern: "*laguna*", format: "openai", reasons: true},                             // ref #93
	{pattern: "*muse*spark*", format: "openai", reasons: true},                         // ref #94
	{pattern: "*hunyuan*", format: "hunyuan", reasons: true},                           // ref #95
	{pattern: "hy3*", format: "hunyuan", reasons: true},                                // ref #96
	{pattern: "*step-*", format: "step", reasons: true},                                // ref #97
	{pattern: "*nemotron*", reasons: true},                                             // ref #98
	{pattern: "*ling-*", reasons: true},                                                // ref #99
}

// providerThinkingIDs is the reference's PROVIDER_CAPABILITIES layer reduced to
// its reasoning decision, keyed by (provider, model). It wins over the exact and
// pattern layers because the reference consults it first (capabilities.js:586-590).
//
// Only the kept providers' rows are carried: the reference's other provider
// overrides belong to codex, kiro, poolside, and ollama, which the owner's KEEP
// set (2026-09-26) removed, so no endpoint can select them.
var providerThinkingIDs = map[string]map[string]thinkingRule{
	"nvidia": {
		"minimaxai/minimax-m2.7":        {format: "openai", cannotDisable: true, reasons: true},
		"minimaxai/minimax-m3":          {format: "openai", cannotDisable: true, reasons: true},
		"z-ai/glm-5.2":                  {format: "openai", reasons: true},
		"deepseek-ai/deepseek-v4-pro":   {format: "openai", reasons: true},
		"deepseek-ai/deepseek-v4-flash": {format: "openai", reasons: true},
	},
	"opencode-go": {
		"glm-5.3-flash": {format: "openai", cannotDisable: true, reasons: true},
	},
	"codebuddy-cn": {
		"glm-5.2":             {format: "openai", reasons: true},
		"glm-5.1":             {format: "openai", cannotDisable: true, reasons: true},
		"glm-5.0":             {format: "openai", reasons: true},
		"glm-5v-turbo":        {format: "openai", cannotDisable: true, reasons: true},
		"glm-4.7":             {format: "openai", reasons: true},
		"minimax-m3":          {format: "openai", cannotDisable: true, reasons: true},
		"kimi-k2.7":           {format: "openai", cannotDisable: true, reasons: true},
		"kimi-k2.6":           {format: "openai", cannotDisable: true, reasons: true},
		"kimi-k2.5":           {format: "openai", cannotDisable: true, reasons: true},
		"hy3-preview":         {format: "openai", cannotDisable: true, reasons: true},
		"deepseek-v4-flash":   {format: "openai", cannotDisable: true, reasons: true},
		"deepseek-v3-2-volc":  {format: "openai", cannotDisable: true, reasons: true},
		"hy3":                 {format: "openai", cannotDisable: true, reasons: true},
		"hy4-preview":         {format: "openai", cannotDisable: true, reasons: true},
		"glm-5.3":             {format: "openai", reasons: true},
		"glm-5.3-flash":       {format: "openai", reasons: true},
		"kimi-k3-1":           {format: "openai", cannotDisable: true, reasons: true},
		"deepseek-v4-pro":     {format: "openai", reasons: true},
		"deepseek-v4.1-flash": {format: "openai", reasons: true},
	},
}

// thinkingExactIDs is the reference's MODEL_CAPABILITIES layer reduced to its
// reasoning decision: the ids whose answer the pattern table alone gets wrong.
// It is consulted after the provider layer and before the patterns, and only the
// ids a kept model carries are listed.
var thinkingExactIDs = map[string]thinkingRule{
	"claude-opus-5":                   {format: "claude-adaptive", reasons: true},
	"claude-fable-5-1":                {format: "claude-adaptive", cannotDisable: true, reasons: true},
	"claude-sonnet-5":                 {format: "claude-adaptive", reasons: true},
	"claude-sonnet-4-6":               {format: "claude-adaptive", reasons: true},
	"claude-sonnet-4.6":               {format: "claude-adaptive", reasons: true},
	"claude-opus-4.6":                 {format: "claude-adaptive", reasons: true},
	"claude-opus-4.7":                 {format: "claude-adaptive", reasons: true},
	"claude-opus-4-6":                 {format: "claude-adaptive", reasons: true},
	"claude-opus-4-7":                 {format: "claude-adaptive", reasons: true},
	"claude-opus-4-8":                 {format: "claude-adaptive", reasons: true},
	"muse-spark-1.2-contributor-free": {format: "openai", reasons: true},
	"muse-spark-1.3-contributor-free": {format: "openai", reasons: true},
	"glm-5.2":                         {format: "zai", cannotDisable: true, reasons: true},
	"kimi-k2.7-code":                  {format: "kimi", cannotDisable: true, reasons: true},
	"kimi-k2.7-code-highspeed":        {format: "kimi", cannotDisable: true, reasons: true},
	"kimi-k3":                         {format: "kimi", cannotDisable: true, reasons: true},
	"k3":                              {format: "kimi", cannotDisable: true, reasons: true},
	"kimi-for-coding":                 {format: "kimi", cannotDisable: true, reasons: true},
	"kimi-for-coding-highspeed":       {format: "kimi", cannotDisable: true, reasons: true},
	"deepseek-v4.1-flash":             {format: "deepseek", reasons: true},
	"deepseek-v4-flash-vision-exp":    {format: "deepseek", reasons: true},
	"deepseek-flash":                  {format: "deepseek", reasons: true},
	"glm-5.3-flash":                   {format: "zai", reasons: true},
	"glm-4.6v":                        {format: "zai", reasons: true},
}

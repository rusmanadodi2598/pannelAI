// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/capability_patterns.go
// @for       The ordered vision pattern table, ported from the reference's own
// //
//
//	PATTERN_CAPABILITIES.
//
// @uses      (none; the table is data read by capability_resolve.go).
// @reason    The table is one row per reference pattern and nothing else, so it
//
//	grows whenever the reference adds a family. Keeping it in its own
//	file is what holds capability.go inside the AGENTS.md §1.1 budget,
//	and it makes the port's shape obvious: a reader comparing this list
//	to open-sse/providers/capabilities.js sees one row per pattern.
//
//	The rows are in the reference's own order because the first match
//	wins: reordering them silently changes answers for every id that
//	matches two patterns (the codex and image variants are the ones that
//	rely on it).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-24
package registry

// visionRules is the reference's PATTERN_CAPABILITIES reduced to its vision
// decision, in its order: the first match wins, and a match returns even when
// the reference's entry carried no vision flag — that flag merges to the false
// floor, which is why "*gpt-5*image*" answers false rather than falling through
// to "*gpt-5*".
var visionRules = []visionRule{
	{pattern: "*claude*opus-5*", vision: true},
	{pattern: "*claude*opus-4.6*", vision: true},
	{pattern: "*claude*opus-4.7*", vision: true},
	{pattern: "*claude*opus-4.8*", vision: true},
	{pattern: "*claude*sonnet-4.6*", vision: true},
	{pattern: "*claude*sonnet-4.7*", vision: true},
	{pattern: "*claude*haiku*", vision: true},
	{pattern: "*claude*opus*", vision: true},
	{pattern: "*claude*sonnet*", vision: true},
	{pattern: "*claude*fable*", vision: true},
	{pattern: "*claude*mythos*", vision: true},
	{pattern: "*claude-3*", vision: true},
	{pattern: "*claude*", vision: true},
	{pattern: "*gemini*image*", vision: true},
	{pattern: "*gemini-3.8*", vision: true},
	{pattern: "*gemini-3.7*", vision: true},
	{pattern: "*gemini-3*pro*", vision: true},
	{pattern: "*gemini-3*", vision: true},
	{pattern: "*gemini-2.5*", vision: true},
	{pattern: "*gemini-2*", vision: true},
	{pattern: "*gemini*", vision: true},
	{pattern: "*gemma*", vision: true},
	{pattern: "*nanobanana*", vision: true},
	{pattern: "*gpt-6*", vision: true},

	{pattern: "*gpt-5*image*", vision: false},
	{pattern: "*gpt-5*codex*", vision: false},

	{pattern: "*gpt-5*", vision: true},
	{pattern: "*gpt-4o*", vision: true},
	{pattern: "*gpt-4.1*", vision: true},
	{pattern: "*gpt-4-turbo*", vision: true},

	{pattern: "*gpt-4*", vision: false},
	{pattern: "*gpt-3.5*", vision: false},
	{pattern: "*gpt-oss*", vision: false},
	{pattern: "*o1-mini*", vision: false},

	{pattern: "*o1*", vision: true},
	{pattern: "*o3*", vision: true},
	{pattern: "*o4*", vision: true},

	{pattern: "*grok*image*", vision: false},
	{pattern: "*grok-code*", vision: false},

	{pattern: "*grok-4.6*", vision: true},
	{pattern: "*grok-4.5*", vision: true},
	{pattern: "*grok-4*", vision: true},
	{pattern: "*grok-3*", vision: true},
	{pattern: "*grok*", vision: true},
	{pattern: "*qwen*vl*", vision: true},
	{pattern: "*qwen*omni*", vision: true},

	{pattern: "*qwen*coder*", vision: false},

	{pattern: "*qwen*max*", vision: true},
	{pattern: "*qwen3.5*", vision: true},
	{pattern: "*qwen3.6*", vision: true},
	{pattern: "*qwen3.7*", vision: true},
	{pattern: "*qwen*plus*", vision: true},

	{pattern: "*qwen*235b*", vision: false},
	{pattern: "*qwq*", vision: false},
	{pattern: "*qwen*", vision: false},

	{pattern: "*kimi*k3*", vision: true},
	{pattern: "*kimi*for-coding*", vision: true},
	{pattern: "*kimi*k2.7*code*", vision: true},
	{pattern: "*kimi*k2*", vision: true},

	{pattern: "*kimi*", vision: false},
	{pattern: "*glm-5.3*", vision: false},
	{pattern: "*glm-5.2*", vision: false},
	{pattern: "*glm-5*", vision: false},
	{pattern: "*glm-4.7*", vision: false},
	{pattern: "*glm-4*", vision: false},
	{pattern: "*glm*", vision: false},

	{pattern: "*deepseek-v4.*", vision: true},

	{pattern: "*deepseek-v4*", vision: false},
	{pattern: "*reasoner*", vision: false},
	{pattern: "*deepseek-r*", vision: false},
	{pattern: "*deepseek-chat*", vision: false},
	{pattern: "*deepseek*", vision: false},
	{pattern: "*minimax*image*", vision: false},

	{pattern: "*minimax-m3*", vision: true},
	{pattern: "*minimax-m2.7*", vision: true},
	{pattern: "*minimax-m2.5*", vision: true},

	{pattern: "*minimax*", vision: false},

	{pattern: "*mimo*v2.6*", vision: true},
	{pattern: "*mimo*v2.5*", vision: true},
	{pattern: "*mimo*omni*", vision: true},
	{pattern: "*mimo*", vision: true},
	{pattern: "*llama-4*", vision: true},

	{pattern: "*llama*", vision: false},
	{pattern: "*codestral*", vision: false},

	{pattern: "*mistral-large*", vision: true},

	{pattern: "*mistral*", vision: false},

	{pattern: "*command-a-vision*", vision: true},

	{pattern: "*command*", vision: false},
	{pattern: "*sonar*", vision: false},
	{pattern: "*pplx*", vision: false},
	{pattern: "*perplexity*", vision: false},
	{pattern: "*laguna-s-2.1*free*", vision: false},
	{pattern: "*laguna-s-2.1*", vision: false},
	{pattern: "*laguna*", vision: false},

	{pattern: "*muse*spark*", vision: true},

	{pattern: "*hunyuan*", vision: false},
	{pattern: "hy3*", vision: false},
	{pattern: "*step-*", vision: false},
	{pattern: "*nemotron*", vision: false},
	{pattern: "*ling-*", vision: false},
}

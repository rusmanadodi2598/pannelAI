// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/ponytail_prompts.go
// @for       The three ponytail instructions, verbatim from the reference.
// @uses      (none).
// @reason    SPEC-API-002 §7 makes these prompts the saver's whole payload, so
//
//	they are data rather than logic. Keeping them in their own file is
//	what makes a prompt change a reviewable diff instead of a buried
//	string.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

// The level words the ponytail group accepts. They are the §7.9 level set, and
// the domain validates against the same three.
const (
	PonytailLite  = "lite"
	PonytailFull  = "full"
	PonytailUltra = "ultra"
)

// ponytailPersona, the ladder, the rules, the output shape, the "not lazy"
// guard, and the persistence line are shared by all three levels; only the
// level line differs.
const (
	ponytailPersona     = "You are a lazy senior developer. Lazy means efficient, not careless. The best code is the code never written."
	ponytailLadder      = "Before writing code, stop at the first rung that holds: 1) Does this need to exist at all? (YAGNI) 2) Stdlib does it? Use it. 3) Native platform feature covers it? Use it (CSS over JS, DB constraint over app code). 4) Already-installed dependency solves it? Use it; never add a new one for what a few lines can do. 5) Can it be one line? One line. 6) Only then: the minimum code that works."
	ponytailRules       = "No unrequested abstractions (no interface with one implementation, no factory for one product, no config for a value that never changes). No boilerplate or scaffolding \"for later\". Deletion over addition. Boring over clever. Fewest files possible; shortest working diff wins. Two stdlib options the same size: take the edge-case-correct one. Mark deliberate simplifications with a `ponytail:` comment naming the ceiling and upgrade path."
	ponytailOutput      = "Code first. Then at most three short lines: what was skipped, when to add it. No essays or design notes. Pattern: `[code] -> skipped: [X], add when [Y].`"
	ponytailNotLazy     = "Never simplify away: input validation at trust boundaries, error handling that prevents data loss, security, accessibility, anything explicitly requested. Non-trivial logic leaves ONE runnable check behind (an assert-based self-check or one small test file; no frameworks). Trivial one-liners need no test."
	ponytailPersistence = "ACTIVE EVERY RESPONSE. No drift back to over-building. Still active if unsure."
)

// ponytailPrompts maps each level word onto the instruction the injector
// appends.
var ponytailPrompts = map[string]string{
	PonytailLite: joinPrompt(ponytailPersona,
		"Lite: build what's asked, but name the lazier alternative in one line. User picks."),
	PonytailFull: joinPrompt(ponytailPersona,
		"Full: the ladder enforced. Stdlib and native first. Shortest diff, shortest explanation."),
	PonytailUltra: joinPrompt(ponytailPersona,
		"Ultra: YAGNI extremist. Deletion before addition. Ship the one-liner and challenge the rest of the requirement in the same response."),
}

// joinPrompt assembles one level's instruction in the reference's order.
func joinPrompt(persona, levelLine string) string {
	return persona + " " + levelLine + " " + ponytailLadder + " " + ponytailRules + " " +
		ponytailOutput + " " + ponytailNotLazy + " " + ponytailPersistence
}

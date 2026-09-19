// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/ponytail_prompts_test.go
// @for       The prompt table's two standing claims: it is keyed by the domain's
//
//	level vocabulary, and it stays ASCII.
//
// @uses      internal/domain, strings, testing, unicode.
// @reason    SPEC-API-002 §7 ports the reference's prompts verbatim, and the
//
//	panel accepts the same three level words. A test is what keeps the
//	two from drifting silently: the schema tags, the domain validator,
//	and this table are three separate copies of one vocabulary.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"strings"
	"testing"
	"unicode"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// TestPonytailPrompts_MatchTheDomainVocabulary pins the level words the engine
// implements against the ones the settings surface accepts.
func TestPonytailPrompts_MatchTheDomainVocabulary(t *testing.T) {
	if len(ponytailPrompts) != len(domain.TokenSaverLevels) {
		t.Fatalf("the prompt table carries %d levels, want the %d canonical words",
			len(ponytailPrompts), len(domain.TokenSaverLevels))
	}
	for _, level := range domain.TokenSaverLevels {
		prompt, ok := ponytailPrompts[level]
		if !ok {
			t.Fatalf("the domain accepts %q but the table has no prompt for it", level)
		}
		if strings.TrimSpace(prompt) == "" {
			t.Fatalf("the %q prompt is empty", level)
		}
		if !domain.ValidTokenSaverLevel(level) {
			t.Fatalf("the table carries %q but the domain rejects it", level)
		}
	}
	if PonytailLite != "lite" || PonytailFull != "full" || PonytailUltra != "ultra" {
		t.Fatalf("the level constants drifted: %q, %q, %q", PonytailLite, PonytailFull, PonytailUltra)
	}
}

// TestPonytailPrompts_AreDistinctPerLevel guards against a copy-paste that
// leaves two levels sharing one instruction.
func TestPonytailPrompts_AreDistinctPerLevel(t *testing.T) {
	seen := map[string]string{}
	for level, prompt := range ponytailPrompts {
		if other, ok := seen[prompt]; ok {
			t.Fatalf("levels %q and %q carry the same instruction", level, other)
		}
		seen[prompt] = level
	}
}

// TestPonytailPrompts_AreAscii keeps the port's one deliberate text deviation in
// place: the reference's output line uses a Unicode arrow, and every prompt here
// stays ASCII. The rule is project-wide (antislop R-02), so a future edit that
// pastes the arrow back fails here rather than in a diff review.
func TestPonytailPrompts_AreAscii(t *testing.T) {
	for level, prompt := range ponytailPrompts {
		for _, r := range prompt {
			if r > unicode.MaxASCII {
				t.Fatalf("the %q instruction carries %q; the prompts stay ASCII", level, r)
			}
		}
	}
	if !strings.Contains(ponytailPrompts[PonytailFull], "-> skipped:") {
		t.Fatalf("the output line lost its ASCII arrow: %s", ponytailPrompts[PonytailFull])
	}
}

#!/usr/bin/env bash
# Gate: the antislop comment rules, checked by machine instead of by reading.
#
# Why this gate exists. R-02 is the only project rule with no script behind it,
# and the evidence of that is two audits of the same cleanup: the em dash was
# removed from 725 comment lines in one pass, survived at all only because
# nothing re-checked it, and a sweep that then swapped the character for a comma
# left `colliding , so` in six places, which a reviewer caught by hand. A rule
# with a gate stays clean. A rule without one drifts until someone reads it.
#
# Scope, and why it is not "everything":
#
#   - R-02 and the citation rule run on CHANGED files, the same incremental
#     contract AGENTS.md §1.1 and §1.2 use. Tree-wide they would fail today on
#     821 markdown lines that audit 005 left open deliberately (dated planning
#     records and the contract of record), which would stop every unrelated pull
#     request and train people to ignore the gate. The backlog is counted and
#     printed instead, so it stays visible and can only shrink.
#   - Citations are a WARNING, not a failure. The tree carries 142 pre-existing
#     ones, so failing would hold every commit hostage to lines its author did
#     not write. Audit 005 lists the sweep as its own task.
#   - `docs/**` and `AGENTS.md` are excluded from R-02 by name, and the exclusion
#     is printed on every run that touches them. They are audit 005's open
#     decision: dated planning records, the contract of record, and a governance
#     file. Without this the file's own 50 legacy dashes would make AGENTS.md
#     impossible to amend, which is how a gate gets disabled rather than obeyed.
#   - The structural checks run tree-wide, because the tree is clean for them
#     now and a diff-only check would let a pattern return in an untouched file.
#   - Citations ignore the region above a Go file's `package` line. AGENTS.md
#     §1.2 mandates a @reason field there and names it where a file's rationale
#     lives, so a draft reference inside it is the convention working as
#     designed. That boundary is audit 004 finding 5.
#
# What these patterns deliberately do not match, because each one produced a
# false positive while this gate was being written:
#
#   - string literals and for-loop conditions. `OutboundNoProxy: " other.test ,
#     api.example.com "` and `for i := 0; ; i++` are fixtures and syntax, not a
#     substitution artifact, so the artifact pattern only reads comment lines.
#   - markdown table separator rows. A `| --- |` rule is how a table is built,
#     so the separator pattern never applies to markdown.
#   - generated and vendored output, which nobody hand-writes.
#
# A line may opt out with a trailing `antislop:ignore` marker, for text quoting
# a violation rather than committing one. Every use is printed, so an exemption
# stays a decision a reviewer sees instead of a hole that widens.
#
# Exit codes: 0 clean, 1 at least one violation, 2 a required tool is missing.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"

root="$(repo_root)"

readonly SELF="scrypts/gates/antislop.sh"

# The characters R-02 forbids outright: em dash and en dash.
readonly DASH_RE='—|–'
# A space, a comma, a space inside a comment line is the signature of replacing
# a dash with a comma and leaving both spaces behind. Ordinary prose never
# writes `word , word`, which is what makes the pattern safe to apply broadly.
# Code, strings and loop headers are excluded by requiring a comment first.
readonly ARTIFACT_RE='^[[:space:]]*(//|/#|\*|/\*|#|--)[^"]*[A-Za-z0-9] , [A-Za-z]'
# A comment line whose whole body is a run of repeated punctuation around a
# label. Markdown is not scanned: table rules are structure, not decoration.
readonly SEPARATOR_RE='^[[:space:]]*(#|//|/\*|\*)[[:space:]]*[-=~^#*_]{4,}[[:space:]]*[A-Za-z]{2,}|^[[:space:]]*(#|//)[[:space:]]*[-=~#*_]{8,}[[:space:]]*$'
# An ALL CAPS banner label.
readonly BANNER_RE='^[[:space:]]*(#|//|/\*|\*)[[:space:]]+[A-Z][A-Z0-9 /_&-]{3,}[[:space:]]*$'
# A comment that only marks where a block ended.
readonly ENDMARK_RE='\}[[:space:]]*//[[:space:]]*end|^[[:space:]]*//[[:space:]]*end of [a-z]'
# Scratch-work citations: planning drafts and audit passes, which rot when they
# close. A SPEC-API or AGENTS.md section reference is NOT in this pattern: that
# is the live contract, and pointing at it is an API-contract reference.
readonly CITATION_RE='[dD]raft [0-9]+ +[FR§]|[dD]raft [0-9]+$|audit anti-slop [0-9]+|register G[0-9]+'

# code_globs are the file types the tree-wide structural checks read.
readonly CODE_GLOBS=(
	'*.go' '*.ts' '*.tsx' '*.js' '*.mjs' '*.svelte' '*.sh' '*.bash'
	'*.sql' '*.yaml' '*.yml'
)

# grep_files runs one pattern over the given path globs.
grep_files() {
	local re="$1"
	shift
	git -C "$root" grep -nIE "$re" -- "$@" 2>/dev/null | grep -v 'antislop:ignore' || true
}

# tree_scan runs a pattern over the code globs, minus vendored and generated
# output and this gate's own source, which contains every pattern by necessity.
tree_scan() {
	local re="$1"
	grep_files "$re" "${CODE_GLOBS[@]}" \
		':(exclude)**/node_modules/**' ':(exclude)**/dist/**' ':(exclude)**/build/**' \
		':(exclude)**/*_gen.go' ':(exclude)*.pb.go' ":(exclude)$SELF"
}

# text_paths lists tracked files the R-02 rule may apply to.
text_paths() {
	git -C "$root" ls-files -- \
		'*.go' '*.ts' '*.tsx' '*.js' '*.mjs' '*.svelte' '*.css' \
		'*.sh' '*.bash' '*.yml' '*.yaml' '*.json' '*.sql' '*.md' \
		':(exclude)**/node_modules/**' ':(exclude)**/dist/**' \
		':(exclude)**/build/**' ':(exclude)**/*_gen.go' ':(exclude)*.pb.go' \
		":(exclude)$SELF"
}

# changed_text_paths narrows that to files this change touches.
changed_text_paths() {
	comm -12 <(text_paths | sort -u) <(changed_files | sort -u)
}

# go_body prints the lines of a Go file below its `package` declaration, which
# is where the §1.2 mandated header ends.
go_body() {
	local file="$1" pkg
	pkg="$(grep -n '^package ' "$root/$file" | head -1 | cut -d: -f1 || true)"
	awk -v start="${pkg:-0}" 'NR > start' "$root/$file"
}

# report prints each hit and returns 1 when there were any.
report() {
	local label="$1" hits="$2"
	if [ -z "$hits" ]; then
		gate_pass "$label"
		return 0
	fi
	local n
	n="$(printf '%s\n' "$hits" | grep -c .)"
	printf '%s\n' "$hits" | head -n 8 | while IFS= read -r line; do
		gate_fail "$label: $(printf '%s' "$line" | cut -c1-150)"
	done
	[ "$n" -gt 8 ] && gate_fail "$label: ...and $((n - 8)) more"
	return 1
}

failed=0
scope="$(changed_text_paths)"

# ---- incremental: rules that cannot be enforced tree-wide yet ----

gate_start "antislop R-02 and citations (changed files)"
if [ -z "$scope" ]; then
	gate_skip "no changed text files; set GATES_BASE_REF to check a diff range"
else
	dash_hits=0
	cite_hits=0
	open_scope=0
	while IFS= read -r f; do
		[ -n "$f" ] || continue
		# Markdown under docs/ and the governance file are audit 005's open
		# decision: dated planning records, the contract of record, and a file
		# whose own text reserves its edits for review. Enforcing R-02 there
		# would make every one of them uneditable to fix one clause, which is
		# the opposite of what a gate is for. They are reported, not blocked.
		case "$f" in
		docs/* | AGENTS.md)
			open_scope=1
			continue
			;;
		esac
		h="$(grep_files "$DASH_RE" "$f")"
		if [ -n "$h" ]; then
			printf '%s\n' "$h" | head -3 | while IFS= read -r l; do gate_fail "R-02 dash: $l"; done
			dash_hits=$((dash_hits + 1))
		fi
		if [ "${f##*.}" = "go" ]; then
			c="$(go_body "$f" | grep -E "$CITATION_RE" || true)"
		else
			c="$(grep_files "$CITATION_RE" "$f")"
		fi
		if [ -n "$c" ]; then
			# A warning, not a failure: the tree carries 142 pre-existing
			# citations, and failing here would hold every commit hostage to
			# lines its author did not write. Audit 005 lists the sweep.
			gate_start "scratch citation to review in $f: $(printf '%s\n' "$c" | head -1 | cut -c1-110)"
			cite_hits=$((cite_hits + 1))
		fi
	done <<<"$scope"
	[ "$open_scope" = "1" ] &&
		gate_skip "docs/** and AGENTS.md carry R-02 debt left open by audit 005; excluded from this check on purpose"
	if [ "$dash_hits" -eq 0 ]; then
		gate_pass "antislop R-02 (changed files)"
	else
		failed=1
	fi
	[ "$cite_hits" = "0" ] && gate_pass "scratch citations (changed files)" ||
		gate_skip "$cite_hits changed file(s) still cite a closed draft; see anti-slop/audit-005-*.md"
fi

# ---- tree-wide: rules the tree already satisfies ----

report "substitution artifact in a comment" "$(tree_scan "$ARTIFACT_RE")" || failed=1
report "decorative separator" "$(tree_scan "$SEPARATOR_RE")" || failed=1
report "ALL CAPS banner label" "$(tree_scan "$BANNER_RE")" || failed=1
report "end marker" "$(tree_scan "$ENDMARK_RE")" || failed=1
report "decorative emoji" \
	"$(grep_files '[🚀✅🔒⚡✨📌🧪]' "${CODE_GLOBS[@]}" ':(exclude)**/node_modules/**' ":(exclude)$SELF")" ||
	failed=1

# AGENTS.md §1.4 as amended on 2026-10-05: every suppression carries a reason
# naming the constraint it protects. The clause used to demand a ticket ID too,
# and this repository has no ticket tracker, so the rule was unsatisfiable and
# therefore unenforced; the reason is the part a machine can actually check.
unexplained="$(grep_files '//nolint:' '*.go' ':(exclude)**/node_modules/**' ":(exclude)$SELF" |
	grep -v '// reason:' || true)"
report "suppression without a reason (AGENTS.md §1.4)" "$unexplained" || failed=1

# ---- doc block length, on changed Go files only ----

gate_start "doc block length (changed Go files)"
warned=0
docfailed=0
while IFS= read -r f; do
	[ -n "$f" ] || continue
	[ "${f##*.}" = "go" ] || continue
	[ -f "$root/$f" ] || continue
	long="$(awk '
		/^[[:space:]]*\/\// { c++; next }
		/^(func|type) /    { if (c > 16) printf "FAIL %d\n", c; else if (c > 10) printf "WARN %d\n", c; c = 0; next }
		/^[[:space:]]*$/   { next }
		{ c = 0 }
	' "$root/$f")"
	while IFS= read -r l; do
		case "$l" in
		FAIL*)
			gate_fail "$f has a ${l#FAIL }-line doc block"
			docfailed=1
			failed=1
			;;
		WARN*)
			gate_start "$f has a ${l#WARN }-line doc block (review: one line per fact)"
			warned=1
			;;
		esac
	done <<<"$long"
done <<<"$scope"
if [ "$docfailed" = "1" ]; then
	:
elif [ "$warned" = "1" ]; then
	gate_skip "doc-block warnings require review"
elif [ -n "$scope" ]; then
	gate_pass "doc block length (changed Go files)"
fi

# ---- backlog visibility for audit 005 ----

md_lines="$(git -C "$root" grep -cE "$DASH_RE" -- '*.md' 2>/dev/null | awk -F: '{s+=$2} END{print s+0}')"
md_files="$(git -C "$root" grep -lE "$DASH_RE" -- '*.md' 2>/dev/null | grep -vc 'anti-slop' || true)"
gate_start "markdown R-02 backlog (audit 005 open scope)"
if [ "${md_lines:-0}" -eq 0 ]; then
	gate_pass "no em or en dash left in markdown"
else
	gate_skip "$md_lines lines in $md_files markdown files are not enforced yet; see anti-slop/audit-005-*.md"
fi

if [ "$failed" -ne 0 ]; then
	gate_fail "antislop gate found violations above"
	exit 1
fi
gate_pass "antislop gate"

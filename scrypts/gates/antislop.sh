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
# Scope, and why it is not uniform:
#
#   - R-02 runs TREE-WIDE and fails on any use. It used to be a changed-files
#     check with `docs/**` and `AGENTS.md` excluded, because the tree carried 821
#     legacy lines and enforcing it then would have blocked every unrelated pull
#     request. That debt is swept, so the exclusion went with it instead of
#     staying out of habit. A rule enforced only on the diff is a rule that
#     regrows in the files nobody touched.
#   - Citations are a WARNING, not a failure. 146 of them are legitimate
#     references inside the §1.2 @reason headers, and the body-comment residue is
#     zero, so failing would hold a commit hostage to lines its author did not
#     write.
#   - Citations read only comment text, and only below a Go file's `package`
#     line. Above it, AGENTS.md §1.2 mandates a @reason field and names it where
#     a file's rationale lives, so a draft reference there is the convention
#     working as designed (audit 004 finding 5). Below it, a `draft 017 §4.6`
#     written inside a t.Fatalf message is a test naming the case it asserts, so
#     the string part of a line is dropped before matching: reading the whole
#     region reported five such strings as comment debt that was never there.
#   - The §1.2 header shape fails tree-wide: a field value continued onto a
#     second line, and a prose paragraph between the fields. The header region is
#     where the longest prose in the tree sat, and the doc-block length check
#     below never read it: that awk reports at `^(func|type)`, and a header ends
#     at `package`.
#   - The structural checks run tree-wide, because the tree is clean for them
#     now and a diff-only check would let a pattern return in an untouched file.
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
# is the live contract, and pointing at it is an API-contract reference. The
# `docs/DRAFT/` path form is in it, because a comment that spells the file out
# still points at a document that closes, and stating the fact costs nothing.
readonly CITATION_RE='[dD]raft [0-9]+ +[FR§]|[dD]raft [0-9]+$|audit anti-slop [0-9]+|register G[0-9]+|docs/DRAFT/'
# A panel suppression is an eslint-disable *directive*, not the phrase in prose.
# Requiring the comment to open with the directive, and the rule to carry its
# plugin slash, keeps eslint.config.js's own sentence about leftover directives
# from being read as a violation. `eslint-enable` re-enables a rule rather than
# suppressing one, so it is deliberately not in this pattern and needs no reason.
readonly PANEL_SUPPRESS_RE='^[[:space:]]*(//|\*|/\*|<!--)[[:space:]]*eslint-disable(-next-line|-block)?[[:space:]]+[^[:space:]]*/'
# eslint's own reason separator with something after it. The check used to exempt
# a directive for containing ` -- ` anywhere, which a directive ending at the
# separator also satisfies. Both sides of the separator must carry space, or the
# closing `-->` of a Svelte comment reads as a reason.
readonly PANEL_SUPPRESS_REASON_RE='[[:space:]]--+[[:space:]]+[^-[:space:]]'
# A directive that names no rule disables every rule. Two shapes carry that: the
# keyword ending the line, and the keyword going straight to the separator. Both
# are matched so that eslint.config.js's prose line, which names something after
# the keyword, still falls outside them.
readonly PANEL_SUPPRESS_BARE_RE='^[[:space:]]*(//|\*|/\*|<!--)[[:space:]]*eslint-disable(-next-line|-block)?([[:space:]]*$|[[:space:]]+--([[:space:]]|$))'
# The eight AGENTS.md §1.2 header fields.
readonly FIELD_TAG_RE='^// (@(file|for|uses|reason|author|layer|stability|since))[[:space:]]+'
# A field value continued on an indented line: what gofmt rewrites a hand-wrapped
# value into, and the reason the shape is worth a machine check rather than a
# review note. grep for `@for` stops in mid-sentence.
readonly FIELD_CONT_RE='^//\t'
# A field value continued on a plain comment line, either directly under the tag
# or after an empty comment line. A line opening with a lowercase letter is the
# rest of the sentence; an uppercase one only continues while the field has not
# finished its sentence. That boundary is what keeps a stated worker decision
# (AGENTS.md §1.6), a new section rather than the tail of a field, out of this
# check.
readonly FIELD_PLAIN_RE='^// [^@[:space:]]'
readonly FIELD_LOW_RE='^// [a-z]'
# A field whose text ends here has finished its sentence. A colon is not in this
# set: a colon introduces the line after it, so the value is not finished.
readonly FIELD_ENDS_RE='[.!?)"][[:space:]]*$'
# Go files the header checks read. Generated output is exempt, same as §1.1.
readonly GO_GLOBS=('*.go' ':(exclude)**/*_gen.go' ':(exclude)*.pb.go')

# code_globs are the file types the tree-wide structural checks read. The
# `*.example` entry is not decoration: env templates carry comments and were the
# one place a banner pattern hid from this gate, because they match no normal
# source extension.
readonly CODE_GLOBS=(
	'*.go' '*.ts' '*.tsx' '*.js' '*.mjs' '*.svelte' '*.sh' '*.bash'
	'*.sql' '*.yaml' '*.yml' '*.example'
)

# generated_globs are the outputs no one hand-wrote, which ANTISLOP.md §1 puts
# out of scope for every rule here. `app-ui/src/lib/primitives/` is in that list
# because the project already says so twice, in `.prettierignore` ("Generated by
# the component layer's CLI and not hand-edited") and in `eslint.config.js`'s
# `ignores`; no check can enforce a rule against text the next
# `bun x shadcn-svelte add` rewrites, and a gate that reads the directory can
# fail a commit for output its author never typed.
readonly GENERATED_GLOBS=(
	':(exclude)**/node_modules/**' ':(exclude)**/dist/**' ':(exclude)**/build/**'
	':(exclude)**/*_gen.go' ':(exclude)*.pb.go'
	':(exclude)app-ui/src/lib/primitives/**'
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
	grep_files "$re" "${CODE_GLOBS[@]}" "${GENERATED_GLOBS[@]}" ":(exclude)$SELF"
}

# header_shape_hits prints every §1.2 header line that either continues a field
# value onto a second line or puts a prose paragraph between the fields. The header
# region is everything above `package`.
header_shape_hits() {
	# Paths contain no spaces here, and awk needs one argument per file.
	# shellcheck disable=SC2046
	awk -v tagre="$FIELD_TAG_RE" -v contre="$FIELD_CONT_RE" \
		-v plainre="$FIELD_PLAIN_RE" -v endsre="$FIELD_ENDS_RE" -v lowre="$FIELD_LOW_RE" '
		FNR == 1              { inhdr = 1; anchor = ""; seen = 0 }
		/^package[[:space:]]/ { inhdr = 0 }
		inhdr == 0            { next }
		$0 ~ tagre            { anchor = $0; seen = 1; next }
		/^\/\/[ \t]*$/        { next }
		$0 ~ contre {
			if (anchor != "") {
				frag = $0
				sub(/^\/\/\t+/, "", frag)
				# An indented block after a finished sentence is structure rather than a
				# wrapped value: the runnable `go test` lines and a deliberate paragraph are
				# both written this way. Only a fragment opening in lowercase continues the
				# field, which is the same test the sweep applies.
				if (anchor ~ endsre && frag !~ /^[a-z]/) {
					anchor = ""
					next
				}
				printf "%s:%d: %s\n", FILENAME, FNR, substr($0, 1, 90)
				anchor = anchor " " frag
			}
			next
		}
		$0 ~ plainre {
			if (anchor != "" && (anchor !~ endsre || $0 ~ lowre)) {
				printf "%s:%d: %s\n", FILENAME, FNR, substr($0, 1, 90)
				anchor = anchor " " $0
			} else {
				# AGENTS.md §1.2 fixes the header as eight fields and nothing else, so a
				# paragraph of prose here is either the rest of the field above it or a
				# section that belongs in the file body. An indented block is exempt: it
				# is structure, and that is where the runnable examples live.
				if (seen) {
					printf "%s:%d: prose between the §1.2 fields: %s\n",
						FILENAME, FNR, substr($0, 4, 80)
				}
				anchor = ""
			}
			next
		}
		{ anchor = "" }
	' $(git -C "$root" ls-files -- "${GO_GLOBS[@]}")
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

# changed_text_paths narrows that to files this change touches. An unreadable
# change set is a failure, not an empty one: comm over an empty right side prints
# nothing, and the checks below would read that as a clean diff.
changed_text_paths() {
	local files
	if ! files="$(change_set)"; then
		return 1
	fi
	comm -12 <(text_paths | sort -u) <(printf '%s\n' "$files" | sort -u)
}

# go_body_comments prints only the comment text below a Go file's `package` line,
# which is where the §1.2 mandated header ends.
# The citation rule is about comments, and a `draft 017 §4.6` written into a
# t.Fatalf message is a test naming the case it asserts, not scratch work left in
# a comment, so the string part of a line is dropped before matching.
go_body_comments() {
	local file="$1" pkg
	pkg="$(grep -n '^package ' "$root/$file" | head -1 | cut -d: -f1 || true)"
	awk -v start="${pkg:-0}" '
	NR > start {
		pos = index($0, "//")
		if (pos == 0) next
		prefix = substr($0, 1, pos - 1)
		if (gsub(/"/, "\"", prefix) % 2 == 1) next
		print substr($0, pos)
	}' "$root/$file"
}

# fe_body_comments prints the comment text of a TypeScript or Svelte file: the
# lines that ARE a comment (`//`, a block's `*` continuation, `/*`), plus the
# inside of `<!-- ... -->` for Svelte only, since that markup comment does not
# exist in TypeScript. A line that merely ends with a trailing comment is not
# read, so `it('... draft 036')` stays out: a test naming the case
# it asserts is not comment debt, which is the ruling audit 006 item 6 reached for
# the Go `t.Fatalf` strings and applies here unchanged.
fe_body_comments() {
	local file="$1" html=0
	if [ "${file##*.}" = "svelte" ]; then
		html=1
	fi
	awk -v html="$html" '
	/^[[:space:]]*(\/\/|\*|\/\*)/ { print; next }
	{
		if (!html) next
		open   = index($0, "<!--") > 0
		closed = index($0, "-->") > 0
		if (open && closed) { print; next }
		if (open)           { print; inhtml = 1; next }
		if (inhtml)         { print; if (closed) inhtml = 0 }
	}' "$root/$file"
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
if ! scope="$(changed_text_paths)"; then
	gate_fail "antislop gate cannot read the change set (GATES_BASE_REF is set but does not resolve)"
	exit 1
fi

# ---- R-02, tree-wide ----

# This used to be a changed-files check with docs/** and AGENTS.md excluded,
# because the tree carried 821 legacy lines and enforcing it would have blocked
# every unrelated pull request. The debt is now swept (audit 005 round two), so
# the rule applies to the whole repository and fails on any new use. The
# exclusion is gone rather than left in place out of habit.
gate_start "R-02 no em or en dash (tree-wide)"
dash_hits="$(grep_files "$DASH_RE" '*.go' '*.ts' '*.tsx' '*.js' '*.mjs' '*.svelte' '*.css' \
	'*.sh' '*.bash' '*.yml' '*.yaml' '*.sql' '*.md' '*.example' \
	':(exclude)**/node_modules/**' ':(exclude)**/dist/**' ':(exclude)**/build/**' \
	':(exclude)**/*_gen.go' ':(exclude)*.pb.go' ':(exclude)app-serv/internal/handler/openapi.json' \
	":(exclude)$SELF")"
report "R-02 dash" "$dash_hits" || failed=1

# ---- citations, warning only ----

# ---- citations, on changed files ----

# Go warns and the panel fails, and the difference is the residue, not the rule.
# A Go file's §1.2 header is where AGENTS.md puts a file's rationale, so the 146
# citations there are the convention working, and a body warning must not hold a
# commit hostage to lines its author did not write. The panel has no such exempt
# region and its comment residue is zero, measured across 533 files, so a new
# citation in app-ui is necessarily a line the author wrote, which is the only
# case a failure can legitimately catch.
gate_start "scratch-work citations (changed files)"
if [ -z "$scope" ]; then
	gate_skip "no changed text files; set GATES_BASE_REF to check a diff range"
else
	cite_warn=0
	cite_fail=0
	while IFS= read -r f; do
		[ -n "$f" ] || continue
		[ -f "$root/$f" ] || continue
		case "${f##*.}" in
		go)
			c="$(go_body_comments "$f" | grep -E "$CITATION_RE" || true)"
			if [ -n "$c" ]; then
				gate_start "scratch citation to review in $f: $(printf '%s\n' "$c" | head -1 | cut -c1-110)"
				cite_warn=$((cite_warn + 1))
			fi
			;;
		ts | tsx | js | mjs | svelte)
			case "$f" in */primitives/*) continue ;; esac
			c="$(fe_body_comments "$f" | grep -E "$CITATION_RE" || true)"
			if [ -n "$c" ]; then
				gate_fail "$f cites a closed draft in a comment: $(printf '%s\n' "$c" | head -1 | cut -c1-110)"
				cite_fail=1
			fi
			;;
		esac
	done <<<"$scope"
	if [ "$cite_fail" = "1" ]; then
		failed=1
	elif [ "$cite_warn" -gt 0 ]; then
		gate_skip "$cite_warn changed Go file(s) still cite a closed draft; see anti-slop/audit-005-*.md"
	else
		gate_pass "scratch-work citations (changed files)"
	fi
fi

# ---- tree-wide: rules the tree already satisfies ----

report "substitution artifact in a comment" "$(tree_scan "$ARTIFACT_RE")" || failed=1
report "decorative separator" "$(tree_scan "$SEPARATOR_RE")" || failed=1
report "ALL CAPS banner label" "$(tree_scan "$BANNER_RE")" || failed=1
report "end marker" "$(tree_scan "$ENDMARK_RE")" || failed=1
report "§1.2 header shape (tree-wide)" "$(header_shape_hits)" || failed=1
report "decorative emoji" \
	"$(grep_files '[🚀✅🔒⚡✨📌🧪]' "${CODE_GLOBS[@]}" "${GENERATED_GLOBS[@]}" ":(exclude)$SELF")" ||
	failed=1

# AGENTS.md §1.4 as amended on 2026-10-05: every suppression carries a reason
# naming the constraint it protects. The clause used to demand a ticket ID too,
# and this repository has no ticket tracker, so the rule was unsatisfiable and
# therefore unenforced; the reason is the part a machine can actually check.
unexplained="$(grep_files '//nolint:' '*.go' ':(exclude)**/node_modules/**' ":(exclude)$SELF" |
	grep -v '// reason:' || true)"
report "suppression without a reason (AGENTS.md §1.4)" "$unexplained" || failed=1

# The panel suppresses with eslint directives, and §1.4 is a rule about
# suppressions rather than about Go, so the same requirement needs a second
# pattern or the rule reads as enforced while the panel's suppressions are
# unread. The reason must follow the separator rather than merely appear on the
# same line: exempting any line holding ` -- ` also exempted a directive that
# ended at it, and eleven existing directives all write a real reason.
panel_directives="$(tree_scan "$PANEL_SUPPRESS_RE")"
unexplained_panel="$(printf '%s\n' "$panel_directives" |
	grep -vE -e "$PANEL_SUPPRESS_REASON_RE" || true)"
report "panel suppression without a reason (AGENTS.md §1.4)" "$unexplained_panel" || failed=1
report "panel suppression naming no rule (AGENTS.md §1.4)" \
	"$(tree_scan "$PANEL_SUPPRESS_BARE_RE")" || failed=1

# ---- doc block length, on changed Go files only ----

# The threshold is deliberately not ported to the panel, and the measurement is
# why: across 567 non-test Go files, zero doc block above a `func` or `type` runs
# past 10 lines, while the panel has 14 declaration blocks past 10 and all of them
# pass §2.6's own test, which counts facts rather than lines. A 16-line failure
# copied to `.ts` would fire first on `schemas/quota.ts`, a block holding six
# load-bearing constraints, and the next agent would then compress facts to please
# a number. Panel block length is ANTISLOP.md §3 material: review-only, named.
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

# The markdown backlog this gate used to report is gone: R-02 is now enforced
# tree-wide above, so a dash in docs/** or AGENTS.md fails the run instead of
# being counted and forgiven. What remains as debt is the 170 scratch citations
# inside the §1.2 @reason headers, which are the convention working as designed.

if [ "$failed" -ne 0 ]; then
	gate_fail "antislop gate found violations above"
	exit 1
fi
gate_pass "antislop gate"

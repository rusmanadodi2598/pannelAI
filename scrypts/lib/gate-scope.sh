#!/usr/bin/env bash
# Library: route a change set to the gate set it can actually break.
#
# app-serv (Go) and app-ui (Svelte) share one process boundary and nothing else,
# so the slowest local gate, the panel suite, is wasted on a push that only moves
# Go. Routing is not a relaxation: the gate set is unchanged, only which half of it
# a push runs, and the answer comes from the paths in that push.
#
# The rules lean closed:
#
#   - A path this file does not recognise runs every gate. So does the gate
#     machinery itself (scrypts/, .github/), because a selector that cannot route
#     its own change is exactly the thing under test.
#   - The surfaces both halves read (the SPEC-API and CONTRACT documents, the wire
#     schema package, the generated openapi.json) run both. app-ui renders what
#     app-serv declares there, so drift is not confined to one side.
#   - An unreadable revision, an empty file list, and a force-push are unknown, and
#     unknown runs everything. A gate that reports a pass after inspecting nothing
#     is the failure this file exists to avoid.
#
# Usage:
#   printf '%s\n' "${paths[@]}" | gate_scope_from_files
#   gate_scope_reason
#
# Sets: GATE_SCOPE_GO and GATE_SCOPE_FE (0 skip, 1 run), the two WHY strings, and
#       GATE_SCOPE_FILES with the per-bucket counts for reporting.

GATE_SCOPE_GO=1
GATE_SCOPE_FE=1
GATE_SCOPE_GO_WHY=""
GATE_SCOPE_FE_WHY=""
GATE_SCOPE_FILES=0
GATE_SCOPE_GO_FILES=0
GATE_SCOPE_FE_FILES=0
GATE_SCOPE_BOTH_FILES=0
GATE_SCOPE_UNMATCHED_FILES=0

# gate_scope_contract_path reports the paths both halves read.
gate_scope_contract_path() {
	case "$1" in
	docs/SPEC-API/* | docs/CONTRACT/*) return 0 ;;
	app-serv/internal/schema/*) return 0 ;;
	app-serv/internal/handler/openapi.json | app-serv/tools/openapi-gen/*) return 0 ;;
	esac
	return 1
}

# gate_scope_unknown runs every gate and records why, for a caller that could not
# read the change set at all.
gate_scope_unknown() {
	GATE_SCOPE_GO=1
	GATE_SCOPE_FE=1
	GATE_SCOPE_FILES=0
	GATE_SCOPE_GO_WHY="running: $1"
	GATE_SCOPE_FE_WHY="running: $1"
}

# gate_scope_from_files reads one path per line and decides which gates to run. An
# empty list means nothing was classified, which is unknown rather than clean.
gate_scope_from_files() {
	local path
	GATE_SCOPE_GO=0
	GATE_SCOPE_FE=0
	GATE_SCOPE_GO_WHY=""
	GATE_SCOPE_FE_WHY=""
	GATE_SCOPE_FILES=0
	GATE_SCOPE_GO_FILES=0
	GATE_SCOPE_FE_FILES=0
	GATE_SCOPE_BOTH_FILES=0
	GATE_SCOPE_UNMATCHED_FILES=0

	while IFS= read -r path; do
		[ -n "$path" ] || continue
		GATE_SCOPE_FILES=$((GATE_SCOPE_FILES + 1))
		if gate_scope_contract_path "$path"; then
			GATE_SCOPE_BOTH_FILES=$((GATE_SCOPE_BOTH_FILES + 1))
			GATE_SCOPE_GO=1
			GATE_SCOPE_FE=1
			continue
		fi
		case "$path" in
		scrypts/* | .github/*)
			# The gate machinery: a selector must not be the only thing that
			# vouches for a change to itself.
			GATE_SCOPE_BOTH_FILES=$((GATE_SCOPE_BOTH_FILES + 1))
			GATE_SCOPE_GO=1
			GATE_SCOPE_FE=1
			;;
		app-serv/*)
			GATE_SCOPE_GO=1
			GATE_SCOPE_GO_FILES=$((GATE_SCOPE_GO_FILES + 1))
			;;
		app-ui/*)
			GATE_SCOPE_FE=1
			GATE_SCOPE_FE_FILES=$((GATE_SCOPE_FE_FILES + 1))
			;;
		*)
			# Anything else (docs/DRAFT, DESIGN.md, graphify output, the root
			# manifests, a directory added next week) is unknown territory, and
			# treating one path as both costs seconds.
			GATE_SCOPE_UNMATCHED_FILES=$((GATE_SCOPE_UNMATCHED_FILES + 1))
			GATE_SCOPE_GO=1
			GATE_SCOPE_FE=1
			;;
		esac
	done

	if [ "$GATE_SCOPE_FILES" = 0 ]; then
		gate_scope_unknown "the change set listed no files"
	fi
}

# gate_scope_reason fills the two why strings the hook prints, so a skip is always
# stated with the evidence behind it.
gate_scope_reason() {
	if [ "$GATE_SCOPE_FE" = 1 ]; then
		[ -n "$GATE_SCOPE_FE_WHY" ] || GATE_SCOPE_FE_WHY="running"
	elif [ "$GATE_SCOPE_GO" = 1 ]; then
		GATE_SCOPE_FE_WHY="skipped: this push moves no app-ui, contract, or gate path ($GATE_SCOPE_GO_FILES Go files)"
	else
		GATE_SCOPE_FE_WHY="skipped"
	fi
	if [ "$GATE_SCOPE_GO" = 1 ]; then
		[ -n "$GATE_SCOPE_GO_WHY" ] || GATE_SCOPE_GO_WHY="running"
	else
		GATE_SCOPE_GO_WHY="skipped: this push moves no app-serv, contract, or gate path ($GATE_SCOPE_FE_FILES panel files)"
	fi
}

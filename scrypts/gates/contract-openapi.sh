#!/usr/bin/env bash
# Gate: the served OpenAPI document matches the contract YAML it is generated from.
#
# docs/CONTRACT/001-CONTRACT-API-V1.yaml is the contract of record (AGENTS.md
# §2.4 CDD) and app-serv/internal/handler/openapi.json is generated from it, then
# embedded into the binary that answers GET /api/v1/openapi.json. Nothing in the
# Go build fails when the two drift, so a contract edit without a regeneration
# would ship a served document that contradicts the reviewed source: exactly the
# failure the panel and every CLI tool would read as truth.
#
# The generator runs in -check mode, which regenerates in memory and compares
# bytes. Byte comparison rather than object comparison is deliberate: the
# artifact is committed, so its formatting is part of what a reviewer approved.
#
# Exit codes: 0 the artifact matches, 1 it does not, 2 a source file is missing.

set -euo pipefail
. "$(dirname "$0")/../lib/common.sh"

root="$(repo_root)"
contract="$root/docs/CONTRACT/001-CONTRACT-API-V1.yaml"
module="$root/app-serv"

gate_start "contract (OpenAPI YAML -> served document)"

# A repository without the contract is not drift: the gate reports that it had
# nothing to compare instead of blocking a service-only checkout.
if [ ! -f "$contract" ]; then
	gate_skip "contract YAML not present; nothing to compare"
	exit 0
fi

if [ ! -f "$module/go.mod" ]; then
	gate_skip "app-serv module not present; nothing to compare"
	exit 0
fi

require_tool go || exit 1

if (cd "$module" && go run ./tools/openapi-gen -check); then
	gate_pass "the served document matches the contract YAML"
else
	gate_fail "app-serv/internal/handler/openapi.json is stale"
	gate_fail "run: (cd app-serv && go run ./tools/openapi-gen)"
	exit 1
fi

# SPEC-API-002: pannelAI Native Token-Saver Engine

| | |
|---|---|
| **Spec ID** | 002-TOKEN-SAVER |
| **Status** | Closed |
| **Date** | 2026-09-19 |
| **Organization** | KENTANG TECH |
| **Author** | Dodi Rusmana <rusmanadodi@kentangtech.com> |
| **GitHub** | https://github.com/rusmanadodi2598 |
| **Parent spec** | [`001-SPEC-API.md`](./001-SPEC-API.md) (§4 bypass header, §7.9 config surface, §7.15 request pipeline, §9 egress guard) |
| **Implementation** | `app-serv/internal/tokensaver/`, `app-serv/internal/dataplane/token_saver_translate*.go` |
| **Reference** | `/home/rusmanadodi/ai-gateway` (9Router): RTK filters, `ponytail` prompts, Headroom proxy client. Concepts ported; the JS code is not. |

> **SPEC FIRST.** This document is the contract of record for the native token-saver engine that
> SPEC-API-001 §10 places in P3. Implementation starts only against what is written here; any deviation
> requires editing this spec in the same PR. The configuration surface itself (the `token_saver` settings
> document, its route, and the per-request bypass) stays normative in SPEC-API-001 §7.9 and §4; this spec
> defines what the engine does with that document once an operator enables a group.

---

## 1. Purpose

SPEC-API-001 shipped the token saver as configuration only: §7.9 defined the document and left
`rtk.enabled` a pass-through flag "consumed by the native engine once built". P3 is where it is built.
This spec ports the reference's three savers as native Go:

- **RTK** rewrites tool results inside the request body so a CLI tool's large tool output costs fewer
  input tokens. It is a transform of the request, never of the user's words (§3).
- **Ponytail** appends a level-scoped instruction to the body's system slot so the model answers with
  less scaffolding. It is a bias, not a transform (§7.1 to §7.3).
- **Headroom** hands the message array to an external compression proxy and puts the compressed array
  back. It is the one saver that leaves the process (§7.4 to §7.6).

The engine amends SPEC-API-001's Locked Decision 4 ("token saver = config-only in v1") exactly as far as
§10's P3 row already planned: the native engine ships in v1 under this spec, with the `caveman` group
still frozen and deprecated (§6.4).

## 2. Definitions

| Term | Meaning |
|---|---|
| **Saver group** | One independently optional block of the `token_saver` settings document: `rtk`, `headroom`, `ponytail` |
| **Upstream body** | The request body after the gateway has translated it into the target provider's wire format |
| **Tool result** | The output a client's tool call reports back to the model: an OpenAI tool message, an Anthropic `tool_result` block, or a Responses `function_call_output` item |
| **Blob** | One tool result's text, as a string or as the joined text of its content parts |
| **Filter** | A named, pure string-to-string rewrite a blob may be given (§4) |
| **Allowlist** | The configured `rtk.filters` list; it decides which filters may claim a blob (§4.3) |
| **Fail open** | Returning the body unchanged so the upstream call proceeds; the only permitted failure direction for any saver step (§8.4) |

## 3. The RTK Pass: Scope Rules

### 3.1 Tool results only

RTK rewrites tool results and nothing else. A user message's or an assistant message's text is never
rewritten, whatever it contains: the client's own words are the one thing in the body the model must read
exactly as written.

### 3.2 Errors are preserved

A tool result that reports an error is never rewritten. On the Anthropic wire that is a `tool_result`
block with `is_error: true`; a failed tool result compressed to look clean would be the one place a
rewrite is a lie the model acts on.

### 3.3 The shapes that carry a tool result

The engine finds tool results in exactly these places, string or content-parts forms alike:

| Wire | Carrier | Rewritten member |
|---|---|---|
| OpenAI chat | message with `role: "tool"` | `content` (string, or array of `text` parts) |
| Anthropic | `tool_result` content block | `content` (string, or array of `text` parts) |
| Responses | `function_call_output` input item | `output` (string, or array of `input_text` parts) |

### 3.4 Untouched members survive byte for byte

The rewrite happens at the JSON level, member by member, not through a decode-model-encode round trip:
every field the gateway does not model reaches the provider as it arrived. Bodies are re-encoded without
HTML escaping so a sibling carrying source code is not silently rewritten mid-passage.

### 3.5 Never empty, never larger

A filter that produces an empty string, or an output at least as large as its input, is declined and the
original blob is kept. A filter panic is treated as a decline, the reference's `catch_unwind` equivalent:
a malformed blob must never fail a request (§8.4).

## 4. The Filter Vocabulary and the Allowlist

### 4.1 The twelve canonical names

The engine implements exactly these filters, and `rtk.filters` accepts exactly these values:

`git-diff`, `git-status`, `git-log`, `grep`, `find`, `ls`, `tree`, `dedup-log`, `smart-truncate`,
`read-numbered`, `search-list`, `build-output`

The list is the contract between configuration and engine: the schema's validation vocabulary, the
domain's canonical list, and the engine's registry are three views of the same twelve names, and tests
pin them together so a name can never be configurable but unimplemented, or implemented but unreachable.

### 4.2 The empty allowlist means every filter

An empty `rtk.filters` is a valid configuration and means every filter is eligible. The wire renders the
empty state as `[]`, never `null`, so a panel round-trip cannot confuse "no allowlist" with a missing
member. An unknown name is refused at validation time; a stored row that predates a rename still cannot
break a request, because an unresolvable name claims nothing.

### 4.3 The allowlist gates detection, it does not select

The engine autodetects which filter fits a blob (§5.2). The allowlist decides which filters may be
claimed at all: a filter outside the list is skipped even when its shape matches. Command-line aliases
the reference accepts (`rg`, `fd`) are not configuration values; the panel picks from the canonical list.

## 5. Caps, Sizes, and Detection Order

### 5.1 The caps

Every cap is a documented number, ported from the reference's constants. A blob below the floor or above
the raw cap is passed through untouched: a rewrite below the floor would cost more in markers than it
saves, and a blob above the cap is usually a file the model needs whole.

| Cap | Value | Applies to |
|---|---|---|
| `RAW_CAP` | 10 MiB | Largest blob handed to any filter |
| `MIN_COMPRESS_SIZE` | 500 bytes | Floor below which a blob is not rewritten |
| `DETECT_WINDOW` | 1024 bytes | How much of a blob autodetection reads |
| git-diff | 500 lines total, 100 per hunk | Compacted diff |
| git-log | 200 lines | Compacted log |
| dedup-log | 2000 lines | Deduplicated log |
| grep | 10 matches per file | Grep output |
| find | 10 files per directory, 20 directories | Find output |
| git-status | 10 files per group, 10 untracked | Status output |
| ls | 5 extensions in the summary | Directory listing |
| tree | 200 lines | Tree listing |
| search-list | 10 files per directory, 20 directories | Search result listing |
| smart-truncate | 120 head lines, 60 tail lines, applies above 250 lines | Generic fallback |
| read-numbered | 0.7 minimum hit ratio on the "  N|content" shape, 250 lines minimum | Numbered file dumps |
| build-output | 3 deprecation notices, 5 warnings kept verbatim | Build logs |
| Headroom answer | 16 MiB | External compression response (§7.4) |

Sizes are byte counts. The reference's own arithmetic counts UTF-16 units; the gateway counts bytes, which
is what its constants mean in practice, and the difference is recorded here rather than silently inherited.

### 5.2 The detection order

Detection is a fixed, observable order, because the order is behaviour: build output must be claimed
before the porcelain check or a `cargo` run reads as a dirty tree, and a git log must be claimed before a
diff or a log with an embedded diff loses its subjects. A blob is claimed by the first filter that
matches and is allowed:

`git-log` → `git-diff` → `git-status` → `build-output` → porcelain `git-status` → `grep` → `find` →
`tree` → `ls` → `search-list` → `read-numbered` → `dedup-log` → `smart-truncate`

Two deliberate deviations from the reference, recorded rather than inherited:

- **`read-numbered` measures the whole blob**, not the detection window. A numbered file dump is claimed
  by its line count, and the 1024-byte window cannot hold the 250 lines the threshold names; the
  reference's window read makes the rule almost never fire.
- **`dedup-log` claims any blob with five or more non-empty lines** as the generic multi-line fallback,
  which is why `smart-truncate` below it only sees blobs that are almost all blank.

The window is truncated on a byte boundary that may cut a multi-byte rune; the matchers read ASCII
markers only, so the cut byte is inert. That truncation is documented here rather than silently repaired.

## 6. The Settings Document

### 6.1 The shape

The `token_saver` document on the wire (SPEC-API-001 §7.9, as amended by this spec):

```json
{
  "rtk":      { "enabled": false, "filters": [] },
  "headroom": { "enabled": false, "url": "", "compress_user_messages": false },
  "ponytail": { "enabled": false, "level": "full" }
}
```

`rtk` carries no level: its strength is the filter allowlist, which replaces the level field the
reference gave it. `level` belongs to `ponytail` alone (§7.1).

### 6.2 Defaults

Every saver ships **off** (owner decision, 2026-09-19). The pipeline must not rewrite a request until an
operator turns a group on; a fresh install is byte-neutral on the request path.

### 6.3 Validation

- `rtk.filters` entries must be canonical names (§4.1); the empty list is accepted (§4.2).
- `ponytail.level` must be one of `lite`, `full`, `ultra` (§7.1).
- `headroom.url`, when set, must be an absolute http(s) URL.
- A PUT replaces the document: every group is required, and an absent group is a `VALIDATION_ERROR`
  rather than a silent reset to defaults. GET never leaves a stale member behind.

### 6.4 The deprecated group

`caveman` stays accepted, frozen at its default, and scheduled for removal in `/api/v2`. It is not a
group of this engine; its contract lives in SPEC-API-001 §7.9.

## 7. Ponytail and Headroom

### 7.1 Ponytail is a bias

Ponytail appends one instruction to the body's system slot. Three levels share one prompt scaffold
(persona, escalation ladder, rules, output shape, the not-lazy guard, the persistence line) and differ
only in their level line. The prompts are ported verbatim from the reference and held as data, not as
logic, so a prompt change is a reviewable diff.

### 7.2 The injection slot is per wire

The instruction must reach the model as a system-level text, and the wires disagree about where that
lives. Injecting after translation is what keeps the instruction from being translated away:

- **OpenAI chat**: the first `system` or `developer` message's content, as a string append or an appended
  `text` part; a body with neither gets a new leading system message.
- **Responses**: the `instructions` string when present; otherwise a system message item's content
  (`input_text` parts); otherwise a new system message item at the head of `input`.
- **Anthropic**: the top-level `system` member, never a system role inside `messages` (the wire rejects
  one). A string system is appended; a block list receives the instruction as a text block inserted
  **before the last `cache_control` block**, so the instruction joins the cached prefix instead of riding
  outside it and being re-billed on every request. A body with no cached block gets the instruction
  appended.

### 7.3 Injection is idempotent

The instruction is detected as its own `\n\n`-separated segment, so text that merely contains similar
words is never mistaken for it. Re-injecting into an already-injected body is a no-op, and the body is
left unchanged, whatever the level. An unknown level, an unknown wire, or an unreadable body also leaves
the body untouched (§8.4).

### 7.4 The Headroom call

Headroom compression is one POST to the configured base URL with `/v1/compress` appended, preserving an
operator's base path and query and dropping the fragment. The payload is `{messages, model,
config?: {compress_user_messages}}`; the model is sent as configured, including when empty, so the proxy
may pick a tokenizer. The answer is `{messages, tokens_before?, tokens_after?, tokens_saved?}`, where
`messages` must be an array and the token counts are the proxy's own accounting, read and dropped by the
gateway. The answer read is capped at 16 MiB. The gateway's messages are already an OpenAI array when a
body is native OpenAI; a Claude or Responses body is pivoted through the existing translators, and a
Responses input carrying tool or reasoning items is refused (their ordering cannot survive a message
array), which sends the body upstream uncompressed. Restoring replaces only the wire's message members
(`messages` for chat, `messages` and `system` for Claude, `input` and `instructions` for Responses) and
leaves every other member byte-identical (§3.4).

### 7.5 The timeout

The compression call is bounded by 5 seconds, as SPEC-API-001 §7.9 fixes. The reference abandons the call
at 3 seconds; the difference is deliberate and the parent spec stays normative. On timeout, as on any
Headroom failure, the body goes upstream uncompressed.

### 7.6 The egress rule applies

The Headroom client uses the process's single guarded HTTP client (SPEC-API-001 §9): a compression call
dials through the same `internal/netguard` guard and `EGRESS_ALLOWED_TARGETS` allowlist as every upstream
call, so a self-hosted proxy on loopback or a private range needs its address allowlisted exactly like a
self-hosted provider. Error messages name the endpoint redacted, without userinfo, query, or fragment,
because the URL is operator input that may carry a token.

## 8. Pipeline Order and Failure Direction

### 8.1 The order

The pipeline runs **RTK → Headroom → Ponytail**, the reference's order, with each group independently
optional. RTK first so Headroom compresses an already-slimmed body; Ponytail last so the instruction is
appended to whatever body finally goes upstream.

### 8.2 The placement

The pipeline sits inside the relay, applied to the **already translated upstream body** before the
transport call, for streamed and non-streamed calls alike. A saver can therefore never be undone by a
later format conversion, and the panel never sees a rewritten body: only the provider does.

### 8.3 Configuration is read per request

The settings document is read for every relayed call, so an operator's change takes effect on the next
request rather than the next boot, the same rule §7.11 of the parent spec fixes for proxy settings.

### 8.4 Everything fails open

A bypass header (`X-Token-Saver: off`, SPEC-API-001 §4), a settings read failure, a proxy failure, a
malformed optional transform, a filter panic: every one returns the latest body, and the upstream call
proceeds. A saver is an optimization; no request fails, and no body grows, because of one. The savers
write no usage row and no log row of their own: the relayed call keeps the single accounting row the
parent spec's §7.12/§7.13 rules give it.

## 9. Acceptance

- The engine's behaviour is pinned by table-driven tests in `internal/tokensaver` (scope rules, filter
  rules, detection order, prompt fidelity, injection idempotency, Headroom fail-open and timeout) and by
  the relay-seam test in `internal/dataplane` (§8.2's placement and the bypass flag's journey).
- The configuration round-trip is pinned by tests in `internal/schema`, `internal/domain`,
  `internal/service`, `internal/handler`, and `internal/router`.
- **Parity spot-checks vs reference** (P3's exit criterion): the reference checkout is not present in this
  workspace, so parity is carried by the port itself: the prompts are verbatim, the caps are the
  reference's constants, and the detection order is the reference's, with the two §5.2 deviations and the
  §7.5 timeout difference recorded here. A live reference diff remains open until a checkout is available.

---

*Changelog: 2026-09-19: initial draft, written against the P3 implementation it governs: §3 to §5 fix the
RTK pass the engine already performs, §6 the settings shape as amended (filters replace the RTK level;
every saver ships off), §7 the ponytail prompts and slots and the Headroom call contract, §8 the
pipeline's order, placement, and fail-open direction.*

*Changelog: 2026-09-20: closed after the P3.4 review. Every §9 acceptance item was verified against the
implementation: the twelve names are pinned together by `autodetect_test.go` (registry, domain
vocabulary, schema `oneof`), the caps and detection order in `constants.go` and `autodetect.go` match
§5 with both recorded deviations, the relay seam test pins §8.2's placement and the bypass flag's
journey, the config round-trip is pinned in all five layers, and `go test -race ./...`, `go vet`, and
`staticcheck` are clean. The parity spot-check stays as §9 records it: the reference checkout is still
absent, so parity remains carried by the port, with the live diff reopening only if a checkout
appears.*

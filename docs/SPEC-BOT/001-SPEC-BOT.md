# SPEC-BOT-001: pannelAI Telegram Media Bot

|                     |                                                                                                                       |
| ------------------- | --------------------------------------------------------------------------------------------------------------------- |
| **Spec ID**         | 001-SPEC-BOT                                                                                                          |
| **Status**          | Draft for owner review                                                                                                |
| **Date**            | 2026-10-21                                                                                                            |
| **Organization**    | KENTANG TECH                                                                                                          |
| **Author**          | Dodi Rusmana <rusmanadodi@kentangtech.com>                                                                            |
| **Target**          | `pannelAI/app-bot/` (new Go service), Telegram Bot API, `app-serv` data plane                                        |
| **Governance**      | `AGENTS.md`, `docs/RULLES/TDD.md`, `docs/RULLES/OWASP.md`, `docs/RULLES/SSRF.md`, `docs/RULLES/ANTISLOP.md`           |
| **Upstream**        | `docs/CONTRACT/001-CONTRACT-API-V1.yaml` § Media (`/api/v1/images/generations`, `/api/v1/videos/generations`)           |
| **Payment rail**    | `https://docs.kentangtech.com` (Kentang Tech Public API, PG charges, S2S HMAC)                                        |

> **SPEC FIRST.** This document is the contract of record for the media bot. Implementation starts only against what is
> written here. A change in behavior, payload, or data model edits this spec in the same PR. Numbers marked
> `PROPOSED` are owner decisions that are not final, and code must read them from configuration, never hardcode them.

---

## 1. Purpose

A Telegram bot that turns a user's prompt into a generated image or video through the pannelAI gateway, with a
structured master prompt between the user's words and the upstream call, and with plans, credits, referrals, and
payments enforced server side.

The bot is a client of `app-serv`, not a second gateway. It owns Telegram conversation state, prompt compilation,
plan entitlements, credit accounting, referral campaigns, and payment orchestration. It does not own provider
credentials, model routing, retries against AI providers, or quota accounting against providers.

## 2. Scope

### 2.1 In scope

- Telegram webhook intake, private chat only, with duplicate suppression.
- Image generation through `POST /api/v1/images/generations`.
- Video generation through `POST /api/v1/videos/generations` plus polling `GET /api/v1/videos/{task_id}`.
- Structured master prompt compilation, preview, and confirmation before a paid call.
- Plans: `free` (referral only), `basic`, `premium`, `enterprise`.
- Per-generation credit debit with hold, settle, and refund.
- Referral campaigns that grant free access.
- Payment through the Kentang Tech PG charge rail, with the adapter isolated behind one interface.
- Operational surface: health, readiness, metrics, structured logs, graceful shutdown.

### 2.2 Out of scope for v1

- Group or channel usage. The bot answers private chats only.
- Image editing, image-to-image, audio, and transcription.
- Multi-turn creative sessions that remember earlier generations as context.
- A web panel for bot administration. Campaign rows are managed through the internal admin route or SQL.
- Reselling provider subscriptions. Plans sell credits, not provider accounts.

## 3. Verified environment baseline

Checked on the deployment host on 2026-10-21:

| Item                | Value                                                                                     |
| ------------------- | ----------------------------------------------------------------------------------------- |
| OS                  | Ubuntu 24.04.4 LTS, kernel 7.0.0, 2 vCPU, 1 GB RAM, 35 GB free                            |
| Go toolchain        | `go1.27.2 linux/amd64` at `~/sdk/go`; `app-serv/go.mod` still declares `go 1.26`          |
| Redis               | 7.0.15, listening on `127.0.0.1:6379`, password required                                  |
| PostgreSQL          | 17.11, listening on `127.0.0.1:5432`, service active                                       |
| Nginx               | active on 80 and 443, TLS termination in place                                             |
| Live hostnames      | `app-router.kentang.ai.id`, `app-hermes.kentang.ai.id`                                     |
| Certbot             | installed, no certificate yet for a bot hostname                                           |
| Public IPv4         | 13.212.229.76                                                                              |

Consequences for this spec:

1. `app-bot` is a new Go module that declares `go 1.27` and `toolchain go1.27.2`. The `app-serv` module is not bumped
   in this work.
2. The webhook needs one new hostname, `app-bot.kentang.ai.id`, with a certbot certificate and an Nginx server block.
   Until DNS and the certificate exist, local testing uses a tunnel and a webhook secret, never a public IP over HTTP.
3. The host has 1 GB RAM. Worker concurrency is bounded and documented in §14.3, not left to defaults.

## 4. Architecture

### 4.1 Topology

```
Telegram Bot API
      |  HTTPS POST update (X-Telegram-Bot-Api-Secret-Token)
      v
Nginx (app-bot.kentang.ai.id)
      |
      v
app-bot  ---- Redis (dedupe, rate limit, stream queue, poll locks)
   |    \___ PostgreSQL (users, plans, credits, jobs, referrals, payments)
   |
   |  gateway key (Bearer)
   v
app-serv data plane
   |-- POST /api/v1/images/generations
   |-- POST /api/v1/videos/generations
   \-- GET  /api/v1/videos/{task_id}
            |
            v
      provider upstream (owned by app-serv)

app-bot ---- Kentang Tech Public API (PG charges, S2S HMAC)
```

### 4.2 Why webhook, and why a queue behind it

Webhook is mandatory for this deployment: no long polling process, one ingress, and Telegram retries a failed
delivery on its own schedule. The webhook handler does four things and returns: verify the secret, decode into a typed
update, suppress a duplicate `update_id`, and append a work item to a Redis stream. It does not compile prompts, call
the gateway, or send media inline. Telegram's delivery timeout is short enough that any slow work inside the handler
turns into a redelivery storm, and a redelivery storm on a 2 vCPU host is how duplicate paid generations happen.

Ordering is not required between different users. Per chat, the worker processes one item at a time by claiming
`bot:sess:{chat_id}` lock, so a fast double-tap cannot produce two confirms for the same draft.

### 4.3 Service layout

Canonical layout from `AGENTS.md` §1.5, with the bot's own additions:

```
app-bot/
  cmd/app-bot/
    main.go                # compose, run, shutdown only
    wiring_telegram.go
    wiring_media.go
    wiring_billing.go
    wiring_workers.go
    egress_wiring.go       # the single guarded HTTP client
  internal/
    config/                # typed env, fail fast
    domain/                # entities and value objects
    schema/                # request/response DTOs + validator tags
    repository/postgres/
    repository/redis/
    service/               # orchestration
    masterprompt/          # compile, validate, templates
    handler/               # webhook, admin, health
    router/
    telegram/              # Bot API client, typed send helpers
    media/                 # app-serv client, downloader, uploader
    billing/               # plans, credits, ledger, payment adapter
    referral/
    worker/                # stream consumer, video poller, outbox
    netguard/              # egress guard for downloads and payment calls
    logx/
    requestctx/
  migrations/
  go.mod
```

`app-bot` is its own module. It must not import `app-serv/internal/...` (`AGENTS.md` §1.5). The two services talk over
HTTP. Shared types that both genuinely need go into a small module in a later PR, not into a copied file with the same
package name.

### 4.4 Layer rules that matter for this service

- `service` never imports `net/http`. Handlers translate, services decide.
- Redis and PostgreSQL access live in `repository`. A handler never calls Redis directly.
- The Telegram API client is a repository-like boundary (`telegram`), injected into services as an interface so a
  table-driven test can assert the send sequence without network.
- Every external URL the bot downloads passes one guard, `internal/netguard`, built once in `egress_wiring.go`
  (`SSRF.md` §2.1).

## 5. Telegram interface

### 5.1 Commands

| Command              | Behavior                                                                                 |
| -------------------- | ---------------------------------------------------------------------------------------- |
| `/start`             | Register the user, show plan and credit balance. A payload `ref_<code>` redeems a referral. |
| `/start ref_<code>`  | Same, plus referral redemption. The deep link is `https://t.me/<bot>?start=ref_<code>`.  |
| `/image`             | Set the session modality to image.                                                        |
| `/video`             | Set the session modality to video. Refused when the plan does not include video.          |
| `/mode`              | Show the current modality and the two buttons.                                            |
| `/plan`              | Show the four plans, current entitlements, and the top-up buttons.                        |
| `/credits`           | Show balance, the last five ledger rows, and the reset date.                              |
| `/referral`          | Show the user's own referral link, redemptions, and credits earned.                       |
| `/history`           | Last five generation jobs with status and a resend button for succeeded assets.           |
| `/cancel`            | Clear the pending draft, or cancel the active job when it has not reached the upstream.   |
| `/help`              | Usage rules, limits, and support contact.                                                 |
| `/terms`             | Terms and privacy summary, with the retention window from §13.5.                          |

Any plain text message that is not a command is treated as a prompt in the current modality. A user who has never
chosen a modality gets the modality picker first, and the text is kept as the draft prompt.

### 5.2 Image flow

```
User: "kucing oren naik skateboard di Jakarta, cinematic"
Bot : Params: image, 16:9, cinematic. Compiled prompt shown.
      [Generate image - 1 credit] [Change aspect] [Cancel]
User: taps Generate image
Bot : edits the same message: "Queued. 1 credit held."
Bot : sends the photo with a caption: model, aspect, remaining credits
```

### 5.3 Video flow

Video is an async job. The confirm creates the job, the bot answers with a progress message, and one poller owns the
job until it reaches a terminal state. Progress edits are rate limited: at most one edit per 2 seconds and never more
than 30 edits per job, so a stuck job does not turn into an API flood.

```
User: taps [Generate video - 8 credits]
Bot : "Queued. 8 credits held. You will get the video here."
Bot : edits: "Rendering 42%" (edited in place, no new messages)
Bot : sends the video, or a link when the file exceeds the upload cap
Bot : refunds the hold when the job fails or times out
```

### 5.4 Message rules

- Product copy is Indonesian. Error lines are English, per `AGENTS.md` §1.3, and always carry a machine code:
  `GAGAL: credit balance is insufficient (BOT_INSUFFICIENT_CREDITS)`.
- No em dash or en dash in any copy or document (R-02, `docs/RULLES/ANTISLOP.md` §2.1).
- No fabricated numbers in copy. Prices, limits, and capacities come from configuration rows, never from a string
  literal in a handler.
- Every state has a message: queued, running, succeeded, failed with a reason, refunded, and cancelled. A silent bot
  after a paid action is a defect.
- Message text never includes the bot token, gateway key, provider key, or full payment reference.

### 5.5 Telegram limits the design respects

| Limit                             | Value used in this spec                                              |
| --------------------------------- | -------------------------------------------------------------------- |
| Photo upload                      | 10 MB                                                                |
| Video upload                      | 50 MB                                                                |
| Text message                      | 4096 characters; our replies are capped at 3500                     |
| Caption                           | 1024 characters                                                     |
| Inline keyboard buttons per row   | 3                                                                   |
| callback_data                     | 64 bytes; we send an opaque draft id, never the prompt              |

An asset larger than the cap is not uploaded. The bot sends a short message plus the upstream URL when the URL is
publicly reachable, and refuses otherwise. The refusal is a stated behavior, not a silent truncation.

## 6. Master prompt (six sections)

### 6.1 Design rule

The user writes intent. The bot elaborates it into a master prompt with six labeled sections: **Role**, **Task**,
**Context**, **Reasoning**, **Stop conditions**, **Output**. The section model is the owner's master prompt builder
concept and is normative for this service.

Two stages, with different guarantees:

1. **Builder** (creative, non-deterministic): one language model call through the gateway's chat route turns the user
   text plus the selected modality into a typed master prompt document. The model never writes free text into the
   pipeline; it returns JSON that is decoded, validated, and rejected when it does not fit §6.2.
2. **Compiler** (deterministic, pure): maps the validated document onto the upstream request, producing one prompt
   string plus request parameters. Same document, same bytes, so a golden test can pin the output.

Splitting the stages keeps the creative part replaceable and the part that costs money auditable. A builder outage
degrades to a deterministic fallback (§6.3), not to a failed generation.

### 6.2 Master prompt schema (`mpv2`)

```json
{
  "version": "mpv2",
  "modality": "image",
  "role_family": "visual_designer",
  "role": "senior visual designer with campaign experience",
  "task": "create a hero image of an orange cat riding a skateboard through Jakarta",
  "context": {
    "subject": "an orange cat riding a skateboard",
    "action": "rolling through a busy street",
    "environment": "Jakarta, late afternoon, street vendors",
    "composition": {"shot": "medium", "angle": "eye_level", "aspect_ratio": "16:9"},
    "lighting": "warm golden hour, soft shadows",
    "style": {"medium": "cinematic photo", "palette": ["warm orange", "teal shadow"], "mood": "playful"},
    "details": ["motion blur in the wheels", "street vendors in the background"]
  },
  "reasoning": "The subject is playful and local, so a warm street scene with motion reads faster than a studio setup. Medium shot keeps the cat and the board legible at phone size.",
  "stop_conditions": {
    "negative": ["watermark", "text artifacts", "extra limbs"],
    "max_characters": 3500,
    "aspect_ratio": "16:9",
    "duration_s": null
  },
  "output": {
    "asset": "image",
    "aspect_ratio": "16:9",
    "count": 1,
    "format": "png",
    "duration_s": null
  },
  "seed": null
}
```

Field rules:

| Section           | Field                        | Type            | Rule                                                                  |
| ----------------- | ---------------------------- | --------------- | --------------------------------------------------------------------- |
| Role              | `role_family`                | enum            | Required: `visual_designer`, `photographer`, `illustrator`, `motion_designer`, `three_d_visualizer` |
| Role              | `role`                       | string          | Required, 3 to 200 runes, a visual or creative specialist inside the family, never a task |
| Task              | `task`                       | string          | Required, 5 to 400 runes, one sentence stating what to create          |
| Context           | `context.subject`            | string          | Required, 1 to 300 runes                                               |
| Context           | `context.action`             | string          | Optional, 0 to 200 runes                                               |
| Context           | `context.environment`        | string          | Optional, 0 to 200 runes                                               |
| Context           | `context.composition.shot`   | enum            | `wide`, `medium`, `close_up`, `macro`, `aerial`                        |
| Context           | `context.composition.angle`  | enum            | `eye_level`, `low`, `high`, `top_down`                                 |
| Context           | `context.composition.aspect_ratio` | enum      | `1:1`, `3:4`, `4:3`, `9:16`, `16:9`                                    |
| Context           | `context.lighting`           | string          | Optional, 0 to 120 runes                                               |
| Context           | `context.style.medium`       | enum            | `photo`, `cinematic photo`, `illustration`, `3d render`, `anime`, `watercolor` |
| Context           | `context.style.palette`      | string array    | Max 4 entries, each max 32 runes                                       |
| Context           | `context.style.mood`         | string          | Optional, max 48 runes                                                 |
| Context           | `context.details`            | string array    | Max 6 entries, each max 120 runes                                      |
| Reasoning         | `reasoning`                  | string          | Required, 20 to 600 runes, why these choices serve the task            |
| Stop conditions   | `stop_conditions.negative`   | string array    | Max 10 entries, each max 60 runes                                      |
| Stop conditions   | `stop_conditions.max_characters` | integer     | 500 to 3500; the compiler budget for this document                     |
| Stop conditions   | `stop_conditions.aspect_ratio` | enum          | Same enum as composition; must agree with `output.aspect_ratio`        |
| Stop conditions   | `stop_conditions.duration_s` | integer or null | Video only, 1 to 10                                                   |
| Output            | `output.asset`               | enum            | `image`, `video`; must equal `modality`                                |
| Output            | `output.aspect_ratio`        | enum            | Same enum                                                              |
| Output            | `output.count`               | integer         | 1 only in v1; more than 1 refused with `BOT_PROMPT_UNSUPPORTED_FIELD`  |
| Output            | `output.format`              | enum            | `png`, `jpeg`, `webp` for image; `mp4` for video                       |
| Output            | `output.duration_s`          | integer or null | Video only, 1 to 10                                                    |
| Top level         | `seed`                       | integer or null | `0` to `2147483647`                                                    |

Rules that are enforced, not advisory:

- Unknown fields are rejected with `DisallowUnknownFields`, so a model that invents a section fails validation
  instead of silently losing it.
- `role` must describe visual, design, or creative work inside the declared `role_family`. A role that names a
  non-visual profession, a single brand, a single product, or a single industry vertical is refused with
  `BOT_PROMPT_ROLE_SCOPE`, then remapped to the family's neutral descriptor before preview.
- A `modality: image` document with `duration_s` set, or a `modality: video` document without one, is refused with
  `BOT_PROMPT_UNSUPPORTED_FIELD`.
- `stop_conditions.aspect_ratio` and `output.aspect_ratio` must match. A disagreement is a builder defect, not a user
  error, and is retried once before falling back.
- Every string is normalized (NFC, control characters removed, whitespace collapsed) before validation. A field that
  is empty after normalization is treated as missing.
- A URL in any field is refused with `BOT_PROMPT_UNSUPPORTED_FIELD`. The builder is not a fetch tool.

### 6.3 Builder call

| Item          | Value                                                                                         |
| ------------- | --------------------------------------------------------------------------------------------- |
| Route         | `POST /api/v1/chat/completions` on `GATEWAY_BASE_URL`, gateway key auth                        |
| Model         | `BUILDER_MODEL`, a cheap chat model selected by the owner, never hardcoded                     |
| Temperature   | `0.2`                                                                                          |
| Max tokens    | `BUILDER_MAX_TOKENS`, default `700`                                                            |
| Timeout       | `BUILDER_TIMEOUT`, default `20s`                                                               |
| Response      | JSON object, no markdown fence, decoded with `DisallowUnknownFields`                           |
| Attempts      | 2. The second attempt appends the validator error and asks for a corrected document           |
| Fallback      | Deterministic builder (§6.3.2) when both attempts fail, marked `builder: "fallback"`           |
| Budget        | One builder call per draft, at most 2 attempts, at most 60 builder calls per user per day      |

The system prompt is fixed text from `docs/SPEC-BOT/002-MASTER-PROMPT-METADATA.md`. The user's text is placed in a
single user message inside a JSON envelope, never concatenated into the system prompt:

```json
{
  "modality": "image",
  "locale": "id",
  "user_prompt": "<the user's text, max 2000 runes>",
  "defaults": {"aspect_ratio": "16:9", "style": "cinematic photo"}
}
```

The gateway response is read for three things: the JSON document, the model id that answered, and `usage` token
counts. Token usage is stored on the job for cost reconciliation. A builder response that is valid JSON but fails
validation is not shown to the user; it is retried, then replaced by the fallback.

#### 6.3.1 Semantic checks after decode

1. All six sections are present and non-empty after normalization.
2. `role` belongs to `role_family` and stays product neutral: no brand name, no single product name, no vertical
   that narrows the role to one catalogue.
3. `task` and `context.subject` do not contradict each other on modality (a video task with an image asset fails).
4. No field contains a URL, a newline-only payload, or a control character after normalization.
5. `reasoning` must mention at least one element from `context` (a cheap relevance check against the compiled
   document, not a second model call). A reasoning string that is generic filler fails the check.
6. The compiled prompt from §6.4 stays inside `stop_conditions.max_characters`.

#### 6.3.2 Deterministic fallback builder

When the builder is unavailable, over budget, or returns invalid output twice, the service builds the document
without a model:

1. Normalize the input and cap it at 2000 runes.
2. Pull `aspect` tokens and duration tokens out of the text.
3. Pull a style term and a camera motion term from the closed vocabulary in the metadata document.
4. The first sentence becomes `task` and `context.subject`; a second sentence becomes `context.action`.
5. `role_family` defaults to `visual_designer` for image and `motion_designer` for video, `role` becomes the family's
   neutral descriptor, `reasoning` becomes a fixed sentence naming the extracted elements, and `stop_conditions`
   uses the configured baseline negatives.

The result is labelled `builder: "fallback"` in the job row and in the preview message, so the user knows the prompt
was not model-expanded. The label is honest, not decorative.

### 6.4 Compiler rules

The compiler maps six sections onto two outputs: the upstream prompt string and the request parameters.

| Section           | Goes to the prompt string                                  | Goes to request parameters            |
| ----------------- | ---------------------------------------------------------- | ------------------------------------- |
| Role              | `role_family` selects the style descriptor; the title refines it only when it maps to the vocabulary, otherwise it is kept as metadata | none |
| Task              | Subject and action clause                                  | none                                  |
| Context           | Environment, composition phrase, lighting, style, details  | aspect ratio, size, duration          |
| Reasoning         | Never sent upstream; stored and shown truncated in preview | none                                  |
| Stop conditions   | Negative prompt, character budget                          | duration cap                          |
| Output            | Quality and format suffix when the modality supports it     | asset count, format, aspect, duration |

Compilation rules:

1. Section order is fixed: task (subject and action), environment, composition, lighting, style, details, quality
   suffix, negative prompt.
2. Sections join with `", "`. Empty sections are skipped, never rendered as `", ,"`.
3. Duplicate tokens are removed case-insensitively after trimming, keeping the first occurrence.
4. Whitespace is collapsed, control characters are removed, and the result is NFC normalized.
5. Character budgets are enforced after compilation: 3500 for image, 1800 for video, and never above
   `stop_conditions.max_characters`. Over budget is a validation error, not a truncation.
6. The compiled string contains no delimiters that look like instructions to a model. User text is data, and the
   compiler never wraps it in `system:` or similar markers.
7. Output is deterministic: same document, same bytes. The only exception is an explicit `seed` the user supplied.
8. Number formatting is locale independent. A duration renders as `10 seconds`, never `10,0`.

Template, image:

```
{subject}{", " + action}{", " + environment}{", " + composition_phrase}{", " + lighting}{", " + style_phrase}{", " + details}{", high detail, sharp focus, professional composition"}{", avoid: " + negative}
```

Template, video:

```
{subject}{", " + action}{", " + environment}{", " + composition_phrase}{", " + lighting}{", " + style_phrase}{", " + motion}{", camera: " + camera_motion}{", " + duration_s + " seconds"}{", " + details}{", avoid: " + negative}
```

`composition_phrase` renders as `<shot> shot, <angle> angle, <aspect_ratio> aspect ratio`, with `_` replaced by a
space. `style_phrase` renders as `<medium>, <palette joined by " and ">, <mood> mood`.

Video fields (`motion`, `camera_motion`) arrive in `context.details` and `context.composition` under the metadata
vocabulary. A video document without a camera motion gets `static`, never a guessed move.

### 6.5 Preview and edit

Before any credit is held, the bot shows the elaboration as one message, capped at 3500 characters:

```
MASTER PROMPT
Role: senior automotive photographer
Task: hero image of an orange cat riding a skateboard through Jakarta
Context: medium shot, eye level, 16:9, golden hour, cinematic photo
Reasoning: playful subject, local street scene reads faster than a studio setup
Stop: avoid watermark, text artifacts, extra limbs; max 3500 chars
Output: 1 image, 16:9, png

[Generate image - 1 credit] [Aspect] [Style] [Edit text] [Cancel]
```

The Reasoning line is truncated to 160 characters in the preview and stored in full on the job. The user can edit the
source text, change aspect ratio, or change style, each of which rebuilds the document. Generating from the preview is
the only path that holds credits.

### 6.6 Prompt injection and abuse

- The builder system prompt is fixed and never contains user text. The user message is a JSON data envelope.
- The builder's output is data. It cannot request a tool call, a URL fetch, or a second model call, and any such
  content fails schema validation.
- A URL in any field is refused. So is a field made only of instruction phrases from the configured blocklist.
- Length caps apply before the builder, after the builder, and after compilation.
- A prompt that is only whitespace, only control characters, or only a URL is refused with `BOT_PROMPT_EMPTY`.
- Repeated attempts to break the builder are rate limited per user and logged under A09.
- The compiled prompt and the master prompt document are stored with the job. The raw Telegram message is not
  retained beyond the job row (§13.5).

### 6.7 Metadata and vocabulary

`docs/SPEC-BOT/002-MASTER-PROMPT-METADATA.md` holds the builder system prompt, the six-section contract as the model
sees it, the closed vocabulary (style terms, camera moves, negative baseline), and the per-modality enrichment rules.
The owner maintains that document; code reads the vocabulary as typed data in `internal/masterprompt/vocabulary.go`.

A vocabulary change is a spec change: same PR, same tests, and no silent recompilation of historical jobs. The
`version` field on every stored document is what makes that possible.

### 6.8 Builder model selection

`BUILDER_MODEL` is not chosen by preference. It is chosen by a benchmark that measures four things: valid JSON, schema
and semantic conformance, fidelity to the six sections, and response latency. The harness is `cmd/builderbench`, it
runs against candidate chat models already present in the gateway catalog, and its report is
`docs/SPEC-BOT/003-BUILDER-BENCHMARK.md`.

Eligibility gates, applied before any ranking:

| Gate                    | Threshold |
| ----------------------- | --------- |
| Valid JSON              | 100%      |
| Schema pass             | 98%       |
| Semantic pass           | 95%       |
| URL or brand leak       | 0         |
| Six sections present    | 100%      |

Models that pass are ranked by p95 builder latency first, then by cost per 1000 builds. The top three go to a blind
owner review of prompt quality, because latency and schema validity do not tell you whether the visual direction is
good. The owner's pick is recorded here and set through configuration.

Rules that keep the benchmark honest:

- Fixtures, not user data. The fixture set lives in the benchmark document and is reviewed like code.
- The system prompt is the fixed text from `002-MASTER-PROMPT-METADATA.md`. Tuning a model's prompt per fixture is a
  disqualification, because it measures the tuner, not the model.
- Temperature, token cap, and timeout are identical for every candidate.
- Each prompt runs three times. Latency is reported as p50 and p95 across runs, and a model whose output flips between
  runs is flagged rather than averaged.
- A retry is counted and reported separately, never folded into the first-attempt success rate.

The benchmark is repeated when a candidate model changes version, when the vocabulary version changes, or when the
builder system prompt changes.


## 7. Plans, credits, and billing
## 7. Plans, credits, and billing

### 7.1 Plan matrix (PROPOSED, owner to confirm)

| Plan       | Access        | Price / month  | Credits / month | Image cost | Video cost | Concurrency | Video |
| ---------- | ------------- | -------------- | --------------- | ---------- | ---------- | ----------- | ----- |
| `free`     | Referral only | 0              | 10 one time     | 1          | not allowed | 1           | No    |
| `basic`    | Self serve    | IDR 49,000     | 100             | 1          | 8          | 1           | Yes   |
| `premium`  | Self serve    | IDR 149,000    | 350             | 1          | 6          | 2           | Yes   |
| `enterprise` | Self serve  | IDR 499,000    | 1,200           | 1          | 5          | 4           | Yes   |

Rules that are not proposed, and therefore binding:

- Credits are integers. No fractional credit, no negative balance.
- Every paid generation holds its cost before the upstream call.
- A failure before the upstream returns settles as a refund. A failure reported by the upstream settles as a refund,
  and the refund is recorded with the upstream error code.
- A timeout is a refund only after the job is confirmed terminal or the poll budget expires. The job row keeps the
  provider task id so a late success can be reconciled instead of double charged.
- Plan limits are read from the `plans` table, not from code conditionals. A plan row edit is a behavior change and
  needs a spec note in the PR.
- Top-up packs are credits with a 30 day expiry, separate from the monthly grant. They are a v1.1 item unless the owner
  confirms them for v1.

### 7.2 Ledger

`credit_ledger` is append only and is the source of truth for balance. The balance column on `users` is a cached read,
updated in the same transaction, and a reconciliation job asserts the two agree once per hour.

Every row carries a `reason` from a closed set:

`signup_grant`, `referral_grant`, `subscription_grant`, `generation_hold`, `generation_settle`, `generation_refund`,
`topup_purchase`, `expiry`, `admin_adjustment`.

Hold, settle, and refund are three rows, never an update in place. A hold is negative, a settle converts the hold into
a debit with no second balance change, and a refund adds the amount back. The paired rows share a `reference_id` (the
job id), which makes the sequence auditable.

Atomicity: the hold is one `INSERT ... SELECT` guarded by `balance >= cost` inside a transaction with
`SELECT ... FOR UPDATE` on the user row. Two concurrent confirms cannot both pass the check. A table-driven test with
two goroutines asserts exactly one hold wins.

### 7.3 Payment rail adapter

The payment backend is the Kentang Tech Public API at `https://api-gateway.kentangtech.com`, documented at
`https://docs.kentangtech.com`. It is a B2B S2S contract, not a Telegram-native payment flow.

Adapter interface, owned by `internal/billing`:

```go
type Rail interface {
    CreateCharge(ctx context.Context, req ChargeRequest) (Charge, error)
    ConfirmCharge(ctx context.Context, orderID string) (Charge, error)
    GetCharge(ctx context.Context, orderID string) (Charge, error)
}
```

Wire contract used by the adapter, from the published OpenAPI:

| Call                            | Purpose                          | Notes                                                      |
| ------------------------------- | -------------------------------- | ---------------------------------------------------------- |
| `POST /api/v1/pg/charges`        | Create a charge for a plan       | Body: `orderId`, `amount`, `paymentMethod`, `description`, `customerRef`, `returnUrl` |
| `POST /api/v1/pg/charges/{orderId}/confirm` | Start checkout        | Returns `checkoutUrl`, `qrString`, or `vaNumber` + `vaBank` |
| `GET /api/v1/pg/charges/{orderId}` | Poll status                    | Status: `created`, `pending`, `paid`, `expired`, `failed`, `refunded` |

Authentication is S2S HMAC: `X-Client-Id`, `X-Timestamp`, `X-Nonce`, `X-Signature`, plus `Idempotency-Key` on the two
mutating calls. The signature scheme is implemented once in `internal/billing/signer.go` and covered by a table-driven
test with fixed vectors.

`orderId` is the bot's payment id, formatted `BOT-<ulid>`, matching `^[A-Za-z0-9._-]{4,50}$`. `amount` is the plan
price in IDR, integer. The provider's gross and fee fields are stored for reconciliation and shown to the user only as
the amount they pay.

Status handling:

- `paid`: grant the plan and the credits exactly once, keyed by `orderId` with a unique index. A replay is a no-op.
- `expired` or `failed`: mark the payment row, send one message, do not grant.
- `refunded`: revoke unused credits from that purchase and flag the user for review. Never claw back credits already
  spent without an admin note.

If the payment backend later exposes a callback, the bot adds `POST /internal/payments/callback` behind the same HMAC
verification, nonce replay guard, and timestamp skew check. Until then, a poller owns pending payments: every 30
seconds for the first 10 minutes, then every 5 minutes for 24 hours, then stop and mark `expired`.

### 7.4 Referral campaign

A campaign is a code with a capacity and a grant.

- `code` is uppercase, 6 to 24 characters, `^[A-Z0-9_-]+$`, unique.
- `max_redemptions` is a hard cap enforced under `SELECT ... FOR UPDATE` on the campaign row.
- `grant_credits` and `grant_plan` are copied onto the redemption and the grant is written to the ledger once.
- A user can redeem at most one campaign, ever. The unique index is `(campaign_id, user_id)` plus a global
  `users.referred_by` check.
- Self referral is refused: the referrer and the redeemer cannot be the same Telegram user.
- A user who already has a paid plan cannot redeem a free campaign.
- Code lookup is constant time against an in-memory map refreshed every 60 seconds, and the authoritative check is the
  database transaction.
- Abuse controls: one redemption per Telegram account, one per device fingerprint when a fingerprint exists, IP hash
  stored for forensics, rate limit 5 redemption attempts per hour per IP.
- The capacity counter is `redeemed_count`, incremented in the same transaction as the redemption. A campaign at
  capacity returns `BOT_REFERRAL_EXHAUSTED` and does not leak whether the code exists.

Default campaign values are `PROPOSED`: 500 redemptions, 10 credits, free plan, 90 day window.

## 8. Data model

Schema `bot` in the existing PostgreSQL instance. Migrations live in `app-bot/migrations`, numbered pairs, applied
before the service starts. No foreign keys into `app-serv` tables: the gateway is a separate bounded context.

```sql
CREATE TABLE bot.users (
    id                BIGSERIAL PRIMARY KEY,
    telegram_user_id  BIGINT NOT NULL UNIQUE,
    username          TEXT NOT NULL DEFAULT '',
    first_name        TEXT NOT NULL DEFAULT '',
    language_code     TEXT NOT NULL DEFAULT '',
    plan_code         TEXT NOT NULL DEFAULT 'free',
    credits_balance   INTEGER NOT NULL DEFAULT 0 CHECK (credits_balance >= 0),
    referred_by       BIGINT REFERENCES bot.users(id),
    status            TEXT NOT NULL DEFAULT 'active',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bot.plans (
    code              TEXT PRIMARY KEY,
    display_name      TEXT NOT NULL,
    price_idr         INTEGER NOT NULL CHECK (price_idr >= 0),
    monthly_credits   INTEGER NOT NULL CHECK (monthly_credits >= 0),
    image_cost        INTEGER NOT NULL CHECK (image_cost > 0),
    video_cost        INTEGER NOT NULL CHECK (video_cost >= 0),
    max_concurrency   INTEGER NOT NULL CHECK (max_concurrency > 0),
    video_enabled     BOOLEAN NOT NULL DEFAULT false,
    daily_jobs        INTEGER NOT NULL CHECK (daily_jobs > 0),
    queue_priority    INTEGER NOT NULL DEFAULT 100,
    active            BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE bot.subscriptions (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES bot.users(id),
    plan_code         TEXT NOT NULL REFERENCES bot.plans(code),
    status            TEXT NOT NULL,
    period_start      TIMESTAMPTZ NOT NULL,
    period_end        TIMESTAMPTZ NOT NULL,
    credits_granted   INTEGER NOT NULL,
    payment_id        BIGINT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX subscriptions_user_period_idx ON bot.subscriptions (user_id, period_end DESC);

CREATE TABLE bot.credit_ledger (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES bot.users(id),
    delta             INTEGER NOT NULL,
    reason            TEXT NOT NULL,
    reference_id      TEXT NOT NULL DEFAULT '',
    balance_after     INTEGER NOT NULL CHECK (balance_after >= 0),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX credit_ledger_user_idx ON bot.credit_ledger (user_id, id DESC);
CREATE UNIQUE INDEX credit_ledger_once_idx
    ON bot.credit_ledger (user_id, reason, reference_id)
    WHERE reason IN ('referral_grant', 'subscription_grant', 'topup_purchase');

CREATE TABLE bot.generation_jobs (
    id                UUID PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES bot.users(id),
    kind              TEXT NOT NULL CHECK (kind IN ('image', 'video')),
    status            TEXT NOT NULL,
    model             TEXT NOT NULL,
    master_prompt     JSONB NOT NULL,
    compiled_prompt   TEXT NOT NULL,
    builder           TEXT NOT NULL DEFAULT 'model',
    builder_model     TEXT NOT NULL DEFAULT '',
    provider_task_id  TEXT NOT NULL DEFAULT '',
    result_url        TEXT NOT NULL DEFAULT '',
    cost_credits      INTEGER NOT NULL CHECK (cost_credits > 0),
    error_code        TEXT NOT NULL DEFAULT '',
    telegram_message_id BIGINT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at      TIMESTAMPTZ
);
CREATE INDEX generation_jobs_user_idx ON bot.generation_jobs (user_id, created_at DESC);
CREATE INDEX generation_jobs_active_idx ON bot.generation_jobs (status, updated_at)
    WHERE status IN ('queued', 'submitted', 'running');

CREATE TABLE bot.referral_campaigns (
    id                BIGSERIAL PRIMARY KEY,
    code              TEXT NOT NULL UNIQUE CHECK (code ~ '^[A-Z0-9_-]{6,24}$'),
    name              TEXT NOT NULL,
    grant_credits     INTEGER NOT NULL CHECK (grant_credits >= 0),
    grant_plan        TEXT NOT NULL REFERENCES bot.plans(code),
    max_redemptions   INTEGER NOT NULL CHECK (max_redemptions > 0),
    redeemed_count    INTEGER NOT NULL DEFAULT 0 CHECK (redeemed_count >= 0),
    starts_at         TIMESTAMPTZ NOT NULL,
    ends_at           TIMESTAMPTZ NOT NULL,
    status            TEXT NOT NULL DEFAULT 'active',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (redeemed_count <= max_redemptions)
);

CREATE TABLE bot.referral_redemptions (
    id                BIGSERIAL PRIMARY KEY,
    campaign_id       BIGINT NOT NULL REFERENCES bot.referral_campaigns(id),
    user_id           BIGINT NOT NULL REFERENCES bot.users(id),
    ip_hash           TEXT NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (campaign_id, user_id)
);

CREATE TABLE bot.payments (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES bot.users(id),
    order_id          TEXT NOT NULL UNIQUE,
    plan_code         TEXT NOT NULL REFERENCES bot.plans(code),
    amount_idr        INTEGER NOT NULL CHECK (amount_idr > 0),
    status            TEXT NOT NULL,
    payment_method    TEXT NOT NULL DEFAULT 'qris',
    checkout_url      TEXT NOT NULL DEFAULT '',
    provider_ref      TEXT NOT NULL DEFAULT '',
    idempotency_key   TEXT NOT NULL UNIQUE,
    paid_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX payments_pending_idx ON bot.payments (status, created_at)
    WHERE status IN ('created', 'pending');

CREATE TABLE bot.admin_audit (
    id                BIGSERIAL PRIMARY KEY,
    actor             TEXT NOT NULL,
    action            TEXT NOT NULL,
    target            TEXT NOT NULL,
    detail            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Job status machine:

```
draft -> queued -> submitted -> running -> succeeded
                        |          |
                        +----------+--> failed -> refunded
draft -> cancelled
```

A job moves forward only through a compare-and-set update on `status` and `updated_at`. A late poll response for a
terminal job is ignored, not applied.

## 9. Redis keys and queues

| Key                          | Type   | TTL     | Purpose                                              |
| ---------------------------- | ------ | ------- | ---------------------------------------------------- |
| `bot:upd:{update_id}`        | string | 7 days  | Duplicate update suppression (`SET NX`)              |
| `bot:sess:{telegram_id}`     | hash   | 1 hour  | Modality, draft id, last job id                      |
| `bot:lock:chat:{telegram_id}`| string | 30 s    | Per chat serialization of work items                 |
| `bot:rl:{scope}:{id}:{min}`  | string | 2 min   | Rate limit counters (INCR + EXPIRE)                  |
| `bot:stream:updates`         | stream | none    | Work queue, consumer group `bot-workers`             |
| `bot:stream:payments`        | stream | none    | Confirmed payments to grant                          |
| `bot:dlq:updates`            | stream | 14 days | Failed work items with reason                        |
| `bot:poll:{job_id}`          | string | 15 min  | Poll ownership lock                                  |
| `bot:nonce:{nonce}`          | string | 10 min  | Payment callback replay guard                        |
| `bot:campaigns`              | hash   | 60 s    | Code lookup cache                                    |

Stream consumer rules:

- `XADD` on intake, `XREADGROUP` in workers, `XACK` only after the work item reaches a terminal step.
- A failed item retries 3 times with exponential backoff and jitter (2 s, 8 s, 30 s), then moves to the DLQ with the
  original payload and the failure class.
- A pending entry older than 5 minutes is claimed with `XAUTOCLAIM` by a reaper, so a crashed worker does not strand
  paid work.
- The stream is trimmed by length (`MAXLEN ~ 100000`) only after the DLQ path exists.

## 10. HTTP surface

| Method | Path                              | Auth                        | Purpose                                |
| ------ | --------------------------------- | --------------------------- | -------------------------------------- |
| POST   | `/internal/telegram/webhook`      | Telegram secret token header | Update intake                          |
| GET    | `/healthz`                        | none                        | Process liveness                       |
| GET    | `/readyz`                         | none                        | PostgreSQL and Redis reachability      |
| GET    | `/metrics`                        | loopback only               | Prometheus text                        |
| POST   | `/internal/payments/callback`     | S2S HMAC                    | Optional payment callback (v1.1)       |
| POST   | `/internal/admin/campaigns`       | S2S HMAC                    | Create or update a referral campaign   |
| GET    | `/internal/admin/campaigns/{code}`| S2S HMAC                    | Campaign status and capacity           |

Webhook handler contract:

1. Reject when `X-Telegram-Bot-Api-Secret-Token` is missing or does not match, in constant time. Answer `403` with no
   body detail.
2. Reject a body over 1 MB before decoding.
3. Decode into `models.Update` from the pinned library. Unknown fields are tolerated because Telegram owns the schema.
4. Reject updates without `update_id`, and updates from a chat type other than `private`.
5. `SET NX bot:upd:{update_id}`; a duplicate answers `200` and does nothing.
6. `XADD` a compact work item: update id, chat id, user id, kind, text or callback data, message id.
7. Answer `200` in under 300 ms at p95. Errors after enqueue are worker problems, not webhook problems.

A `403` from a wrong secret is logged as a security event with the source IP and no payload.

## 11. Security

### 11.1 Threat model summary

| Asset                     | Threat                                             | Control                                                        |
| ------------------------- | -------------------------------------------------- | -------------------------------------------------------------- |
| Bot token                 | Leak through logs, error text, repository          | Env only, redaction in `logx`, gitleaks, no echo in errors      |
| Gateway key               | Reuse outside the bot                              | One key per service, scoped, rotated on schedule, never logged |
| Credit balance            | Double spend, negative balance, replay             | Row lock, hold/settle/refund ledger, unique reference index    |
| Referral capacity         | Scripted redemption, code enumeration              | Row lock, one per user, rate limit, uniform refusal            |
| Payment status            | Forged callback, replay, amount tampering          | HMAC, timestamp skew, nonce, server-side amount lookup         |
| Generated asset URL       | SSRF into metadata or internal services            | Guarded downloader, allowlist, no redirects, size cap          |
| User prompt               | Injection into compiler or upstream instruction    | Data-only compilation, caps, no instruction markers            |
| Webhook endpoint          | Update forgery                                     | Secret token, optional Telegram IP allowlist at Nginx          |
| Admin routes              | Unauthorized campaign edits                        | S2S HMAC, admin audit row                                      |

### 11.2 OWASP Top 10:2025 mapping

| Code | Category                     | Bot control                                                                                          |
| ---- | ---------------------------- | ---------------------------------------------------------------------------------------------------- |
| A01  | Broken access control        | Every job and payment is read by `user_id` from the session, never by an id in callback data alone. Callback data carries an opaque draft id that is joined to the owner. Deny by default on unknown plans and unknown callback actions. |
| A02  | Security misconfiguration    | No debug mode in production, generic error pages, `/metrics` loopback only, security headers at Nginx, no default credentials, config validated at boot. |
| A03  | Supply chain                 | `go.sum` committed, exact versions, `govulncheck` and `gitleaks` gates, no `latest` tags, minimal dependency set. |
| A04  | Cryptographic failures       | TLS only for webhooks and upstream, HMAC-SHA256 for S2S, CSPRNG (`crypto/rand`) for ids and nonces, no secrets in source. |
| A05  | Injection                    | Parameterized SQL only, no dynamic identifiers from user input, prompt compiled as data, HTML never rendered from user text. |
| A06  | Insecure design              | Threat model in this section, rate limits by design, per-plan quotas, hold before spend, abuse cases tested. |
| A07  | Authentication failures      | Telegram identity only, no password surface; admin and payment routes use HMAC with replay protection; session state in Redis keyed by chat. |
| A08  | Data integrity               | Typed JSON decode with `DisallowUnknownFields` for internal payloads, JSONB schema validation before use, signed payment calls. |
| A09  | Logging and alerting         | Security events logged with user id hash, source IP, action, outcome; alerts on webhook auth failures, refund spikes, ledger mismatch. |
| A10  | Mishandling exceptions       | Fail closed on unknown plan, unknown status, missing balance, failed HMAC; every goroutine recovers and logs; refunds only on explicit terminal failure. |

### 11.3 Egress and SSRF

All outbound HTTP goes through `internal/netguard`, wired once:

1. HTTPS only for asset download and payment. HTTP is refused.
2. Host allowlist: the gateway base URL, the payment base URL, and the CDN hosts the gateway returns. The allowlist is
   configuration, not a user field.
3. Resolve once, validate every resolved address, connect to the validated IP. Any private, loopback, link-local,
   CGNAT, multicast, or reserved address refuses the whole name (`SSRF.md` §2.3).
4. No redirect following. A `30x` is a failure.
5. Connect timeout 5 s, total timeout 60 s for images and 120 s for video downloads, max body 10 MB and 50 MB.
6. Content type must be in an allowlist (`image/png`, `image/jpeg`, `image/webp`, `video/mp4`, `video/webm`). A
   mismatch refuses the asset and refunds the job.
7. The raw upstream body is never reflected to the user. It is uploaded to Telegram from a bounded buffer.

A table-driven test mirrors the SSRF.md table: public URL allowed, loopback blocked, metadata blocked, private range
blocked, obfuscated IP blocked, non-HTTP scheme blocked, redirect blocked.

### 11.4 Secrets and configuration

- `BOT_TOKEN`, `PAYMENT_CLIENT_SECRET`, `GATEWAY_KEY`, and `DATABASE_URL` come from the environment or a systemd
  `EnvironmentFile` with mode `0600`. No secret in `go.mod`, tests, or documentation.
- `logx` redacts keys matching `token`, `secret`, `key`, `authorization`, `signature` before writing.
- A missing required variable fails boot with one message naming the variable, not a stack trace.
- Test fixtures use obviously fake values and never a live token.

## 12. Reliability

### 12.1 Timeouts and retries

| Call                       | Timeout | Retry                                        |
| -------------------------- | ------- | -------------------------------------------- |
| Telegram `sendMessage`     | 10 s    | 3 attempts, 1 s / 4 s / 10 s, then DLQ       |
| Telegram `sendPhoto`       | 30 s    | 2 attempts, no retry on 400 class             |
| Telegram `sendVideo`       | 120 s   | 2 attempts, no retry on 400 class             |
| `app-serv` image create    | 120 s   | 1 retry only on connection reset, same draft |
| `app-serv` video create    | 30 s    | 1 retry only on connection reset, same draft |
| `app-serv` video poll      | 15 s    | Continue polling, never create a second job   |
| Payment create/confirm     | 20 s    | 2 attempts with the same `Idempotency-Key`    |

Every outbound request takes a `context` with a deadline. `context.Background()` is never passed to a client
(`AGENTS.md` §1.6).

### 12.2 Idempotency

- Telegram updates: `update_id` dedupe.
- Generation create: one upstream call per job id, guarded by a job status compare-and-set from `draft` to `queued`.
- Payment create: `orderId` plus `Idempotency-Key`.
- Referral grant: unique ledger index on `(user_id, reason, reference_id)`.
- Subscription grant: unique index on `payments.order_id` and one grant row per payment.

### 12.3 Concurrency

- Webhook handlers: bounded by Nginx and the Go server's own limit, 64 in flight.
- Stream workers: 3 goroutines, each processing one update at a time.
- Video pollers: one goroutine per active job, capped at 20 globally, with a semaphore.
- PostgreSQL pool: max 10 open, 5 idle, 30 minute lifetime.
- Redis pool: 10.
- Per user: `max_concurrency` from the plan, enforced by counting active jobs in SQL before submit.
- Every goroutine recovers from panic and logs the stack (`AGENTS.md` §1.6). A panic in a poller marks the job failed
  and refunds, it does not leave the hold open.

### 12.4 Shutdown

`SIGTERM` stops accepting webhooks, drains in-flight handlers for 10 s, stops the stream consumer after the current
item, and releases locks. A video poller interrupted by shutdown leaves the job `running`; the reaper picks it up on
next boot by `updated_at` older than 2 minutes.

## 13. Observability

### 13.1 Logs

Structured JSON through `log/slog`. Required fields on every line: `service`, `env`, `request_id`, `update_id` when
present, `job_id` when present, `user_hash` (first 16 hex of SHA-256 of the Telegram id), `event`, `outcome`,
`duration_ms`. Prompt text is not logged at info level; at debug it is truncated to 80 runes and only in non-production.

### 13.2 Metrics

| Metric                                  | Type      | Labels                          |
| --------------------------------------- | --------- | ------------------------------- |
| `bot_updates_total`                     | counter   | `result` (accepted, duplicate, rejected) |
| `bot_webhook_duration_seconds`          | histogram | none                            |
| `bot_jobs_total`                        | counter   | `kind`, `status`                |
| `bot_job_duration_seconds`              | histogram | `kind`                          |
| `bot_credits_held`                      | gauge     | `plan`                          |
| `bot_refunds_total`                     | counter   | `reason`                        |
| `bot_telegram_send_errors_total`        | counter   | `method`, `class`               |
| `bot_payment_status_total`              | counter   | `status`                        |
| `bot_queue_depth`                       | gauge     | `stream`                        |
| `bot_ledger_mismatch_total`             | counter   | none                            |

### 13.3 Alerts

- Webhook auth failures above 10 per minute.
- Queue depth above 500 for 5 minutes.
- DLQ growth above 20 in 10 minutes.
- Refund rate above 10 percent of jobs in 15 minutes.
- Ledger mismatch above zero.
- Payment poller unable to reach the rail for 10 minutes.

## 14. Configuration

| Variable                     | Required | Default            | Notes                                        |
| ---------------------------- | -------- | ------------------ | -------------------------------------------- |
| `APP_ENV`                    | yes      | none               | `development`, `staging`, `production`        |
| `HTTP_ADDR`                  | no       | `127.0.0.1:8090`   | Loopback behind Nginx                        |
| `DATABASE_URL`               | yes      | none               | `postgres://bot:...@127.0.0.1:5432/pannelai?search_path=bot` |
| `DB_POOL_MAX`                | no       | `10`               | Explicit, never library default               |
| `REDIS_ADDR`                 | yes      | none               | `127.0.0.1:6379`                             |
| `REDIS_PASSWORD`             | yes      | none               | Required by the host                          |
| `TELEGRAM_BOT_TOKEN`         | yes      | none               | Secret                                       |
| `TELEGRAM_WEBHOOK_SECRET`    | yes      | none               | 1 to 256 chars, `A-Za-z0-9_-`                |
| `TELEGRAM_API_BASE`          | no       | `https://api.telegram.org` | Override only for tests              |
| `GATEWAY_BASE_URL`           | yes      | none               | `https://app-router.kentang.ai.id`           |
| `GATEWAY_KEY`                | yes      | none               | Bearer for the data plane                    |
| `IMAGE_MODEL`                | yes      | none               | Resolved id, for example `openai/gpt-image-1` |
| `VIDEO_MODEL`                | yes      | none               | Resolved id, for example `google/veo-3.1`    |
| `IMAGE_CREDITS`              | no       | `1`                | Overridden per plan row                      |
| `VIDEO_CREDITS`              | no       | `8`                | Overridden per plan row                      |
| `FREE_REFERRAL_CREDITS`      | no       | `10`               | Campaign default only                        |
| `PROMPT_MAX_CHARS`           | no       | `2000`             | Pre-builder cap                              |
| `BUILDER_MODEL`              | yes      | none               | Chat model for the six-section builder        |
| `BUILDER_MODEL_CANDIDATES`   | no       | none               | Comma separated models for `cmd/builderbench`  |
| `BUILDER_MAX_TOKENS`         | no       | `700`              | Output cap per attempt                        |
| `BUILDER_TIMEOUT`            | no       | `20s`              | One attempt                                   |
| `BUILDER_DAILY_CAP`          | no       | `60`               | Builder calls per user per day                |
| `IMAGE_MAX_BYTES`            | no       | `10485760`         | 10 MB                                        |
| `VIDEO_MAX_BYTES`            | no       | `52428800`         | 50 MB                                        |
| `VIDEO_POLL_BUDGET`          | no       | `15m`              | Terminal after this                          |
| `WORKER_COUNT`               | no       | `3`                | Stream workers                               |
| `POLL_CONCURRENCY`           | no       | `20`               | Active video pollers                         |
| `PAYMENT_BASE_URL`           | yes      | none               | `https://api-gateway.kentangtech.com`        |
| `PAYMENT_CLIENT_ID`          | yes      | none               | S2S client id                                |
| `PAYMENT_CLIENT_SECRET`      | yes      | none               | S2S secret                                   |
| `PAYMENT_MERCHANT_ID`        | no       | none               | When the rail requires it                    |
| `EGRESS_ALLOWED_HOSTS`       | yes      | none               | Comma separated allowlist                    |
| `LOG_LEVEL`                  | no       | `info`             | `debug` refused in production                |

The config struct is validated once at boot. A missing or malformed value stops the process with a single English
message naming the field.

## 15. Testing (TDD contract)

All test functions are table-driven with 3 to 5 cases, following `docs/RULLES/TDD.md` §2.5. Names state given, when,
then. No `t.Skip`, no `-run` filter in CI, `go test -race ./...` is a gate for every package with concurrency.

| Package                | Test focus                                                                         | Cases |
| ---------------------- | ---------------------------------------------------------------------------------- | ----- |
| `masterprompt`         | Compile golden output; empty sections; duplicate tokens; budget overflow; unknown field; injection markers kept as data | 6 |
| `masterprompt`         | Builder decode: valid document; unknown field; missing section; aspect mismatch; URL field; over-length reasoning | 6 |
| `masterprompt`         | Fallback builder: aspect tokens, style vocabulary, duration words, no match, builder unavailable | 5 |
| `masterprompt`         | Preview render: full document; truncated reasoning; fallback label; over 3500 chars refused | 4 |
| `builderbench`         | Scoring: gate pass, gate fail, flip detection, retry counting, p95 across runs      | 5 |
| `billing/plans`        | Entitlement by plan; free cannot video; unknown plan denied; enterprise concurrency  | 5 |
| `billing/credits`      | Hold, settle, refund; insufficient balance; concurrent hold; ledger uniqueness       | 6 |
| `referral`             | Valid code; exhausted; expired; self referral; second redemption; case folding       | 6 |
| `handler/webhook`      | Missing secret; wrong secret; valid update; duplicate update; oversized body; bad JSON | 6 |
| `handler/webhook`      | Group chat rejected; callback without owner rejected                                 | 4 |
| `media/client`         | Image success; upstream 4xx mapped; 5xx retried once; timeout refunds; b64 response  | 5 |
| `media/downloader`     | Public allowed; loopback blocked; metadata blocked; private blocked; obfuscated IP; redirect blocked; oversize; wrong MIME | 8 |
| `worker/poll`          | Queued to running; running to succeeded; failed refunds; budget timeout; late success ignored | 5 |
| `telegram`             | Send retry classes; 429 retry after; 400 no retry; message id capture                | 5 |
| `billing/payment`      | Signer vectors; wrong signature rejected; nonce replay; stale timestamp; paid grants once | 6 |
| `repository/postgres`  | Hold transactional; job CAS; referral capacity lock; payment grant once              | 5 |
| `config`               | Missing required; malformed int; production debug refused; defaults applied          | 5 |

Integration tests that need PostgreSQL use a dedicated `bot_test` database, guarded by `TEST_DATABASE_URL`. A missing
variable fails the integration suite with a clear message rather than silently skipping.

Definition of done for a phase: tests written first and failing for the right reason, implementation green, `go vet`
clean, `staticcheck` clean, `go test -race` clean, `scrypts/gates/antislop.sh` clean, and the phase's acceptance items
in §18 checked off in the PR body.

## 16. Deployment

### 16.1 Host layout

```
/etc/app-bot/app-bot.env          # mode 0600, owner appbot
/usr/local/bin/app-bot            # static binary
/etc/systemd/system/app-bot.service
/etc/nginx/sites-available/app-bot.kentang.ai.id
```

Systemd unit essentials: `User=appbot`, `NoNewPrivileges=yes`, `ProtectSystem=strict`,
`ProtectHome=yes`, `PrivateTmp=yes`, `ReadWritePaths=/var/lib/app-bot`, `Restart=on-failure`,
`RestartSec=3`, `EnvironmentFile=/etc/app-bot/app-bot.env`, `ExecStart=/usr/local/bin/app-bot`.

### 16.2 Nginx

`app-bot.kentang.ai.id` proxies to `127.0.0.1:8090`, TLS from certbot, `client_max_body_size 1m`,
`proxy_read_timeout 15s`, and an optional Telegram IP allowlist for `149.154.160.0/20` and `91.108.4.0/22`
on the webhook path only.

### 16.3 Webhook registration

`setWebhook` with `url=https://app-bot.kentang.ai.id/internal/telegram/webhook`,
`secret_token` from the env, `allowed_updates=["message","callback_query"]`, `drop_pending_updates=false`.
The command is run once per environment and recorded in the runbook. A `getWebhookInfo` check is part of the smoke
test.

### 16.4 Rollout order

1. Create the database and role, run migrations.
2. Seed plan rows and the first campaign with SQL, reviewed like code.
3. Deploy the binary, start the unit, check `/readyz`.
4. Register the webhook.
5. Send `/start` from an owner account and run one image, one video, one referral, one payment in sandbox.
6. Enable the public campaign.

## 17. Delivery phases

Each phase is one PR into `development`, reviewed before merge. Branch names follow the repo convention.

| Phase | Branch                          | Content                                                              | Exit criteria                                  |
| ----- | ------------------------------- | -------------------------------------------------------------------- | ---------------------------------------------- |
| P0    | `feat(app-bot)-spec`            | This document                                                        | Owner approves scope and the PROPOSED numbers  |
| P1    | `feat(app-bot)-fase-1`          | Module, config, logx, router, health, migration 001, CI gates         | Binary boots, `/readyz` green, tests green      |
| P2    | `feat(app-bot)-fase-2`          | Webhook intake, dedupe, stream, worker skeleton, Telegram send client | Duplicate update test, ack under 300 ms        |
| P3    | `feat(app-bot)-fase-3`          | Master prompt schema, builder call, fallback, compiler, preview keyboard | Golden tests, builder validation tests, preview flow |
| P3b   | `feat(app-bot)-fase-3b`         | `cmd/builderbench`, fixture set, report, owner pick                  | Report with gates, p95, cost, and a chosen model |
| P4    | `feat(app-bot)-fase-4`          | Image generation, download, upload, ledger hold and settle            | One real image end to end with refund path     |
| P5    | `feat(app-bot)-fase-5`          | Video generation, poller, progress edits, timeout refund              | One real video end to end, late success handled |
| P6    | `feat(app-bot)-fase-6`          | Plan rows, entitlements, concurrency, `/plan`, `/credits`, `/history` | Plan gating tests, concurrency test            |
| P7    | `feat(app-bot)-fase-7`          | Referral campaigns, redemption, `/referral`, capacity lock            | Capacity, self referral, replay tests          |
| P8    | `feat(app-bot)-fase-8`          | Payment adapter, charge, confirm, poller, subscription grant          | Sandbox payment grants once, expiry path       |
| P9    | `feat(app-bot)-fase-9`          | Hardening, load test, alerts, runbook, deployment                    | Load report, alert drill, deployed and verified |

## 18. Acceptance criteria

1. A new user can start the bot, redeem a valid referral, and generate one image within the free grant.
2. The same Telegram update delivered twice produces one generation and one ledger hold.
3. Two concurrent confirms on one balance produce one hold and one `BOT_INSUFFICIENT_CREDITS`.
4. An image job that fails upstream refunds the hold with a ledger pair and a user message carrying the error code.
5. A video job that exceeds the poll budget is marked failed, refunded, and its provider task id remains queryable.
6. A paid plan grants credits exactly once per payment, including when the poll response is replayed.
7. An exhausted referral campaign refuses uniformly, without revealing whether the code exists.
8. Every outbound download refuses loopback, private, link-local, metadata, obfuscated, and redirect targets.
9. The webhook rejects a wrong secret, a missing secret, an oversized body, and a group chat update.
10. `go test -race ./...`, `go vet`, `staticcheck`, `govulncheck`, and the antislop gate are clean on the phase branch.
11. The production webhook answers at p95 under 300 ms as measured by `bot_webhook_duration_seconds`.
12. A runbook exists with deploy, rollback, webhook re-registration, and ledger reconciliation steps.
13. `docs/SPEC-BOT/003-BUILDER-BENCHMARK.md` holds a completed run, every candidate passes the eligibility gates or
    is excluded with the failed gate named, and `BUILDER_MODEL` matches the owner's recorded pick.

## 19. Open decisions

| # | Decision                                                                 | Owner input needed | Default until answered                          |
| - | ------------------------------------------------------------------------ | ------------------ | ----------------------------------------------- |
| 1 | Master prompt vocabulary per modality (six-section concept received 2026-10-21) | Owner extends 002 | Seed vocabulary in `002-MASTER-PROMPT-METADATA.md` |
| 2 | Final pricing and monthly credits                                         | Owner              | PROPOSED table in §7.1                          |
| 3 | Free campaign capacity and window                                         | Owner              | 500 redemptions, 10 credits, 90 days            |
| 4 | Bot hostname and DNS                                                      | Owner              | `app-bot.kentang.ai.id`                         |
| 5 | Bot token and Telegram account ownership                                  | Owner              | Placeholder, never committed                    |
| 6 | Payment client id and secret, merchant id                                 | Owner              | Sandbox credentials requested from the rail     |
| 7 | Image and video model ids per plan                                        | Owner              | One model per modality, configured globally     |
| 8 | Whether free plan may generate video                                      | Owner              | No                                              |
| 9 | Prompt retention window                                                   | Owner              | 30 days for job rows, raw message not stored    |
| 10 | Top-up packs in v1 or v1.1                                               | Owner              | v1.1                                            |
| 11 | Builder model and fallback model                                          | Benchmark P3b      | Candidates from the gateway catalog, benchmarked |

## 20. Change log

| Date       | Change                                        |
| ---------- | --------------------------------------------- |
| 2026-10-21 | Initial draft, spec first, no implementation. |
| 2026-10-21 | Master prompt reworked to the owner's six-section builder (Role, Task, Context, Reasoning, Stop conditions, Output), with an LLM builder stage and a deterministic fallback. |
| 2026-10-21 | Builder model selection made a benchmarked phase with eligibility gates and a blind owner review. |

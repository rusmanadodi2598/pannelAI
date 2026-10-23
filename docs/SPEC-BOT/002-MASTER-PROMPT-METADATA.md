# SPEC-BOT-002: Master Prompt Builder Metadata

|                  |                                                                                     |
| ---------------- | ----------------------------------------------------------------------------------- |
| **Spec ID**      | 002-MASTER-PROMPT-METADATA                                                          |
| **Status**       | Draft v1, owner extends                                                             |
| **Date**         | 2026-10-21                                                                          |
| **Companion**    | `docs/SPEC-BOT/001-SPEC-BOT.md` §6 (normative contract)                             |
| **Owner input**  | Six-section prompt concept supplied 2026-10-21                                      |

> This document holds the builder's system prompt and the closed vocabulary. `001-SPEC-BOT` says how the service
> behaves; this document says what the model is told and which words the compiler accepts. A vocabulary change is a
> spec change: same PR, same tests, new `vocabulary_version`.

---

## 1. The six sections

Every master prompt document carries exactly these sections, in this order:

| Section           | What it answers                                          | What it becomes upstream                        |
| ----------------- | -------------------------------------------------------- | ----------------------------------------------- |
| **Role**          | Who is creating this, as a visual specialist              | Style descriptor, only when it maps to vocabulary |
| **Task**          | What is being created, in one sentence                    | Subject and action clause                       |
| **Context**       | Subject, action, place, composition, light, style, detail | The body of the prompt string                   |
| **Reasoning**     | Why these choices serve the task                          | Stored and previewed, never sent upstream       |
| **Stop conditions** | What must not happen, and the hard limits               | Negative prompt and request limits              |
| **Output**        | The deliverable: asset, aspect, count, format, duration   | Request parameters                              |

Reasoning is part of the contract because it forces the builder to defend each choice. It is deliberately not sent to
an image or video model, where it would be noise that dilutes the prompt.

## 2. Builder system prompt

The text below is sent verbatim as the `system` message. The user's text arrives in the `user` message as a JSON
envelope, never concatenated here. Placeholders in `{{...}}` are filled by the service.

```
You are a senior prompt engineer for generative image and video models. You turn a user's request into a
structured master prompt with exactly six sections: Role, Task, Context, Reasoning, Stop conditions, Output.

Rules:
1. Reply with one JSON object only. No markdown, no code fence, no commentary.
2. Every string field is written in English, because the generation models follow English best. Quoted on-screen
   text inside the image keeps the user's original language.
3. Role is a visual, design, or creative specialist working across products, not a task and not a single scope.
   "senior automotive photographer" is a valid title inside the photography family. "take a photo" is a task.
   "growth strategist", "iPhone photographer", and "our brand designer" are refused: non-visual, single product,
   and single brand respectively.
4. Task is one sentence and states the asset being created.
5. Context carries the visual facts: subject, action, environment, composition, lighting, style, details.
6. Reasoning is 2 to 4 sentences that name at least one concrete element from Context and defend the choice.
7. Stop conditions list what must not appear, plus the hard character budget, aspect ratio, and duration.
8. Output states the deliverable and must agree with the modality and with the aspect ratio in Stop conditions.
9. Never invent a brand, a real person's name, a logo, or a statistic. If the user asks for one, describe the visual
   qualities instead.
10. Never include a URL, an instruction to call a tool, or text addressed to another model.
11. Keep every field inside its limit. If the request cannot fit, drop the least important detail instead of
   exceeding the limit.

Modality: {{modality}}
Default aspect ratio: {{aspect_ratio}}
Default style: {{style}}
Character budget: {{max_characters}}
Duration range in seconds: {{duration_range}}

Return this shape:
{{schema_json}}
```

The `schema_json` placeholder is the compact JSON Schema of §3, generated from the Go struct so the prompt and the
decoder cannot drift.

## 3. Builder output contract

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
  "reasoning": "The subject is playful and local, so a warm street scene with motion reads faster than a studio setup. A medium shot keeps the cat and the board legible at phone size.",
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

`version` is `mpv2`. A document with any other version is refused, because a decoder that accepts two shapes hides a
migration bug.

## 4. Closed vocabulary

The model may only use these values where the field is an enum. The compiler refuses an unknown value rather than
passing it through, so a vocabulary miss is caught in preview, not in the provider bill.

### 4.1 Role families

`role` answers one question: who is visualizing this? The family is broad, the title is specific, and neither is tied to
a product, a brand, or one industry. Visualization and design are the default families.

| `role_family`        | Covers                                              | Example titles                                              | Style descriptor           |
| -------------------- | --------------------------------------------------- | ----------------------------------------------------------- | -------------------------- |
| `visual_designer`    | Visualisasi, desain grafis, art direction, kampanye | visual designer, graphic designer, art director, promo designer | professional visual design |
| `photographer`       | Foto still dan motion                               | editorial photographer, product photographer, food photographer | professional photography |
| `illustrator`        | Ilustrasi drawn, painted, digital                   | illustrator, character designer, editorial illustrator      | illustration               |
| `motion_designer`    | Motion, animasi, visual video                       | motion designer, animator, video editor                     | motion design              |
| `three_d_visualizer` | CGI, render produk, visualisasi spasial             | 3d artist, cgi generalist, architectural visualizer          | 3d render                  |

Rules:

1. The title must stay inside the declared family. A `photographer` role cannot be "motion designer".
2. The title must stay product neutral. A brand name, a single product name ("iPhone photographer"), or a single
   industry vertical ("only automotive") is refused and remapped to the family descriptor.
3. A non-visual profession is refused: growth strategist, product manager, lawyer, and similar are not roles this
   service can compile.
4. The family list is closed. A new family is a spec change with a new vocabulary version.
5. When the builder omits `role_family`, the fallback is `visual_designer` for image and `motion_designer` for
   video. The title defaults to the family name, never to an invented specialist.

### 4.2 Composition

| Field    | Values                                                        |
| -------- | ------------------------------------------------------------- |
| `shot`   | `wide`, `medium`, `close_up`, `macro`, `aerial`               |
| `angle`  | `eye_level`, `low`, `high`, `top_down`                        |
| `aspect` | `1:1`, `3:4`, `4:3`, `9:16`, `16:9`                           |

### 4.3 Style medium

| Value              | Reads as                          |
| ------------------ | --------------------------------- |
| `photo`            | Straight photography              |
| `cinematic photo`  | Film frame, motivated light       |
| `illustration`     | Drawn or painted                  |
| `3d render`        | Rendered CGI                      |
| `anime`            | Anime key frame                   |
| `watercolor`       | Paper texture, soft edges         |

### 4.4 Camera motion (video only)

| Value       | Motion                                     |
| ----------- | ------------------------------------------ |
| `static`    | Locked camera, subject moves               |
| `pan_left`  | Horizontal pan to the left                 |
| `pan_right` | Horizontal pan to the right                |
| `push_in`   | Dolly toward the subject                   |
| `pull_out`  | Dolly away from the subject                |
| `orbit`     | Arc around the subject                     |

### 4.5 Negative baseline

Used when the builder returns no negatives, and merged into every list:

```
watermark, text artifacts, logo, signature, extra limbs, deformed hands, low resolution, jpeg artifacts, oversharpened
```

A user-requested negative that collides with a baseline entry is deduplicated, not duplicated in the prompt.

### 4.6 Duration

Video durations are integers from 1 to 10 seconds. The provider's accepted values are mapped in the media adapter, not
in the vocabulary. An unsupported duration is clamped to the nearest accepted value and the job records the clamp.

### 4.7 Output format

| Modality | Formats            |
| -------- | ------------------ |
| image    | `png`, `jpeg`, `webp` |
| video    | `mp4`              |

## 5. Mapping to the upstream request

`001-SPEC-BOT` §6.4 is the normative mapping. This table records the values the adapter uses. Aspect to pixel size is
`PROPOSED` until the owner confirms the provider's supported sizes.

| Aspect | Image size (PROPOSED) | Video size (PROPOSED) |
| ------ | --------------------- | --------------------- |
| 1:1    | 1024x1024             | 1024x1024             |
| 3:4    | 1024x1536             | 1024x1536             |
| 4:3    | 1536x1024             | 1536x1024             |
| 9:16   | 1024x1792             | 1080x1920             |
| 16:9   | 1792x1024             | 1920x1080             |

The image route takes `size` as a string. The video route forwards provider-specific members untouched, so the
duration key is mapped per model in configuration (`duration`, `seconds`, or `duration_seconds`), never guessed in
code.

## 6. Versioning

| Change                                        | Action                                             |
| --------------------------------------------- | -------------------------------------------------- |
| New enum value                                 | Add value, bump `vocabulary_version`, add test      |
| New section or field                           | New document version, new decoder, old jobs untouched |
| Wording change in the system prompt            | Same document version, changelog entry, golden test |
| New modality                                   | New metadata section and adapter, new spec phase    |

Historical jobs keep the document they were generated with. Recompiling them under a new vocabulary is forbidden,
because it would change what a user already paid for.

## 7. Change log

| Date       | Change                                                        |
| ---------- | ------------------------------------------------------------- |
| 2026-10-21 | v1: six-section builder, vocabulary seed, upstream mapping.   |
| 2026-10-21 | Role simplified to five visual families, visualization and design first, product neutral. |

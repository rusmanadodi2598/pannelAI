# DESIGN.md: pannelAI Panel (`app-ui`)

|                 |                                                                                      |
| --------------- | ------------------------------------------------------------------------------------ |
| **Status**      | Active. Replaces the "draft without direction" placeholder recorded in SPEC-UI §9.1. |
| **Owner**       | Dodi Rusmana <rusmanadodi@kentangtech.com>                                           |
| **Written**     | 2026-09-17                                                                           |
| **Applies to**  | `app-ui/**` only. `app-serv` has no rendered surface.                                |
| **Consumed by** | `docs/SPEC-UI/001-SPEC-UI.md` §9, `src/app.css`, `tests/tokens/contrast.test.ts`     |

This file is the panel's style direction. SPEC-UI §9.1 recorded that no direction existed, so every
styling detail there was a placeholder. This document replaces that state: the palette, the typeface, the
radius scale, and the identity motif are decided here, and a screen built afterwards is a deliverable
rather than a draft.

Where this file and SPEC-UI disagree about a visual value, this file wins. Where they disagree about a
behaviour, an endpoint, or a route, SPEC-UI wins: this file holds design direction, not product scope.

---

## 1. Identity

| Question  | Answer                                                                                                                                          |
| --------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| Product   | pannelAI, an AI gateway control panel for one operator or a small team                                                                          |
| Audience  | A technical operator who reads tables all day: routing health, usage, quota, logs                                                               |
| Tone      | Quiet, dense, factual. The panel reports; it does not sell.                                                                                     |
| Reference | The legacy 9Router panel is the structural reference (owner direction, 2026-09-17). Structure and density are inherited; its decoration is not. |
| Promise   | One accent, warm neutrals, and every control backed by a real endpoint. A figure on screen is a figure the API returned.                        |

The panel is an instrument. An instrument earns trust by being legible and by never lying, so the design
spends its budget on hierarchy and density rather than on ornament.

---

## 2. Design Read

> Reading this as: **operator panel** for **a technical gateway operator**, in a **warm-neutral
> instrument** style with one coral accent, dial **ENERGY 1 / RHYTHM 2 / MOTION 1**.

### 2.1 Dials

| Dial       | Value | Reason                                                                                                                                                                                                                                                                                                                                |
| ---------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **ENERGY** | 1     | An instrument states its values and stops. Density is the point, not decoration.                                                                                                                                                                                                                                                      |
| **RHYTHM** | 2     | The shell is deliberately uniform so muscle memory holds across screens, while the sidebar's five task groups and the screen bodies vary in composition. Uniform shell plus varied content is the point of a control panel. A flat 1 would make every screen one undifferentiated strip, and a 3 would cost the operator orientation. |
| **MOTION** | 1     | Hover, focus, and state transition only, plus the mobile drawer slide. No scroll choreography on a screen an operator opens twenty times a day. One stated exception: the live routing indicator moves while a provider is routing (a pulse, the beam, a soft glow, a pulsing gateway), a state that ends rather than a loop (§6.5).  |

---

## 3. Palette

One accent, warm neutrals, and three status colours. Neutrals do not count toward the R-29 cap, so the
active brand palette is **one colour**.

### 3.1 What changed from the legacy reference, and why

The legacy palette centres on coral `#E56A4A` (9Router `globals.css`, `--color-brand-500`). Taken
unchanged as a light-theme button fill it measures **3.23:1** against white, below the 4.5:1 floor R-25
requires, and as white-on-coral it fails the same way. The coral is therefore kept as the identity and
re-tuned per theme rather than used verbatim:

- The light theme uses `#B8412A`, the same hue one step darker, which clears **5.49:1** with white.
- The dark theme keeps `#E56A4A` and pairs it with a near-black label instead of white, which clears
  **5.85:1**. A near-black label on a bright coral is also the treatment that keeps the accent from
  glowing on a dark surface (R-13).

The legacy surfaces are also re-tuned. The reference light background `#FDFAF6` sits only 1.02:1 from
white, so surfaces there were effectively one flat field. The tokens below separate surface levels enough
to be readable as hierarchy rather than as a tint.

### 3.2 Tokens

| Token                  | Light     | Dark      | Role                                                      |
| ---------------------- | --------- | --------- | --------------------------------------------------------- |
| `--color-surface`      | `#FCFAF7` | `#1A1917` | Page ground                                               |
| `--color-surface-2`    | `#F4F1EC` | `#24221F` | Raised panel: sidebar, card, table header                 |
| `--color-surface-3`    | `#E9E4DC` | `#302D29` | Pressed or selected row, input well                       |
| `--color-border`       | `#DFD9D0` | `#3A362F` | One hairline weight everywhere                            |
| `--color-text`         | `#1A1614` | `#EDEAE6` | Body text                                                 |
| `--color-text-muted`   | `#5F574E` | `#A69E93` | Secondary text, labels, icon rest state                   |
| `--color-accent`       | `#B8412A` | `#E56A4A` | The single accent: active nav, primary action, focus ring |
| `--color-accent-text`  | `#FFFFFF` | `#14100E` | Label on an accent fill                                   |
| `--color-ok`           | `#14743A` | `#5CC97B` | Healthy, active, passing                                  |
| `--color-warn`         | `#8F5C0A` | `#E8A93A` | Degraded, nearing a limit                                 |
| `--color-danger`       | `#B3261E` | `#F08A84` | Failed, revoked, destructive                              |
| `--color-state-hover`  | ink @ 9%  | ink @ 9%  | Hovered control on any ground (§3.5)                      |
| `--color-state-press`  | ink @ 16% | ink @ 16% | Pressed control on any ground (§3.5)                      |
| `--color-state-deep`   | ink @ 20% | ink @ 20% | Hover over a chosen control (§3.5)                      |
| `--color-accent-hover` | `#AB311C` | `#F07A5A` | Primary action under a pointer                            |
| `--color-accent-press` | `#9C2412` | `#FA8A6C` | Primary action while held                                 |
| `--color-danger-hover` | `#A11815` | `#FD9992` | Destructive control, hovered                              |
| `--color-danger-press` | `#90070C` | `#FFAAA3` | Destructive control, held                                 |
| `--color-danger-ink`   | `#FFFFFF` | `#14100E` | Label on a destructive fill                               |

The seven state tokens are declared in `src/lib/states.css`, which `src/app.css` imports. They live in
their own file for the same reason the motion tokens do: `app.css` carries the palette and the alias
layer, and it sits at the project's file-size ceiling. The contrast suites read both files.

### 3.3 Measured contrast

Every pair below is asserted by `tests/tokens/contrast.test.ts`, which reads these values from
`src/app.css` and fails the build when one regresses. Measured 2026-09-17.

| Pair                              | Light | Dark  | Floor |
| --------------------------------- | ----- | ----- | ----- |
| `text` on `surface`               | 17.25 | 14.65 | 4.5   |
| `text` on `surface-2`             | 15.95 | 13.23 | 4.5   |
| `text` on `surface-3`             | 14.20 | 11.42 | 4.5   |
| `text-muted` on `surface`         | 6.81  | 6.64  | 4.5   |
| `text-muted` on `surface-2`       | 6.30  | 5.99  | 4.5   |
| `text-muted` on `surface-3`       | 5.61  | 5.17  | 4.5   |
| `accent` on `surface`             | 5.27  | 5.43  | 4.5   |
| `accent` on `surface-2`           | 4.87  | 4.90  | 4.5   |
| `accent-text` on `accent`         | 5.49  | 5.85  | 4.5   |
| `ok` on `surface-2`               | 5.19  | 7.63  | 4.5   |
| `warn` on `surface-2`             | 5.03  | 7.68  | 4.5   |
| `danger` on `surface-2`           | 5.80  | 6.55  | 4.5   |
| `accent` on `surface-3`, non-text | 4.34  | 4.23  | 3.0   |

The lowest text pair is `accent` on `surface-2` at 4.87:1, which is why the accent is not allowed to
carry small text on a raised panel. The active navigation label uses `text`, and the accent marks the row.

### 3.4 Accent discipline

The accent appears in exactly five places, and nowhere else:

1. The active navigation row (icon plus a 3px left marker on the row).
2. The one primary action on a screen.
3. The focus ring.
4. A link.
5. The active tab underline.

It is not used for borders, background washes, icon fills across a list, chart fills beyond one series, or
status. Status has its own three colours and is never carried by the accent.

### 3.5 Interaction states

Every control that can be pressed takes four states from one scale: rest, hover, pressed, disabled. The
scale is `src/lib/states.css`; `primitives/button` is the only component that reads it into classes, and
a screen takes its controls from that component rather than writing a `hover:` utility of its own (SPEC-UI
§9.3). The rule exists because the panel had no pressed state at all: on Endpoint & Key, eight control
groups painted no colour under a click, three painted no hover either, and the only feedback a press got
was a label swap that appears once the request has started.

**A press is a colour, not only a movement.** Tailwind applies `hover:` only under
`@media (hover: hover)`, so on a touch layout a control whose sole state is a hover wash has no state.
A pressed colour is the one cue that survives both pointer types, and it is the answer to the question
this panel asks twenty times a day: did that click land. The 1px `active:` shift stays as a secondary
cue, not the primary one.

**A step is measured in lightness.** A contrast ratio cannot size a state change, because a wash and the
ground beneath it are deliberately close: the washes this scale replaced measured 1.08:1 against the page
at any usable size, and moved 3.1 L\*, which reads as a flicker. Steps are therefore sized in L\*, with the
label's ratio asserted separately so a step cannot buy its visibility with contrast.

| Ramp                                   | Rest L\* | Hover L\* | Press L\* | Step | Label on the deepest step |
| -------------------------------------- | ------- | -------- | -------- | ---- | ------------------------- |
| Neutral wash on the page surface       | 98.3    | 91.1     | 85.7     | ~6   | `text` at 12.39:1         |
| Neutral wash on a raised panel         | 95.2    | 88.2     | 82.9     | ~6   | `text` at 11.45:1         |
| Accent fill, light (darker going in)   | 44.4    | 39.5     | 34.7     | ~5   | `accent-text` at 7.85:1   |
| Accent fill, dark (lighter going in)   | 59.4    | 64.3     | 69.2     | ~5   | `accent-text` at 8.03:1   |
| Destructive fill, light                | 39.7    | 34.6     | 29.7     | ~5   | `danger-ink` at 9.45:1    |
| Destructive fill, dark                 | 68.3    | 73.4     | 77.7     | ~5   | `danger-ink` at 10.39:1   |

`tests/tokens/state-scale.test.ts` measures the step and the label for every ramp in both themes, on both
grounds a wash can land on.

**Two mechanisms, chosen by what the control is painted with.**

1. A control with no colour of its own takes an ink wash: `state-hover` and `state-press` are the theme's
   own ink at 9% and 16%, so one token takes the same perceptual step on the page, on a raised panel, and
   on a table header. An absolute hex can only ever be calibrated for one of the three.
2. A control that carries a colour carries its own ramp instead: `accent-hover`/`accent-press` for the
   primary action, `danger-hover`/`danger-press` for a destructive one. A coloured label cannot sit under
   an ink wash: the danger glyph on a pressed wash measures 4.17:1 on a raised panel, under the AA floor,
   where stepping it along the danger ramp holds 5.02:1.

**Direction follows the theme's own surface ramp.** §3.2 gives `surface-3` the pressed row, which is
darker than the page in light and lighter than it in dark. A state step moves the same way: further in is
deeper in light, brighter in dark. Both directions raise the label's contrast rather than spending it.

**A chosen control is a state, not a hover.** A filter chip that is selected sits on `surface-3`, the
ground §3.2 already calls pressed, and its hover reaches `state-deep` from there. It asks for no fourth
step: pressing a control that is already chosen changes nothing, so the 1px shift is the whole answer. The
selection is read from `aria-pressed`, which is what announces it to a screen reader, so a screen cannot
render the chosen state without also reporting it. That is also why a screen cannot pick its own ground for
a chosen control: `/quota`'s pause chip marked itself with `surface-2`, the hovered ground, so a chosen
control looked like every pointer that was merely passing over one.

**A muted glyph lifts to full text the moment it is hovered, and holds there while pressed.** That is not
only hierarchy: `text-muted` on a pressed wash measures 3.78:1 in dark, so the pairing is forbidden by the
arithmetic as well as by taste. `text-muted` stays the icon rest state (§3.2), and the state scale is what
takes it off.

**Two destructive shapes, one ramp.** A destructive action is either the commit in a confirmation dialog,
which takes the danger colour as its fill and needs a fixed label pair (`danger-ink` is to `danger` what
`accent-text` is to `accent`; the raw `text-white` it replaced measured 2.42:1 in dark), or one glyph among
four in a table row, which stays quiet because a red block on every row shouts. The quiet glyph steps its
own colour along the same ramp. Four dialogs paired that fill with `accent-text` instead of `danger-ink`,
and no contrast test caught it: the two tokens hold the same value in both themes today. The pairing belongs
to the ramp anyway, because a re-tune of the accent would move a destructive label with it. The quiet shape
is not only a glyph: `/logs`'s purge is a labelled control at the end of the filter bar, and it takes the
same ramp with a danger border, because the border is the one thing that says this one deletes while the
commit that actually deletes waits in the dialog. `/console-log`'s clear arrived from the other side of the
same rule: a neutral outline that deleted the server-side buffer on the first click. It wears the border now,
and the DELETE waits behind the confirmation §6.14 already asks for.

**A disabled control says it is disabled in one way.** Ghost, outline, and link controls fade to 50%
opacity, the panel's single value (it was 60% on the two filled buttons, which faded a label and its own
fill toward the page together and left the pair at 1.87:1). A filled button instead leaves the accent: it
takes `surface-3` with a `text-muted` label, 5.61:1 in light and 5.17:1 in dark, which is both legible and
honest about §3.4, because an action that cannot be taken no longer claims the accent for itself.

**This stays inside MOTION 1.** §2.1 allows hover, focus, and state transition; a colour step between
states is the third of those. The transition is `transition-colors` only, so no geometry enters the
screen that the dial did not ask for.

**A row-shaped control is still a control.** A section fold and a tab strip are pressed as often as a button
and were painted the same way: a hover wash on `surface-2`, nothing under a press. Both now come through
`primitives/button` and keep their identity in the class list it accepts, which is the bottom rule on a tab
and the chevron, the left-aligned label and the text colour on a fold. Routing them through the primitive
rather than writing two more class lists is what keeps the exception from becoming the rule: a screen that
has to remember to add a state is a screen that will forget one. `/token-saver`'s bypass copy control was
that shape inside a card: a hover onto `surface-2` and nothing under a press. The shared control answered it
twice over, because the one the screen wrote also had a single clipboard path, and `navigator.clipboard`
does not exist on the insecure origin this panel is usually reached at.

**A switch carries its own state.** The round-robin control is a `role="switch"` track: the thumb moves and
the track takes the accent, which is the value, not the feedback. It takes the ink wash as well, because the
finger that has not flipped it yet is still asking whether the press landed, and a hover rule gated behind
`(hover: hover)` answers nothing on a touch screen.

**An action link is a control.** A standalone link that routes the operator somewhere is pressed as often as
a button, and `/settings`' two pointer tabs were bare underlines carrying no state rule at all: nothing
painted under a press, and there was no hover to survive a touch screen either. `primitives/button` renders an
anchor when it is given `href`, so the link keeps its role and takes the scale from the same component. A
discard written as an underlined button is the same case: it is the ghost variant with `underline` in the
class list it accepts, not a second control shape a screen has to remember.

**One write band, not three.** Every settings tab ends with its commit, its unsaved marker and its outcome
line, and the three tabs each wrote that row by hand, including the `disabled:opacity-60` the ramp above
already records as the wrong fade. The band is one component now, taking the answers from the tab that owns
the draft, so the rule §6.13 states per tab is stated once. `/token-saver`'s section shell was a fourth
writer of the same row with the same wrong fade on its commit, so the component is `SaveBar` rather than a
settings file: §6.7 makes the save per section and §6.13 makes it per tab, and those are the same band.

---

## 4. Typography

| Role                        | Family                 | Size         | Weight                          |
| --------------------------- | ---------------------- | ------------ | ------------------------------- |
| Body, controls, tables      | Inter                  | 14px         | 400                             |
| Secondary and label         | Inter                  | 12px to 13px | 400 to 500                      |
| Screen title                | Inter                  | 18px         | 600                             |
| Group label in the sidebar  | Inter                  | 11px         | 600, uppercase, 0.06em tracking |
| Code, IDs, cost, log bodies | System monospace stack | 12px to 13px | 400                             |

**Typeface reason (R-06):** Inter is a neutral workhorse that holds up at 12px in dense tables and has a
tall x-height, which is what an operator reads all day. It is also the legacy reference's typeface, so
structure and density carry over unchanged (owner direction). Inter is on the default-roster list R-06
flags, and the reason above is why it is the right pick here rather than a default: legibility at small
sizes in a table, plus parity with the panel this one replaces.

Inter is **self-hosted** as a variable font at `static/fonts/InterVariable.woff2` (344 KB, one file
covering 100 to 900). SPEC-UI §11.7 forbids a font CDN, so no third-party request leaves the panel.

Letter spacing stays at the font default. Uppercase with wide tracking appears only on sidebar group
labels, where its job is to be scannable and clearly not a clickable item.

---

## 5. Radius, borders, elevation

| Token           | Value  | Used for                      |
| --------------- | ------ | ----------------------------- |
| `--radius-sm`   | 6px    | Input, button, chip, menu row |
| `--radius-md`   | 8px    | Panel, card, table container  |
| `--radius-lg`   | 12px   | Dialog, drawer                |
| `--radius-full` | 9999px | Avatar and the logo only      |

Radius is a hierarchy tool (R-11). Nothing else is a pill: no pill buttons, no pill inputs, no pill
badges beyond a real status chip whose shape is shared with every other status chip.

**Border:** one weight, 1px, `--color-border`. There is no second hairline colour and no heavy divider.
Borders measure 1.35:1 (light) and 1.46:1 (dark) against their surface, so they read as a seam, not as a
line of text.

**Elevation:** the panel is flat by default. A shadow appears on exactly two surfaces that genuinely float
above the page: a dialog and the mobile drawer. Sidebar, cards, and toolbars sit flat (R-12).

**Glass, glow, gradient:** not used anywhere. Blur, glow, and gradient are the three techniques R-10,
R-13, and R-01 keep for accents, and this panel has no surface that needs one. It is dense enough to hold
hierarchy on its own.

---

## 6. Identity motif

**The left status marker.** A 3px vertical marker on the leading edge of a navigation row or a table row,
rendered in a status colour, with the same shape everywhere it appears.

It exists because it carries information rather than decoration, which is the condition R-01 puts on a
coloured edge:

- In the sidebar, the marker is `--color-accent`, and it marks the active row.
- In a table, the marker is `ok`, `warn`, or `danger`, and it marks the row's health.

The reason it is the motif: an operator's core question is "which of these is the one that matters", and
the answer is always at the left edge. One gesture, repeated, in one place, doing one job. It is
deliberately not a coloured stripe on every card, which is the decoration pattern R-01 rejects.

The motif never appears alone. A marker is always paired with a text label, so status is never carried by
colour only (§8.9.3 of SPEC-UI, and R-25 in spirit).

---

## 7. Density and spacing

| Value | Use                                                                |
| ----- | ------------------------------------------------------------------ |
| 4px   | Icon to label gap, chip internal spacing                           |
| 8px   | Between rows in a menu group, between form field and its help text |
| 12px  | Row padding, panel padding at the compact end                      |
| 16px  | Panel padding, gaps between form fields                            |
| 24px  | Gap between page sections                                          |
| 32px  | Page edge gutter on desktop                                        |

Row height is 36px in dense tables and 44px for anything in the sidebar, which is a touch surface at every
breakpoint (R-03).

---

## 8. Responsive

| Breakpoint                 | Shell behaviour                                                                                                                                                                                                                                 |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `<768px` Mobile            | Sidebar is an off-canvas drawer over a dimmed overlay. Opens from the header control, closes on Escape, on overlay click, and on navigation. Page gutters drop to 16px, tables become stacked key-value rows.                                   |
| `768px` to `1023px` Tablet | Sidebar shows as a 64px icon rail with tooltips on hover and focus. A control expands it to full width over the content. This is the band the legacy panel left as a plain drawer, which wasted the icon-and-label space a tablet actually has. |
| `>=1024px` Desktop         | Sidebar is full width at 264px, collapsible to the same 64px rail. The collapsed state persists.                                                                                                                                                |

Shell height uses `100dvh`, not `100vh`, so the mobile browser chrome does not clip the header. The shell
itself never scrolls; the content column does. Safe-area insets pad the drawer and the header on a notched
device.

---

## 9. Theme

Light and dark are both authored from the token tables above, not derived by inversion (R-34). The system
preference decides the first paint; an explicit choice is remembered.

Dark is a legitimate default for a developer-facing tool (R-21), and the legacy panel ships a theme
provider and a toggle, so both modes are kept and both are verified per screen before that screen ships
(SPEC-UI §9.4.2).

---

## 10. Asset policy

| Asset                                                 | Value                                                                                        | Status                                         |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| Logo                                                  | `static/logo-mark.png`, the owner's mark cropped to its square emblem and masked to a circle | Real asset, supplied by the owner (2026-09-17) |
| Logo source                                           | `assets/static/logo.png`, the owner's original 1254x1254 file                                | Kept as the source of truth                    |
| Favicon                                               | `static/favicon.png`                                                                         | Derived from the same source                   |
| Icon set                                              | `@lucide/svelte`, one set, no mixing                                                         | Real                                           |
| Icons in navigation                                   | Each has a written reason in `src/lib/icons.ts`                                              | Real                                           |
| Avatars, photography, illustrations                   | None                                                                                         | Not used. The panel shows no people.           |
| Statistics, testimonials, uptime or compliance claims | None anywhere in the panel                                                                   | Forbidden (R-17, R-18, R-36)                   |

The logo's circular frame takes the elevation treatment described in §5: the emblem is masked to a circle
and the frame carries a soft ring so it reads as a physical button rather than a floating sticker. That is
the one place in the panel where a shadow is allowed on a non-floating element, and the reason is that it
is the product's face, not a control.

No emoji anywhere in UI copy, headings, empty states, or buttons. Icons are line glyphs from the single
installed set.

---

## 11. Reason log (R-31)

| Decision                                                                                                      | One-line reason                                                                                                                                                                                                                                                                            |
| ------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Coral accent kept from the legacy panel                                                                       | It is the product's existing identity, and the owner asked for structural parity.                                                                                                                                                                                                          |
| Light accent darkened to `#B8412A`                                                                            | The legacy coral fails AA as a button fill with white text; the same hue one step darker passes.                                                                                                                                                                                           |
| Dark accent keeps `#E56A4A` with a near-black label                                                           | On a dark surface a bright accent needs a dark label, and coral with near-black clears 5.85:1.                                                                                                                                                                                             |
| Warm neutrals rather than pure grey                                                                           | The source mark and the legacy panel are warm; a cold grey would fight the one accent.                                                                                                                                                                                                     |
| Inter, self-hosted                                                                                            | Legible at 12px in dense tables, which is the panel's main surface; self-hosted because a font CDN is excluded.                                                                                                                                                                            |
| One accent in five places                                                                                     | An accent stops being an accent when it is everywhere; five is the count that survives review.                                                                                                                                                                                             |
| Left status marker as the motif                                                                               | It answers the operator's recurring question, "which row matters", in the same place every time.                                                                                                                                                                                           |
| Flat surfaces, shadow only on a dialog and a drawer                                                           | Elevation that appears everywhere communicates nothing.                                                                                                                                                                                                                                    |
| No glass, glow, or gradient, except the live routing node                                                     | The panel has no surface that needs one. The exception is the node the gateway is routing to now: the owner chose parity with the reference fork, so it carries that fork’s soft glow (16px, 25% of the status colour) while a request is in flight (draft 013 F3).                        |
| The live routing line carries a moving dash                                                                   | The edge says which way traffic is going right now, and the moving dash is the reference fork’s own treatment. The pattern is normalized with pathLength because one viewBox is stretched across the box, so a raw dash would be longer on one axis (draft 013 F1).                        |
| The drawing moves only while frames arrive                                                                    | Motion is the one thing on that screen that claims "happening now", so a paused or dropped stream keeps the last known state in colour and stops every moving part (R-36, draft 013 F2).                                                                                                   |
| `100dvh` shell                                                                                                | `100vh` clips the header under mobile browser chrome.                                                                                                                                                                                                                                      |
| Tablet gets an icon rail, not a drawer                                                                        | A tablet has room for a 64px rail, which keeps navigation reachable while editing.                                                                                                                                                                                                         |
| Two radius values plus a dialog value                                                                         | Radius is a hierarchy tool; a single value everywhere erases the difference between an input and a panel.                                                                                                                                                                                  |
| Sidebar groups named by operator intent                                                                       | A flat list of fifteen items is a list, not a structure.                                                                                                                                                                                                                                   |
| Expanded sidebar width set to 264px in the primitive layer's constants                                        | §8 fixes 264px and the component layer ships 256px, so this is the one primitive edit SPEC-UI §10.7 permits, it is recorded here as that rule requires, and it is re-applied after a regeneration rather than assumed to survive one. The 64px icon rail already matches and is untouched. |
| Three savers share one section shell                                                                          | Three groups of equal weight should read as siblings, not as three unrelated forms that happen to sit on one page.                                                                                                                                                                         |
| The per-request bypass is copyable text, not a disabled control                                               | A control that cannot act is a dead control under R-26; text the operator can copy into a client is a real function.                                                                                                                                                                       |
| The native token saver engine is a Planned note, not a greyed toggle                                          | A disabled toggle promises a capability the repository does not have yet, which is the claim R-36 rules out.                                                                                                                                                                               |
| Pool test result sits in the row, with "Not tested" kept distinct from a failure                              | The operator's question is which row is broken, and a row nobody has tested is not a broken row.                                                                                                                                                                                           |
| The password cell reads "Set" or "Not set"                                                                    | The API returns `has_password` and never the secret, so presence is the only thing the cell can honestly report; a blank cell would read as no password.                                                                                                                                   |
| Batch add shows the parse before any write                                                                    | A pasted list is where the input goes wrong, so the operator sees what the panel understood before a single request is sent.                                                                                                                                                               |
| Network settings live on the Proxy Pools screen, and `/settings` links there                                  | They are proxy settings, and a second copy under Settings is the duplication the owner's KEEP list removes.                                                                                                                                                                                |
| The outbound card states that its scope is global                                                             | A control sitting beside a pool table reads as per-row routing, which the egress path does not implement.                                                                                                                                                                                  |
| The delete dialog names the object and says it does not reroute traffic                                       | The dangerous assumption is that removing a row changes routing; naming both the object and the non-effect is what makes the confirm safe to press.                                                                                                                                        |
| The media base URL field shows the registry's value beside it rather than pre-filling it                      | A field pre-filled with a value the panel did not write would save that value back as an override, changing where the address comes from without changing the address.                                                                                                                     |
| The media default model is a selector, not a text field                                                       | The service declares the models it can route, so the set is a choice; free text invites a value nothing downstream can serve.                                                                                                                                                              |
| A media kind that declares no models gets a stated fact instead of an empty selector                          | A select with no options is a dead control under R-26, and the fact is the honest shape of "there is nothing to choose".                                                                                                                                                                   |
| The media endpoint count is stated, not linked                                                                | The count is scoped to one provider and one kind, and the endpoint screen filters by provider alone, so a link would promise a narrower list than it shows.                                                                                                                                |
| The media provider name links to its registry entry                                                           | The card names a provider, and the registry entry is where its facts and its endpoint list live.                                                                                                                                                                                           |
| A sidebar row may link to a parameterised route when the row fixes the parameter                              | A Media kind is part of the row's identity rather than something an operator picks after opening the screen, so the row can build the address; a provider id is the opposite case and stays unlinkable.                                                                                    |
| Disabling lives on the catalog row and enabling lives in a second list                                        | The merged catalog hides a disabled model, so one list cannot carry both states: a state control on the catalog could only ever read "enabled", and the disabled list is the only place a disabled model can be seen or turned back on.                                                    |
| A write to the disabled set carries every provider's rows                                                     | `PUT /models/disabled` replaces the whole set, so a write built from this provider's slice would erase another provider's rows; the panel merges over the whole last-read set and renders the server's answer.                                                                             |
| A Disable press before the set has loaded is refused with a sentence, not a disabled button                   | The merge runs over the set the panel holds, so writing before the first read would send an empty set; a disabled button hides why, while the refusal names the cause and a test proves it.                                                                                                |
| The last write's outcome renders in whichever list holds the affected model                                   | The row that could not be disabled is still in the catalog and the row that was disabled is in the disabled list, so one rule puts the message beside the action that produced it, exactly once.                                                                                           |
| A disabled row shows its model id rather than a display name                                                  | The catalog cannot name a row it hides, and the id is what the API stores and keys on, so the panel shows what it can prove instead of fetching a second list to decorate the row.                                                                                                         |
| The custom model capabilities field is free text                                                              | The API accepts any value and publishes no vocabulary, so a picker would have to invent one and a value outside it would be unreachable; the hint names the two filters the panel itself offers.                                                                                           |
| The custom model removal dialog says a shadowed registry row reappears                                        | A custom row overrides a registry row with the same pair, so removing it puts the registry's version back in the catalog, and an operator who believed the model itself was deleted would not press the button.                                                                            |
| The alias set renders whole on every provider's detail screen, and says so                                    | Neither alias route takes a provider, so the set is not this provider's; a table narrowed to one provider would imply an ownership the API does not have, and one sentence is what keeps the section from reading as scoped.                                                               |
| The alias form is both the add path and the edit path                                                         | `alias` is the table's primary key, so a second row with the same name is a database error whose answer names the constraint rather than the alias; changing what an existing name targets is the only shape that can succeed, and it saves a second control.                              |
| An alias write carries every alias the panel last read                                                        | `PUT /models/aliases` replaces the whole set, so a write built from one screen's slice would delete the rest; the body is sorted the way the read route sorts, which keeps the table from reshuffling after a reload.                                                                      |
| An alias write before the set has loaded is refused with a sentence, not a disabled button                    | The merge runs over the set the panel holds, so writing first would send an empty set and clear every alias; the refusal names that cause and a test proves no request was sent.                                                                                                           |
| The alias target field suggests the catalog ids and the combo names                                           | Those are the two things the API resolves a target from, so a third suggestion would offer a value the write refuses; the field stays typable because a stored target can have left both lists since it was written.                                                                       |
| The combo delete dialog's lead sentence follows the error code                                                | Only a `CONFLICT` means an alias still references the combo, so a server failure that read "still referenced" would be a claim the panel cannot support.                                                                                                                                   |
| The combo editor offers the alias names as references                                                         | §7.7 lets a ref be a catalog id, a combo name, or an alias, so a picker offering two of the three would hide a value the API accepts.                                                                                                                                                      |
| The OAuth section renders only for a provider the registry marks `has_oauth`                                  | That flag is the API's own answer to whether the provider has a flow, and a section rendered for every provider would show an empty one on most of them.                                                                                                                                   |
| The authorize URL is a link the panel does not follow                                                         | A scripted navigation to a third party is a redirect the operator did not ask for, and a link shows the host before it is followed.                                                                                                                                                        |
| A flow the panel cannot start gets its reason instead of a disabled button                                    | A control that cannot act is a dead control under R-26, and the reason says where the connection is actually made instead of leaving the operator to guess.                                                                                                                                |
| The callback's outcome is read once and its keys are dropped from the address                                 | An address can be edited by anyone and a reload would otherwise announce a past result as if it had just happened; the sentence is held in memory so dropping the keys does not drop the report.                                                                                           |
| The failed authorization shows the gateway's sentence rather than a classified cause                          | Classifying it would mean pattern-matching the gateway's English, and the panel can attribute what the gateway said without claiming to know which class it is.                                                                                                                            |
| The API Docs screen renders the served document rather than a written-out copy of it                          | A second copy of the contract drifts the first time either side is edited, and the served document is already pinned to the registered routes by a test on the service side.                                                                                                               |
| The catalog groups itself by the document's own tags                                                          | A grouping invented on the screen would have to be edited whenever the contract adds a tag, and the document already states which operations belong together.                                                                                                                              |
| The example call is composed from the method and the path, with `<id>` where a parameter goes                 | Ninety-three examples cannot each be hand-written without drift, and an invented identifier would be a working-looking call that resolves to nothing.                                                                                                                                      |
| A block the document does not carry is stated as absent, not left blank                                       | An empty space reads as a rendering defect, while a sentence that names the missing block tells the operator the contract itself is silent there.                                                                                                                                          |
| The credential column shows the document's scheme name beside the plain-language label                        | The wire name is what an operator matches against the contract, and the label is what makes the column readable without it.                                                                                                                                                                |
| Copy controls sit beside the base URL and beside each example                                                 | The value the operator copies is the value on screen, so a copy that reached for anything else would be the panel's paraphrase of the document.                                                                                                                                            |
| The error codes render as one table per plane, sorted by status then code                                     | The contract states which plane owns which codes, and one merged table would have to invent an ordering the document does not have.                                                                                                                                                        |
| The token state is the gateway's `refresh_state`, with only the expiry told apart                             | Re-deriving the classification would be a second opinion about the gateway's own clock; the expiry is the one fact that separates "inside the window" from "already expired".                                                                                                              |
| The manual refresh is per account rather than one button for the provider                                     | The answer names the accounts it moved, and an id is what the API reads as "this one", so the operator acts on the row they can see.                                                                                                                                                       |
| The refresh re-reads the status without blanking the table                                                    | The re-read would otherwise unmount the table and the sentence reporting what the gateway just did, which is the outcome the operator asked for.                                                                                                                                           |
| The combo test asks for confirmation before probing                                                           | One click sends a one-token request per stored reference and spends upstream accounts, so the operator needs the cost and sequential behavior in view before starting it.                                                                                                                  |
| A failed combo member stays a result beside the healthy members                                               | The diagnostic question is which member is down, so turning one failure into a route error would hide the members that answered.                                                                                                                                                           |
| The combo test result is a modal with one row per reference                                                   | A table cell cannot hold identity, latency, role, and a gateway error for every member; the modal keeps the complete answer beside the row that asked for it.                                                                                                                              |
| A fusion judge is labeled separately and appears last                                                         | The judge is part of the probe chain but is not one of the combo's model references, so the role and stored order must remain visible.                                                                                                                                                     |
| The test dialog can close while probes run, but cannot cancel them                                            | The API runs the sequential loop without a cancellation contract, so a disabled close would trap the operator while a fake cancel would claim work stopped.                                                                                                                                |
| Each modal gets its own title id                                                                              | The combo screen can render a test dialog beside a delete dialog, and one fixed id would give a screen reader the first dialog's heading for both.                                                                                                                                         |
| The budget cap is a picker plus one form, not a column on the window table                                    | The collection route carries no cap, so a cap column would be one read per endpoint, and a cap belongs to an endpoint rather than to a window row.                                                                                                                                         |
| The cap section renders whether or not any window exists                                                      | A cap is legal before the first routed request, which is exactly when an operator sets a first budget; gating it on traffic would hide it then.                                                                                                                                            |
| Both cap fields stay read-only until the stored cap has been read                                             | A field typed into before the answer lands is overwritten by it, and a save would then replace a cap the operator could not see.                                                                                                                                                           |
| The read-back says no cap is stored rather than a ceiling of zero                                             | No cap and a cap of zero are opposite routing rules, so printing zero would state the one that stops the router picking the endpoint.                                                                                                                                                      |
| The form states that a blank field clears that cap                                                            | The route replaces the whole cap set, so a blank field is not "leave it alone"; an operator who read it that way would clear a budget without being told.                                                                                                                                  |
| The routing warning sits where the cap is saved                                                               | A cap changes routing rather than reporting it, and the commit happens in this form, so the consequence belongs beside the button that causes it.                                                                                                                                          |
| The zero-cost-alone rule and the bounds are mirrored from the API, with the messages computed from the bounds | The form refuses what the API refuses, one step earlier, and computing the message from the bound keeps the copy from drifting from the rule.                                                                                                                                              |
| The sentence under the form reports the read that follows a save, not the draft                               | The write replaces the set, so only a read shows what is stored; a sentence built from the draft would report what was sent instead.                                                                                                                                                       |
| The cost field accepts a narrower spelling than the API's parser                                              | The parser takes `1e9` and `1/2`, which a budget field should not accept, so the panel is deliberately the stricter of the two and §14 Q24 records the divergence.                                                                                                                         |
| The Skills screen renders the served catalog rather than §6.10's two named extras                             | The seven rows are what `GET /api/v1/skills` actually returns, so a screen built from the other list would render entries no request produces, which is the claim R-38 rules out; §6.10 was amended to the served catalog.                                                                 |
| Each source address is asked before a copy control is offered                                                 | §6.10 forbids shipping a control that copies a broken link, and the answer is knowable from the browser because the raw host allows a cross-origin read, so the panel asks instead of assuming the file is there.                                                                          |
| An unresolved source renders its cause and the address, never a copy control                                  | A copy control on a 404 is a dead control under R-26, and naming the cause tells the operator whether to retry (a timeout, a dropped connection) or to fix the path (a 404), which a bare "unavailable" would not.                                                                         |
| The install line names the raw address and the human link is the blob address                                 | The raw form is the file an agent reads directly and the blob form is the page a person reads, so two readers get two addresses and the row says which is which rather than printing one for both.                                                                                         |
| The source summary counts published rows, not answered ones                                                   | Every probe answers, so an answered count would read "7 of 7" while seven rows report a 404 and the number would never move; the count that means something is the one the reader can check by counting the rows marked available.                                                         |
| The playground key lives in the panel server and the browser is never given one                               | A key in the page is readable by any script on the origin and persists in storage if it is saved, so server-side injection removes the class of problem instead of mitigating it.                                                                                                          |
| The two playground routes sit outside `/api/v1` rather than teaching the general forwarder to inject          | The forwarder serves every management path a browser can ask for, so an injection there would attach the credential to requests the playground never intended to make.                                                                                                                     |
| The playground answers in the panel's own failure envelope                                                    | Both planes use the code name `UNAUTHORIZED`, so a forwarded 401 would read as an expired panel session and sign the operator out for a gateway key problem; the gateway's own code travels inside the panel's so the machine fact stays visible.                                          |
| The unavailable state is rendered from the sentence the panel's own route wrote                               | The bundle is not allowed to name the variable, so the one sentence that names it comes from the server; the screen renders no send control with it, because a control that cannot send is a dead control (R-26).                                                                          |
| A fact the wire did not state reads `not stated` rather than zero or blank                                    | A stream can end without a usage frame, and a zero would be a token count the gateway never reported, which is the claim R-38 rules out.                                                                                                                                                   |
| The frames are disclosed verbatim in a collapsed block                                                        | A contract divergence is diagnosable from the bytes that produced it, and a paraphrase would hide the very thing the disclosure exists to show.                                                                                                                                            |
| Three stream endings get three sentences                                                                      | Ending on the documented `[DONE]` sentinel, being stopped by the operator, and closing without the sentinel are three different facts, and only the third means the answer may be cut off.                                                                                                 |
| The reader reports a malformed tail as truncated instead of tolerating it                                     | The gateway currently glues the sentinel to the last chunk, and teaching the reader to accept that would hide a wire defect behind a panel that looked satisfied; the defect is recorded as a request to `app-serv` instead.                                                               |
| The cost sentence sits beside the send control rather than in a help block                                    | The request routes to a real provider and writes a usage record, so the price belongs where the decision is made (SPEC-UI §6.15 rule 4).                                                                                                                                                   |
| The model note sits outside the control's label                                                               | A note inside the label becomes part of the control's accessible name, so a screen reader would announce the field's explanation as its name.                                                                                                                                              |
| The answer's status marker reuses the 3px left motif and never carries the state alone                        | The marker is the panel's identity element, and the state is also written in words, so a reader who cannot see the colour still gets the fact.                                                                                                                                             |
| A release date renders as the gateway wrote it, not through the zoned timestamp formatter                     | A calendar date has no zone: formatting `2026-09-20` in the browser's zone prints the previous day for a reader behind UTC, which is a release date the gateway never reported.                                                                                                            |
| The version slot says it is reading before the first answer lands                                             | `unknown` is a claim about a read that finished, and R-38 rules out a placeholder that states something false while the truth is still in flight.                                                                                                                                          |
| A release note renders as one block rather than a bullet list                                                 | The served entry carries a paragraph and no category; splitting it would attribute a structure nobody wrote, and inventing the old `category` field would put words in the gateway's mouth.                                                                                                |
| An uncomparable running version gets its own sentence and no release is marked                                | A count of zero newer releases is not "up to date" when nothing could be compared; a `-dev` build reports exactly that value, and marking every release `Newer` would tell an operator to upgrade to what they already run.                                                                |
| One navigation guard in the root layout, not one per form                                                     | A screen that had to remember the rule could forget it, and what is lost is the operator's work rather than one screen's layout, which is the same reason the session gate lives there.                                                                                                    |
| The in-panel question is `window.confirm`, while the unload question is the browser's own dialog              | A native prompt cannot be styled into the panel's language, but it is the only synchronous question `beforeNavigate` can ask, and the browser already asks its own dialog when the tab is closed; a custom modal for the in-panel half would be a second dialog language for one rule.      |
| Six forms register with the guard, not every form on the panel                                                 | Only a form with a server-confirmed baseline can say what "unsaved" means; a dialog that discards its draft on Cancel has nothing to lose, and registering it would warn about a draft the operator already chose to drop.                                                                 |
| The combo editor's strategy fields moved to their own component                                               | The §8.4.4 wiring pushed the editor past the 220-line warning, and §6.4's rule that a field a strategy ignores is hidden is the seam both blocks already shared.                                                                                                                            |
| The API base is a tabbed dialog opened from one header button                                                 | The strip had room for one form of the address and none to report a refused copy; a tablist is the shape for three renderings of one fact, and each tab keeps its copy control beside the value it copies (R-26).                                                                           |
| A refused copy does not unlock the one-time key modal                                                         | The plaintext exists nowhere else, so dismissal is safe only once the key actually reached the clipboard; the modal opens up on a copy that reported success, not on a press.                                                                                                               |
| The advertised API base is read from the panel server                                                         | `location.origin` is the panel, and a panel opened at a bind-all address advertises one no client can call; the panel server is the component that holds `PANEL_API_TARGET`.                                                                                                                |
| The copy control writes through the selection path on an insecure origin                                      | The async clipboard API exists only in a secure context and the panel is normally opened on an HTTP origin, so the deprecated `execCommand` path is deliberate: the alternative is a control that cannot work where the panel runs.                                                         |
| The custom provider section reads its own route rather than the registry page                                 | A node set is hand-configured and the route returns all of it, so it has no paging to share, and the category filter narrows the registry rather than this set.                                                                                                                             |
| A node is told apart by its id prefix, not by a flag                                                          | The provider response carries no custom flag, and the id prefix is the contract SPEC-API §7.4 states, so the screen reads what the wire actually carries.                                                                                                                                   |
| The custom provider section repeats no write actions                                                          | The reference puts edit, test, and delete on the node's own detail screen, which is where the node's other facts are read, so a second copy here would be two places to keep in step.                                                                                                       |
| The edit form states the api type rather than offering it                                                     | Type and api type are identity in §7.4's patch, so a control there could not act, which is a dead control under R-26.                                                                                                                                                                       |
| An empty test credential means test without one                                                               | The route reads an absent credential as a node whose upstream needs none, and an empty string would ask the gateway to authenticate with nothing.                                                                                                                                           |
| A refused node delete keeps the dialog open                                                                   | CONFLICT means an endpoint still references the node, and the operator needs the object and the gateway's reason in view to act on it.                                                                                                                                                      |
| The endpoint the gateway will call is composed from the base URL                                              | The path is decided by the wire shape, so the form shows the joined URL the gateway will call rather than leaving the operator to join it.                                                                                                                                                  |
| One refresh control re-reads both the registry and the node set                                               | §8.6.2 says the control repeats the read the screen is showing, and this screen shows two sets, so a registry-only refresh would leave a node just changed on the list.                                                                                                                     |
| The CodeBuddy pair is a register, not a panel section                                                         | Both are registry entries rather than nodes and the registry's own entry is hidden, so rendering them in the panel would show providers no request can reach, which R-38 rules out.                                                                                                         |
| The built server binds APP_PORT, and an explicit PORT wins over it                                            | adapter-node reads PORT while the env template calls it APP_PORT, so the boot preload maps one to the other; mapping only when PORT is unset keeps an explicit adapter override authoritative rather than silently replaced.                                                                |
| The breakdown table shows every value column at once                                                          | The reference switches its table between a cost view and a token view, and doing the same here would hide figures the table already shows in one line; the toggle belongs to the chart, which can draw only one series at a time (draft 014 F6, F2).                                       |
| The Tokens/Cost switch sits on the value chart                                                                | One card draws one series, so the switch chooses which series is drawn rather than which figures are hidden, and it is local state because it changes no read (draft 014 F2).                                                                                                              |
| The breakdown's sort is applied by the panel                                                                  | SPEC-API §7.12 has no sort parameter and the API sends the whole group block for the window, so the order is the panel's own and it orders every group rather than a page of them (draft 014 F1).                                                                                          |
| Provider names resolve on the breakdown; endpoint and gateway-key ids do not                                  | The registry read is one request and the table resolves the dimension the reference resolves; the two id dimensions are named on the screens that own them, which this tab does not read (draft 014 F6).                                                                                   |
| The Records filter bar reads its own two vocabularies                                                         | Neither read builds the request, and the bar is what needs the names; a list that cannot be read leaves the select offering the value the URL carries and a line saying why (draft 014 F3).                                                                                                |
| The finished list's clock ticks whether or not frames are arriving                                            | The row is a claim about now, so a frozen "just now" would be a wrong figure a minute later; this is text that has to stay true, not the drawing's motion, which stays gated on arriving frames (draft 013 F2, draft 014 F4).                                                              |
| The routing edge carries the reference fork's beam, not a single moving dash                                  | The owner chose motion parity, and the beam is how that fork draws an active edge: a halo, a plasma and a core stroke with six orbs and five sparks, whose counts, widths, speeds and phases the panel keeps (draft 015 F1).                                                               |
| The beam's two dash periods are the panel's own numbers                                                       | The reference moves its offset by -36 over patterns of 14 and 24 units, so its dashes jump at every cycle; the panel's path is normalized to 100 and both periods divide it, so the loop closes (draft 015 F1).                                                                            |
| The gateway pulses, shakes its mark and flickers its label while it routes                                    | That is the reference's router node with a request in flight, and it makes the same statement at the centre that the beam makes along the edge; the count in its badge is the number of requests, not of providers (draft 015 F1).                                                         |
| Each moving gateway part keeps one glow layer, not the reference's four-layer neon stack                      | R-13 caps the glow dose, so the cycles and the transforms are the reference's while the stacked yellow, cyan and inset shadows become one layer in the status colour (draft 015 F3).                                                                                                       |
| Every third beam dot is drawn in the text colour                                                              | Measured in the browser, dots in the status colour alone merged into the plasma stroke and the beam read as one solid bar in both themes; the reference alternates its dots for the same reason (draft 015 F2).                                                                            |
| The beam's turbulence is a still texture, at a displacement scaled to the drawing box                         | The reference animates the frequency with SMIL, which cannot honour a reduced-motion preference, and its 3.5 displacement is in pixel coordinates where this box's unit is a hundredth of the box (draft 015 F4).                                                                          |
| The drawing's motion tokens live in `src/lib/motion.css`                                                      | The beam and gateway keyframes pushed `app.css` past the 250-line ceiling, so they moved to their own file that `app.css` imports, which keeps the root layout's single entry point (draft 015 F5).                                                                                        |
| The two bar charts use the panel's one accent, not the reference's eight                                      | The reference colours each provider bar from a fixed eight-colour array (`ProviderBarChart.js:17`); R-29 keeps this panel to one accent plus status colours, and a provider is not a status (draft 016 F1).                                                                                |
| A bar carries a compact figure and the exact one sits one disclosure away                                     | A bar's length is a shape, so the figure beside it is rounded; the exact number stays in the chart's own table, which is what §6.5 asks of a chart that states its numbers in words (draft 016 F1).                                                                                        |
| The compact figure is Intl's, not the reference's hand-rolled thresholds                                      | The reference's thresholds print `1000.0K` for 999,999; a wart is not a behaviour to copy, and the standard formatter with a pinned locale prints `1M` (draft 016 F1).                                                                                                                     |
| The top-models chart draws five and says how many it left out                                                 | A ranking's tail is noise, so five are drawn (`TopModelsChart.js:40`) while the sentence names the leader and the count with usage, which makes the truncation visible rather than silent (draft 016 F2).                                                                                  |
| The live token split says in and out rather than the reference's arrows                                       | The row is read beside tiles that say Tokens in and Tokens out, and a bare arrow needs a legend; a figure the frame never sent is not printed as zero either (draft 016 F3).                                                                                                               |
| The drawing's facts are one line above it, in the drawing's colour rule                                       | The owner asked for the explanation paragraph to go and the facts to move up, and only a routing node's label takes the status colour in the drawing, so only the in-flight name takes it here (draft 023 F1). Replaced 2026-09-27 by draft 035 F2: the sentence left the frame, the colour rule did not.                                                 |
| The facts line keeps its slot while there is nothing to state                                                 | The line appears and disappears with the stream, and a slot that collapsed would move the drawing under the reader's eyes while they are watching it (draft 023 F1). Retired 2026-09-27 by draft 035 F2: the slot belonged to a paragraph inside the frame, and the frame no longer carries one.                                                 |
| The live facts are a tab of the controls row, not a sentence in the drawing's frame                           | The owner's correction of 2026-09-27: the dynamic text interfered with the node animation it sat above, so it moved beside the connection chip and the pause control, keeping the drawing's colour rule and the no-absence rule with it (draft 035 F2, SPEC-UI §6.5).                                                 |
| Every tab the live row renders wears one box, controls included                                               | The chip had been a plain text span beside two 44px buttons; the owner asked for same size and symmetric, so chip, facts, pause and retry share one box, and only the real controls carry the hover affordance that tells a statement from something clickable (draft 035 F1, D2).                                                 |
| The finished list is set off by the tab idiom with nothing focusable in it                                    | The owner asked for the separator to use a tab rather than to be a tab anyone can click, so the label carries the active tab's accent rule and the strip its border, and no control is added (draft 023 F2).                                                                               |
| The live drawing is a request path: combos above the gateway, upstreams below, terminals on the axis          | The owner asked to see the combo a client addressed, not only the provider that answered, so the drawing gained two stages and now reads `Client >> Combo >> Gateway >> Upstream >> Response`. The bands are disjoint rather than concentric because two nodes mirrored across the horizontal axis share an x, and the drawing's collision rule has only x to fall back on: a shared row at zero gap is a share of zero, which is a drawing with no boxes in it (draft 043 F1). |
| Combo nodes are capped at eight, and the row says which ones the drawing left out                             | Half an ellipse holds about half as many readable labels as the ring the drawing used before combos joined it: measured, eight nodes per band still draw 98px boxes and twelve draw 62px. A silent cap would be a map that quietly omits, so the count that is missing is stated (draft 043 F1).                                                                            |
| A beam's direction is its endpoint order, not a second keyframe                                               | `beam-dash` moves the offset by -100 along a path normalized to 100 units, which carries every dash from a line's start to its end, so the way back is drawn as a line that starts at the upstream. One animation, and the answer still reads as returning rather than as a second request (draft 043 F2).                                                                       |
| A card fades in as it joins the drawing and the box transitions its height                                    | The owner's complaint about the node animation was that it jumped: a combo appears with traffic and the box grew instantly under the reader's eyes. Neither claim is about what is happening now, so neither is the drawing's state motion, and both are still dropped for a reader who asked for reduced motion (draft 043 F3).                                             |
| Interaction states are one scale, read by one component                                                       | Eight control groups on one screen had each written their own `hover:` utility, which is why a press step had nowhere to be decided and the same affordance measured 50% in one place and 60% in another. A value a screen can rewrite is a value a screen will rewrite (SPEC-UI §9.3).                                                                                       |
| A pressed control paints a colour, not only a shift                                                           | `hover:` does not exist on a touch layout, so a wash plus a 1px move leaves a phone with no answer to "did that land". The shift stays as the secondary cue.                                                                                                                                                |
| Steps are sized in L\*, labels in contrast ratio                                                              | A ratio cannot size a state change: a wash and its ground are close by design, so the pair sits near 1.08:1 however visible the step is. Measuring both ways stops a step being bought with the label's contrast, which is how the previous wash ended up at 3.1 L\*.                                                                                                       |
| A colourless control takes an ink wash, a coloured one takes its own ramp                                     | The wash calibrates itself against whatever ground it lands on, which one absolute hex cannot do across a page, a card, and a table header. A coloured label under an ink wash loses its AA (the danger glyph measures 4.17:1 pressed on a raised panel), so it steps along its own ramp instead.                                                                          |
| Further in moves deeper in light, brighter in dark                                                            | It is the direction §3.2 already gives `surface-3` for a pressed row. A ramp running the other way in the same theme would read as a control lifting off the page while it is being held.                                                                                                                     |
| A disabled primary leaves the accent rather than fading it                                                    | Element opacity fades a label and its own fill toward the page together, which measured 1.87:1 on the washed pair; `surface-3` with a muted label reads 5.61:1 and stops an action that cannot be taken from claiming the screen's one accent.                                                                                                                               |
| A destructive commit in a dialog carries `danger-ink`, the way the accent carries `accent-text`               | A solid action control needs a fixed light-on-dark or dark-on-light pair per theme. The raw `text-white` it replaced measured 2.42:1 on the dark danger token, which is below the floor and was in three screens.                                                                                             |
| A destructive action in a table row stays a quiet glyph                                                       | One danger action among four reads as a set; a red block on every row shouts, and the row is where an operator scans.                                                                                                                                                                                        |
| A chosen control keeps answering a pointer, one step past its own ground                      | A filter chip that is selected sits on the pressed surface, so its hover needs a step of its own; `state-deep` is that step, and re-pressing a chosen chip asks for no fourth because it changes nothing. The selection is read from `aria-pressed` rather than a class, because that attribute is already what announces it.                                                  |
| A control's colour comes only from a declared token, and two typos showed why that is a rule          | The bulk bar painted its commit with `--color-primary-contrast` and its tag field with `--color-surface-1`. Neither is declared anywhere, so the utility resolved to nothing and the button inherited the page's text colour: near-white on the coral fill in dark, 1.44:1. An undeclared token is not a missing value, it is a silent fallthrough, and a shared control leaves no room for one.          |
| A chosen model in the picker is marked on the pressed surface, not with an accent fill                | A dozen coral chips in one dialog is the accent everywhere §3.4 forbids it, and it made the chosen set read as the screen's primary action. Selection is one reading on every screen now: `surface-3`, the text border, and the weight.                                                                                                                              |
| A text action that pages a list is one component, not three copies                                    | The paging band was copied on three screens and each copy had drifted to its own padding, and one of them had no 44px target at all. A band that pages is the same band everywhere.                                                                                                                                            |
| The dense icon button matches the dense text button's height                                                  | `icon-sm` was 32px beside a 36px text button, so a toolbar with both finished on two lines. Measured against §7's density, the dense tier is one height.                                                                                                                                                    |

---

## 12. antislop binding

Mode **DURING** while a screen is written, mode **AFTER** as an audit before a screen ships. SPEC-UI §16
records the gate for the spec; this section records the gate for rendered screens.

What this direction settles, and what the Delivery Gate therefore checks against:

| Rule                       | How this file satisfies it                                                                                       |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| R-01 (color and gradients) | One accent, warm neutrals, no gradient and no glow. Reason per token in §3.                                      |
| R-04 (icons)               | One installed set, one written reason per navigation icon, no sparkle or robot glyphs.                           |
| R-06 (typography)          | Inter with a stated reason (dense-table legibility plus parity), self-hosted, tracking left at the font default. |
| R-07 (background)          | No grid, blueprint, or dot pattern. The surfaces are a warm neutral.                                             |
| R-09 (badges)              | A badge carries a real status: endpoint state, key health, quota source, request status.                         |
| R-10, R-13 (glass, glow)   | Not used. The panel has no surface that needs either.                                                            |
| R-11 (radius)              | Two functional values plus a dialog value and an avatar value, with a stated hierarchy. The shared button carries `--radius-sm`, the value §5 assigns a button.                          |
| R-12 (shadow)              | Two floating surfaces only, plus the logo frame, each with a stated reason.                                      |
| R-19 (animation)           | MOTION 1, plus the live indicator while a request is in flight: a pulse and a flowing dash. The hover and pressed colour steps (§3.5) are state transitions, which is what the dial already allows; nothing else moves.                      |
| R-21 (theme)               | Both themes authored from §3.2, both verified per screen.                                                        |
| R-25 (contrast)            | Every pair in §3.3 measured, asserted by a test that reads `src/app.css`, lowest text pair 4.87:1. The state steps and their labels are measured in §3.5 by `tests/tokens/state-scale.test.ts`.               |
| R-29 (palette cap)         | One accent. Neutrals are excluded by the rule; status colours are not brand colours.                             |
| R-31 (reason per decision) | §11. Every visual decision carries a one-line reason.                                                            |
| R-37 (direction required)  | This file. A screen styled from it is a deliverable, not a draft.                                                |

---

_Created 2026-09-17. Direction supplied by the owner the same day: the legacy 9Router panel is the
structural reference, delivered in a cleaner and fresher component layer, with mobile, tablet, and desktop
responsive behaviour as an explicit requirement. The logo asset was supplied the same day at
`app-ui/assets/static/logo.png`, cropped to a circular mark for the panel._

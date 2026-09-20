# DESIGN.md: pannelAI Panel (`app-ui`)

| | |
|---|---|
| **Status** | Active. Replaces the "draft without direction" placeholder recorded in SPEC-UI §9.1. |
| **Owner** | Dodi Rusmana <rusmanadodi@kentangtech.com> |
| **Written** | 2026-09-17 |
| **Applies to** | `app-ui/**` only. `app-serv` has no rendered surface. |
| **Consumed by** | `docs/SPEC-UI/001-SPEC-UI.md` §9, `src/app.css`, `tests/tokens/contrast.test.ts` |

This file is the panel's style direction. SPEC-UI §9.1 recorded that no direction existed, so every
styling detail there was a placeholder. This document replaces that state: the palette, the typeface, the
radius scale, and the identity motif are decided here, and a screen built afterwards is a deliverable
rather than a draft.

Where this file and SPEC-UI disagree about a visual value, this file wins. Where they disagree about a
behaviour, an endpoint, or a route, SPEC-UI wins: this file holds design direction, not product scope.

---

## 1. Identity

| Question | Answer |
|---|---|
| Product | pannelAI, an AI gateway control panel for one operator or a small team |
| Audience | A technical operator who reads tables all day: routing health, usage, quota, logs |
| Tone | Quiet, dense, factual. The panel reports; it does not sell. |
| Reference | The legacy 9Router panel is the structural reference (owner direction, 2026-09-17). Structure and density are inherited; its decoration is not. |
| Promise | One accent, warm neutrals, and every control backed by a real endpoint. A figure on screen is a figure the API returned. |

The panel is an instrument. An instrument earns trust by being legible and by never lying, so the design
spends its budget on hierarchy and density rather than on ornament.

---

## 2. Design Read

> Reading this as: **operator panel** for **a technical gateway operator**, in a **warm-neutral
> instrument** style with one coral accent, dial **ENERGY 1 / RHYTHM 2 / MOTION 1**.

### 2.1 Dials

| Dial | Value | Reason |
|---|---|---|
| **ENERGY** | 1 | An instrument states its values and stops. Density is the point, not decoration. |
| **RHYTHM** | 2 | The shell is deliberately uniform so muscle memory holds across screens, while the sidebar's five task groups and the screen bodies vary in composition. Uniform shell plus varied content is the point of a control panel. A flat 1 would make every screen one undifferentiated strip, and a 3 would cost the operator orientation. |
| **MOTION** | 1 | Hover, focus, and state transition only, plus the mobile drawer slide. No scroll choreography on a screen an operator opens twenty times a day. |

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

| Token | Light | Dark | Role |
|---|---|---|---|
| `--color-surface` | `#FCFAF7` | `#1A1917` | Page ground |
| `--color-surface-2` | `#F4F1EC` | `#24221F` | Raised panel: sidebar, card, table header |
| `--color-surface-3` | `#E9E4DC` | `#302D29` | Pressed or selected row, input well |
| `--color-border` | `#DFD9D0` | `#3A362F` | One hairline weight everywhere |
| `--color-text` | `#1A1614` | `#EDEAE6` | Body text |
| `--color-text-muted` | `#5F574E` | `#A69E93` | Secondary text, labels, icon rest state |
| `--color-accent` | `#B8412A` | `#E56A4A` | The single accent: active nav, primary action, focus ring |
| `--color-accent-text` | `#FFFFFF` | `#14100E` | Label on an accent fill |
| `--color-ok` | `#14743A` | `#5CC97B` | Healthy, active, passing |
| `--color-warn` | `#8F5C0A` | `#E8A93A` | Degraded, nearing a limit |
| `--color-danger` | `#B3261E` | `#F08A84` | Failed, revoked, destructive |

### 3.3 Measured contrast

Every pair below is asserted by `tests/tokens/contrast.test.ts`, which reads these values from
`src/app.css` and fails the build when one regresses. Measured 2026-09-17.

| Pair | Light | Dark | Floor |
|---|---|---|---|
| `text` on `surface` | 17.25 | 14.65 | 4.5 |
| `text` on `surface-2` | 15.95 | 13.23 | 4.5 |
| `text` on `surface-3` | 14.20 | 11.42 | 4.5 |
| `text-muted` on `surface` | 6.81 | 6.64 | 4.5 |
| `text-muted` on `surface-2` | 6.30 | 5.99 | 4.5 |
| `text-muted` on `surface-3` | 5.61 | 5.17 | 4.5 |
| `accent` on `surface` | 5.27 | 5.43 | 4.5 |
| `accent` on `surface-2` | 4.87 | 4.90 | 4.5 |
| `accent-text` on `accent` | 5.49 | 5.85 | 4.5 |
| `ok` on `surface-2` | 5.19 | 7.63 | 4.5 |
| `warn` on `surface-2` | 5.03 | 7.68 | 4.5 |
| `danger` on `surface-2` | 5.80 | 6.55 | 4.5 |
| `accent` on `surface-3`, non-text | 4.34 | 4.23 | 3.0 |

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

---

## 4. Typography

| Role | Family | Size | Weight |
|---|---|---|---|
| Body, controls, tables | Inter | 14px | 400 |
| Secondary and label | Inter | 12px to 13px | 400 to 500 |
| Screen title | Inter | 18px | 600 |
| Group label in the sidebar | Inter | 11px | 600, uppercase, 0.06em tracking |
| Code, IDs, cost, log bodies | System monospace stack | 12px to 13px | 400 |

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

| Token | Value | Used for |
|---|---|---|
| `--radius-sm` | 6px | Input, button, chip, menu row |
| `--radius-md` | 8px | Panel, card, table container |
| `--radius-lg` | 12px | Dialog, drawer |
| `--radius-full` | 9999px | Avatar and the logo only |

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

| Value | Use |
|---|---|
| 4px | Icon to label gap, chip internal spacing |
| 8px | Between rows in a menu group, between form field and its help text |
| 12px | Row padding, panel padding at the compact end |
| 16px | Panel padding, gaps between form fields |
| 24px | Gap between page sections |
| 32px | Page edge gutter on desktop |

Row height is 36px in dense tables and 44px for anything in the sidebar, which is a touch surface at every
breakpoint (R-03).

---

## 8. Responsive

| Breakpoint | Shell behaviour |
|---|---|
| `<768px` Mobile | Sidebar is an off-canvas drawer over a dimmed overlay. Opens from the header control, closes on Escape, on overlay click, and on navigation. Page gutters drop to 16px, tables become stacked key-value rows. |
| `768px` to `1023px` Tablet | Sidebar shows as a 64px icon rail with tooltips on hover and focus. A control expands it to full width over the content. This is the band the legacy panel left as a plain drawer, which wasted the icon-and-label space a tablet actually has. |
| `>=1024px` Desktop | Sidebar is full width at 264px, collapsible to the same 64px rail. The collapsed state persists. |

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

| Asset | Value | Status |
|---|---|---|
| Logo | `static/logo-mark.png`, the owner's mark cropped to its square emblem and masked to a circle | Real asset, supplied by the owner (2026-09-17) |
| Logo source | `assets/static/logo.png`, the owner's original 1254x1254 file | Kept as the source of truth |
| Favicon | `static/favicon.png` | Derived from the same source |
| Icon set | `@lucide/svelte`, one set, no mixing | Real |
| Icons in navigation | Each has a written reason in `src/lib/icons.ts` | Real |
| Avatars, photography, illustrations | None | Not used. The panel shows no people. |
| Statistics, testimonials, uptime or compliance claims | None anywhere in the panel | Forbidden (R-17, R-18, R-36) |

The logo's circular frame takes the elevation treatment described in §5: the emblem is masked to a circle
and the frame carries a soft ring so it reads as a physical button rather than a floating sticker. That is
the one place in the panel where a shadow is allowed on a non-floating element, and the reason is that it
is the product's face, not a control.

No emoji anywhere in UI copy, headings, empty states, or buttons. Icons are line glyphs from the single
installed set.

---

## 11. Reason log (R-31)

| Decision | One-line reason |
|---|---|
| Coral accent kept from the legacy panel | It is the product's existing identity, and the owner asked for structural parity. |
| Light accent darkened to `#B8412A` | The legacy coral fails AA as a button fill with white text; the same hue one step darker passes. |
| Dark accent keeps `#E56A4A` with a near-black label | On a dark surface a bright accent needs a dark label, and coral with near-black clears 5.85:1. |
| Warm neutrals rather than pure grey | The source mark and the legacy panel are warm; a cold grey would fight the one accent. |
| Inter, self-hosted | Legible at 12px in dense tables, which is the panel's main surface; self-hosted because a font CDN is excluded. |
| One accent in five places | An accent stops being an accent when it is everywhere; five is the count that survives review. |
| Left status marker as the motif | It answers the operator's recurring question, "which row matters", in the same place every time. |
| Flat surfaces, shadow only on a dialog and a drawer | Elevation that appears everywhere communicates nothing. |
| No glass, glow, or gradient | The panel has no surface that needs one, and all three are accents by definition. |
| `100dvh` shell | `100vh` clips the header under mobile browser chrome. |
| Tablet gets an icon rail, not a drawer | A tablet has room for a 64px rail, which keeps navigation reachable while editing. |
| Two radius values plus a dialog value | Radius is a hierarchy tool; a single value everywhere erases the difference between an input and a panel. |
| Sidebar groups named by operator intent | A flat list of fifteen items is a list, not a structure. |
| Expanded sidebar width set to 264px in the primitive layer's constants | §8 fixes 264px and the component layer ships 256px, so this is the one primitive edit SPEC-UI §10.7 permits, it is recorded here as that rule requires, and it is re-applied after a regeneration rather than assumed to survive one. The 64px icon rail already matches and is untouched. |
| Three savers share one section shell | Three groups of equal weight should read as siblings, not as three unrelated forms that happen to sit on one page. |
| The per-request bypass is copyable text, not a disabled control | A control that cannot act is a dead control under R-26; text the operator can copy into a client is a real function. |
| The native token saver engine is a Planned note, not a greyed toggle | A disabled toggle promises a capability the repository does not have yet, which is the claim R-36 rules out. |
| Pool test result sits in the row, with "Not tested" kept distinct from a failure | The operator's question is which row is broken, and a row nobody has tested is not a broken row. |
| The password cell reads "Set" or "Not set" | The API returns `has_password` and never the secret, so presence is the only thing the cell can honestly report; a blank cell would read as no password. |
| Batch add shows the parse before any write | A pasted list is where the input goes wrong, so the operator sees what the panel understood before a single request is sent. |
| Network settings live on the Proxy Pools screen, and `/settings` links there | They are proxy settings, and a second copy under Settings is the duplication the owner's KEEP list removes. |
| The outbound card states that its scope is global | A control sitting beside a pool table reads as per-row routing, which the egress path does not implement. |
| The delete dialog names the object and says it does not reroute traffic | The dangerous assumption is that removing a row changes routing; naming both the object and the non-effect is what makes the confirm safe to press. |
| The media base URL field shows the registry's value beside it rather than pre-filling it | A field pre-filled with a value the panel did not write would save that value back as an override, changing where the address comes from without changing the address. |
| The media default model is a selector, not a text field | The service declares the models it can route, so the set is a choice; free text invites a value nothing downstream can serve. |
| A media kind that declares no models gets a stated fact instead of an empty selector | A select with no options is a dead control under R-26, and the fact is the honest shape of "there is nothing to choose". |
| The media endpoint count is stated, not linked | The count is scoped to one provider and one kind, and the endpoint screen filters by provider alone, so a link would promise a narrower list than it shows. |
| The media provider name links to its registry entry | The card names a provider, and the registry entry is where its facts and its endpoint list live. |
| A sidebar row may link to a parameterised route when the row fixes the parameter | A Media kind is part of the row's identity rather than something an operator picks after opening the screen, so the row can build the address; a provider id is the opposite case and stays unlinkable. |

---

## 12. antislop binding

Mode **DURING** while a screen is written, mode **AFTER** as an audit before a screen ships. SPEC-UI §16
records the gate for the spec; this section records the gate for rendered screens.

What this direction settles, and what the Delivery Gate therefore checks against:

| Rule | How this file satisfies it |
|---|---|
| R-01 (color and gradients) | One accent, warm neutrals, no gradient and no glow. Reason per token in §3. |
| R-04 (icons) | One installed set, one written reason per navigation icon, no sparkle or robot glyphs. |
| R-06 (typography) | Inter with a stated reason (dense-table legibility plus parity), self-hosted, tracking left at the font default. |
| R-07 (background) | No grid, blueprint, or dot pattern. The surfaces are a warm neutral. |
| R-09 (badges) | A badge carries a real status: endpoint state, key health, quota source, request status. |
| R-10, R-13 (glass, glow) | Not used. The panel has no surface that needs either. |
| R-11 (radius) | Two functional values plus a dialog value and an avatar value, with a stated hierarchy. |
| R-12 (shadow) | Two floating surfaces only, plus the logo frame, each with a stated reason. |
| R-19 (animation) | MOTION 1: hover, focus, state transition, drawer slide. No loop, no scroll choreography. |
| R-21 (theme) | Both themes authored from §3.2, both verified per screen. |
| R-25 (contrast) | Every pair in §3.3 measured, asserted by a test that reads `src/app.css`, lowest text pair 4.87:1. |
| R-29 (palette cap) | One accent. Neutrals are excluded by the rule; status colours are not brand colours. |
| R-31 (reason per decision) | §11. Every visual decision carries a one-line reason. |
| R-37 (direction required) | This file. A screen styled from it is a deliverable, not a draft. |

---

*Created 2026-09-17. Direction supplied by the owner the same day: the legacy 9Router panel is the
structural reference, delivered in a cleaner and fresher component layer, with mobile, tablet, and desktop
responsive behaviour as an explicit requirement. The logo asset was supplied the same day at
`app-ui/assets/static/logo.png`, cropped to a circular mark for the panel.*
# 001-DESIGN-USAGE: Usage Screen Audit and Improvement

| Field | Value |
| --- | --- |
| **Status** | Active audit and improvement specification |
| **Owner** | Dodi Rusmana <rusmanadodi@kentangtech.com> |
| **Written** | 2026-10-23 |
| **Route** | `/usage` (`app-ui/src/routes/usage/+page.svelte` and the `Usage*` components) |
| **Direction** | `DESIGN.md` (warm-neutral instrument, one coral accent, `ENERGY 1 / RHYTHM 2 / MOTION 1`) |
| **Governs** | `docs/SPEC-UI/001-SPEC-UI.md` §6.5 for layout and content needs |
| **Hard limit** | No new palette, no neon, no cyberpunk. Every colour is an existing token in `src/app.css`. |

This document records what the Usage screen says now, what it fails to say, and the
changes that make it legible as a telemetry instrument. It covers the full screen
(Overview and Records) and gives the live routing drawing a dedicated motion section.
The reason for each decision is one line in §9, as `R-31` asks.

---

## 1. Design Read

> Reading this as: **operator telemetry and live routing surface** for **a technical
> gateway operator**, in a **warm-neutral precision instrument** style with one coral
> accent, dial **ENERGY 1 / RHYTHM 2 / MOTION 1**, plus the bounded live-routing motion
> that `DESIGN.md` §2.1 and §11 already allow while frames arrive.

An operator opens `/usage` to answer two questions, at two altitudes:

1. **Over the window:** how much the gateway routed, what it consumed, and what it cost,
   and which models or providers drove that figure.
2. **Right now:** whether traffic is flowing, which combo and upstream provider are
   holding the requests in flight, and which upstream last failed.

The screen already answers both with honest data. What it does not do is order that data
so the operator reaches the right answer without scanning. The audit below is about order,
hierarchy, and a live drawing whose motion reads as a calm instrument rather than as a
legacy fork's arcade overlay.

---

## 2. Scope and Method

Read in full for this audit:

- `app-ui/src/routes/usage/+page.svelte`
- `UsageOverviewTab`, `UsageOverviewControls`, `UsageOverviewEmpty`, `UsageOverviewCharts`,
  `UsageOverviewBarCharts`, `UsageOverviewBreakdown`, `UsageGroupTable`
- `UsageLivePanel`, `UsageLiveTabs`, `UsageRecentList`, `UsageTopology`,
  `UsageTopologyDrawing`, `UsageTopologyEdges`, `UsageTopologyHop`
- `UsageRecordsTab`, `UsageRecordsFilters`, `UsageRecordTable`, `UsageRecordDrawer`
- `UsageTotalsTiles`, `UsageChart`, `UsageBarChart`
- `src/lib/motion.css`, `src/lib/schemas/usage-beam.ts`,
  `src/lib/schemas/usage-topology-geometry.ts`, `src/lib/schemas/usage-topology-view.ts`
- Tests that pin the drawing: `tests/components/usage-topology-motion.test.ts`,
  `tests/components/usage-topology-beam.test.ts`, `tests/schemas/usage-beam.test.ts`,
  `tests/schemas/usage-topology-geometry.test.ts`

Each finding states the rule it sits against (`R-XX`), what the screen does now, why that
costs the operator something, and the change that answers it.

---

## 3. Full Screen UX and UI Audit

### U-01: The live drawing is the last thing on the page (R-05, C-3)

- **Now:** `UsageOverviewTab.svelte` renders `UsageLivePanel` after the controls, the
  window line, nine total tiles, two series charts, two bar charts, and the breakdown
  table. It also sits outside the loading and empty branches, so it is always last.
- **Cost:** an operator who opens `/usage` during an incident to watch routing must scroll
  past four blocks of history first. The one live thing on the screen is below the fold
  on every viewport.
- **Change:** keep the aggregate read order for the historical question, and give the live
  question its own place at the top of the Overview body, directly under the toolbar, as a
  compact instrument card. The full live detail (tabs, facts, finished list) stays where it
  is. One card, one anchor, no data removed.

### U-02: Nine identical tiles flatten the totals (R-14, C-1)

- **Now:** `UsageTotalsTiles.svelte` renders nine cards in one `grid-cols-2 sm:grid-cols-3`
  grid: Requests, Tokens in, Tokens out, Cache read, Cache write, Cost (USD), Error rate,
  Latency p50, Latency p95. Every card has the same padding, the same `text-lg font-medium`
  value, the same border.
- **Cost:** the operator's first figure (requests or cost) carries the same weight as a
  cache sub-counter. With nine equal blocks there is no focal point, and the eye reads all
  nine to find the one it wanted.
- **Change:** keep `DESIGN.md` §11's three-column block (three columns from `sm`, so nine
  figures draw a 3x3), and build hierarchy inside it. Three lead cells carry the
  decision figures (Requests, Cost, Tokens total = in plus out) at a larger weight; the six
  supporting cells (Tokens in, Tokens out, Cache read, Cache write, Error rate, and the two
  latency figures folded into one Reliability cell) sit as a dense strip below. The exact
  nine figures §6.5 lists stay on screen.

### U-03: The window line floats between the toolbar and the totals (R-05, R-20)

- **Now:** `UsageOverviewControls.svelte` renders the Period select, the Group by select,
  and the refresh control in a three-column band. The resolved window
  (`Showing ... to ..., bucketed by ...`) prints as a separate paragraph beneath it,
  inside the tab.
- **Cost:** the query inputs and the window they produced are visually separate, so the
  operator reads two blocks to learn one fact, and the paragraph consumes a full row of
  vertical space that a dense panel cannot spare.
- **Change:** fold the window readout into the toolbar as a trailing instrument readout on
  the same surface (`bg-[var(--color-surface-2)]`, one `--color-border`, `--radius-md`),
  with the range and the granularity in `tabular-nums`. The commit controls stay on the
  leading side. One band, one surface, no floating sentence.

### U-04: The live status readout is dressed as a button (R-20, R-26)

- **Now:** `UsageLivePanel.svelte` gives the status span the same `min-h-11`, border, and
  radius as the Pause and Retry buttons beside it (`TAB_BOX`).
- **Cost:** a non-interactive statement looks pressable. An operator taps it and nothing
  happens, which is the dead-control failure the panel has already ruled out elsewhere.
- **Change:** render the stream state as an honest readout: the panel's 3px left status
  marker (`DESIGN.md` §6) plus the state label, on the card surface, separated from the
  controls. `--color-ok` for `live`, `--color-warn` for `connecting` and `paused`,
  `--color-danger` for `unavailable`. The markers carry a real state, so the marker is a
  signal, not decoration.

### U-05: The stream controls sit in three stacked rows above the drawing (R-05, R-20)

- **Now:** `UsageLivePanel.svelte` stacks the heading, the subtitle, the control row, the
  `statusLine` paragraph, and the node notices before `UsageTopology` begins.
- **Cost:** five rows of chrome separate the operator from the drawing the chrome governs.
  On a phone that is most of the first screen.
- **Change:** consolidate the stream state, the `statusLine`, and the Pause and Retry
  controls into the drawing card's own header bar. The node notices stay directly under the
  header as a `role="status"` line. The heading and subtitle collapse into that header.

### U-06: The topology wastes 128px on tablet and crops on a phone (R-03)

- **Now:** `UsageTopology.svelte` wraps the drawing in
  `mx-2 overflow-x-auto sm:mx-16` around a `min-w-[520px]` inner box.
- **Cost:** on tablet (`768` to `1023`) `sm:mx-16` removes 128px of horizontal room, which
  cramps the node labels the `NODE_MAX_WIDTH` cap already governs. On a phone the fixed
  520px inner width forces horizontal scrolling with no scroll cue.
- **Change:** drop the desktop margin to the panel's standard inner gutter (`sm:mx-4`), keep
  the container unit scaling that already shrinks the drawing, and keep the horizontal
  scroller only where the geometry genuinely cannot compress (below roughly 520px), inside
  a focusable `region` with a label.

### U-07: The live motion has no single focal point (R-13, R-19)

- **Now:** while a request is in flight, `UsageTopologyDrawing.svelte` runs the gateway
  pulse, the gateway logo shake, the gateway label flicker, a glow on every active node, a
  glow on the in-flight badge, an `animate-ping` ring on every active node dot, and a
  terminal pulse on each active end. `UsageTopologyHop.svelte` runs a halo, a plasma, a
  core, six orbs, and five sparks on every active hop.
- **Cost:** when several providers route at once, dozens of parts move and glow at once, so
  none of them reads as "this is the thing to watch". Motion applied everywhere amplifies
  nothing, and the stacked glow breaks the `R-13` dose cap.
- **Change:** one focal mover. The gateway hub is the only pulsing, glowing element. Active
  nodes and terminals keep their state colour and their entry transition, and drop the
  endless ping and the per-node glow. The beam stays, reduced to the readable layers in §5.

### U-08: Endless loops run for reasons `R-19` does not grant (R-19)

- **Now:** `animate-ping` (a built-in endless scale and fade) repeats on every active node
  dot for as long as frames arrive. The gateway pulse, label flicker, and logo shake also
  run on infinite loops.
- **Cost:** an endless loop is noise that never lets the eye rest, and the panel's `MOTION 1`
  dial plus its one stated exception does not license several simultaneous loops.
- **Change:** keep exactly one loop, the gateway hub's harmonic ring, gated on frames
  arriving. Everything else is a one-shot entry, a state colour, or a state transition.
  A status dot that marks a real state stays solid (the colour is the state; the loop was
  not).

### U-09: Numeric columns do not lock their digits (R-31, C-4)

- **Now:** the breakdown table and the totals value already use `tabular-nums`, but the bar
  chart value column, the live in-flight timers, and the finished list mix proportional and
  tabular figures in places.
- **Cost:** a live timer or a cost that changes width on every tick shifts the row beside it,
  which reads as movement in a screen the operator is scanning.
- **Change:** apply `tabular-nums` to every figure that updates or stacks: the totals values,
  the live counters, the cost column, and the bar chart value column. This is a text fix,
  not motion.

### U-10: Investigating a breakdown row costs a tab change and a re-filter (R-26, C-3)

- **Now:** the Overview breakdown lists rows per provider or model, and the Records tab holds
  the requests behind them. Nothing connects a row to its records; the operator switches tab
  and rebuilds the filter by hand.
- **Cost:** the screen states that the tabs answer one question at two depths, but the path
  between them is manual, which is where an operator gives up.
- **Change:** where the breakdown is grouped by provider or model, give the row a
  keyboard-reachable action that opens Records with that dimension already filtered through
  the existing URL parameters. One press from the figure to the rows behind it.

### U-11: The Records tab reads as a second, unrelated screen (R-05, R-20)

- **Now:** `UsageRecordsTab.svelte` renders its own filter bar and its own refresh control,
  then the table and pager. Nothing in the copy or the surface ties it to the Overview.
- **Cost:** two tabs that claim one question read as two screens, and the operator loses the
  sense that the Records rows are the rows behind the Overview figure.
- **Change:** carry the window toolbar idiom across both tabs (same surface, same control
  band), and state the relationship in one short line: the Records rows are the requests the
  Overview counted. No new data, no new read.

### U-12: The duplicate provider read (C-2, C-4)

- **Now:** `UsageOverviewTab.svelte` calls `listProviders({ per_page: 100 })` on mount, and
  `UsageLivePanel.svelte` calls the same route on its own mount for the drawing's labels and
  the topology node set. The two reads carry separate notices.
- **Cost:** the same registry is read twice on one screen, and a failure prints two notices
  for one cause.
- **Change:** read once and pass the result down. The Overview already owns the read for its
  provider chart and its breakdown; the live panel takes the resolved map as a prop and keeps
  its own "drawing left these out" notice, which is a different fact.

---

## 4. Node Animation Audit

### 4.1 Current motion inventory (`src/lib/motion.css`)

| Token | Keyframe | Current behaviour | Reads as |
| --- | --- | --- | --- |
| `--animate-beam-halo` | `beam-flicker` | `steps(2)`, `0.18s`, opacity jumps | A strobing fault |
| `--animate-beam-plasma` | `beam-dash` + `beam-flicker` | `0.18s` dash, `steps(3)` flicker | Electric glitch |
| `--animate-beam-core` | `beam-dash` | `0.22s` dash | A blurred streak |
| `--animate-beam-travel` | `beam-dash` | `0.4s` dot travel | A fast chain, direction lost |
| `--animate-beam-spark` | `beam-dash` + `beam-blink` | `0.4s` travel, `0.35s` hard blink `0,1,0,0,1,0` | Sparks popping in and out |
| `--animate-router-pulse` | `router-pulse` | scale to `1.06`, `14px` glow at `70%` | A glowing sci-fi orb |
| `--animate-router-shake` | `router-shake` | `rotate(-5deg)` to `rotate(5deg)`, `0.45s` | A device shaking, which reads as a fault |
| `--animate-router-flicker` | `router-flicker` | `text-shadow` glow pulsing on a 12px label | Blurry text on a small label |
| `--animate-node-enter` | `node-enter` | `0.3s ease-out` from `opacity 0, scale 0.96` | A pop with no settle |

### 4.2 Why this reads as a legacy overlay rather than this panel

The beam's shape (a halo, a plasma, a core, six orbs, five sparks) and its cycle times are
ported from the reference fork. Stepped `steps()` flickers, a `text-shadow` glow, and a
rotating shake are the visual language of neon dashboards. This panel's direction is the
opposite: warm neutral surfaces, one coral accent, status colours that report state, and a
motion dial that allows hover, focus, and state transition plus one live-routing exception.
The motion also breaks the dose caps: glow lands on every active node, the badge, and the
gateway at once (`R-13`), and several infinite loops run together with no single focal point
(`R-19`).

### 4.3 Target principles

1. **A healthy gateway is stable, not shaking.** Activity is shown by a calm, repeating
   cadence at the hub, never by rotation or strobing.
2. **Direction is readable.** A dash period slow enough that the eye follows the packet from
   source to target beats a fast streak.
3. **One mover.** The hub is the only looping element. Nodes, terminals, and edges carry
   state by colour and weight, and animate only on entry.
4. **Colour is state; motion is not.** When frames stop, colour stays and every moving part
   stops at once.

---

## 5. Node Animation Improvement Specification

All changes stay inside the token names and class names the tests already pin. `R-31` and
the `usage-topology-motion` and `usage-topology-beam` suites depend on the class names
(`animate-beam-halo`, `animate-beam-plasma`, `animate-beam-core`, `animate-beam-travel`,
`animate-beam-spark`, `animate-router-pulse`, `animate-router-shake`,
`animate-router-flicker`, `motion-reduce:animate-none`, `motion-reduce:hidden`) and on the
dash patterns from `usage-beam.ts`. This section changes keyframes and cycles only, and
never removes a token the tests name.

### 5.1 Motion tokens: old to new

| Token | New keyframe and cycle | Purpose (`R-19`, `R-31`) |
| --- | --- | --- |
| `--animate-beam-halo` | `beam-breathe 1.6s ease-in-out infinite`, opacity `0.28` to `0.55` | A calm ambient envelope on the active conduit, with no strobing |
| `--animate-beam-plasma` | `beam-dash 0.9s linear infinite` | A continuous directional flow at a speed the eye can follow |
| `--animate-beam-core` | `beam-dash 0.6s linear infinite` | A second, faster core layer that reads as priority traffic over the flow |
| `--animate-beam-travel` | `beam-dash 1.1s cubic-bezier(0.4, 0, 0.2, 1) infinite` | Discrete request packets that ease into and out of each hop |
| `--animate-beam-spark` | `beam-dash 1.1s linear infinite, beam-pulse 1.1s ease-in-out infinite` | A soft phase on the trailing packets, replacing the hard `0,1,0,0,1,0` blink |
| `--animate-router-pulse` | `router-pulse 1.5s cubic-bezier(0.22, 1, 0.36, 1) infinite`, scale to `1.03`, ring at `color-mix(in srgb, var(--color-ok) 20%, transparent)` | A composed hub heartbeat, not a heavy glow |
| `--animate-router-shake` | `router-breathe 1.5s ease-in-out infinite`, `scale(1)` to `scale(1.04)`, rotation held at `0deg` | Shows hub activity without the fault-shaped rotation |
| `--animate-router-flicker` | `router-label 1.5s ease-in-out infinite`, opacity `0.9` to `1` | Keeps the 12px label sharp while the hub is active |
| `--animate-node-enter` | `node-enter 0.26s cubic-bezier(0.22, 1, 0.36, 1) 1`, from `opacity 0, translateY(3px), scale(0.98)` | A card that settles into the band instead of popping |

The cluster's rule of thumb: no `steps()` timing, no rotation, no `text-shadow`, and no
cycle shorter than `0.5s`. The one loop that stays infinite is the hub pulse; it stops the
moment frames stop.

### 5.2 Edge and beam layers (`UsageTopologyHop.svelte`)

Keep the three visible layers and the particles, and change only their time:

1. **Conduit (always present).** One line in the state's colour and weight, unchanged: idle
   border, active `--color-ok` at `2px`, last `--color-warn` at `1.5px`, error
   `--color-danger` at `2px`.
2. **Halo (active and live).** The wide, faint `--color-ok` envelope, now a slow breathe,
   dose-capped at one glow layer (already true in the current build, and kept).
3. **Plasma and core (active and live).** The dashed flow, now at `0.9s` and `0.6s`.
4. **Orbs and sparks.** The counts, widths, phases, and the two dash patterns from
   `usage-beam.ts` stay exactly as they are, since the beam test pins them; only the travel
   duration token slows, which the component already reads from each particle's own style.

Direction is unchanged: the endpoint order plus the negative dash offset along the
normalized `pathLength="100"` carries each packet from the hop's start to its end.

### 5.3 Gateway hub node (`UsageTopologyDrawing.svelte`)

- **Idle:** `--color-surface` fill, `--color-border` border, mark in `--color-text-muted`.
  No glow, no motion.
- **Active and live:** border to `--color-ok`, the calm `router-pulse` ring, the gentle
  `router-breathe` on the mark, and the label at full opacity. The in-flight badge stays,
  and its glow is removed so the hub carries the single glow `R-13` allows.
- **Active and not live (paused or dropped):** the state colour stays, the ring stops, the
  mark stops. This is already the build's rule and it is kept.

### 5.4 Hop node cards (`UsageTopologyDrawing.svelte`)

- Add the panel's **3px left status marker** inside each node card, matching `DESIGN.md` §6:
  `--color-ok` while routing, `--color-danger` on the last error, and the neutral border
  while idle. The marker repeats the one identity motif where the operator scans most.
- Replace the `animate-ping` ring on the active dot with a solid state dot. The colour is
  the state; the ring was a second, endless bid for the same fact, and it is the loop `R-19`
  and `R-13` send away. This also returns the drawing to one mover.
- Keep `animate-node-enter` as the only card-level animation, now with the settle from §5.1.
- Keep the `transition-all duration-300` on background and border so a state change eases
  rather than snaps; it is a state transition, which the dial allows.

### 5.5 Terminals (Client, Response)

- Keep the pill shape and the state colours.
- Remove the endless `animate-router-pulse` from an active terminal. The terminal is lit by
  what crosses it; that is a colour, not a loop. This leaves the hub as the only pulsing
  element and follows `antislop-ui` on a state mark that does not need an endless pulse.

### 5.6 Gating and reduced motion

- Every moving part keeps its two conditions, unchanged: the state names the node, and
  frames are arriving (`live`). A paused or dropped stream freezes colour and motion.
- Every animated class keeps `motion-reduce:animate-none`, and every particle keeps
  `motion-reduce:hidden`, so a reader who asked for less motion gets the conduit, the state
  colour, and the marker, with nothing moving.

---

## 6. UI and UX Improvement Specification

### 6.1 Unified instrument toolbar (answers U-02, U-03)

- One surface: `bg-[var(--color-surface-2)]`, one `--color-border`, `--radius-md`, `p-3`.
- Leading side: Period, Group by, refresh. Trailing side from `sm`: the window readout
  (`from` to `to`, granularity) in `tabular-nums`, muted.
- The control's own press and hover states come from the button primitive and the state
  scale (`states.css`), not from a screen-authored utility.

### 6.2 Two-tier totals (answers U-02)

- **Lead row:** Requests, Cost (USD), Tokens (in plus out), at `text-xl font-semibold
  tabular-nums`, each with its supporting note beside it (the cost estimate note, the
  error split for Requests).
- **Support strip:** Tokens in, Tokens out, Cache read, Cache write, Error rate, Reliability
  (p50 and p95 in one cell) as a dense single-surface strip with `tabular-nums`.
- Three columns from `sm`, so the block still reads as a 3 by 3 grid per `DESIGN.md` §11.
- Reason: the operator's decision figures need more weight than their subdivisions (`R-14`).

### 6.3 Live card header (answers U-04, U-05)

- One header bar on the live card holding: the 3px status marker with the state label, the
  status sentence, and the Pause and Retry controls from the button primitive.
- The node notices render under the header as a `role="status"` line.
- The drawing sits directly below the header, so the controls and the thing they govern are
  one block.

### 6.4 Topology frame and responsive (answers U-06)

- Replace `sm:mx-16` with `sm:mx-4`, and keep the container-unit scaling that already sizes
  the drawing from the box width.
- Keep the horizontal scroller only for the widths where the geometry cannot compress, with
  `role="region"`, an `aria-label`, and `tabindex="0"` so a keyboard user can reach a
  scrolled part. A visible scroll shadow or a short "scroll to see all nodes" line makes the
  overflow honest.
- Do not introduce a second drawing at a different shape: one drawing that scales, per the
  existing container-unit design.

### 6.5 Records continuity and cross-tab jump (answers U-10, U-11)

- Give the Overview breakdown a row action that opens Records filtered by that provider or
  model through the existing URL parameters. It is a real link, so `R-26` holds.
- Reuse the toolbar idiom on Records so the two tabs read as one screen at two depths, and
  state the relationship in one line in the Records header.

### 6.6 One registry read (answers U-12)

- `UsageOverviewTab` owns the `listProviders` read and passes the resolved map and its
  notice to the live panel. The live panel keeps only its own "the drawing left these out"
  notice, which is a different fact from a failed read.

---

## 7. Responsive and Accessibility Guarantees

- **`R-03`:** the toolbar reflows to stacked controls on a phone, the totals keep a
  two-column phone state before the three-column `sm` state, the tables keep the
  `mobile-key-value-table` key-value reflow, and no element exceeds the viewport.
- **Touch targets:** every control keeps the `44px` minimum already in the build
  (`min-h-11` on the toolbar fields, the status readout, and the live controls).
- **`R-25`:** the strip and the header bar reuse existing surface and text tokens, so all
  pairs stay on the measured values in `tests/tokens/contrast.test.ts`. No new colour pair
  is introduced.
- **`R-32`:** the drawing scroller is focusable and labelled; the row action is a link; the
  disclosure tables keep their `summary` elements; focus stays on the visible accent ring.
- **`R-34`:** every surface and status token is authored in both themes, so the toolbar and
  the drawing render in light and dark without a derived colour.
- **`R-19`:** the drawing honors `prefers-reduced-motion`, and a paused or dropped stream
  stops all motion while keeping colour.

---

## 8. Palette and Aesthetic Guardrails

- **No new palette:** only `--color-surface`, `--color-surface-2`, `--color-surface-3`,
  `--color-border`, `--color-text`, `--color-text-muted`, the single `--color-accent`, and
  the status tokens `--color-ok`, `--color-warn`, `--color-danger`.
- **No neon, no cyberpunk:** no cyan or magenta, no purple-black scheme, no layered glow
  stack, no scan lines, no grid or dot background. The `feTurbulence` texture on the beam
  stays as a still texture, which the panel already uses for parity, and it is not animated.
- **`R-13`:** one glow, on the gateway hub, at the existing `25%` alpha.
- **`R-29`:** the palette stays one accent plus warm neutrals, which is the direction already
  recorded in `DESIGN.md`.
- **`R-30`:** the aim is not to copy the reference fork's look; it is this panel's
  instrument language holding the same structure.

---

## 9. Reason Log (`R-31`)

| Decision | One line reason |
| --- | --- |
| Give the live card a place at the top of the Overview body | The one live thing on the screen should not sit below four blocks of history. |
| Split the nine totals into three lead cells and a support strip | The decision figures need more weight than their subdivisions (`R-14`). |
| Fold the window readout into the toolbar surface | The query inputs and the window they produced are one statement, and one band saves a row. |
| Dress the stream state as a readout with the 3px marker, not a button box | A statement that looks pressable is a dead control (`R-26`). |
| Consolidate the stream controls into the drawing card header | The controls govern the drawing directly beneath them. |
| Keep one glow on the gateway hub | The glow dose cap is one or two elements; today it lands on every active node (`R-13`). |
| Keep one infinite loop, the hub ring, and make node state static colour | Motion applied everywhere amplifies nothing, and an endless state dot was a second bid for a fact the colour already states (`R-19`). |
| Replace `steps()` flickers, rotation, and `text-shadow` with smooth eased cadences | Stepped strobing and shaking read as a fault, not as healthy throughput (`R-19`). |
| Slow the beam to `0.6s` to `1.1s` | Direction is only legible when the eye can follow a packet from source to target. |
| Add the 3px left marker to active hop cards | It repeats the panel's one identity motif where the operator scans. |
| Reduce the topology margin from `sm:mx-16` to `sm:mx-4` | The desktop margin removed 128px of room on tablet and cramped the labels the node cap governs (`R-03`). |
| Read the provider registry once and pass it down | One screen reading the same route twice prints two notices for one cause (`C-2`). |
| Give a breakdown row a link into Records with the filter applied | The panel states the two tabs answer one question, and the path between them should be one press (`R-26`). |
| Add `tabular-nums` to every updating or stacked figure | A figure that changes width on every tick moves the row beside it. |

---

## 10. Implementation Plan

Ordered from the drawing (the change the request names first) to the layout.

1. **`app-ui/src/lib/motion.css`:** new keyframes and cycles per §5.1. Token names, class
   names, and the `motion-reduce` variants stay as they are. `beam-dash` keeps its
   `stroke-dashoffset: -100` contract.
2. **`app-ui/src/lib/components/UsageTopologyDrawing.svelte`:** remove `animate-ping` and
   the per-node glow and the terminal pulse; keep the hub pulse; add the 3px left marker to
   hop cards; keep the entry transition.
3. **`app-ui/src/lib/components/UsageTopology.svelte`:** margin and scroller change per §6.4.
4. **`app-ui/src/lib/components/UsageTotalsTiles.svelte`:** two-tier structure per §6.2.
5. **`app-ui/src/lib/components/UsageOverviewControls.svelte`:** toolbar surface and window
   readout per §6.1.
6. **`app-ui/src/lib/components/UsageLivePanel.svelte`:** header consolidation per §6.3, and
   the passed-in provider map per §6.6.
7. **`app-ui/src/lib/components/UsageOverviewTab.svelte`:** live card placement, provider
   read ownership, and the window line moving into the toolbar.
8. **`app-ui/src/lib/components/UsageGroupTable.svelte`:** row action into Records per §6.5,
   and `tabular-nums` on remaining figures.
9. **`app-ui/src/lib/components/UsageRecordsTab.svelte`:** shared toolbar idiom and the one
   relationship line per §6.5, U-11.

Tests to run after the edits: `tests/components/usage-topology-motion.test.ts`,
`tests/components/usage-topology-beam.test.ts`, `tests/schemas/usage-beam.test.ts`,
`tests/schemas/usage-topology-geometry.test.ts`, `tests/components/usage-overview*.test.ts`,
`tests/components/usage-*.test.ts`, `tests/tokens/contrast.test.ts`,
`tests/tokens/touch-targets.test.ts`, and `scrypts/gates/antislop.sh`. Any new class the
motion tests do not name must not replace a class they do.

---

## 11. Delivery Gate

Run in `DURING` mode while the screen is edited, and re-run as an `AFTER` check before the
screen ships. The gate is the `antislop` core's Delivery Gate; the rows below record how
this direction satisfies it.

| Gate item | Status | Evidence |
| --- | --- | --- |
| `R-01` colour and gradients | PASS | One coral accent, warm neutrals, no gradient, one glow on the hub. |
| `R-03` mobile responsiveness | PASS | Stacked toolbar, two-column phone totals, key-value table reflow, focusable topology scroller. |
| `R-04` icons | PASS | Existing single icon set; no sparkle, star, robot, or orb glyphs. |
| `R-06` typography | PASS | Inter at the existing scale; `tabular-nums` on figures; no monospace headings. |
| `R-07` background | PASS | Warm neutral surfaces; the `feTurbulence` texture stays a still texture. |
| `R-09` badges | PASS | The in-flight counter and the stream state carry a real status. |
| `R-10`, `R-12`, `R-13` glass, shadow, glow | PASS | No glass; shadow on floating surfaces only; one glow on the hub. |
| `R-11` radius | PASS | Existing `--radius-sm`, `--radius-md`, `--radius-full` only; no new pill shapes. |
| `R-14` card hierarchy | PASS | Lead cells and support strip give the totals a stated hierarchy. |
| `R-17`, `R-18`, `R-36`, `R-38` data honesty | PASS | Every figure comes from the API or the stream; no invented numbers, no testimonials, no claims. |
| `R-19` animation | PASS | One loop on the hub, eased one-shot entries, motion gated on live frames, reduced motion honored. |
| `R-21`, `R-34` theme | PASS | Both themes use authored tokens; the toolbar and drawing render in both. |
| `R-25` contrast | PASS | Only existing token pairs, asserted by `tests/tokens/contrast.test.ts`. |
| `R-26` interactive elements | PASS | Every control and the new row link has a real action; no dead control. |
| `R-27` UI states | PASS | The empty, loading, error, and stream states already exist and are kept. |
| `R-29` palette cap | PASS | One accent plus neutrals and status colours. |
| `R-30` no cloning | PASS | The result is this panel's instrument language, not the reference fork's look. |
| `R-31` reason per decision | PASS | §9 records one line per decision. |
| Block 3 liveliness | PASS | Dials held: calm surfaces, varied section composition, one deliberate accent, one identity motif (the 3px left marker), one focal mover on the drawing. |

Nothing in this document is applied to source until the edits in §10 are made and the tests
and the antislop gate pass on the changed files.

---

_This audit is the design record for the Usage screen. Source edits follow §10 and are
verified against the suites listed there._

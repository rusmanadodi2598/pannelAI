# app-ui

Operator panel for the KENTANG TECH pannelAI gateway. It is a thin, strict client of the management
API in `app-serv` and never touches PostgreSQL or Redis.

Spec of record: [`docs/SPEC-UI/001-SPEC-UI.md`](../docs/SPEC-UI/001-SPEC-UI.md).
API contract: [`docs/SPEC-API/001-SPEC-API.md`](../docs/SPEC-API/001-SPEC-API.md).

Status: **U2 capabilities complete**, verified locally. Screens built: `/login`, `/endpoint-keys`,
`/settings`, `/providers` and `/providers/[provider_id]`, `/combos`, `/usage`, `/quota`, `/logs`,
`/console-log`, `/changelog`, `/media-providers/[kind]`, `/proxy-pools`, `/token-saver`, `/api-docs`,
`/skills`, and `/playground`. The shell is complete: a themed sidebar with five groups and 20 nodes,
responsive across phone, tablet, and desktop. Measured 2026-09-21, the tree holds 19 routable leaves
(13 static routes plus the 6 media kinds), 1 container, and no row marked `Planned`; the one `planned`
flag left in `src/lib/navigation.ts` sits on the Media Provider container, which renders as a disclosure.
A navigation item without a route is a defect, so a row carries an `href` only when its route file
exists. All nine capabilities the U2 row of SPEC-UI §12 lists are landed; two of that row's four exit
criteria are verified against a running `app-serv`, and the remaining two need a provider account and a
browser click-through rather than unbuilt work (§12 records both).

## Requirements

| Tool | Version used here | Notes                                                           |
| ---- | ----------------- | --------------------------------------------------------------- |
| Bun  | 1.3.14            | Installer, dev server, test runner, and the production process. |
| Go   | not required      | Only `app-serv` needs it.                                       |

## Commands

```bash
bun install          # install dependencies
bun run dev          # dev server on http://127.0.0.1:5173
bun run build        # production build into build/
bun run start        # run the built server on Bun, on APP_PORT, and print the boot banner
bun run check        # svelte-check with the native TypeScript 7 compiler
bun run test         # unit tests (Vitest, jsdom)
bun run format       # Prettier write
bun run lint         # Prettier check
bun run lint:ts      # ESLint (types, unused bindings, floating promises)
```

`bun run start` reads `APP_ENV` and `APP_PORT` from `.env` (defaults `development` and `3000`), prints both
in its banner, and binds `APP_PORT`; `PORT` wins when both are set, so an adapter-level override is not
silently replaced. A value the schema refuses stops the process before the server module loads. `bun run
dev` keeps its own port (5173): the dev server and the built server are different processes with different
jobs, and a shared port would make them fight over one socket.

`bun run lint` and `bun run lint:ts` are separate on purpose: Prettier owns
formatting, ESLint owns the problems a formatter cannot see (an explicit `any`,
a binding left unused by a rename, an `eslint-disable` that no longer suppresses
anything).

### Bun on a CPU without AVX2

Bun ships two x86-64 builds. The default one requires AVX2 and dies with SIGILL
on older CPUs (Sandy/Ivy Bridge era) even for `bun --version`. Install the
baseline build, which does not:

```bash
curl -fsSL -o bun.zip \
  https://github.com/oven-sh/bun/releases/latest/download/bun-linux-x64-baseline.zip
unzip bun.zip && install -m755 bun-linux-x64-baseline/bun ~/.local/bin/bun
```

`scrypts/gates/panel-check.sh` probes Bun by running it, so it uses a working
baseline build when PATH has one and reports the fallback to npm/node when it
does not.

Set `PANEL_API_TARGET` before starting (see `.env.example`). A missing or malformed value fails the
first `/api/v1` request with a message naming the variable.

## How requests reach the gateway

The panel serves one origin. Its own server forwards `/api/v1/*` to `PANEL_API_TARGET`
(`src/hooks.server.ts` and `src/lib/server/proxy.ts`), so the browser never makes a cross-origin
request and the HttpOnly session cookie stays `SameSite=Lax`. Development and production share that
single path, which is why there is no dev-only CORS proxy in `vite.config.ts`.

## Recorded decisions

| Decision                                   | Reason                                                                                                                                                                                                                                                                                                             |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Official Node adapter, executed by Bun     | The Bun-specific SvelteKit adapter is still on a 1.0.x release; `docs/SPEC-UI/001-SPEC-UI.md` §10.1.2 allows this fallback, and the runtime stays Bun. Verified by the boot banner printing `Bun 1.3.14`.                                                                                                          |
| TypeScript 7 with `typescript` 6 alongside | `svelte-check` requires both packages before it will use the TypeScript 7 compiler, and it needs the `--tsgo` flag. `typescript` resolves to 6.0.3 for the tool, and `@typescript/native` is an alias of TypeScript 7.0.2. This is the setup `svelte-check` itself documents.                                      |
| Zod 4.6.5 as the only validator            | Owner decision, `docs/SPEC-UI/001-SPEC-UI.md` §7. One schema module per resource feeds forms, URL parameters, and response parsing. No ad-hoc checks in components.                                                                                                                                                |
| `@lucide/svelte` icons, no emoji           | The set the component layer ships with. The older `lucide-svelte` package is deprecated on npm in favour of this one. Every navigation icon has a written reason in `src/lib/icons.ts`, and `tests/navigation/icons.test.ts` fails the build when a row has no icon, an icon has no row, or the reason is missing. |
| shadcn-svelte as the primitive layer       | Owner decision. `src/lib/primitives/` is generated by the component layer's CLI and is not hand-edited, with one exception: the sidebar rail width was changed from 48px to 64px so a 44px touch row is not cramped, and that change is recorded in `DESIGN.md` §11.                                               |
| Navigation as data, not markup             | `src/lib/navigation.ts` holds the tree, so a row carries an `href` only when a route exists. R-24 becomes a property of the type rather than a review habit, and `tests/navigation/navigation.test.ts` cross-checks the tree against the routes discovered on disk.                                                |
| Design direction from `DESIGN.md`          | The palette, typeface, radius and elevation scale, and identity motif come from `DESIGN.md` at the repository root, written 2026-09-17. `src/app.css` implements it and `tests/tokens/contrast.test.ts` measures it. The earlier placeholder state is closed.                                                      |
| Inter self-hosted                          | `DESIGN.md` §4 chose Inter for legibility at 12px in dense tables and for parity with the legacy panel. It ships as one variable file at `static/fonts/InterVariable.woff2` because a font CDN is excluded by spec §11.7.                                                                                          |
| Token bridge for the primitive layer       | The panel's own vocabulary is `--color-*`. The primitive layer reads `--background`, `--primary`, `--sidebar`. Those are aliases pointing back at `--color-*` in an `@theme inline` block, so a colour is decided once and there is no second palette to keep in sync.                                             |

## Layout

```
src/lib/api/         the only place that calls fetch: client, error mapping, one module per resource
src/lib/schemas/     Zod schemas: field primitives, sanitization transforms, response contracts
src/lib/server/      server-only code: environment validation and the /api/v1 forwarder
src/lib/components/  panel components (shell, sidebar, header, dialogs, table rows, forms)
src/lib/primitives/  shadcn-svelte components, generated, not hand-edited
src/lib/stores/      session, theme, and sidebar preference state
src/lib/navigation.ts  the sidebar tree as data, five groups and 20 nodes
src/lib/icons.ts     the one icon map, one written reason per icon
src/lib/utils.ts     class-name helper (clsx plus tailwind-merge)
src/app.css          the token layer: DESIGN.md §3 to §5
assets/static/       owner-supplied source assets (the logo original)
static/              served assets: circular logo mark, favicon, self-hosted Inter
src/routes/          one directory per route from the spec
tests/               table-driven unit tests, plus token and navigation assertions
tests/support/       shared test helpers: the seeded corpus generator and the table runner
```

## Verification state

Twenty-one passes are recorded here. The first is the U0 scaffold, measured 2026-09-16. The second is the
shell and sidebar work, measured 2026-09-18, and it is the R-35 click-through with its outcomes per
element. The third is the Token Saver and Proxy Pools pair, measured 2026-09-20. The fourth is the Media
Provider screen, measured the same day. The fifth is the provider detail model writes, measured the same
day. The sixth is the alias set, measured the same day. The seventh is the OAuth section, measured the
same day. The eighth is the combo test action, measured the same day. The ninth is the quota budget caps,
measured the same day, and it closes U2 on the panel side. The tenth is the API Docs screen, measured
2026-09-21. The eleventh is the Skills screen, measured the same day, and it closes F2 of
`docs/DRAFT/007-UI-ENDPOINT-READINESS.md`. The twelfth is the Playground Chat screen, measured the same
day, and it closes F3 of that draft. The thirteenth is the Changelog screen, measured the same day, and it
closes F4 of that draft. The fourteenth is the readiness batch F5 to F9 of the same draft, measured the
same day: the five MEDIUM findings, each as its own commit, with the gates below measured on the tree that
carries all five. The fifteenth is the readiness pair F10 and F11 of the same draft, measured the same day:
the §8.4.4 dirty guard with its §8.4.5 amendment, and the document numbers re-measured with the commands
that produce them. The sixteenth is the API base dialog and the one-time key copy, measured the same day:
the header's address strip replaced by a tabbed dialog, and both copy controls moved onto the shared
control that reports a refused write. The seventeenth is the two defects the owner's click-through of that
surface found, measured the same day: the address is read from the panel server instead of derived from the
browser, and the copy control writes on an origin without the async clipboard API. The eighteenth is the
custom provider surface, measured 2026-09-22: the two compatible provider types the embedded registry cannot
carry, built as operator-created nodes through §7.4, with the CodeBuddy pair the owner asked for filed as an
app-serv register rather than drawn in the panel. The nineteenth is the panel runtime config, measured the
same day: `APP_ENV` and `APP_PORT` are parsed and validated at boot, and the port the built server binds
comes from `APP_PORT`. The twentieth is the usage live surface, measured 2026-09-22: the stream reader, the
provider topology, and the page-size URL, with the CLOSED status withheld from two of the three because the
gateway does not serve the route they are built against. The twenty-first is the node motion on that
drawing, measured the same day: the active edge flows, the active node takes the status colour and the
reference fork's own soft glow, the gateway counts what is in flight, and every moving part stops unless
frames are arriving, with the browser click-through the previous pass recorded as blocked now recorded
against a contract double.

### Node motion, 2026-09-22

Run with Bun 1.3.0 (`bun --version`; the earlier pass rows state 1.3.14, which no binary on this machine
reports, filed as F13 of `docs/DRAFT/007-UI-ENDPOINT-READINESS.md`). The owner's item was the motion half of
the live drawing, filed as `docs/DRAFT/013-USAGE-NODE-MOTION-READINESS.md`: F1, the parity gap against the
reference fork; F2, motion running without a frame; F3, the glow's collision with R-13's dose cap. The
chosen shape is parity, so the active edge carries a moving dash, the active node takes the status colour
with the reference's own glow, and the gateway carries the in-flight count. One rule the reference does not
have was added, and it is the one that keeps the drawing honest: motion exists only while frames are
arriving, so a paused, connecting, or unavailable stream keeps the last known state in colour and stops
every moving part.

| Check             | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `bun run test`    | 2448 tests passed across 138 files (the last recorded pass was 2435 across 137, so this pass adds 13: eight in `tests/components/usage-topology-motion.test.ts`, one in `tests/components/usage-live-drawing.test.ts`, and four from the two new text pairs in `tests/tokens/contrast.test.ts`, which are measured once per theme)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `bun run build`   | succeeds, exit 0, output in `build/`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| Live pass         | a recorded browser click-through, which the previous pass could not produce: headless Chrome 150.0.7871.114 drove the built panel against the gateway built from HEAD plus a contract double for the live route, because the gateway still serves no `GET /api/v1/usage/live` (`grep -rn "usage/live" app-serv/` returns nothing at `e5b3016`, and `origin/main`, which now carries the merged `feat/app-ui-u0` branch, has no `app-serv` tree at all). Ten acts, screenshots and measured values kept: the panel's own form signs in; `/usage` shows `Connecting` with six nodes and no motion; frames arrive and exactly one edge flows (`animation-name: flow`, `stroke-dasharray: 3px, 3px`, `stroke-dashoffset` moving between captures, node border and label `rgb(20, 116, 58)`, glow `0 0 16px` at 25% of the status colour, gateway chip `1`); `Pause live updates` stops the flow and the pulse while the node keeps its colour and the chip its count; resume brings both back; the double closing the stream renders `Unavailable` with "The gateway closed the live stream. Another attempt is scheduled."; spending the retry budget renders "The live stream answered 503. Retrying has stopped. Use Try again to reconnect."; `Try again` recovers to `Live`; the dark theme flips the whole treatment (`rgb(92, 201, 123)`, glow theme-aware); and a 390px viewport measures `scrollWidth == clientWidth == 390`. Baseline re-counted before and after and matched exactly (`usage=0 logs=0 keys=0 endpoints=0 upkeys=0 nodes=2 caps=0 settings=0 auth_null=0 media_settings=0 proxies=0`), servers stopped by captured PID, and the pass records one setup fact: the panel refused to boot while `app-serv`'s `.env` was exported into its environment (`PANEL_BOOTSTRAP_PASSWORD: Unknown panel variable`), which is the env strictness the nineteenth pass built, so the panel was restarted with a clean environment |
| File size         | the largest file this pass touched is 207 lines (`tests/tokens/contrast.test.ts`, under the 220 warning); `src/lib/components/UsageTopology.svelte` is 201, `src/app.css` 189, `UsageLivePanel.svelte` 186, and the three test files are 149 each. No pre-existing over-cap file was touched: the twenty test files over the 250 ceiling listed in the previous pass are unchanged, and this pass adds no file to that list                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| Text hygiene      | 0 em dashes in the new and edited files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |

What changed, and what is deliberately not claimed:

| Behaviour                                        | Detail                                                                                                                                                                                                                                                                                                                                                                                               |
| ------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The edge says where traffic is going             | The active edge carries `stroke-dasharray: 3 3` in the path's own units and one `stroke-dashoffset` cycle of exactly one period, so the dash marches toward the provider without a seam. `pathLength="100"` normalizes the pattern because the drawing stretches one viewBox across the box; without it a dash would be longer on one axis than the other.                                           |
| The active node is marked, not just dotted       | Border and label take `--color-ok`, the dot pulses, and the node carries the reference's soft glow (`0 0 16px` at 25% of the status colour). R-13's one-or-two-element dose cap is exceeded when more than two providers route at once, which the owner chose as parity; the override and its bound (active state only, nothing else on any screen glows) are recorded in DESIGN.md §11.             |
| The gateway counts what is in flight             | The chip shows the number of in-flight entries, not the number of active nodes, and it is absent at zero rather than showing a `0`. It is state rather than motion, so it stays while the stream is paused.                                                                                                                                                                                          |
| Motion is reserved for a live stream             | The drawing takes a `live` prop from the connection's own status, and the pulse and the flow both require it. A paused or dropped stream keeps the last known state in colour and stops every moving part, which is what makes "no activity is claimed without a frame" structural rather than a rule someone remembers (R-36).                                                                      |
| Reduced motion is honoured for every moving part | The pulse keeps its `motion-reduce:hidden` and the flow adds `motion-reduce:animate-none`, so a reader who asked for less motion gets a still drawing with the same states.                                                                                                                                                                                                                          |
| The words keep up with the drawing               | The figure's caption now names the moving dash, the soft glow, and the count, because the drawing is `aria-hidden` and the sentences beside it are its only record for assistive technology.                                                                                                                                                                                                         |
| Limit                                            | The route the motion is built for still does not exist: the live pass's frames came from a contract double in front of the real gateway, so what is proven is the panel's half end to end in a browser, not the gateway's route. CLOSED stays withheld from F2 and F3 of `docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md`, and the real-route click-through stays outstanding until `app-serv` serves it. |

### Usage live surface, 2026-09-22

Run with Bun 1.3.0 (`bun --version`; the earlier pass rows state 1.3.14, which no binary on this machine
reports, filed as F13 of `docs/DRAFT/007-UI-ENDPOINT-READINESS.md`). The owner's item was
`docs/DRAFT/010-USAGE-ENDPOINT-READINESS.md`'s remaining panel work: the `per_page` URL the Records tab
silently ignored (F12 there, F1 here), the live reader the Usage screen never had (F2), and the provider
topology with the motion decision left unwritten (F3). SSE parity was chosen over polling, the indicator
pulses like the reference fork with the panel's own drawing, and the session's scope was `app-ui` alone.
One fact decides the status: the gateway does not serve `GET /api/v1/usage/live` yet (F4), so the panel
is built against the planned contract, states the absence rather than a live claim, and **the CLOSED
status is withheld** until a live pass records a real frame.

| Check             | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `bun run test`    | 2435 tests passed across 137 files (the last recorded pass was 2303 across 125, so this pass adds 132 tests in twelve new test files); nine of the files this pass wrote were reformatted by Prettier after that run, and the thirteen test files that cover them were re-run green (141 tests) rather than restarting the suite, because a reflow changes whitespace and not logic                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `bun run build`   | succeeds, exit 0, output in `build/`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| Live pass         | 17 checks, 0 failures, against a booted pair: `app-serv` built from HEAD with `git archive` (at the time of the pass that was `c2b8f1e`; the working tree carries another team's in-flight `app-serv` work, so a working-tree build would have compiled it) and the panel started detached on port 3000 with `PANEL_API_TARGET` exported. There is no browser click-through, because the panel is client-rendered (`ssr = false`), so the pass drove what a click would drive: `openUsageLive` reported `unavailable` with the gateway's own sentence and zero frames, and the retry it had scheduled was released by `stop` (0 reports over 2.5 s); the URL parser corrected `per_page=100` and `cleanedUsageSearch` dropped it; the gateway was measured honouring `per_page=100` (200, `meta.per_page` 100) and refusing `per_page=101` (`VALIDATION_ERROR "per_page must be at most 100"`), which is what makes the panel's correction load-bearing; the node set came back as 6 of the registry's 86 rows, drawn idle at distinct positions. Baseline re-counted and matched exactly (`usage=0 logs=0 keys=0 endpoints=0 upkeys=0 nodes=2 caps=0 settings=0 auth_null=false media_settings=0 proxies=0`); the login used the `.env` password, so no hash was rotated. The state source the route needs landed after this pass (`62d21e1`, the app-serv side of draft 010 F4: a Redis Pub/Sub publisher and a real subscriber), and the route itself is still absent at HEAD, which a grep for `usage/live` over `app-serv/` shows |
| File size         | the largest source this pass wrote is 222 lines (`src/lib/usage-live.ts`, two over the 220 warning and 28 under the ceiling, where the comments carry the connection's rules); every other file it wrote is under the warning, the largest being `UsageOverviewTab.svelte` at 217. Three pre-existing over-cap test files were touched: `tests/components/usage-overview.test.ts` (296 → 306) was split by concern into 165 plus `usage-overview-url.test.ts` (113) and a shared stub (96); `tests/schemas/usage-view.test.ts` (447) had the pass's own 133 lines extracted into `usage-search-page-size.test.ts` (153) and is left at 438; `tests/components/usage-records.test.ts` (295 → 336) is left where it was and reported, because moving the pass's three cases would have duplicated its 79 lines of fixtures. Twenty test files were already over the 250 ceiling at HEAD (582, 483, 447, 398, 372, 355, 326, 319, 310, 296, 295, 290, 284, 280, 279, 273, 269, 267, 262, 251) and this pass does not retrofit them                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| Text hygiene      | 0 em dashes in the new and edited files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |

What changed, and what is deliberately not claimed:

| Behaviour                                        | Detail                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The stream is read by `fetch`, not `EventSource` | An `EventSource` retries on its own and never says why it failed, so a route that does not exist (today's state) and a dropped connection would look identical and the screen could not state a cause. A fetch answers with a status, and the panel already reads the playground's stream by fetch, so there is one streaming technique rather than two.                                                                                                                          |
| The stream cannot write an aggregate             | The live state has no field for a total or a bucket, so the merge is structurally unable to touch what the REST reads returned; the rendered half of that proof gives a frame `requests: 999999` and asserts the tiles still show 120. The live panel is also mounted outside the aggregate read's loading, error, and empty branches, so a failed summary still shows what the gateway is routing.                                                                               |
| `Live` is a claim with a source                  | The label comes from the connection's own state, and a reconnect is not live until it delivers a frame. A gateway that answers 404 is stated as unavailable with that reason, never replaced by a poll under the same label (R-36).                                                                                                                                                                                                                                               |
| Hidden tabs and pauses stop the read             | §8.6.1's three rules are applied to the stream as a medium: the state is labelled, the read is pausable, and it stops while the tab is hidden and reads once on the way back. The retry budget is bounded at both ends, and after the last delay the screen offers the retry control instead of looping.                                                                                                                                                                          |
| The drawing is a derivation, not a layout        | `configuredProviders` turns the registry into the node set (a provider with an endpoint or no auth), and `topologyNodes` turns that set plus the live states into positions on an ellipse, as percentages, so no measurement is needed and the same numbers place the nodes and draw the edges.                                                                                                                                                                                   |
| The pulse ends                                   | DESIGN.md MOTION 1 is amended with one stated exception: the indicator pulses only while a provider has a request in flight, so it is a state that ends rather than a loop (§2.1, R-19). Every fact it carries is also stated in words beside the drawing, and the drawing itself is `aria-hidden`.                                                                                                                                                                               |
| The page size the URL asked for                  | The screen reads 25 rows while the API accepts up to 100, so a `per_page` from a shared link is corrected to 25, the correction is visible until a filter changes, the parameter is removed from the URL, and the read happens once, after the URL is clean (§7.1.1, §8.10). The live pass measured both halves: the gateway honours `per_page=100`, and the panel does not.                                                                                                      |
| Limit                                            | No browser click-through, and no frame has ever reached the screen: the panel is client-rendered (`ssr = false`), and the gateway does not serve the route it reads (F4), so the browser click-through for this surface is outstanding. It is verified by 132 new tests, 17 live checks against the booted pair, and the `unavailable` state rendered for the reason the gateway actually gave. CLOSED is withheld from F2 and F3 in `docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md`. |

### Panel runtime config, 2026-09-22

Run with Bun 1.3.0 (`bun --version`; the earlier pass rows state 1.3.14, which no binary on this machine
reports, filed as F13 of `docs/DRAFT/007-UI-ENDPOINT-READINESS.md`). The owner's `.env` template declares
`APP_ENV` and `APP_PORT` under `[P0][TAG:RUNTIME]`, and neither was read by anything: the built server bound
adapter-node's own default whatever the file said, so `APP_PORT=3001` would have served 3000 with nothing on
screen pointing at the file.

| Check             | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `bun run test`    | 2303 tests passed across 125 files (the last recorded pass was 2288 across 124, so this pass adds 15 tests in one new file, `tests/schemas/env-runtime.test.ts`)                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `bun run build`   | succeeds, exit 0, output in `build/`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| Live boot pass    | five cases against the built server, each started with `bun run start` and stopped by PID: `APP_PORT=eighty` exits 1 with `APP_PORT: must be a port number, digits only`; `APP_ENV=staging` exits 1 with `APP_ENV: must be development, production, or test`; `APP_PORT=3131` prints `Listening on port 3131`, binds 3131 (HTTP 200) and leaves 3000 unanswered; `PORT=4141 APP_PORT=3131` binds 4141; with no override the `.env` value 3000 answers. No browser click-through applies here: this pass adds no screen and no control, and the surface it changes is the boot path, which those five cases drive directly |
| File size         | largest file this pass touched is 195 lines (`tests/schemas/env-and-contracts.test.ts`); `src/lib/schemas/env.ts` is 157, `scripts/boot-log.ts` is 38, and the new table lives in `tests/schemas/env-runtime.test.ts` at 77 because the first draft pushed the existing test file to 263, over the 250 ceiling                                                                                                                                                                                                                                                                                                            |
| Text hygiene      | 0 em dashes in the new and edited files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |

What changed, and what is deliberately not claimed:

| Behaviour                                                | Detail                                                                                                                                                                                                                                                                                                      |
| -------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The runtime variables are parsed where they are consumed | `APP_ENV` and `APP_PORT` live in `src/lib/schemas/env.ts` beside the gateway target, and `scripts/boot-log.ts` validates them before the server module loads. The runtime fields are declared once, so the preload's subset and the server's `envSchema` cannot drift into two definitions of a valid port. |
| A value the schema refuses stops the boot                | A port that cannot be parsed is not bootable, so the preload exits 1 with the variable name and the reason rather than leaving a panel on a port nobody asked for. A missing gateway target is deliberately not pulled forward: that stays a per-request state the panel reports (§6.15).                   |
| The port comes from `APP_PORT`, and `PORT` wins          | adapter-node reads `PORT` and the env template calls it `APP_PORT`, so the preload maps one to the other only when `PORT` is unset. An explicit adapter override stays authoritative instead of being silently replaced.                                                                                    |
| Dev keeps its own port                                   | `bun run dev` binds 5173 from `vite.config.ts`. The dev server and the built server are different processes with different jobs, and one shared port would make them fight over one socket.                                                                                                                 |
| The banner states what is running                        | The first line now carries the runtime environment and a second line carries the port (SPEC-UI §10.1 item 5), so the operator reads the resolved configuration instead of the file.                                                                                                                         |

### Custom provider surface, 2026-09-22

Run with Bun 1.3.0 (`bun --version`; the earlier pass rows state 1.3.14, which no binary on this machine
reports, filed as F13 of `docs/DRAFT/007-UI-ENDPOINT-READINESS.md`). The owner's item was that the Provider
screen was missing four providers: OpenAI Compatible, Anthropic Compatible, CodeBuddy Int, and CodeBuddy CN.
Two of them were buildable from the panel
and two were not, and that split is the substance of this pass. A compatible provider is not a registry
entry: §7.4 has the operator define one as a _node_, the gateway synthesizes a provider entry from it, and
the node's own routes are what the panel had never called. CodeBuddy CN and CodeBuddy Int are the opposite
case, registry entries the embedded registry does not carry at all, so they cannot be built from the panel;
the gap is filed as `docs/DRAFT/011-CODEBUDDY-PROVIDER-READINESS.md` rather than drawn as a screen that
shows providers no request can reach.

| Check             | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `bun run test`    | 2288 tests passed across 124 files (the last recorded pass was 2204 across 116, so this pass adds 84 tests in the eight test files it created)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `bun run build`   | succeeds, exit 0, output in `build/`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| Live wire pass    | not run in this pass. All six node routes are registered on the gateway (`app-serv/internal/router/router.go:125-130`), so the panel is not ahead of the gateway, but nothing here was driven against a booted gateway in a browser: this surface joins the outstanding click-through that F12 of `docs/DRAFT/007-UI-ENDPOINT-READINESS.md` records, and that register's count moved from nine of seventeen pass sections to ten of eighteen in this commit, with the command that produces it                                                                                                                                           |
| File size         | largest source this pass touched is 214 lines (`src/lib/schemas/provider-node.ts` and `src/lib/components/CustomProviderCard.svelte`); the largest new test file is 190 (`tests/schemas/provider-node-draft.test.ts`), and every file this pass created is under the 220 warning line; two files crossed the 250 ceiling while the pass was being written (the schema test at 303 and the route test at 255) and were split by concern, which is why the slice ships nine test files and four shared support modules where five test files were written first; the largest source in `src/` is still 220 (`CombosTab.svelte`, untouched) |
| Text hygiene      | 0 em dashes in the new and edited files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |

What changed, and what is deliberately not claimed:

| Behaviour                                             | Detail                                                                                                                                                                                                                                                                                                               |
| ----------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The section reads its own route                       | `GET /api/v1/provider-nodes` answers every node with no paging envelope, so the section reads the whole set and is not narrowed by the category filter, which filters the registry and not this set. Deriving it from the registry page would have meant filtering a page of providers the API never marks as nodes. |
| One refresh control, both reads                       | The page's `RefreshControl` re-reads the registry and the node set, because §8.6.2 says the control repeats the read the screen is showing and this screen shows two sets. Each read owns its own failure state, so a node outage leaves the table standing rather than blanking the screen.                         |
| A node is told apart by its id prefix                 | `openai-compatible-` and `anthropic-compatible-` are the public contract (§7.4), and the provider response carries no flag for a node, so the detail screen reads the prefix it was routed by.                                                                                                                       |
| The row states what the table cannot                  | A node appears in the registry table too, because the gateway synthesizes an entry from it. What that entry does not carry is the prefix its models are addressed by, the endpoint label, and the URL the gateway will call, so those are what the section states.                                                   |
| Write actions live on the node's own screen           | Edit, Test, and Delete are on the detail card rather than in the list, which is where the reference puts them and where the node's other facts are read.                                                                                                                                                             |
| The edit form states the api type                     | Type and api type are identity in §7.4's patch, so the form states them and says they are not editable rather than drawing a control that cannot act (R-26).                                                                                                                                                         |
| The probe's credential is optional                    | An empty field sends `{}` rather than an empty string, because §7.4 reads an absent credential as a node whose upstream needs none, and the answer is a state with a latency rather than an HTTP failure.                                                                                                            |
| The joined URL is shown while typing                  | The gateway appends the path to whatever base URL is stored, so a base URL one segment off is a request to the wrong path. Showing the join makes the field readable before the node exists.                                                                                                                         |
| The gateway's refusals are rendered as it stated them | A taken prefix (CONFLICT on create) and a node an endpoint still references (CONFLICT on delete) are the gateway's answers, not the panel's predictions, so both are rendered with the dialog open and the list is not re-read until the write actually lands.                                                       |
| CodeBuddy is a register, not a section                | Both are registry entries rather than nodes, and the registry's own entry is hidden, so rendering them in the panel would show providers no request can reach, which is the claim R-38 rules out.                                                                                                                    |
| Tests                                                 | The stub stores nodes and applies every write, because the rule this slice turns on is about state: the panel re-reads after a write (§8.6.3), and a stub that answered the same document would let a screen that never re-reads pass. Both refusals are expressible in it.                                          |
| Limit                                                 | No browser click-through, and no live pass against a booted gateway. The routes exist and are registered, but nothing here has been driven from a browser, so the surface joins F12.                                                                                                                                 |

### API base dialog and the one-time key copy, 2026-09-21

Run with Bun 1.3.14. The owner asked for the header's API base to be presented properly rather than as an
address and a copy control squeezed into the strip, and for the one-time key's copy control to work after a
key is created. Underneath, both were the same defect: a refused clipboard write. `navigator.clipboard` is
absent outside a secure context, which is how the panel is opened on a LAN address, and the header kept its
failure sentence inside an `sr-only` span while the one-time key modal did not catch the failure at all. A
refused copy therefore read as a dead button in both places.

| Check             | Result                                                                                                                                                                                                                                                                                                                                                                                           |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                                                             |
| `bun run test`    | 2191 tests passed across 113 files (the previous pass was 2185 across 111), 6 of them new in this pass across 2 new files                                                                                                                                                                                                                                                                        |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                                                                                                                                                                                                             |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                                                                                                                                                                                                   |
| `bun run build`   | succeeds, exit 0, output in `build/`                                                                                                                                                                                                                                                                                                                                                             |
| Live wire pass    | not run in this pass. Both changes are client-side, and the snippets the dialog hands out name routes the router already registers (`GET /api/v1/models` on the data plane with `Authorization: Bearer`, §7.15) rather than a new one. Superseded: the owner ran the click-through afterwards and it found two defects, recorded in the next section                                             |
| File size         | largest file this pass touched is 171 lines (`tests/components/gateway-keys-tab.test.ts`); the new component is 60 (`src/lib/components/ApiBaseDialog.svelte`) and the new strings module 38 (`src/lib/strings/api-base.ts`); `PanelHeader.svelte` came back from 93 to 68 by moving the copy control into the dialog; the largest source in `src/` is still 220 (`CombosTab.svelte`, untouched) |
| Text hygiene      | 0 em dashes in the new and edited files                                                                                                                                                                                                                                                                                                                                                          |

What changed, and what is deliberately not claimed:

| Behaviour                | Detail                                                                                                                                                                                                                                                                                                                                                                                                             |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| One copy control         | `CopyButton.svelte` is now the only control that writes the clipboard on these two paths. It reports "Copied." or "Copy failed. Select the text and copy it.", the sentence is visible rather than announced only, and the value stays on screen beside it so a refused write is still selectable.                                                                                                                 |
| The API base is a dialog | The header keeps one button (`aria-haspopup="dialog"`) and `ApiBaseDialog.svelte` states the address in three tabs: the base URL, a cURL line against `GET /api/v1/models`, and the two names an OpenAI-compatible client reads. Each tab carries its own copy control, and the tabs are the panel's own `PanelTabs`, so the panel has one tab language rather than two.                                           |
| Derived, not written out | All three forms are composed from `location.origin` plus `/api/v1`, the origin the panel server forwards to the gateway (§3.1). The key in the two snippets is the placeholder `sk-...`, and the sentence under each snippet says where the real one comes from: the panel holds no gateway key. Superseded in the next pass: `location.origin` is the panel, and a bind-all address is not one a client can call. |
| The dismissal rule       | §6.2 said the modal opens up "after the copy control is used", which neither a reader nor the code could check. It now says "reports success", and the code matches: `CopyButton` fires an optional `oncopied` callback only after the write resolved, and a refused copy leaves the modal gated because the plaintext exists nowhere else.                                                                        |
| Tests                    | The three tabs are compared value by value against what their copy control wrote, both refusal shapes (the API absent, and the write rejecting) are asserted as readable rather than `sr-only`, and the header's dialog is asserted closed until used, open, and closed from its own control.                                                                                                                      |
| Limit                    | No browser click-through. The header's dialog is new surface, so the 2026-09-18 shell row for the old inline control is marked superseded above, and this surface joins the outstanding click-through that F12 of the register records.                                                                                                                                                                            |

### The click-through's two findings, 2026-09-21

Run with Bun 1.3.14. The owner ran the previous pass's surface in a browser, which is the R-35 click-through
that pass recorded as outstanding, and it found two defects. Both are fixed here, and the fixes themselves
are not yet re-verified in a browser.

1. **The copy control still reported a failure.** The first fix only had the async path, and on the origin
   the panel normally runs at `navigator.clipboard` is absent rather than refusing, so the button answered
   "Copy failed. Select the text and copy it." The control now tries the async API and falls back to a
   selection-based write, and reports a failure only when both paths fail.
2. **The address was the panel's own.** The dialog composed `location.origin` plus `/api/v1`, so a panel
   opened at `http://0.0.0.0:3000` advertised exactly that, which no client can call: the gateway answers on
   its own host and port. The address is now read from the panel server, which is the component that holds
   it, and the dialog states the read rather than guessing it.

| Check             | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `bun run test`    | 2204 tests passed across 116 files (the previous pass was 2191 across 113): 17 tests in 3 new files, 3 rows added to the dialog test, and 7 rows moved out of `tests/server/playground-context.test.ts` into the new `tests/server/session.test.ts`, a net 13                                                                                                                                                                                                                                                              |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `bun run build`   | succeeds, exit 0, output in `build/`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| Live wire pass    | the owner's pass is the evidence for both findings and the only browser evidence: the fixes are not re-verified in a browser, so R-35 stays outstanding for this surface, and the click-through that exists describes the pre-fix behavior, recorded in F12 of `docs/DRAFT/007-UI-ENDPOINT-READINESS.md`                                                                                                                                                                                                                   |
| File size         | largest file this pass touched is 184 lines (`tests/components/api-base-dialog.test.ts`); the new server rule is 54 (`src/lib/server/api-base.ts`), the new route 11 (`src/routes/api-base/+server.ts`), the new client module 51 (`src/lib/api/api-base.ts`), the extracted session check 40 (`src/lib/server/session.ts`), and the new schema 12 (`src/lib/schemas/api-base.ts`); `CopyButton.svelte` is 99 and `PanelHeader.svelte` stays 68; the largest source in `src/` is still 220 (`CombosTab.svelte`, untouched) |
| Text hygiene      | 0 em dashes in the new and edited files                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |

What changed, and what is deliberately not claimed:

| Behaviour                  | Detail                                                                                                                                                                                                                                                                                                                                     |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Two write paths            | `CopyButton.svelte` tries `navigator.clipboard.writeText` first and falls back to a hidden textarea plus `document.execCommand('copy')`, which is the only write that works on an insecure origin. `execCommand` is deprecated and is used deliberately: the alternative is a control that cannot work where the panel is normally opened. |
| The address is served      | A new panel route `GET /api-base` answers the origin of `PANEL_API_TARGET` plus `/api/v1`. It sits outside `/api/v1` so the forwarder never swallows it, it is session-gated, and it is `no-store` because the address comes from the panel's environment and a restart can change it.                                                     |
| The dialog states the read | `ApiBaseDialog.svelte` fetches on every open, says it is reading, and on a refusal shows the gateway's own sentence with a `Try again` control. It never falls back to `location.origin`, because an address that looks right and is not is worse than no address.                                                                         |
| The session check moved    | `sessionIsValid` moved from `src/lib/server/playground-context.ts` to `src/lib/server/session.ts` when the API base route became its second caller.                                                                                                                                                                                        |
| Tests                      | The two write paths are each driven, plus the case where neither works; the dialog is driven through loading, loaded, refused, and a retry that is answered; and the server rule covers the target origin, a path in the target, a refused session, a misconfigured panel, and the `no-store` header.                                      |
| Limit                      | No browser re-verification. The route is new surface, so the click-through the owner ran describes the pre-fix behavior only.                                                                                                                                                                                                              |

### Readiness findings F10 and F11, 2026-09-21

Run with Bun 1.3.14. This pass closes the last two LOW findings of
`docs/DRAFT/007-UI-ENDPOINT-READINESS.md`. The owner answered D5 on 2026-09-21: implement the guard for
the forms that hold a draft, and amend §8.4.5 with a written reason instead of adding blur validation.

| Check             | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `bun run test`    | 2185 tests passed across 111 files (the previous pass was 2168 across 108), 17 of them new in this pass across 3 files                                                                                                                                                                                                                                                                                                                                         |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `bun run build`   | succeeds, exit 0, output in `build/`                                                                                                                                                                                                                                                                                                                                                                                                                           |
| Live wire pass    | not run in this pass. The guard is a client-side rule and neither finding needed an `app-serv` change                                                                                                                                                                                                                                                                                                                                                          |
| File size         | largest source in `src/` is 220 lines (`src/lib/components/CombosTab.svelte`, untouched, exactly at the warning line); the largest new test file is 178 lines (`tests/components/dirty-forms.test.ts`), and every file this pass touched is under the warning; `ComboEditor.svelte` came back from 218 to 201 by moving its strategy fields to `ComboStrategyFields.svelte` (60 lines), because the guard wiring would have pushed the editor past the warning |
| Text hygiene      | 0 em dashes in the new and edited files                                                                                                                                                                                                                                                                                                                                                                                                                        |

What F10 changed, and where it states a limit rather than hiding one:

| Behaviour          | Detail                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| One guard          | `src/lib/dirty-guard.ts` holds the registry and the cancel decision, and `src/routes/+layout.svelte` is the one place that asks, for the same reason the session gate lives there: a screen that had to remember the rule could forget it. The registry reads each form's dirty state when a navigation happens, so a saved or discarded draft stops warning with no bookkeeping in the form.                                                                      |
| Two questions      | A navigation that unloads the document is cancelled and the browser asks its own question; a navigation inside the panel has no browser dialog, so the guard asks with `window.confirm`, which is synchronous and therefore what `beforeNavigate` can use. A custom modal would have to defer the navigation and would be a second dialog language for one rule.                                                                                                   |
| Six forms register | The three Settings tabs, the Token Saver form (any of its three sections), the outbound proxy card, and the combo editor. Only a form with a server-confirmed baseline can say what "unsaved" means, so a dialog that discards on Cancel is not registered. The outbound card is not a draft until its document has been read, which is why it registers `server !== null && dirty`.                                                                               |
| Combo baseline     | `comboFormDirty` in `combo-form.ts` compares the draft against a snapshot of the seeded form, field by field. A snapshot rather than the live object, because a `$state` proxy writes through to the object it wraps and one shared object would move the baseline with every keystroke.                                                                                                                                                                           |
| Test split         | The eight wiring cases were one file until Prettier reflowed it to 225 lines, past the 220-line warning. They are two files now, on the form-family seam: the five settings-style forms in `tests/components/dirty-forms.test.ts` (178 lines) and the combo editor's two cases in `tests/components/dirty-combo-form.test.ts` (95), because the combo editor is the one form whose dirty state is compared field by field rather than against a settings document. |
| Limit              | What is tested is the decision function and each form's registration, not `beforeNavigate` itself, because the test harness renders components without a router. Pressing a nav link with a dirty form is a browser click-through, and it stays outstanding with F12. Cancel and Discard stay silent on purpose: an explicit discard is not leaving a form.                                                                                                        |

F11 re-measured every number it names, with the command beside it:

| Claim              | Measurement and result                                                                                                                                                                                                                                                                                                                                                      |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Sidebar tree       | `bun -e` over `NAV_GROUPS` and `allNodes` in `src/lib/navigation.ts`: 5 groups, 20 nodes, 19 routable leaves (13 static routes plus the 6 media kinds), 1 container, 0 rows labelled `Planned`. `grep -c 'planned: true'` finds 1, on the Media Provider container, which renders as a disclosure                                                                           |
| Panel test count   | `bun run test`: 2185 passing across 111 files                                                                                                                                                                                                                                                                                                                               |
| README status line | It read 18 routable and 1 `Planned` from before `/playground`; it now carries the measurement above, and `/playground` is in the built-screens list                                                                                                                                                                                                                         |
| README open items  | The drift-gate item claimed no gate was wired. `scrypts/gates/contract-drift.sh` compares SPEC-API §8 with the panel's enum and runs in `all.sh`, and `scrypts/gates/contract-openapi.sh` pins the served document to the contract YAML, so the item now states the half that is still open: the panel's response schemas are not compared to the served contract by a gate |
| SPEC-UI §15        | The panel test count was 1053 across 39 files; it is the measurement above. The `app-ui/` path row said the remaining screens were `Planned`; every sidebar leaf is routable now                                                                                                                                                                                            |
| SPEC-UI §12        | U1 had no status paragraph while U0, U2, and U3 had one; it now records what is built and that the recorded click-through is the outstanding exit step                                                                                                                                                                                                                      |
| SPEC-UI §5.1       | The `/api-docs` row said the screen renders SPEC-API §7; it renders the served OpenAPI document                                                                                                                                                                                                                                                                             |

### Readiness findings F5 to F9, 2026-09-21

Run with Bun 1.3.14. This pass covers the five MEDIUM findings that were still open in
`docs/DRAFT/007-UI-ENDPOINT-READINESS.md`: F5 (the upstream tab's filters), F6 (the repeatable key
rows), F7 (the four screens that had no render test), F8 (the refresh control), and F9 (the rate
limit column). Each landed as its own commit; the gates below were measured on the tree that
carries all five, and the register records each finding as CLOSED with its own evidence.

| Check             | Result                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `bun run test`    | 2168 tests passed across 108 files (the previous pass was 2065 across 96); the five findings added the four missing screen suites, the bulk row mode, the countdown column, and the shared refresh control with a case in each of the eleven screens that carry it, one of which the full suite then caught as flaky                                                                                                                                   |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `bun run build`   | succeeds, exit 0, output in `build/`                                                                                                                                                                                                                                                                                                                                                                                                                   |
| Live wire pass    | not run in this pass. None of the five needed an `app-serv` change, and the browser click-through stays outstanding for the U0/U1 screens, which is F12 of the register                                                                                                                                                                                                                                                                                |
| File size         | largest source in `src/` is 220 lines (`src/lib/components/CombosTab.svelte`, exactly at the warning line, up from 217 because the mount is three lines); every test file this batch created is under 250 (largest `tests/components/endpoint-detail-drawer.test.ts` at 201), while the pre-existing test files it touched were already over the cap (largest touched: `tests/components/media-providers.test.ts`, 303 before the batch and 319 after) |
| Text hygiene      | 0 em dashes in the new files; the new copy carries no marketing vocabulary (R-16)                                                                                                                                                                                                                                                                                                                                                                      |

What each finding changed, and where it states a limit rather than hiding one:

| Finding | Behaviour                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| F5      | The upstream tab's filters live in the URL and are applied by the server (`parseEndpointSearch` / `nextEndpointSearch`), so the table renders the rows the gateway returned for the filter instead of computing a narrower list in the browser and rendering the whole page. The provider options come from a second, unfiltered read, because options derived from a filtered result would drop every other provider from the select.                                                                                                                                                                                  |
| F6      | The drawer carries §6.2's repeatable row mode as a second tab: `One key` posts one key, `Several keys` posts the batch and shows the per-row verdict by index when the gateway refuses the set all-or-nothing.                                                                                                                                                                                                                                                                                                                                                                                                          |
| F7      | Four screens gained render suites (`/login`, both tabs of `/endpoint-keys`, `/providers` list, `/changelog`), and writing them against the served shapes exposed three wire defects the panel had been hiding: the create response names the plaintext key `plaintext_key` (not `key`), an update answers the key row with no plaintext at all, and a key that was never used omits `last_used_at` rather than sending null. The same pass made a refused rename or revoke visible instead of silently discarded, and gave the login screen the §6.1 copy plus the lockout countdown from the response's `Retry-After`. |
| F8      | One shared `RefreshControl` (42 lines) on eleven list screens; `/quota` keeps the control it already had. The button stays enabled during a read and swaps its label to "Refreshing" rather than disabling, which is the rule `/quota` records: a disabled gate swallows the click and reads as broken. The control repeats the read on screen, filters and all, and it is deliberately absent at a `/media-providers` address that names no kind, where there is nothing to read.                                                                                                                                      |
| F9      | The endpoint-keys table prints `rate limited until <zoned time> (<time left>)` through the helpers the quota table already used, with the drawer ticking `now` once a second while it is open, so the countdown is live rather than frozen.                                                                                                                                                                                                                                                                                                                                                                             |

#### What the render tests caught in the wire

The three defects above were not in the panel's logic but in its reading of the gateway, and each
one broke a screen in a way no unit test of the panel alone could see:

| Defect                      | What it broke                                                                                                          |
| --------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `plaintext_key`             | Every gateway-key create failed its parse, so the one-time key never reached the modal.                                |
| Update answers no plaintext | Every rename and every disable failed its parse, and the failure was discarded, so the row silently kept its old name. |
| `omitempty` timestamps      | A key that had never been used failed the parse of a `nullable` field, which took the whole list read down.            |

### Changelog, 2026-09-21

Run with Bun 1.3.14. This pass covers `/changelog` (SPEC-UI §6.16): the screen that reads the release notes
the gateway serves and marks each release against the running build.

| Check             | Result                                                                                                                                                                                                            |
| ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                              |
| `bun run test`    | 2065 tests passed across 96 files, 33 more than the previous pass; the changelog suite is 78 tests across 3 files, one of them rewritten from the shape this screen used to demand                                |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                              |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                    |
| `bun run build`   | succeeds, output in `build/`                                                                                                                                                                                      |
| Live wire pass    | 18 checks, 0 failures, against a booted `app-serv`: the served notes read through the panel's own client and schema, every marker and the newer count recomputed independently, and four controls                 |
| Contrast          | the screen's one new graphic pair (the warn marker on a release row) was added to `tests/tokens/contrast.test.ts`, which now runs 52 assertions over both themes                                                  |
| File size         | largest new source is 182 lines (`src/routes/changelog/+page.svelte`); largest new test is 211 lines (`tests/components/changelog.test.ts`), and the rewritten schema test is 214, all under the 220 warning line |
| Text hygiene      | 0 em dashes in the new files; the new copy carries no marketing vocabulary (R-16)                                                                                                                                 |

What the screen does, and where it states a limit rather than hiding one:

| Area                      | Behaviour                                                                                                                                                                                                                                          |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The source                | One read: `GET /api/v1/changelog` (SPEC-API §7.18), the release notes the binary carries. The panel bundles no release text of its own, which is what §14 Q11 settled: a bundled file would drift from the running gateway and then lie.           |
| The comparison            | `GET /api/v1/version` decides what each release is marked: `Running`, `Newer`, or `Installed`. Both reads run at once, and the fact row names the route the list came from.                                                                        |
| The date                  | Rendered as the gateway sent it (`2026-09-20`), not through the zoned timestamp formatter: a calendar date has no zone, and formatting it in the browser's zone prints the previous day for a reader behind UTC.                                   |
| The note                  | The gateway's own paragraph, rendered as one block. The served entry has no category and no bullet list, so the panel invents neither.                                                                                                             |
| A version it cannot place | Two sentences, one per cause: the read failed, or the gateway reports a value that is not a release number. Neither says "up to date", and no release is marked, because "I could not place your version" is not "you are behind on all of these". |
| The empty state           | Names the route that answered with an empty list, because the notes travel inside the binary: an empty answer is the gateway's own history being empty, not a source the panel is missing.                                                         |
| States                    | Loading, failure with Try again, the empty state above, and a version failure that leaves the release list on screen.                                                                                                                              |

#### Live pass against `app-serv`, 2026-09-21

`app-serv` was built from **committed HEAD** with `git archive HEAD app-serv` and `go build`, the way the
playground pass built it, because another actor was mid-pass in that tree. It booted from its own `.env`
with a run env that added only `HTTP_ADDR=127.0.0.1:9090`, `PUBLIC_BASE_URL` at the panel's origin,
`EGRESS_ALLOWED_TARGETS=127.0.0.1/32`, and a run-local `PANEL_BOOTSTRAP_PASSWORD`. The built panel ran with
`PANEL_API_TARGET` at that address and answered `/login` with 200.

The driver logged in through the panel's own origin, then read the route three ways: as raw bytes through
the panel's forwarder (200, `application/json`, 1821 bytes, five releases), through the panel's own
`fetchChangelog` so the live payload parsed by `schemaChangelog`, and through `fetchSystemInfo`. Every
marker and the newer count were recomputed in the driver with its own arithmetic and compared to the
module's, so the pass does not check the module against itself. 18 checks, 0 failures.

| What the pass proved | Result                                                                                                                                                                                                                                                         |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The served contract  | Five releases, `v0.4.0` (2026-09-20) down to `v0.0.1` (2026-09-16), newest first by date, each with a version, a calendar date, a title, and a note                                                                                                            |
| The panel's schema   | The live payload parses through `schemaChangelog`, and the page's sort keeps the served order                                                                                                                                                                  |
| The comparison       | The running build reports `0.1.0-dev`; the driver's own arithmetic marks `v0.4.0` through `v0.1.0` newer (a prerelease is older than its release) and `v0.0.1` installed, which is what the screen says                                                        |
| Controls             | The envelope this panel used to demand (`{entries: [...]}`) is refused, a live row with its date removed is refused, an empty list parses (the empty state's data path), and the route answers 401 with no session, both directly and through the panel origin |

One fact about the served data came out of the pass and belongs to `app-serv` rather than the panel: the
running build reports `0.1.0-dev` while its own changelog goes to `v0.4.0`, so the screen honestly reads
"4 releases newer than this build" for the binary that is serving those notes. The panel cannot fix that by
guessing; it prints the version the gateway reports.

The live database was restored to its exact baseline: `usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0
caps=0 settings=1 auth_null=true media_settings=0 proxies=0`. This pass writes no rows at all, so the only
change was the bootstrap password hash the boot seeded, nulled after the server stopped, and the run's temp
files were removed.

The honest limit is the same one the other passes carry: the panel is client-rendered, so the render half is
covered by jsdom tests and the browser click-through is outstanding.

### Playground Chat, 2026-09-21

Run with Bun 1.3.14. This pass covers `/playground` (SPEC-UI §6.15): the screen that sends one chat request
through the data plane by hand, and the credential path that makes it possible without the browser ever
holding a key.

| Check             | Result                                                                                                                                                                                            |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                              |
| `bun run test`    | 2032 tests passed across 94 files, 133 of them new in this pass across 12 files                                                                                                                   |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                              |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                    |
| `bun run build`   | succeeds, output in `build/`                                                                                                                                                                      |
| The bundle        | `grep -rl PANEL_PLAYGROUND_KEY build/client/` finds nothing; the name appears only under `build/server/`                                                                                          |
| Live wire pass    | 37 checks, 0 failures, in four phases against a booted `app-serv`, plus a fifth isolated phase (5 checks) that pointed a second panel at a frame-by-frame stub                                    |
| File size         | largest new source is 170 lines (`src/routes/playground/+page.svelte`); largest new test is 174 lines (`tests/api/playground.test.ts`), both under the 250 ceiling and under the 200 warning line |
| Text hygiene      | 0 em dashes in the new files                                                                                                                                                                      |

What the screen does, and where it states a limit rather than hiding one:

| Area                   | Behaviour                                                                                                                                                                                                                                                       |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The injection path     | `PANEL_PLAYGROUND_KEY` is read from the panel's server environment, and the credential is written in exactly one function (`playgroundHeaders`). The browser is never given a key, so there is no store, form field, or URL that could hold one.                |
| The two panel routes   | `GET /playground/models` and `POST /playground/chat` sit outside `/api/v1`, so the general forwarder keeps passing `/api/v1/*` through untouched. Teaching that forwarder to inject would attach the credential to every management request a browser can make. |
| A session is required  | Every playground route first asks `GET /api/v1/auth/status` with the caller's cookie, so the session's truth stays in the gateway's store. The key check comes first, which is why an unconfigured panel dials nothing at all.                                  |
| The failure vocabulary | The routes answer in the panel's own envelope and keep the gateway's error object inside it. Both planes use the code name `UNAUTHORIZED`, so a forwarded 401 would read as an expired panel session and sign the operator out for a gateway key problem.       |
| The answer             | The text grows frame by frame while it arrives, with a Stop control that aborts the read. The facts beside it are the resolved model, the finish reason, the token counts, and the HTTP status the panel's route answered with.                                 |
| A fact the wire omits  | Reads `not stated` rather than a zero or a blank. A stream can end without a usage frame, and printing zero would be a token count the gateway never reported.                                                                                                  |
| Three endings          | Ending on the documented `[DONE]` sentinel, being stopped by the operator, and closing without the sentinel get three sentences, because only the third means the answer may be cut off.                                                                        |
| The raw disclosure     | A collapsed block holds the frames exactly as they arrived, which is what makes a contract divergence diagnosable instead of mysterious.                                                                                                                        |
| The unavailable state  | When the key is unset the screen renders the sentence the panel's own route wrote, which is the only place allowed to name the variable, and renders no send control. It does not ask the operator to paste a key.                                              |
| States                 | Loading, failure with Try again, an empty model list, and the unavailable state above.                                                                                                                                                                          |

#### Live pass against `app-serv`, 2026-09-21

`app-serv` was built from **committed HEAD** rather than the working tree, because another actor was
mid-pass in `internal/handler/` at the time: `git archive HEAD app-serv` into `/tmp`, then `go build`
there. That is read-only for the repository and it builds exactly the committed contract, which is the
thing the panel talks to. The binary booted from its own `.env` with a run env that added only
`HTTP_ADDR=127.0.0.1:9090`, `PUBLIC_BASE_URL` at the panel's origin,
`EGRESS_ALLOWED_TARGETS=127.0.0.1/32`, and a run-local `PANEL_BOOTSTRAP_PASSWORD`. A loopback stub on a
fresh port answered SSE with a 0.6 s pause after its first frame, so a buffering hop and a streaming one
would look different.

The driver imported the panel's own client modules by absolute path and shimmed `fetch` to resolve the
relative paths against the panel's origin and attach the session cookie, because bun has no cookie jar and
that is what `credentials: 'same-origin'` means in a browser. So the pass drove the code the screen calls,
not a curl that happens to return bytes.

| Phase     | Checks | What it proved                                                                                                                                                                                                                                 |
| --------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Fixture   | 5      | A provider node, an endpoint with its key, a gateway key whose plaintext is only in the create response, and the panel session through the panel's own origin                                                                                  |
| No key    | 5      | 503 `PLAYGROUND_KEY_MISSING` naming the variable, and neither the gateway nor the stub dialed at all, which is what "the key check comes first" means in practice                                                                              |
| With key  | 23     | The session gate, the models read through the panel's client, the streamed answer with its facts, the empty-body refusal, no response carrying either credential, and the stub log proving the endpoint key and `stream_options.include_usage` |
| Wrong key | 4      | `GATEWAY_KEY_REFUSED` with the gateway's own `UNAUTHORIZED` inside the panel envelope, and the panel session untouched, so a key problem cannot sign the operator out                                                                          |
| Isolated  | 5      | A second panel pointed at a frame-by-frame stub: the answer arrived over five reads spread across 700 ms and the reader ended `done`, which proves the relay streams independently of the gateway's own buffering                              |

Three defects in `app-serv` came out of this pass. They are recorded here and in
`docs/DRAFT/007-UI-ENDPOINT-READINESS.md` as requests to that actor, not absorbed by the panel:

| Defect                                            | Where                                                       | What the panel sees                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| ------------------------------------------------- | ----------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A chunk is marshalled without being framed        | `internal/dataplane/translate_stream_openai.go` `mustFrame` | The stream carries a bare JSON object and `data: [DONE]` is glued to it with no blank line, so the reader ends `truncated` rather than `done`                                                                                                                                                                                                                                                                                                                              |
| The status is committed before the model resolves | `internal/handler/chat.go`                                  | A streamed request for an unknown model answers 200 with an error body under an SSE content type, where the non-streamed path answers 400                                                                                                                                                                                                                                                                                                                                  |
| The whole answer is written at once               | the gateway's stream handler                                | `sseSink.Flush()` never reaches the client, so nothing streams; the stub's own frame-by-frame writes arrive as one 1074-byte read. **Resolved 2026-09-22 by app-serv draft 010 F5**: every middleware wrapper now forwards `Flush()` and `Unwrap()`, the sink flushes through `http.ResponseController`, and live evidence on a throwaway stack measured the first frame at 16.7 ms with the fix against 770.7 ms without it. The other two rows remain open with app-serv |

The panel is deliberately not taught to tolerate the malformed tail: `truncated` is the honest reading of
the bytes it received, and a reader that accepted the glued sentinel would hide a wire defect behind a
screen that looked satisfied.

The live database was restored to its exact baseline: `usage=2 logs=1 keys=0 endpoints=0 upkeys=0
nodes=0 caps=0 settings=1 auth_null=true media_settings=0 proxies=0`. The pass's `usage_records` and
`request_logs` rows were deleted, the gateway key was hard-deleted (revoking leaves the row), the
`panel_auth` hash was nulled after the server stopped, and the run's temp files were removed.

The honest limit is the same one U0, U1, and the Skills half carry: the panel is client-rendered, so the
browser click-through is still outstanding and the render half is covered by jsdom tests only.

### Skills, 2026-09-21

Run with Bun 1.3.14. This pass covers `/skills` (SPEC-UI §6.10): the screen that hands out one install
line per capability skill, and the reachability check that decides whether a row may offer one.

| Check             | Result                                                                                                                                                                                         |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                           |
| `bun run test`    | 1898 tests passed across 82 files, 30 of them new in this pass across 3 files                                                                                                                  |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                           |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                 |
| `bun run build`   | succeeds, output in `build/`                                                                                                                                                                   |
| Live wire pass    | 26 checks, 0 failures, against a booted `app-serv`; the live catalog parsed through the panel's own schema, every derivation driven over it, and each source address asked with a real request |
| File size         | largest new source is 140 lines (`src/routes/skills/+page.svelte`); largest new test is 243 lines (`tests/components/skills.test.ts`), under the 250 ceiling; `navigation.ts` is line-neutral  |
| Text hygiene      | 0 em dashes in the new files; the new copy carries no marketing vocabulary (R-16)                                                                                                              |

What the screen does, and where it states a limit rather than hiding one:

| Area            | Behaviour                                                                                                                                                                                                                                                                                                                       |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Source          | One read: `GET /api/v1/skills` through the panel's forwarder, so the session cookie is the only credential involved. Nothing on the screen is typed by hand: the order is the catalog's own, the entry row is the row the catalog marks `entry`, and every install line is composed from that row's own `raw_url`.              |
| The catalog     | Seven rows: the entry skill first, then one per data-plane route. Each row carries the name, the one-line description the gateway wrote, the route it teaches (or `No endpoint` for the entry row, which indexes the others), and the two addresses it was served with.                                                         |
| Two addresses   | `raw_url` is what an agent fetches and what the copy control hands out, rendered as copyable text. `blob_url` is what a person reads, rendered as a link, because that address renders the file instead of downloading it.                                                                                                      |
| Availability    | Each `raw_url` is asked with a HEAD before its row offers a copy control. A row that answered gets the control and the link; a row that did not states the cause and keeps the address on screen, because that address names the path that has to be published. `Check again` re-asks every row.                                |
| The four causes | A 404 or 410 is `missing`; any other non-2xx is reported with its status; a request that did not finish is the timeout; anything else is the network. They are separate sentences because the operator's next move differs: a file that is not published is a push, and a request that never completed is worth pressing again. |
| Today's answer  | All seven answer 404, because the documents are in this working tree and the ref the catalog names (`main`) does not carry them yet. So the screen renders seven unavailable states, no copy control, and the summary reads "0 of 7 sources are published at the ref the catalog names."                                        |
| States          | Loading, error with Try again, and an empty state for a catalog that answers no rows.                                                                                                                                                                                                                                           |

#### Live pass against `app-serv`, 2026-09-21

`app-serv` was built from the working tree and booted from its own `.env` with a run env that added only
`HTTP_ADDR=127.0.0.1:9090`, `PUBLIC_BASE_URL` at the panel's origin,
`EGRESS_ALLOWED_TARGETS=127.0.0.1/32`, and a run-local `PANEL_BOOTSTRAP_PASSWORD`. The built panel ran
with `PANEL_API_TARGET` at that address and answered `/skills` with 200. The pass needed no stub and no
provider fixture: the screen reads a session-gated management route and then asks a public file host.

Because the panel is client-rendered, the pass verified the half a browser cannot: it logged in through
the panel's own forwarder (204 plus the session cookie), read the catalog with the panel's own
`fetchSkillCatalog`, and drove every derivation the screen uses (`orderSkills`, `installLine`,
`probeSkillSources`, `causeSentence`, and `SKILLS_COPY.sources.summary`) over the live rows. 26 checks,
0 failures.

| What the pass proved            | Result                                                                                                                                                                                                                  |
| ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The live catalog parses         | 200 through the forwarder, and `fetchSkillCatalog` returns ok                                                                                                                                                           |
| The parse is not vacuous        | The same schema refuses a row whose `raw_url` is relative and a payload with no `data` array                                                                                                                            |
| The catalog is the spec's       | 7 rows with the 7 ids SPEC-API §7.16 names, exactly one `entry`, and the entry row teaches no route                                                                                                                     |
| The addresses are the handler's | Every `raw_url` and `blob_url` equals the address derived from the repository constant, the ref `main`, and `skills/<id>/SKILL.md`                                                                                      |
| The documents exist             | Every served id has `skills/<id>/SKILL.md` in this tree, with frontmatter naming that row, a description, and a heading                                                                                                 |
| The order rule holds            | `orderSkills` keeps the entry first on the wire order and moves it first from a shuffled order, leaving the rest in the order the gateway sent                                                                          |
| The install line                | Every row's line starts `Read this skill and use it: ` and names that row's own address                                                                                                                                 |
| The probes are real             | Seven HEAD requests, all seven answering 404, and every row classified `missing`, so the screen offers no copy control                                                                                                  |
| The probe discriminates         | A file that is on the same ref reads `available` and a path that is not reads `missing`, so the answer is about the address rather than about the check                                                                 |
| The classifier splits           | `classifySourceStatus` reads 200 as available, 404 as missing, and 500 as neither                                                                                                                                       |
| The sentence matches the rows   | Composed from these probes the way the page composes it, the summary reads "0 of 7 sources are published at the ref the catalog names", which is the count of rows marked available and can be checked by counting them |

The honest limit is the state itself. `origin/main` is 106 commits behind this working tree and 0 ahead,
and `git ls-tree origin/main skills` finds nothing, so the seven documents cannot resolve at the ref the
catalog names until the owner pushes. The screen is built for both answers and is rendering the true one:
seven rows, seven `missing` causes, no copy control. The copy control appears the moment the push makes
those paths readable, with no panel change. This is recorded in SPEC-UI §6.10, and the two facts the pass
could not settle are recorded there too: the panel is client-rendered, so the render half comes from jsdom
tests, and the documents do not exist on the named ref yet.

Still outstanding: a browser click-through of the screen. Every element is covered by the jsdom render
test (the three states, the entry block, the capability rows, both availability shapes, the cause
sentences, and `Check again`), and the wire contract is now verified against the service, but no one has
pressed the controls in a browser and watched the screen answer.

The database was counted before and after and is unchanged:
`usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0 caps=0 settings=1 auth_null=true media_settings=0
proxies=0`. The pass wrote no business rows; the run env, the built binary, and the session cookie were
removed, and `panel_auth.password_hash` was nulled after the server stopped.

### API Docs, 2026-09-21

Run with Bun 1.3.14. This pass covers `/api-docs` (SPEC-UI §6.12): the screen that renders the contract
the gateway serves rather than a written-out copy of it. It closes F1 of
`docs/DRAFT/007-UI-ENDPOINT-READINESS.md`.

| Check             | Result                                                                                                                                                           |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                             |
| `bun run test`    | 1868 tests passed across 79 files, including 92 new API Docs tests across 6 files                                                                                |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                             |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                   |
| `bun run build`   | succeeds, output in `build/`                                                                                                                                     |
| Live wire pass    | 31 checks, 0 failures, against a booted `app-serv`; the live document parsed through the panel's own schema and every derivation driven over the result          |
| File size         | largest new file is 210 lines (`tests/components/api-docs.test.ts`); largest new source is 116 lines (`openapi-credentials.ts`); `navigation.ts` is line-neutral |
| Text hygiene      | 0 em dashes in the new files; the new copy carries no marketing vocabulary (R-16)                                                                                |

What the screen does, and where it states a limit rather than hiding one:

| Area        | Behaviour                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Source      | One read: `GET /api/v1/openapi.json` through the panel's forwarder, so the session cookie is the only credential involved. Nothing on the screen is typed by hand: the grouping is the document's own tag order, the credential column is derived from each scheme's declaration, the example call is composed from the operation's method and path, and an error meaning is the description of the response whose body is that plane's error envelope. |
| Groups      | One section per tag the document declares, each with a count, an anchor, and a place in the jump list. A tag declared but unused renders no empty section; an operation with no tag lands in an Untagged tail; an operation with several tags is listed under each.                                                                                                                                                                                     |
| Credentials | The document's own scheme names with the panel's label beside them (`sessionCookie`, `gatewayKey`), each with the count of operations that declare it, plus the sentence counting the operations that declare none. A scheme an operation names but the document does not define is rendered verbatim and the row says it is undefined.                                                                                                                 |
| Examples    | One per section, composed from the section's first GET when it has one. A path parameter prints as `<id>`, so no example is a URL with braces in it, and the credential is a placeholder (`Authorization: Bearer sk-...`, `Cookie: pannel_session=<session cookie>`), never a key: a test asserts no example matches a real-key shape.                                                                                                                  |
| Error codes | One table per plane in the document's `x-contract.planes`, ordered by status then code, with the plane's error envelope named. All 20 codes the live document declares carry a meaning.                                                                                                                                                                                                                                                                 |
| Absences    | A document with no servers, no schemes, no `x-contract`, or no paths renders a sentence that names the missing block, rather than an empty space or a value the panel invented.                                                                                                                                                                                                                                                                         |
| States      | Loading, error with Try again, and an empty state for a document that declares no paths.                                                                                                                                                                                                                                                                                                                                                                |

#### Live pass against `app-serv`, 2026-09-21

`app-serv` was built from the working tree and booted from its own `.env` with a run env that added only
`HTTP_ADDR=127.0.0.1:9090`, `PUBLIC_BASE_URL` at the panel's origin,
`EGRESS_ALLOWED_TARGETS=127.0.0.1/32`, and a run-local `PANEL_BOOTSTRAP_PASSWORD`. The built panel ran
with `PANEL_API_TARGET` at that address and answered `/login` with 200.

Because the panel is client-rendered, the pass verified the half a browser cannot: it logged in through
the panel's own forwarder (204 plus the session cookie), read the document through it, parsed the answer
with the panel's own `schemaOpenAPIDocument`, and drove every derivation the screen uses over the result.
31 checks, 0 failures.

| What the pass proved        | Result                                                                                                                                                                                                                                          |
| --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The live document parses    | 200 through the forwarder, and `safeParse` succeeds                                                                                                                                                                                             |
| The parse is not vacuous    | The same schema refuses a document with no `paths` and one whose `openapi` field is a number                                                                                                                                                    |
| The catalog is the document | 93 rows for 93 operations measured independently; 23 groups for the 23 declared tags; no empty group; unique anchors; an example for every group                                                                                                |
| Credentials resolve         | Every scheme an operation names is defined in the document; `sessionCookie` covers 76 operations and `gatewayKey` 12, with 5 declaring none                                                                                                     |
| Examples are composed       | No example leaves a `{param}` brace, every one starts on the document's own base URL, a data-plane example carries the bearer placeholder, a management example carries `Cookie: pannel_session=<session cookie>`, and none prints key material |
| Error tables                | One per plane (11 management, 9 data plane), each naming its envelope and ordered by status then code; 20 of 20 codes carry a meaning                                                                                                           |
| The route answers           | `GET /api-docs` on the built panel returns 200                                                                                                                                                                                                  |

One finding came out of the pass, and it is about the document rather than the panel: the served
document's `servers` block says `http://localhost:8080` with the description "Local app-serv", because
the block is fixed by the generator rather than taken from the running instance. The screen renders the
document's value with the document's own description beside it, which is what §6.12 asks for, so a reader
on another address sees the document's claim rather than a claim the panel made. It is recorded as a
request in `docs/DRAFT/007-UI-ENDPOINT-READINESS.md` F1.

The database was counted before and after and is unchanged:
`usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0 caps=0 settings=1 auth_null=true media_settings=0
proxies=0`. The pass wrote no business rows; the run env, the built binary, and the session jar were
removed, and `panel_auth.password_hash` was nulled after the server stopped.

Still outstanding: a browser click-through of the screen. Every element is covered by the jsdom render
test (the three states, the group headings, the catalog rows, both copy outcomes, the no-server
statement, and the per-plane tables) and the wire contract is now verified against the service, but no one
has pressed the controls in a browser and watched the screen answer.

### Quota budget caps, 2026-09-20

Run with Bun 1.3.14. This pass covers the budget-cap section of `/quota` (SPEC-UI §6.6): the per-endpoint
picker, the read of the stored cap, the whole-set write, and the read-back that follows it.

| Check             | Result                                                                                                                                                                                                                                                                                                                                   |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                     |
| `bun run test`    | 1776 tests passed across 73 files, including 62 new quota-cap tests across 3 files                                                                                                                                                                                                                                                       |
| `bun run lint`    | Prettier reports every file conforms                                                                                                                                                                                                                                                                                                     |
| `bun run lint:ts` | ESLint exits 0                                                                                                                                                                                                                                                                                                                           |
| `bun run build`   | succeeds, output in `build/`                                                                                                                                                                                                                                                                                                             |
| Live wire pass    | 23 checks, 0 failures, against a booted `app-serv`; every response parsed through the panel's own schemas                                                                                                                                                                                                                                |
| File size         | largest changed source is 210 lines (`QuotaCaps.svelte`); `quota.ts` crossed the 220 warning line while the cap half lived in it, so the half was split into `quota-cap.ts`, and the schema test file crossed the 250 ceiling at 355 lines, so its mapping half was split into `quota-cap-form.test.ts` (the same 51 tests, now 31 + 20) |
| Text hygiene      | 0 em dashes; the only emoji in `src` and `tests` is one test vector proving the sanitizer refuses one                                                                                                                                                                                                                                    |

What the section does, and where it states a limit rather than hiding one:

| Area               | Behaviour                                                                                                                                                                                                                                                                                                       |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Who can be capped  | The picker offers every endpoint the label read returned, plus any endpoint the window table names past that read's first page. A cap is legal before the first routed request, so the section renders whether or not a window exists, and it is the one part of the screen that is useful on an empty gateway. |
| What a save means  | The write replaces the whole cap set, so an empty field clears that cap and the form says so. The routing warning §6.6 asks for is stated where the cap is saved: once the month-to-date spend reaches a cap, the router stops picking the endpoint.                                                            |
| What is on screen  | Choosing an endpoint reads its cap and seeds both fields from the answer, so a save replaces what the gateway holds rather than a value the operator could not see. After a save the panel reads the cap again, and the sentence under the form is that read.                                                   |
| No cap versus zero | "No cap is stored for this endpoint" is a rule, not a ceiling of zero, and the sentence says which. A cost cap of zero with no token cap beside it is refused by the form before any request, because the API refuses it too.                                                                                   |
| Bounds             | Non-negative, at most 1,000,000,000 USD and 1,000,000,000,000 tokens, with the messages computed from the bounds themselves so the copy cannot drift from the rule.                                                                                                                                             |
| A refused endpoint | An endpoint the gateway does not carry is refused with the gateway's own sentence (`upstream endpoint not found`), and the panel shows it rather than inventing one.                                                                                                                                            |
| Read before write  | Both fields stay read-only until the stored cap has been read. The tests found the two defects that come from skipping this: a field the operator could type into was overwritten by the answer when it landed, and the summary claimed "no cap is stored" before any answer had arrived.                       |
| Save confirmation  | The sentence says "Saved." only until the form is edited again, because a confirmation left on screen while the field holds a value that was never sent would describe a save that did not happen.                                                                                                              |

Two shapes are recorded rather than smoothed over. The API's cost parser accepts more spellings than the
form does (`1e9` and `1/2` both parse there), so the panel is deliberately the stricter of the two; §14 Q24
records it. And the collection route carries no cap, so a cap column on the window table would be an N+1
read; the caps live in their own section instead.

Not yet verified: the rendered half in a browser. The wire half was verified against a running `app-serv`:
a throwaway driver logged in, created an endpoint, and drove the cap routes while parsing every response
through the panel's own schema modules. 23 checks, 0 failures. It proved the read of an endpoint with
nothing stored (`cap: null`), both caps written and read back with the amount printed as an 8-place
decimal, the form's own round trip, an omitted field clearing that cap while the other survived, an empty
body answering a cap object with no amounts rather than a null, the panel's sentence reading that as "no
cap is stored", and four refusals (a zero cost alone, a cost and a token cap past their ceilings, and an
endpoint the gateway does not carry, which answers `404 upstream endpoint not found`). The database was
returned to its baseline, counted before and after; the endpoint's `quota_caps` row needed an explicit
delete, because removing an endpoint does not cascade it.

### Combo test action, 2026-09-20

Run with Bun 1.3.14. This pass covers the `Test` action on every row of `/combos` (SPEC-UI §6.4): its
confirmation, one-token sequential probe, and complete per-reference readout.

| Check             | Result                                                                                                            |
| ----------------- | ----------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                              |
| `bun run test`    | 1714 tests passed across 70 files, including 33 new combo-test tests across 2 files                               |
| `bun run lint`    | Prettier reports every file conforms                                                                              |
| `bun run lint:ts` | ESLint exits 0                                                                                                    |
| `bun run build`   | succeeds, output in `build/`                                                                                      |
| File size         | largest changed source is 217 lines (`CombosTab.svelte`); combo-test source files stay below the 220 warning line |
| Text hygiene      | 0 em dashes; the only emoji in `src` and `tests` is one test vector proving the sanitizer refuses one             |

The first full-suite run under concurrent host load had seven 5-second jsdom timeouts, not assertion
failures. Rerunning those seven files passed all 69 tests; the count above is the complete 1714-test corpus,
not a claim that the timed-out run itself was green.

What the action does, and where it states a limit rather than hiding one:

| Area                | Behaviour                                                                                                                                                                                                                                                                           |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Before the request  | The Test button opens a confirmation that names the combo and strategy, says each reference gets a one-token probe in turn, and says the diagnostic spend is not written to usage. No request is sent until the operator confirms.                                                  |
| The result          | One row per stored reference, in the answer's order, with the stored ref, role, resolved provider/model and endpoint when present, latency, and either `Answered` or the gateway's own error code and message. A failed member remains a result, so healthy members are not hidden. |
| Fusion              | The judge is probed after the stored model references and is labeled `Judge`, so its role is not confused with a chain member.                                                                                                                                                      |
| While it runs       | The dialog says it is probing sequentially, keeps Close available, and disables Test again. Close does not pretend to cancel the server-side loop; a completed request from a closed dialog is ignored rather than writing stale results into a reopened one.                       |
| Two modal instances | The shared Modal primitive now gives each instance its own title id, so the test readout and delete confirmation cannot name each other for assistive technology.                                                                                                                   |

Not yet verified: the rendered half in a browser, and the wire half against a running `app-serv`. The
section is client-rendered (`ssr = false`), so tests exercise the route against a stateful stub that applies
the server's result shape, including failed members and a missing combo.

### OAuth section, 2026-09-20

Run with Bun 1.3.14. This pass covers the OAuth section of `/providers/[provider_id]` (SPEC-UI §6.3): the
start action, the callback's return to the page, the connected-account table, and the manual refresh.

| Check             | Result                                                                                                                    |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                      |
| `bun run test`    | 1681 tests passed across 68 files, 55 of them new in this pass                                                            |
| `bun run lint`    | Prettier reports every file conforms                                                                                      |
| `bun run lint:ts` | ESLint exits 0                                                                                                            |
| `bun run build`   | succeeds, output in `build/`                                                                                              |
| File size         | the largest file this pass touches is 187 lines, and the repository's largest source file stays at 218, under the warning |
| Text hygiene      | 0 em dashes; the only emoji in `src` and `tests` is one test vector proving the sanitizer refuses one                     |

What the section does, and where it states a limit rather than hiding one:

| Area                | Behaviour                                                                                                                                                                                                                                                                                                                                                |
| ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| When it renders     | Only for a provider the registry marks `has_oauth`, which is the API's own answer to whether the provider has a flow. The panel does not infer it from the provider's category.                                                                                                                                                                          |
| What the flow means | The panel starts `code` and nothing else: `device`, `connector`, and `none` get their reason in words instead of a disabled button, because a control that cannot act is a dead control (R-26). The stub's default flow is `code` so the start path is exercised; no live provider reports it today (Q23).                                               |
| The start           | `POST .../oauth/start` is called with no body, because the gateway derives its own callback from its base URL, and the authorize URL it answers is rendered as a link rather than a navigation the panel performs. A scripted redirect to a third party is one the operator did not ask for, and a link shows the host before it is followed.            |
| The callback return | The gateway sends the browser back with `oauth`, `oauth_error`, and `endpoint_id`. The section reads the outcome once, attributes it to the gateway, and drops the three keys with `replaceState`, so a reload does not announce a past result. A key belonging to something else is left alone, and an outcome the panel does not know renders nothing. |
| The accounts table  | One row per endpoint: label and id, status, expiry, last refresh, and the token state the gateway derives from `refresh_state`. The panel does not re-derive that classification; the row adds only what the expiry tells apart, so a `due` token whose expiry has passed reads as expired and one inside its window reads as inside it.                 |
| The refresh         | Per row, because the answer names the accounts it moved, and it re-reads the status without blanking the table. The first version did blank it, which destroyed the sentence reporting what the gateway had just done; a test caught that, and `load` now keeps the section mounted when it already has state.                                           |
| The read cadence    | Once per visit, plus a re-read after a refresh or a callback return. No timer: a self-firing request is not the operator's action, and the consequence (a token that crosses into its refresh window while the section sits open) is recorded as Q22 rather than hidden.                                                                                 |

One defect was found and fixed by this pass, and it is recorded because the pattern matters more than the
fix:

1. **The refresh blanked the section that reported it.** `onrefreshed` re-read the status with the same
   loading state the first read uses, so the `{#if loading}` branch unmounted the accounts table and, with
   it, the outcome sentence the operator had just earned. The test asserted both the sentence and the moved
   token state, which is what caught it; `load` now takes a `keep` flag for the re-read.

Not yet verified: the rendered half in a browser, and the wire half against a running `app-serv`. The
section is client-rendered (`ssr = false`), so the tests exercise it against a stub that applies writes and
answers with the server's own refusals. The start path cannot be driven live in this registry at all, since
the one `has_oauth` provider reports the `device` flow (Q23).

### Alias set, 2026-09-20

Run with Bun 1.3.14. This pass covers the alias table of `/providers/[provider_id]` (SPEC-UI §6.3): the
alias to target table, the form that adds a row or changes what an existing one targets, and the target
field's suggestions. It also carries the §6.4 combo delete refusal, which now says where the fix lives.

| Check             | Result                                                                                                |
| ----------------- | ----------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                  |
| `bun run test`    | 1626 tests passed across 66 files, 75 of them new in this pass                                        |
| `bun run lint`    | Prettier reports every file conforms                                                                  |
| `bun run lint:ts` | ESLint exits 0                                                                                        |
| `bun run build`   | succeeds, output in `build/`                                                                          |
| File size         | largest source file this pass is 218 lines, under the 220 warning line                                |
| Text hygiene      | 0 em dashes; the only emoji in `src` and `tests` is one test vector proving the sanitizer refuses one |

What the section does, and where it states a limit rather than hiding one:

| Area               | Behaviour                                                                                                                                                                                                                                                                                                                                                       |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The set is global  | Neither the read nor the write route takes a provider, and `alias` is the table's primary key, so the table is the same on every provider's detail screen and the section says so instead of implying it belongs to this provider. §6.4's delete refusal cannot link to it for the same reason: the route needs a provider id and the refusal names none (Q20). |
| Whole-set writes   | `PUT /models/aliases` replaces the set, so every change merges over the whole last read, and the body is sorted the way the read route sorts, which keeps the table from reshuffling after a reload. A write before the first successful read is refused with a sentence, because the merge would send an empty set and clear every alias.                      |
| One form, two jobs | A name already in the table changes that alias's target instead of adding a second row: a duplicate is a primary-key violation whose answer names the constraint rather than the alias. The outcome line says which of the two happened, so a write that changed a target is not reported as an add.                                                            |
| The suggestions    | The target field suggests the catalog ids and the combo names, the two things a target may be. They are suggestions, not a constraint: a failed read leaves the field usable and says so, and a combo list longer than the API's 100-row page says it is truncated rather than looking whole.                                                                   |
| Stale targets      | A stored alias whose target has since been disabled or deleted still renders, and a write that carries it is refused by the API with a sentence naming the alias. Q21 records that one such row refuses every alias edit until it is removed.                                                                                                                   |
| The combo delete   | The dialog's lead sentence follows the error code, so only a `CONFLICT` reads as "still referenced" and a server failure no longer does. The editor's reference suggestions gained the alias names, the third source §7.7 resolves a ref from.                                                                                                                  |

Not yet verified: the rendered half in a browser, and every write against a running `app-serv`. The
screen is client-rendered (`ssr = false`), so the tests exercise it against a stub that applies writes and
answers with the server's own refusals.

### Provider detail model writes, 2026-09-20

Run with Bun 1.3.14. This pass covers the disabled and custom model writes of `/providers/[provider_id]`
(SPEC-UI §6.3): the Disable action on every catalog row, the list of models this provider cannot route,
and the custom-model table with its add form and removal dialog.

| Check             | Result                                                                                                |
| ----------------- | ----------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                  |
| `bun run test`    | 1551 tests passed across 62 files, 114 of them new in this pass                                       |
| `bun run lint`    | Prettier reports every file conforms                                                                  |
| `bun run lint:ts` | ESLint exits 0                                                                                        |
| `bun run build`   | succeeds, output in `build/`                                                                          |
| File size         | largest source file this pass is 179 lines, under the 220 warning line                                |
| Text hygiene      | 0 em dashes; the only emoji in `src` and `tests` is one test vector proving the sanitizer refuses one |

What the screen does, and the four places it states a limit rather than hiding one:

| Area             | Behaviour                                                                                                                                                                                                                                                                                                                                              |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| The write rule   | `PUT /models/disabled` replaces the whole set for every provider, so a write built from this provider's slice would erase another provider's rows. The panel merges over the whole last-read set and renders the server's answer, and the two set operations are tested as set algebra rather than as fixtures.                                        |
| The two lists    | The merged catalog excludes a disabled model, so a disabled row has no place in it: the catalog offers Disable and a second list, "Models this provider cannot route", offers Enable. That list is the only place a disabled model is visible, which is why it exists at all. SPEC-UI §14 Q19 records the spec sentence this does not match.           |
| The refusal      | A Disable press before the disabled set has loaded is refused with a sentence, not with a disabled button. The merge runs over the set the panel holds, so writing first would send an empty set and clear every provider's rows; the refusal names that cause, and a test asserts no request was sent.                                                |
| The outcome line | The last write's result renders in whichever list holds the affected model: a model that could not be disabled is still in the catalog, and one that was disabled is in the disabled list. One rule, so the message appears beside the action that produced it, exactly once.                                                                          |
| Custom models    | Rows are read whole and narrowed to this provider, each write is one row, and the removal dialog states what a custom row shadows: a custom row overrides a registry row with the same pair, so removing it puts the registry's version back in the catalog. Capabilities are free text because the API accepts any value and publishes no vocabulary. |
| The catalog      | The table moved into its own component, because the row's action and its outcome message both name a model. The list keeps the filters, the request, and the states, and the page bumps a token after any write that changes the catalog, since all four writes do.                                                                                    |

One defect was found and fixed by this pass, and it is recorded because the pattern matters more than the
fix:

1. **`ModelCatalogTable.svelte` used `catalogSourceLabel` without importing it.** `svelte-check` was clean
   on the file list it was given and the component compiled, so the failure only appeared when a test
   rendered the table: the render threw, the component stayed on its loading state, and every catalog
   assertion failed with "Unable to find role=table". The page-level tests caught it because they render
   the whole screen rather than the component that changed, which is the reason those tests exist in that
   shape.

Not yet verified: the rendered half in a browser, and every write against a running `app-serv`. The
screen is client-rendered (`ssr = false`), so the tests exercise it against a stub that applies writes and
answers with the server's own refusals; the wire half for the two earlier U2 screens is verified live
below.

### Media Provider, 2026-09-20

Run with Bun 1.3.14. This pass covers `/media-providers/[kind]` (SPEC-UI §6.8), one screen at six
addresses, and the navigation change that lets a sidebar row link to a parameterised route.

| Check             | Result                                                                                                |
| ----------------- | ----------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                  |
| `bun run test`    | 1437 tests passed across 57 files, 228 of them this slice plus navigation                             |
| `bun run lint`    | Prettier reports every file conforms                                                                  |
| `bun run lint:ts` | ESLint exits 0                                                                                        |
| `bun run build`   | succeeds, output in `build/`                                                                          |
| File size         | largest source file this slice is 195 lines, under the 220 warning line                               |
| Text hygiene      | 0 em dashes; the only emoji in `src` and `tests` is one test vector proving the sanitizer refuses one |

What the screen does, and the three places it states a limit rather than hiding one:

| Area               | Behaviour                                                                                                                                                                                                                                                                                                                                                                                    |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The kind           | The panel's six kinds are `embedding`, `image`, `video`, `tts`, `stt`, `web`, and the API's sixth is `search`. The mapping is total in both directions, so `/media-providers/web` asks for `?kind=search` and a row answering `kind: "search"` renders under the `web` heading. An address naming no kind is refused before any request, with the six real addresses offered as the way out. |
| The base URL       | The field is the override, not the resolved value, because an empty override is how one is undone. It is empty unless the stored source is `override`, and the registry's value is named beside it. The panel blocks the one save it can prove the server would refuse, quoting the server's sentence, and lets the server answer the cases it cannot prove.                                 |
| The default model  | A selector over the models the service declares, plus the registry default, so an invalid model is structurally impossible. A kind that declares none gets a stated fact instead, because a select with no options is a dead control. A stored override the service no longer declares is named rather than dropped, so the operator can see why the selector reset.                         |
| The endpoint count | Scoped to this provider and this kind, and stated rather than linked: the endpoint screen filters by provider alone, so a link there would promise a narrower list than it shows. The provider name links to its registry entry, which is where its endpoints are listed.                                                                                                                    |

Two defects were found and fixed by this pass, and both are recorded because the pattern matters more
than the fix:

1. **The sidebar's disclosure check read `child.href`.** With the media kinds linking through a
   parameterised route, that check would have stopped matching and a reload on a Media kind page would
   have shown the container closed with the operator's own row hidden inside it. Nothing else failed,
   because no test covered the auto-open rule. Fixed by resolving every row through one shared
   `navPath`, and `tests/navigation/sidebar-disclosure.test.ts` now pins the rule for a static child, a
   parameterised one, and a route that matches no child at all.
2. **A child link never carried `aria-current`.** The top-level branch set it and the child branch did
   not, which never mattered before because no child was routable. With the media kinds routable, the
   current page was marked for sighted users and not for a screen reader. Found by the new test
   asserting the marker as well as the attribute, since DESIGN.md §6 requires position never to be
   carried by colour alone.

Not yet verified: the rendered half in a browser, and every write against a running `app-serv`. The
screen is client-rendered (`ssr = false`), so the tests exercise it against a stub that applies writes
and answers with the server's own refusals; the wire half for the two earlier U2 screens is verified
live below.

### Token Saver and Proxy Pools, 2026-09-20

Run with Bun 1.3.14. This pass covers the two U2 screens that are landed, `/token-saver` (SPEC-UI §6.7)
and `/proxy-pools` (§6.9), and the fold of the Settings Network tab into the proxy screen.

| Check             | Result                                                                                                                            |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                              |
| `bun run test`    | 1307 tests passed across 52 files (was 396 across 13)                                                                             |
| `bun run lint`    | Prettier reports every file conforms                                                                                              |
| `bun run lint:ts` | ESLint exits 0                                                                                                                    |
| `bun run build`   | succeeds, output in `build/`                                                                                                      |
| File size         | largest source file is 218 lines, under the 220 warning line; test files run to 582 in this repository, so the source limit binds |
| Text hygiene      | 0 em dashes; the only emoji in `src` and `tests` is one test vector proving the sanitizer refuses one                             |

What each screen does, and the two places it states a limit rather than hiding one:

| Screen         | Behaviour                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/token-saver` | RTK with its twelve-filter allowlist, Headroom with the external URL and the fails-open copy, Ponytail with its level. Each group saves as a whole-document `PUT`, because that is the only write route the API has, and the draft helpers merge the edited group over the last document read, so saving RTK cannot rewrite Headroom and an invalid value in one group does not block another. A filter name the panel has no checkbox for is kept and named rather than dropped on save. The deprecated `caveman` key is parsed and never rendered. |
| `/proxy-pools` | The pool table with its nine columns, the add and edit dialog with a candidate test before saving, the batch paste add with a per-row outcome, the per-row test, the delete dialog, and the outbound settings card. The page re-reads the pool after every write, so a row's state is the server's answer rather than the panel's assumption.                                                                                                                                                                                                        |

Two facts about the proxy path were settled by reading `app-serv`, and the screen states both instead of
implying otherwise. The pool routes nothing: the egress path reads `settings.network.outbound_proxy_url`
and nothing else, so a stored pool row is a candidate rather than a live route, and the page header says
so. An enabled proxy with an empty URL dials direct, so the panel refuses to write that combination and
names it when a stored document already holds it. Both are recorded as SPEC-UI §14 Q15 and Q16.

Not yet verified: the rendered half of either screen in a browser. The panel sets `ssr = false` in
`src/routes/+layout.ts`, so it is client-rendered and there is no server HTML to read; the tests exercise
the panel against a stateful stub that applies writes and returns the API's error shapes. The wire half
is now verified live, below.

#### Live pass against `app-serv`, 2026-09-20

`app-serv` booted from its own `.env` with a run env that added only `HTTP_ADDR=127.0.0.1:9090`,
`PUBLIC_BASE_URL`, `EGRESS_ALLOWED_TARGETS=127.0.0.1/32`, `PROXY_TEST_URL` at a loopback origin, and a
run-local `PANEL_BOOTSTRAP_PASSWORD`. Two loopback helpers in `/tmp` answered the pass: an origin serving
`/proxy-check`, and a forwarding HTTP proxy on `127.0.0.1:8096` that the panel registered as a candidate,
so a proxy could be tested without touching the internet. The built panel ran with `PANEL_API_TARGET` at
that address, and the pass logged in through it (204 plus a session cookie).

Because the panel is client-rendered, the pass verified the half a browser cannot: it called the panel's
own `/api/v1` routes and parsed every response through the exact Zod schema the screen uses. 25 checks, 0
failures.

| What the pass did             | Result                                                                                                                                                                |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /proxies`                | parses as `schemaProxyList`; the pool started empty                                                                                                                   |
| `POST /proxies`               | parses as `schemaProxy`; `has_password` false and no secret echoed                                                                                                    |
| `PATCH /proxies/{id}`         | parses as `schemaProxy`; the rename and the disable both took                                                                                                         |
| `POST /proxies/{id}/test`     | `{"state":"ok","latency_ms":6}`, and the helper log shows `PROXY GET http://127.0.0.1:8095/proxy-check`, so the probe really went through the candidate               |
| `POST /proxies/test` (live)   | `{"state":"ok","latency_ms":2}`, no `message`                                                                                                                         |
| `POST /proxies/test` (dead)   | `{"state":"fail","latency_ms":0,"message":"the proxy could not be reached"}`                                                                                          |
| `POST /proxies/test` (socks5) | `{"state":"fail","latency_ms":10007,"message":"the proxy did not answer within the time limit"}`, which is the protocol mismatch timing out rather than erroring fast |
| `GET /proxies` after a test   | the row carries the `status` the test wrote, and `updated_at` moved to the test time                                                                                  |
| `GET` and `PATCH /settings`   | parses as `schemaSettings`; the three network keys are present, the write reflects on a fresh read                                                                    |
| `GET` and `PUT /token-saver`  | parses as `schemaTokenSaver`; the saved document came back exactly as sent, and a fresh read agreed                                                                   |
| `DELETE /proxies/{id}`        | 204, and the pool returned to its starting size                                                                                                                       |

One finding came out of the pass, and it was the pass's own error rather than the panel's: the first
`PUT /token-saver` used filter names (`strip_ansi`, `collapse_blank_lines`) that the API refuses, and the
gateway answered `VALIDATION_ERROR`. That confirms the panel's twelve names are the API's twelve, since
the panel's own list is what the corrected body used.

The database was returned to the baseline it started from, counted before and after:
`usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0 caps=0 settings=1 auth_null=true media_settings=0
proxies=0`. The token saver document was put back to its pre-pass value, the `network` settings row the
pass wrote was deleted, the proxy rows were deleted, the panel hash was nulled after the server stopped,
and the run env, session jar, and built binary were removed.

Still outstanding: a browser click-through of both screens. Every interactive element is covered by a
jsdom test that asserts the DOM it produces, and the wire contract is now verified against the service,
but no one has yet pressed the controls in a browser and watched the screens answer.

### Shell and sidebar, 2026-09-18

Run with Bun 1.3.14. `app-serv` could not be started: its P1 tree fails at boot on migration
`000007_usage_quota.up.sql`, which uses `window` as a column name (a PostgreSQL reserved word). The shell
needs one public endpoint to decide what to render, so `/api/v1/auth/status` was answered by a throwaway
stub, and the stub was deleted after the pass. Nothing else was stubbed.

| Check           | Result                                               |
| --------------- | ---------------------------------------------------- |
| `bun run check` | 0 errors, 0 warnings                                 |
| `bun run test`  | 396 tests passed across 13 files (was 310 across 11) |
| `bun run build` | succeeds, output in `build/`                         |

Click-through, element by element, at 1440x900 unless stated otherwise:

| Element                                    | Action                | Outcome                                                                                                                                                                                                                                                                                                                       |
| ------------------------------------------ | --------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Header, theme toggle                       | Click twice           | `document.documentElement` gains and loses `dark`; body background measured `rgb(252, 250, 247)` light and `rgb(26, 25, 23)` dark, matching `DESIGN.md` §3.2                                                                                                                                                                  |
| Header, API base Copy                      | Click                 | Label switches to "Copied", the value is on the clipboard, a live region announces it, and it reverts after 2s. **Superseded 2026-09-21**: this row describes the inline control of 2026-09-18. The control is now a button that opens the API base dialog, and the pass section of 2026-09-21 above records what replaced it |
| Header, Sign out                           | Click                 | Calls the session store; with `require_login = false` the panel stays usable, which is the stub's shape                                                                                                                                                                                                                       |
| Header, sidebar trigger at 390px           | Click                 | Drawer opens at 293px over a dimmed overlay, focus moves inside it                                                                                                                                                                                                                                                            |
| Drawer, Escape                             | Press                 | Drawer closes, focus returns to the header control                                                                                                                                                                                                                                                                            |
| Drawer, a nav link                         | Click                 | Navigates to `/settings` and the drawer closes behind it                                                                                                                                                                                                                                                                      |
| Sidebar, `Endpoint & Key`                  | Click                 | Navigates to `/endpoint-keys`; row carries `aria-current="page"` and the 3px accent marker                                                                                                                                                                                                                                    |
| Sidebar, `Setting`                         | Click                 | Navigates to `/settings`                                                                                                                                                                                                                                                                                                      |
| Sidebar, `Media Provider`                  | Click                 | Expands to the six kinds; `aria-expanded` goes `false` then `true`, and back on a second click                                                                                                                                                                                                                                |
| Sidebar, `Media Provider` from a cold load | Observe               | Starts closed, and starts open only when a media kind is the current route                                                                                                                                                                                                                                                    |
| Sidebar, a planned row                     | Click                 | Inert by design: no `href`, no pointer cursor, a visible `Planned` chip, and the title attribute says why                                                                                                                                                                                                                     |
| Sidebar, rail collapse at 1440px           | Click                 | `data-state` goes `expanded` to `collapsed` and back; the preference is written to the `sidebar_state` cookie                                                                                                                                                                                                                 |
| Sidebar at 820px                           | Load                  | Starts as a 64px icon rail, expands to full width on the trigger, collapses again, cookie reads `false` afterwards                                                                                                                                                                                                            |
| Keyboard, full pass                        | Tab from the top      | Order is logo, then each navigation row, then the header controls, then the page controls. Every stop shows a 2px accent outline. Enter follows a link. Enter and Space toggle the disclosure. Escape closes the drawer.                                                                                                      |
| Overflow                                   | All three breakpoints | `document.documentElement.scrollWidth` never exceeds `clientWidth`, in both themes                                                                                                                                                                                                                                            |
| Tap targets                                | 390px and 820px       | Every visible sidebar, drawer, and header control is at least 44px tall. The desktop sidebar rail is a 14px drag handle with `tabindex="-1"`, so it is not a touch target and is not counted.                                                                                                                                 |
| Console                                    | All three breakpoints | No page errors. The only 404 is `/api/v1/gateway-keys`, which the stub deliberately does not implement.                                                                                                                                                                                                                       |

Two defects were found by this pass and fixed, and both are recorded because the pattern matters more
than the fix:

1. **The token layer was never imported.** `src/app.css` was written and tested in U0 but no file imported
   it, so the whole panel rendered unstyled while `tests/tokens/contrast.test.ts` passed. The test read the
   file; the browser never did. Fixed by importing it once from `src/routes/+layout.svelte`, and the rule is
   now written into spec §10 boundaries 6 and this README.
2. **Three component headers sat outside the script block.** A leading `//` comment placed above
   `<script>` in a `.svelte` file is template text, not a comment, so Svelte rendered the whole comment
   block into the page. Found by reading the rendered body text, not by a type check. Fixed by moving the
   comments inside `<script>`.

Two smaller defects were also found and fixed in the same pass: the media disclosure opened on first load
instead of closed, and a planned media kind rendered as an anchor with no destination, which is a dead
control under R-26.

### U0 scaffold, 2026-09-16

| Check           | Result                                                                                                                                                                                                    |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check` | 0 errors, 0 warnings                                                                                                                                                                                      |
| `bun run test`  | 310 tests passed across 11 files                                                                                                                                                                          |
| `bun run build` | succeeds, output in `build/`                                                                                                                                                                              |
| `bun run start` | boots on Bun, prints `pannelAI panel 0.0.1 on Bun 1.3.14` and the resolved API target                                                                                                                     |
| Panel routes    | `/` returns 200, `/login` returns 200                                                                                                                                                                     |
| Forwarding      | with `app-serv` down, `/api/v1/auth/status` returns 500 with the panel's error envelope; with a malformed `PANEL_API_TARGET`, it returns 500 naming the variable, while the panel itself still serves 200 |
| Property tests  | 1600 generated inputs across 4 profiles in `tests/schemas/sanitize-fuzz*.test.ts`; 0 unstable outputs. They found the composition defect fixed in `src/lib/schemas/sanitize.ts`                           |
| File size       | largest file is 202 lines and the largest under `src/` is 202, both under the 250 limit                                                                                                                   |
| Test shape      | 35 test functions: 30 generated from a table, 4 iterating a table inside the body, 1 regression case with two assertions, so 0 assert a single input                                                      |
| Text hygiene    | 0 em dashes and 0 emoji in `src`, `tests`, and `scripts`                                                                                                                                                  |

Not yet verified: any call that needs a running `app-serv`. Sign-in, key creation, and settings writes
were exercised against the panel's own error paths only, so those screens' happy paths still need a live
gateway.

## Open items that block later phases

1. **The panel's response schemas are not compared against the served contract by a gate.** The
   error-code half is wired: `scrypts/gates/contract-drift.sh` compares SPEC-API §8 with the panel's own
   enum in `src/lib/schemas/error.ts`, and it runs in `scrypts/gates/all.sh`. The document half is pinned
   twice: `scrypts/gates/contract-openapi.sh` compares the served `GET /api/v1/openapi.json` with
   `docs/CONTRACT/001-CONTRACT-API-V1.yaml`, and `TestOpenAPICoversEveryRegisteredRoute` (in
   `app-serv/internal/router`) pins the document to the router. What is left is the panel side of a field
   rename: a field the panel reads strictly is caught by that resource's tests only where a fixture covers
   it. The file this item originally waited for (`docs/SPEC-API/002-SPEC-API-openapi.md`) is superseded;
   see spec §14 Q3.
2. `AGENTS.md` scopes itself to the Go services (`app-*/**`), so it does not govern this panel. The
   panel's own rules come from `docs/SPEC-UI/001-SPEC-UI.md` §10, §11, and §7.1.5, and they are applied by
   hand here. The panel's gates live in `scrypts/gates/panel-check.sh`.
3. Gateway key `status` values are not enumerated in SPEC-API §7.3. The panel currently writes `active`
   and `disabled` and renders any other value verbatim; see §14 Q9.

Three items left this list on 2026-09-21 because the work they blocked on landed: `app-serv` boots (P1
fixed the reserved-word migration), Playground Chat is built (§6.15), and the changelog has a served source
(§7.18), so none of them is open. The list is measured rather than assumed: each item names the check that
would close it.

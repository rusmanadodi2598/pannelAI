# app-ui

Operator panel for the KENTANG TECH pannelAI gateway. It is a thin, strict client of the management
API in `app-serv` and never touches PostgreSQL or Redis.

Spec of record: [`docs/SPEC-UI/001-SPEC-UI.md`](../docs/SPEC-UI/001-SPEC-UI.md).
API contract: [`docs/SPEC-API/001-SPEC-API.md`](../docs/SPEC-API/001-SPEC-API.md).

Status: **U0 scaffold**, verified locally. Screens built: `/login`, `/endpoint-keys` (gateway keys
tab), `/settings` (security tab). The shell is complete: a themed sidebar with five groups and 19 owner
rows, responsive across phone, tablet, and desktop. Every other screen is marked Planned in the sidebar,
because a navigation item without a route is a defect.

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
bun run start        # run the built server on Bun (prints a boot banner)
bun run check        # svelte-check with the native TypeScript 7 compiler
bun run test         # unit tests (Vitest, jsdom)
bun run format       # Prettier write
bun run lint         # Prettier check
bun run lint:ts      # ESLint (types, unused bindings, floating promises)
```

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
src/lib/navigation.ts  the sidebar tree as data, five groups and 19 rows
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

Six passes are recorded here. The first is the U0 scaffold, measured 2026-09-16. The second is the
shell and sidebar work, measured 2026-09-18, and it is the R-35 click-through with its outcomes per
element. The third is the Token Saver and Proxy Pools pair, measured 2026-09-20. The fourth is the Media
Provider screen, measured the same day. The fifth is the provider detail model writes, measured the same
day. The sixth is the alias set, measured the same day.

### Alias set, 2026-09-20

Run with Bun 1.3.14. This pass covers the alias table of `/providers/[provider_id]` (SPEC-UI §6.3): the
alias to target table, the form that adds a row or changes what an existing one targets, and the target
field's suggestions. It also carries the §6.4 combo delete refusal, which now says where the fix lives.

| Check             | Result                                                                    |
| ----------------- | ------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                      |
| `bun run test`    | 1626 tests passed across 66 files, 75 of them new in this pass            |
| `bun run lint`    | Prettier reports every file conforms                                      |
| `bun run lint:ts` | ESLint exits 0                                                            |
| `bun run build`   | succeeds, output in `build/`                                              |
| File size         | largest source file this pass is 218 lines, under the 220 warning line    |
| Text hygiene      | 0 em dashes and 0 emoji across `src` and `tests`                          |

What the section does, and where it states a limit rather than hiding one:

| Area              | Behaviour                                                                                                                                                                                                                                                                                                                                                    |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| The set is global | Neither the read nor the write route takes a provider, and `alias` is the table's primary key, so the table is the same on every provider's detail screen and the section says so instead of implying it belongs to this provider. §6.4's delete refusal cannot link to it for the same reason: the route needs a provider id and the refusal names none (Q20). |
| Whole-set writes  | `PUT /models/aliases` replaces the set, so every change merges over the whole last read, and the body is sorted the way the read route sorts, which keeps the table from reshuffling after a reload. A write before the first successful read is refused with a sentence, because the merge would send an empty set and clear every alias.                      |
| One form, two jobs | A name already in the table changes that alias's target instead of adding a second row: a duplicate is a primary-key violation whose answer names the constraint rather than the alias. The outcome line says which of the two happened, so a write that changed a target is not reported as an add.                                                        |
| The suggestions   | The target field suggests the catalog ids and the combo names, the two things a target may be. They are suggestions, not a constraint: a failed read leaves the field usable and says so, and a combo list longer than the API's 100-row page says it is truncated rather than looking whole.                                                                   |
| Stale targets     | A stored alias whose target has since been disabled or deleted still renders, and a write that carries it is refused by the API with a sentence naming the alias. Q21 records that one such row refuses every alias edit until it is removed.                                                                                                                |
| The combo delete  | The dialog's lead sentence follows the error code, so only a `CONFLICT` reads as "still referenced" and a server failure no longer does. The editor's reference suggestions gained the alias names, the third source §7.7 resolves a ref from.                                                                                                                |

Not yet verified: the rendered half in a browser, and every write against a running `app-serv`. The
screen is client-rendered (`ssr = false`), so the tests exercise it against a stub that applies writes and
answers with the server's own refusals.

### Provider detail model writes, 2026-09-20

Run with Bun 1.3.14. This pass covers the disabled and custom model writes of `/providers/[provider_id]`
(SPEC-UI §6.3): the Disable action on every catalog row, the list of models this provider cannot route,
and the custom-model table with its add form and removal dialog.

| Check             | Result                                                                 |
| ----------------- | ---------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                   |
| `bun run test`    | 1551 tests passed across 62 files, 114 of them new in this pass        |
| `bun run lint`    | Prettier reports every file conforms                                   |
| `bun run lint:ts` | ESLint exits 0                                                         |
| `bun run build`   | succeeds, output in `build/`                                           |
| File size         | largest source file this pass is 179 lines, under the 220 warning line |
| Text hygiene      | 0 em dashes and 0 emoji across `src` and `tests`                       |

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

| Check             | Result                                                                    |
| ----------------- | ------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                      |
| `bun run test`    | 1437 tests passed across 57 files, 228 of them this slice plus navigation |
| `bun run lint`    | Prettier reports every file conforms                                      |
| `bun run lint:ts` | ESLint exits 0                                                            |
| `bun run build`   | succeeds, output in `build/`                                              |
| File size         | largest source file this slice is 195 lines, under the 220 warning line   |
| Text hygiene      | 0 em dashes and 0 emoji across `src` and `tests`                          |

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
| Text hygiene      | 0 em dashes and 0 emoji across `src` and `tests`                                                                                  |

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

| Element                                    | Action                | Outcome                                                                                                                                                                                                                  |
| ------------------------------------------ | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Header, theme toggle                       | Click twice           | `document.documentElement` gains and loses `dark`; body background measured `rgb(252, 250, 247)` light and `rgb(26, 25, 23)` dark, matching `DESIGN.md` §3.2                                                             |
| Header, API base Copy                      | Click                 | Label switches to "Copied", the value is on the clipboard, a live region announces it, and it reverts after 2s                                                                                                           |
| Header, Sign out                           | Click                 | Calls the session store; with `require_login = false` the panel stays usable, which is the stub's shape                                                                                                                  |
| Header, sidebar trigger at 390px           | Click                 | Drawer opens at 293px over a dimmed overlay, focus moves inside it                                                                                                                                                       |
| Drawer, Escape                             | Press                 | Drawer closes, focus returns to the header control                                                                                                                                                                       |
| Drawer, a nav link                         | Click                 | Navigates to `/settings` and the drawer closes behind it                                                                                                                                                                 |
| Sidebar, `Endpoint & Key`                  | Click                 | Navigates to `/endpoint-keys`; row carries `aria-current="page"` and the 3px accent marker                                                                                                                               |
| Sidebar, `Setting`                         | Click                 | Navigates to `/settings`                                                                                                                                                                                                 |
| Sidebar, `Media Provider`                  | Click                 | Expands to the six kinds; `aria-expanded` goes `false` then `true`, and back on a second click                                                                                                                           |
| Sidebar, `Media Provider` from a cold load | Observe               | Starts closed, and starts open only when a media kind is the current route                                                                                                                                               |
| Sidebar, a planned row                     | Click                 | Inert by design: no `href`, no pointer cursor, a visible `Planned` chip, and the title attribute says why                                                                                                                |
| Sidebar, rail collapse at 1440px           | Click                 | `data-state` goes `expanded` to `collapsed` and back; the preference is written to the `sidebar_state` cookie                                                                                                            |
| Sidebar at 820px                           | Load                  | Starts as a 64px icon rail, expands to full width on the trigger, collapses again, cookie reads `false` afterwards                                                                                                       |
| Keyboard, full pass                        | Tab from the top      | Order is logo, then each navigation row, then the header controls, then the page controls. Every stop shows a 2px accent outline. Enter follows a link. Enter and Space toggle the disclosure. Escape closes the drawer. |
| Overflow                                   | All three breakpoints | `document.documentElement.scrollWidth` never exceeds `clientWidth`, in both themes                                                                                                                                       |
| Tap targets                                | 390px and 820px       | Every visible sidebar, drawer, and header control is at least 44px tall. The desktop sidebar rail is a 14px drag handle with `tabindex="-1"`, so it is not a touch target and is not counted.                            |
| Console                                    | All three breakpoints | No page errors. The only 404 is `/api/v1/gateway-keys`, which the stub deliberately does not implement.                                                                                                                  |

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

1. **`app-serv` cannot boot on its P1 tree.** Migration `000007_usage_quota.up.sql` uses `window` as a
   column name, which is a PostgreSQL reserved word, and the runner stops with `SQLSTATE 42601` after
   migrations 000005 and 000006 apply. Until that is fixed no panel screen can be verified end-to-end.
   The fix belongs to `app-serv` P1 and is recorded in `SYSTEM_MAP.md`. Commands for a local run are in
   `app-serv/README.md`.
2. **Playground Chat cannot be built yet, but its auth model is decided.** The panel server injects the
   gateway key, so the browser never holds a credential (owner, 2026-09-17; spec §6.15). What remains is
   the `app-serv` side and a phase. The sidebar row carries `Planned`.
3. **Changelog has no release-note source.** SPEC-API §7 defines no changelog endpoint, so the screen at
   `/changelog` renders the running version from `GET /api/v1/version` and an honest empty release list.
   The list, ordering, per-release markers, categories, and all three states are implemented, so a source
   only has to feed `schemaChangelog`. Spec §14 Q11 lists the three options.
4. `AGENTS.md` scopes itself to the Go services (`app-*/**`), so it does not govern this panel. The
   panel's own rules come from `docs/SPEC-UI/001-SPEC-UI.md` §10, §11, and §7.1.5, and they are applied by
   hand here. The panel's gates live in `scrypts/gates/panel-check.sh`.
5. The drift gate that compares these schemas against SPEC-API §7 is not wired yet, because
   `docs/SPEC-API/002-SPEC-API-openapi.md` does not exist to compare against (spec §14 Q3).
6. Gateway key `status` values are not enumerated in SPEC-API §7.3. The panel currently writes `active`
   and `disabled` and renders any other value verbatim; see §14 Q9.

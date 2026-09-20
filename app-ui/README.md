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

Three passes are recorded here. The first is the U0 scaffold, measured 2026-09-16. The second is the
shell and sidebar work, measured 2026-09-18, and it is the R-35 click-through with its outcomes per
element. The third is the Token Saver and Proxy Pools pair, measured 2026-09-20.

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

Not yet verified: every write on both screens against a running `app-serv`. The tests exercise the panel
against a stateful stub that applies writes and returns the API's error shapes, which covers the panel's
half of the contract and not the service's.

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

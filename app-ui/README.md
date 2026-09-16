# app-ui

Operator panel for the KENTANG TECH pannelAI gateway. It is a thin, strict client of the management
API in `app-serv` and never touches PostgreSQL or Redis.

Spec of record: [`docs/SPEC-UI/001-SPEC-UI.md`](../docs/SPEC-UI/001-SPEC-UI.md).
API contract: [`docs/SPEC-API/001-SPEC-API.md`](../docs/SPEC-API/001-SPEC-API.md).

Status: **U0 scaffold**, verified locally. Screens built: `/login`, `/endpoint-keys` (gateway keys
tab), `/settings` (security tab). Everything else in the sidebar is marked Planned, because a nav item
without a route is a defect.

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
```

Set `PANEL_API_TARGET` before starting (see `.env.example`). A missing or malformed value fails the
first `/api/v1` request with a message naming the variable.

## How requests reach the gateway

The panel serves one origin. Its own server forwards `/api/v1/*` to `PANEL_API_TARGET`
(`src/hooks.server.ts` and `src/lib/server/proxy.ts`), so the browser never makes a cross-origin
request and the HttpOnly session cookie stays `SameSite=Lax`. Development and production share that
single path, which is why there is no dev-only CORS proxy in `vite.config.ts`.

## Recorded decisions

| Decision                                   | Reason                                                                                                                                                                                                                                                                        |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Official Node adapter, executed by Bun     | The Bun-specific SvelteKit adapter is still on a 1.0.x release; `docs/SPEC-UI/001-SPEC-UI.md` §10.1.2 allows this fallback, and the runtime stays Bun. Verified by the boot banner printing `Bun 1.3.14`.                                                                     |
| TypeScript 7 with `typescript` 6 alongside | `svelte-check` requires both packages before it will use the TypeScript 7 compiler, and it needs the `--tsgo` flag. `typescript` resolves to 6.0.3 for the tool, and `@typescript/native` is an alias of TypeScript 7.0.2. This is the setup `svelte-check` itself documents. |
| Zod 4.6.5 as the only validator            | Owner decision, `docs/SPEC-UI/001-SPEC-UI.md` §7. One schema module per resource feeds forms, URL parameters, and response parsing. No ad-hoc checks in components.                                                                                                           |
| `lucide-svelte` icons, no emoji            | The set the component layer ships with. Every navigation icon has a written reason in `src/lib/icons.ts`, and all 22 identifiers were verified against the installed package rather than guessed.                                                                             |
| Design tokens marked as a draft            | `DESIGN.md` does not exist yet, so `src/app.css` carries the minimum palette needed to build screens, with the status stated in the file header. §9.1 of the spec forbids inventing the missing direction.                                                                    |

## Layout

```
src/lib/api/         the only place that calls fetch: client, error mapping, one module per resource
src/lib/schemas/     Zod schemas: field primitives, sanitization transforms, response contracts
src/lib/server/      server-only code: environment validation and the /api/v1 forwarder
src/lib/components/  panel components (shell, dialogs, table rows, forms, state messages)
src/lib/stores/      session and theme state
src/routes/          one directory per route from the spec
tests/               table-driven unit tests
tests/support/       shared test helpers: the seeded corpus generator and the table runner
```

## Verification state

Measured on 2026-09-16 with Bun 1.3.14 and Go 1.26.5 available on the machine:

| Check           | Result                                                                                                                                                                                                    |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check` | 0 errors, 0 warnings                                                                                                                                                                                      |
| `bun run test`  | 310 tests passed across 11 files                                                                                                                                                                          |
| `bun run build` | succeeds, output in `build/`                                                                                                                                                                              |
| `bun run start` | boots on Bun, prints `pannelAI panel 0.0.1 on Bun 1.3.14` and the resolved API target                                                                                                                     |
| Panel routes    | `/` returns 200, `/login` returns 200                                                                                                                                                                     |
| Forwarding      | with `app-serv` down, `/api/v1/auth/status` returns 500 with the panel's error envelope; with a malformed `PANEL_API_TARGET`, it returns 500 naming the variable, while the panel itself still serves 200 |
| Property tests  | 1600 generated inputs across 4 profiles in `tests/schemas/sanitize-fuzz*.test.ts`; 0 unstable outputs. They found the composition defect fixed in `src/lib/schemas/sanitize.ts`                           |
| File size       | largest file is 184 lines and the largest under `src/` is 165, both under the 250 limit and below the 200 warn line                                                                                       |
| Test shape      | 35 test functions: 30 generated from a table, 4 iterating a table inside the body, 1 regression case with two assertions, so 0 assert a single input                                                      |
| Text hygiene    | 0 em dashes and 0 emoji in `src`, `tests`, and `scripts`                                                                                                                                                  |

Not yet verified: any call that needs a running `app-serv`. Sign-in, key creation, and settings writes
were exercised against the panel's own error paths only, so the screens' happy paths still need a live
gateway.

## Open items that block later phases

1. `AGENTS.md` exists and scopes itself to the Go services (`app-*/**`), so it does not govern this
   panel. The panel's own rules come from `docs/SPEC-UI/001-SPEC-UI.md` §10, §11, and §7.1.5 and are
   followed by hand here, because `scrypts/` does not exist yet to enforce them.
2. `DESIGN.md` does not exist, so U1 screens cannot be styled and this phase's visuals stay a draft
   without direction (§9.1).
3. `scrypts/` does not exist, so the drift gate that compares these schemas against SPEC-API §7 is not
   wired yet.
4. Gateway key `status` values are not enumerated in SPEC-API §7.3. The panel currently writes `active`
   and `disabled` and renders any other value verbatim; see §14 Q9.

# 045-DRAFT: Phase 1 UX/UI Baseline Audit

| Field | Value |
|---|---|
| Document ID | `045-PHASE-1-UX-BASELINE-AUDIT` |
| Status | Draft; static repository inspection only |
| Date | 2026-10-15 |
| Repository | `rusmanadodi2598/pannelAI` |
| Scope | `app-ui/**`, with `DESIGN.md` and `docs/SPEC-UI/001-SPEC-UI.md` as authorities |
| Related plan | `docs/DRAFT/044-DRAFT-PANEL-UX-UI-IMPROVEMENTS.md` |
| Implementation status | No UI implementation changes in this phase |

## 1. Purpose and audit boundary

Phase 1 establishes a repeatable baseline before UI implementation. It inventories documented routes against the route structure visible in the repository and defines the evidence required for a live visual and interaction audit.

**Important limitation:** this is a static repository/documentation audit. No live browser session, screenshots, runtime console/network capture, accessibility scanner, or agent execution was performed for this document. Route directories and files indicate implementation structure, not proof that every route works at runtime.

## 2. Source-of-truth rules

1. `DESIGN.md` controls visual direction: warm-neutral surfaces, a single coral accent, Inter, restrained motion, approved tokens, and explicit contrast requirements.
2. `docs/SPEC-UI/001-SPEC-UI.md` controls route, screen behavior, and product/API scope.
3. A screen may render only contract-backed data. No fabricated metrics, provider state, usage, quota, uptime, or activity.
4. Existing routes and design tokens should be refined rather than replaced without explicit approval.
5. Any proposed API, schema, or route change must be separately scoped and approved, with specification and contract tests updated as needed.

## 3. Route inventory

The table compares the documented route map with the route entries visible under `app-ui/src/routes` on the default branch. “Structure present” means a corresponding route file/directory was visible in the repository listing; it is not a runtime pass.

| Route | Intended task (SPEC-UI) | Documented phase | Static structure | Baseline verification still required |
|---|---|---:|---|---|
| `/login` | Password login | U0 | Present | Login success/failure, focus, validation, expired/invalid session |
| `/` | Redirect to `/endpoint-keys` | U0 | Present | Redirect target, unauthenticated behavior, refresh |
| `/endpoint-keys` | Gateway keys | U0, U1 | Present | Empty/loading/error, key create/revoke, secret masking, duplicate submit |
| `/providers` | Provider registry, search/filter | U1 | Present | Search, pagination, status semantics, API failure |
| `/providers/[provider_id]` | Provider detail and model catalog | U1, U2 | Present under provider route group | Invalid ID, tabs, edit/test flows, not-found and API errors |
| `/combos` | Combos and Vision Adapter | U1, U2 | Present | Tab behavior, validation, save/error state |
| `/usage` | Usage overview and records | U1 | Present | Units/time windows, zero vs unavailable, filters/pagination |
| `/quota` | Quota windows and budget caps | U1, U2 | Present | Boundary values, missing data, safe edits and failures |
| `/token-saver` | Token-saver configuration | U2 | Present | Field validation, unsaved changes, persisted/read-back values |
| `/media-providers/[kind]` | Media provider by kind | U2 | Parent route group present | All supported kinds, invalid kind, planned items do not act as links |
| `/proxy-pools` | Proxy rows, tests, outbound proxy settings | U2 | Present | Safe test behavior, enabled-with-empty-URL guard, mutation errors |
| `/skills` | Agent skill catalog | U3 | Present | Empty/unavailable state, data freshness and link semantics |
| `/logs` | Request logs | U1 | Present | Filter/pagination, long rows, no-results and API failure |
| `/console-log` | Console ring buffer | U1 | Present | Clear action confirmation, empty state, failure handling |
| `/playground` | Playground chat | U3 | Present | Loading/error, message submission, credential never exposed to browser |
| `/changelog` | Gateway release notes and running build | U0 | Present | Version mismatch, empty release list, unavailable endpoint |
| `/api-docs` | Read-only API v1 reference | U1 | Present | OpenAPI loading/error, long schema layout, read-only behavior |
| `/settings` | Security, Routing, Network, Logging | U1, U2 | Present | Tab persistence, validation, unsaved changes, safe secret handling |
| Unknown path | Not-found view with return navigation | U0 | Shared error component visible | Authenticated/unauthenticated unknown path, keyboard access |

### Inventory interpretation

- The route map documents 18 named routes including the root redirect, plus unknown-path handling.
- The route directory contains route groups for the documented feature areas. Some route patterns are dynamic (provider ID and media kind), so a directory alone does not prove all parameter cases work.
- The specification and repository notes record prior successful checks for some shell/API flows, but those historical notes are not a fresh Phase 1 browser verification.
- The current phase does not mark any route as visually passed or runtime passed.

## 4. Required live audit matrix

For each route above, the implementation PR must add evidence for the following applicable dimensions:

| Dimension | Minimum evidence | Pass condition |
|---|---|---|
| Route and navigation | Direct load, in-app navigation, refresh, back/forward | Correct destination and active navigation state; no dead interactive links |
| Authentication | Signed-out and signed-in behavior where applicable | No protected content or credential exposed before authentication |
| Loading | Browser capture during pending request or deterministic test | Loading state is visible and does not imply false success |
| Success | Valid API-backed happy path or deterministic fixture | Displayed state matches the API response |
| Empty/no-results | Empty collection and filtered no-match | Explicit, non-error empty state with clear next action where appropriate |
| Error | API unavailable, validation failure, unauthorized, server failure as applicable | Actionable, localized feedback; no stack trace, secret, or raw upstream payload |
| Keyboard | Keyboard-only pass and focus screenshots where useful | Logical focus order, visible focus, all controls operable |
| Accessibility | Automated scan plus manual semantic/focus review | No new serious/critical automated violations; manual issues recorded |
| Responsive | 390px, 820px, and desktop viewport captures | No accidental page overflow; controls and navigation remain usable |
| Theme | Light and dark captures for each touched screen | Both modes use approved tokens and meet documented contrast constraints |
| Console/network | Browser console and request log | No unexplained runtime errors; API requests and responses match expected contracts |
| Sensitive data | Inspect UI, URL, browser storage, console, and network behavior | No unintended secret exposure or persistence |

Do not claim a row passes based solely on a unit test that reads token files or a stubbed API response. Rendered output and contract fidelity must be checked independently.

## 5. Viewport and theme matrix

Minimum viewport targets are based on the existing responsive design direction and the UX/UI improvement plan:

- **Mobile:** 390px viewport width.
- **Tablet:** 820px viewport width.
- **Desktop:** 1440px viewport width.
- **Themes:** light and dark.

For the full baseline, capture the shell/navigation in all six viewport/theme combinations. For feature screens, capture at least one desktop and one mobile state in both themes; add tablet captures where navigation, table layout, or dialogs differ. Include at least one loading, empty, and error state for data-driven screens, and successful flows for screens with mutations.

Store evidence in the implementation PR or an agreed audit artifact location. Avoid screenshots containing real credentials, API keys, session cookies, or sensitive production data.

## 6. Finding format and severity

Every finding must include:

- **ID:** stable identifier, e.g. `UX-P1-001`.
- **Severity:** `P0` blocker/security/data-integrity; `P1` critical operator workflow or major usability defect; `P2` consistency/accessibility/responsive defect with a viable workaround; `P3` minor polish.
- **Route and state:** exact route, theme, viewport, and reproduction state.
- **Evidence:** screenshot, test output, console/network observation, or exact specification reference.
- **Expected vs actual:** observable behavior, not subjective language alone.
- **Recommendation:** smallest safe change aligned with `DESIGN.md` and SPEC-UI.
- **Owner:** named maintainer/reviewer or explicitly unassigned.
- **Acceptance test:** verifiable condition that closes the finding.

Security, contract fidelity, and critical-flow failures cannot be waived by an average score.

## 7. Phase 1 exit criteria

Phase 1 is complete only when all of the following are recorded:

- [x] Static route inventory created from the documented route map and repository route structure.
- [x] Visual and behavioral sources of truth documented.
- [x] Runtime/browser verification clearly marked as outstanding rather than assumed.
- [x] Repeatable live-audit dimensions and viewport/theme targets defined.
- [x] Finding format and severity scale defined.
- [ ] Live browser baseline screenshots captured.
- [ ] Console/network observations recorded from a running panel.
- [ ] Route-by-route loading, success, empty, and error states verified where applicable.
- [ ] Keyboard/accessibility and responsive baseline completed.
- [ ] Findings prioritized with owners and acceptance tests.

The unchecked items require a running browser environment and should be completed before selecting broad UI implementation scope. This PR does not claim they are done.

## 8. Recommended first implementation slice after baseline

After the live baseline is captured, choose one small P0 slice rather than refreshing every screen at once. The preferred candidate is the shared shell and page-state feedback because it affects multiple routes:

1. Validate active navigation and route semantics.
2. Validate shared loading, empty, and API-error presentation.
3. Verify that recoverable failures preserve form values and do not report false success.
4. Add focused regression tests and rendered-browser evidence.
5. Keep API, route, and schema behavior unchanged.

If the baseline identifies a security or data-integrity P0 issue elsewhere, address that first.

## 9. Agent/reviewer coordination

The following names were supplied as intended participants: Impeccable test project, Blackarch, Xman Autonomous Agent, Orchestra, Astra, and UI. Their permissions, availability, and actual execution results are not verified by this document.

If used, assign explicit independent deliverables:
- UX audit: task flow, navigation, empty/error states.
- Visual audit: token compliance and viewport/theme screenshots.
- Accessibility audit: keyboard, semantics, focus, contrast, reduced motion.
- Engineering audit: SvelteKit/API boundaries, contract fidelity, tests.
- Coordinator/maintainer: deduplicate findings, assign owners, decide implementation scope.

Do not treat a participant mention as evidence that an audit or test ran.

## 10. Decision requested

Reviewers are asked to:
1. Accept this as the Phase 1 **static baseline artifact**, not a completed live audit.
2. Assign an owner to complete the unchecked browser/runtime verification items.
3. Approve the severity and evidence format for findings.
4. Select the first implementation slice only after reviewing the live findings.

**Recommendation:** accept the inventory and audit protocol, then complete the browser baseline before making broad visual changes.
# 044-DRAFT: pannelAI (`app-ui`) UX/UI Improvement Plan

| Field | Value |
|---|---|
| Document ID | `044-DRAFT-PANEL-UX-UI-IMPROVEMENTS` |
| Status | Draft for review |
| Date | 2026-10-15 |
| Repository | `rusmanadodi2598/pannelAI` |
| Target | `app-ui/**` |
| Related documents | `DESIGN.md`, `docs/SPEC-UI/001-SPEC-UI.md`, `app-ui/README.md` |
| Decision type | UX/UI improvement proposal; implementation is not authorized by this draft |

## 1. Executive summary

This draft proposes a focused UX/UI improvement pass for the pannelAI operator dashboard. The product is an AI Gateway control panel for technical operators who regularly inspect provider health, routing, usage, quotas, logs, and configuration. The interface should optimize for fast scanning, safe configuration changes, clear system state, and predictable navigation—not marketing-style visuals.

The repository already defines an active design direction in `DESIGN.md`: a quiet, dense, factual operator interface; warm-neutral surfaces; one coral accent; Inter typography; restrained motion; and explicit contrast requirements. Therefore, this proposal recommends **refining and validating the existing design system**, not replacing it with an unrelated visual theme.

The `app-ui` is a SvelteKit application using Bun, Zod, shadcn-svelte primitives, and a single-origin `/api/v1/*` forwarding path. UX changes must preserve these boundaries and must not invent UI data or controls unsupported by the management API and product specification.

## 2. Goals

1. Reduce time required to locate and understand an operational issue.
2. Make navigation, screen hierarchy, and page-level actions consistent.
3. Improve readability of dense tables, status indicators, forms, and logs.
4. Make destructive or security-sensitive actions deliberate.
5. Improve keyboard, screen-reader, responsive, and touch usability.
6. Make loading, empty, success, partial-failure, and API-error states explicit.
7. Preserve documented design tokens, API contracts, strict validation, and test-first workflow.

## 3. Non-goals and constraints

- Do not redesign the brand identity or introduce competing accent colours.
- Do not add decorative gradients, excessive shadows, gratuitous animation, or marketing-dashboard patterns.
- Do not fabricate metrics, provider health, quota, usage, or activity when the API has not returned them.
- Do not expose secrets in plain text, browser logs, URLs, or persisted client state.
- Do not add screens, fields, controls, or API semantics outside `docs/SPEC-UI/001-SPEC-UI.md` and the API contract without updating the relevant specification in the same change.
- Do not connect the UI directly to PostgreSQL or Redis; the panel remains a client of the management API.
- Do not hand-edit generated shadcn-svelte primitives when token or composition-level changes are sufficient.
- Keep user-facing UI copy in English, consistent with the existing UI specification.

## 4. Repository baseline and design authority

The current repository documentation establishes these constraints:

- `DESIGN.md` is the active visual source of truth: warm-neutral palette, coral accent discipline, Inter typography, dense readable content, restrained motion, and measured contrast.
- `docs/SPEC-UI/001-SPEC-UI.md` is the behavioral and screen-scope contract. `DESIGN.md` takes precedence for visual values; the UI specification takes precedence for routes, behavior, and endpoint scope.
- `app-ui/README.md` documents the U0 scaffold and prior checks. It also notes that some successful API-backed flows still need verification against a running `app-serv`.
- The UI uses SvelteKit, Bun, Zod, shadcn-svelte primitives, self-hosted Inter, centralized navigation, and shared design tokens.
- A prior regression occurred when CSS tokens existed and tests passed but the stylesheet was not imported. Visual verification must inspect the rendered browser output, not rely on token tests alone.

These are documentation-derived findings, not a claim that every current screen has been freshly audited in a live browser.

## 5. Proposed workstreams

### P0 — Correctness, orientation, and trust

#### 5.1 Navigation and information architecture

- Preserve the documented five task groups and make their boundaries easy to scan.
- Make the active route unmistakable without relying on colour alone.
- Ensure each visible navigation item leads to a working route; unsupported destinations must not masquerade as links.
- Use consistent page titles, concise contextual descriptions where useful, and predictable placement for primary actions.
- Preserve location after refresh where appropriate and avoid unexpected navigation after save or error.
- Check long labels, nested items, collapsed navigation, and narrow viewports.

**Acceptance criteria**
- Every navigable item maps to an existing route, verified by navigation tests.
- Current location is identifiable without colour alone.
- No visible dead links or controls.
- Navigation is keyboard- and touch-usable across supported breakpoints.

#### 5.2 Page states and API feedback

Define consistent patterns for initial loading, background refresh, empty state, filtered no-results, API unavailable, expired session/unauthorized, validation failure, server error, and successful or failed writes.

Keep errors close to the relevant control or content. Preserve user-entered values after recoverable failures. Avoid replacing a whole page with a generic error when a localized message is sufficient. Do not automatically retry mutations unless the operation is explicitly safe and idempotent.

**Acceptance criteria**
- Each touched data screen has tests for loading, empty, success, and failure states.
- Error copy is actionable without leaking stack traces, internal URLs, secrets, or raw upstream payloads.
- Failed writes never falsely show success or discard user input.

#### 5.3 Forms and sensitive actions

- Group fields by task; use explicit labels, concise help text, and field-level validation.
- Associate validation messages with their fields and focus or summarize invalid fields after submit.
- Use the existing unsaved-change guard where supported.
- Distinguish create, edit, and read-only states.
- Default secret/key material to masked presentation; reveal only through a deliberate action.
- Confirm destructive/security-sensitive actions with the target and consequence stated clearly.
- Prevent duplicate submissions while a mutation is in flight, while preserving a usable failure path.

**Acceptance criteria**
- Keyboard users can complete every form.
- Destructive actions cannot be triggered by ambiguous icon-only controls.
- Secrets are not written to console output, analytics, URLs, or local storage.

### P1 — Operational efficiency

#### 5.4 Dense tables and operational data

- Standardize alignment, row height, column labels, truncation, and overflow.
- Use tabular numerals for quantities such as tokens, latency, quota, and cost where appropriate.
- Right-align comparable numeric values; use monospace for identifiers and log excerpts where useful.
- Make sorting/filtering state visible and provide a clear reset path.
- Preserve filter context during pagination/refresh when supported.
- Pair status colours with text or accessible labels.
- Use an intentional narrow-screen strategy: selected columns, controlled horizontal scrolling, or compact presentation.

**Acceptance criteria**
- Long IDs, model names, provider names, and log messages do not break the layout.
- Numeric columns remain easy to compare.
- Sort/filter/pagination states are perceivable and test-covered.
- Zero values are distinguishable from missing or unavailable data.

#### 5.5 Overview and operational hierarchy

Where an overview is defined by the UI/API specification:
- Put decision-relevant information first.
- Separate current state from historical trends and configuration.
- Include labels, units, and time windows for metrics.
- Show freshness/last-updated information only when available from a reliable source.
- Add charts only when they answer an operational question.
- Distinguish stale, unavailable, and zero-valued metrics.

Do not add new routes or API-backed metrics solely for a visual refresh. Confirm screen and data scope first.

#### 5.6 Consistency across task groups

Create shared conventions for page headers, primary/secondary actions, search/filter rows, table/detail layouts, status badges, inline feedback, dialogs/drawers, and loading/empty states. Let each screen use a composition suited to its task; consistency should come from shared behavior and tokens, not identical card grids.

### P2 — Accessibility, responsive behavior, and polish

#### 5.7 Accessibility

- Verify text and non-text contrast against thresholds in `DESIGN.md`.
- Ensure visible focus indicators and logical keyboard order.
- Use semantic headings, buttons, links, labels, and table headers.
- Provide accessible names for icon-only controls.
- Announce asynchronous success/error feedback where appropriate.
- Respect reduced-motion preferences.
- Never communicate status, selection, or errors through colour alone.
- Verify dialog focus entry, containment, Escape behavior, and focus restoration.

**Acceptance criteria**
- Core flows are keyboard-operable without a pointer.
- Automated accessibility checks introduce no new serious/critical violations.
- Manual checks cover focus order, dialogs, form errors, and status announcements.

#### 5.8 Responsive and touch behavior

- Validate at 390px, 820px, and desktop viewport classes.
- Keep visible interactive touch targets at least 44px high, consistent with existing panel notes.
- Ensure the sidebar drawer can be opened, navigated, and dismissed on small screens.
- Prevent fixed headers, drawers, and dialogs from obscuring focused controls.
- Avoid page-level horizontal overflow; permit it only in deliberately designed data regions.
- Test long content, browser zoom, and narrow device widths.

#### 5.9 Motion and visual polish

- Limit motion to hover, focus, state transitions, and documented mobile drawer behavior.
- Avoid scroll choreography and animations that slow repeated operator tasks.
- Ensure transitions do not conceal loading/error state changes.
- Reuse established spacing, border, radius, and surface tokens.
- Keep the accent within the roles specified in `DESIGN.md`.

## 6. Evaluation rubric

Score each applicable screen from 0 to 2: 0 = missing/broken, 1 = partial/inconsistent, 2 = meets standard.

| Criterion | Verification |
|---|---|
| Orientation | Purpose, active route, and primary action are clear |
| State clarity | Loading, empty, success, error, stale, and unavailable states are unambiguous |
| Data legibility | Data, units, IDs, status, and numeric values are easy to scan |
| Action safety | Sensitive/destructive actions are explicit; duplicate submissions are prevented |
| Accessibility | Keyboard, focus, semantics, labels, contrast, and status announcements |
| Responsive layout | Small, medium, and desktop layouts remain usable |
| Design consistency | Approved tokens and shared interaction patterns are used |
| Contract fidelity | No fabricated data or unsupported routes/actions; errors respect API boundaries |

**Release gate:** no criterion scores 0 for a critical operator flow. Accessibility and contract fidelity must pass before merge. Record exceptions with rationale and owner.

## 7. Implementation sequence

1. **Inventory and baseline:** map implemented routes to the UI spec; capture browser screenshots at narrow, medium, and desktop widths; record console/network/accessibility findings; separate verified defects from hypotheses.
2. **Shared foundations:** review shell, navigation, page headers, form feedback, status badges, and table patterns. Reuse tokens/primitives. Add or update tests before behavior changes under the documented test-first rules.
3. **P0 fixes:** resolve navigation correctness, page-state ambiguity, form feedback, and sensitive-action safety. Inspect rendered browser output, not only unit tests or token files.
4. **P1 operational patterns:** improve table readability, filters, metric labeling, and consistency. Do not expand scope without specification/API review.
5. **P2 quality pass:** run keyboard/accessibility, responsive, reduced-motion, and visual-regression checks.
6. **Release review:** run documented panel checks, tests, type checks, and production build. Exercise API-backed happy paths against a running `app-serv`; error-path tests alone do not prove successful integration. Attach before/after evidence and acceptance results to the implementation PR.

## 8. Suggested review roles

The following labels were provided as project participants/references: **Impeccable test project**, **Blackarch**, **Xman Autonomous Agent**, **Orchestra**, and **Astra**. Their availability, permissions, and role definitions have not been verified. If these are intended as agents/reviewers, assign explicit responsibilities rather than assuming checks have already been executed.

Suggested responsibilities:
- **UX audit:** task flows, navigation, empty/error states, operator efficiency.
- **Visual design audit:** alignment with `DESIGN.md`, token reuse, hierarchy, responsive composition.
- **Accessibility audit:** keyboard, focus, semantics, contrast, reduced motion.
- **Engineering review:** SvelteKit boundaries, API contract fidelity, Zod validation, test coverage, regressions.
- **Orchestration/maintainer:** scope control, duplicate findings, prioritization, acceptance, final merge decision.

No audit by the named participants is claimed to have been completed.

## 9. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Visual refresh conflicts with existing direction | Treat `DESIGN.md` as visual source of truth |
| UI displays invented/stale values | Render only contract-backed data; show freshness/unavailable states honestly |
| Scope expands into backend/product changes | Update UI/API specifications and review scope before implementation |
| Shared primitive changes create regressions | Prefer tokens/composition; add focused tests and inspect affected screens |
| Tests pass while rendered UI is broken | Include browser visual inspection and console/network checks |
| Sensitive values become easier to expose | Mask by default; review logs, storage, clipboard, and reveal interactions |
| Mobile usability is checked too late | Include responsive screenshots and touch-target checks in baseline |

## 10. Definition of done

- [ ] All changes map to documented screens and behavior.
- [ ] `DESIGN.md` tokens and accent rules are preserved, or updated in the same PR with rationale.
- [ ] UI/API contract remains unchanged unless the relevant specification is updated and reviewed.
- [ ] Loading, empty, success, validation, and API-error states are tested for touched flows.
- [ ] Keyboard and responsive checks pass for touched flows.
- [ ] No new serious/critical accessibility violations are introduced.
- [ ] No unsupported or fabricated data is shown.
- [ ] No secrets are exposed in logs, URLs, or persistent browser storage.
- [ ] Browser console and network behavior are inspected.
- [ ] Panel lint, type checks, tests, and production build pass.
- [ ] API-backed happy paths are verified against a running backend where relevant.
- [ ] Before/after evidence and unresolved limitations are recorded in the implementation PR.

## 11. Decision requested

Reviewers are asked to decide:
1. Approve refining the existing `DESIGN.md` system rather than replacing it.
2. Approve the P0 → P1 → P2 sequence.
3. Confirm which named participants are actual agents/reviewers and assign concrete deliverables.
4. Select the first implementation slice based on a live route audit and captured evidence.
5. Keep this document as a proposal until scope and acceptance criteria are approved.

**Recommendation:** approve the direction conditionally, then perform a route-by-route baseline audit before implementation. Prioritize correctness, trustworthy operational states, navigation, and form safety before cosmetic polish.

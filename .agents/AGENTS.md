# AGENTS.md — Agent Rules Index

This file is the entry point for AI coding agents working in this repository. Read the applicable rules before planning, editing, reviewing, or testing.

## Mandatory rules

- [OWASP security rules](./rulles/OWASP.md) — security requirements, prohibited patterns, and security testing.
- [TDD rules](./rulles/TDD.md) — test-driven development, regression coverage, and testing discipline.

## Stack guidance

Apply language- and framework-specific guidance only to the component being changed. Preserve the repository's existing architecture and manifests; do not impose Go-only rules on Rust/Python components or Rust/Python rules on Go/Svelte/TypeScript components.

- **Rust:** use the repository's pinned toolchain and Cargo configuration; use typed errors, safe ownership/borrowing, and narrowly scoped documented `unsafe` code. Run configured formatting, lint, and tests where available.
- **Python:** follow the repository's configured Python version and environment/package manager; use type hints for public interfaces, validate inputs at boundaries, and use parameterized SQL and safe subprocess argument arrays.
- **Go / TypeScript / Svelte:** follow existing project conventions and manifests for the affected component.

## Execution policy

- Inspect the relevant source and project configuration before proposing changes.
- Add or update regression tests for behavior changes.
- Never claim a build, test, lint, or security check passed unless it was actually run and the result observed.
- If a rule conflicts with the actual project configuration, document the conflict and follow the concrete configuration without weakening security or correctness.
- Keep changes focused; avoid unrelated refactors.

## Directory map

```text
.agents/
├── AGENTS.md          # This index map
└── rulles/
    ├── OWASP.md       # Security rules
    └── TDD.md         # Test-driven development rules
```

## Maintenance

Add new mandatory policies under `.agents/rulles/` and link them here with a short description. Keep the source documents under `docs/RULLES/` unchanged unless separately requested.

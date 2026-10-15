# AGENTS.md — Index Map

This file is the central index for AI-agent instructions and mandatory engineering policies in this repository.

## Mandatory rules

- [OWASP.md](./rulles/OWASP.md) — security requirements, prohibited patterns, and security testing.
- [TDD.md](./rulles/TDD.md) — test-driven development, regression coverage, and testing discipline.

## Stack guidance

Apply language- and framework-specific guidance only to the component being changed. Preserve existing architecture and manifests; do not impose Go-only rules on Rust/Python components or Rust/Python rules on Go/Svelte/TypeScript components.

- **Rust:** follow the repository's pinned toolchain and Cargo configuration; prefer typed errors and safe ownership/borrowing; narrowly scope and document any `unsafe` code.
- **Python:** follow the configured Python version and environment/package manager; use type hints for public interfaces, validate inputs at boundaries, and use parameterized SQL and safe subprocess argument arrays.
- **Go / TypeScript / Svelte:** follow the existing project conventions and manifests for the affected component.

## Execution policy

- Inspect relevant source and project configuration before changing code.
- Add or update regression tests for behavior changes.
- Never claim a build, test, lint, or security check passed unless it was run and the result observed.
- If a rule conflicts with the actual project configuration, document the conflict and follow the concrete configuration without weakening security or correctness.
- Keep changes focused and avoid unrelated refactors.

## Directory map

```text
.agents/
├── AGENTS.md          # This index map
└── rulles/
    ├── OWASP.md       # Security rules
    └── TDD.md         # Test-driven development rules
```

Keep source documents under `docs/RULLES/` unchanged unless separately requested. Add future mandatory policies under `.agents/rulles/` and link them here.

# Agent Rules Index

This directory is the central index for AI-agent guidance and engineering policies in this repository.

## Required reading

Before planning, editing, reviewing, or testing code, read:

1. [Agent rules](./rules/AGENT-RULES.md) — workflow, architecture, coding conventions, stack-specific guidance, and PR checklist.
2. [OWASP security rules](./rules/OWASP.md) — security requirements, prohibited patterns, and security test cases.
3. [Strict TDD rules](./rules/TDD.md) — anti-hardcoding requirements, implementation analysis, and testing discipline.

## How to apply these rules

- Treat the rules above as mandatory unless a higher-priority instruction or an explicit repository policy says otherwise.
- Apply language-specific requirements only to the stack/component being changed; do not impose Go-only rules on Rust or Python code, or vice versa.
- Preserve the existing architecture and package/toolchain choices. Do not introduce migrations or unrelated refactors merely to satisfy generic guidance.
- If a rule conflicts with the actual project manifests or source layout, document the conflict and follow the concrete project configuration while keeping security and correctness requirements intact.
- Never report a test, lint, build, or security check as passing unless it was actually run and the result observed.

## Directory map

```text
.agents/
├── AGENT.md                 # This index / entry point
└── rules/
    ├── AGENT-RULES.md       # Coding-agent workflow and stack guidance
    ├── OWASP.md             # Security policy
    └── TDD.md               # Test-driven development policy
```

## Maintenance

When adding a new agent policy, place it in the appropriate subdirectory under `.agents/` and add a link plus a short description here. Keep canonical source documents under `docs/RULLES/` unchanged unless a separate change explicitly updates those sources.

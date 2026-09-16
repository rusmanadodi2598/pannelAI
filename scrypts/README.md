# scrypts/

Quality gates and git hooks for pannelAI. Everything here is versioned, so a
gate is reviewed like any other source file and CI can call exactly what the
hooks call.

## Install

```bash
scrypts/hooks/install.sh
```

That sets `core.hooksPath` to `scrypts/hooks`. The hooks are not copied into
`.git/hooks`: a copy there is invisible to git, unrunnable in CI, and silently
stale after an update.

## Gates

| Gate | Checks |
|---|---|
| `gates/go-lint.sh` | `go vet`, `gofmt -l`, `staticcheck`, `golangci-lint`, on the default and `integration`-tagged builds |
| `gates/go-headers.sh` | AGENTS.md §1.2 header contract on every hand-authored Go file |
| `gates/go-test.sh` | `go test -race`, plus the tagged integration suite when a DSN is set |
| `gates/panel-check.sh` | app-ui: prettier, ESLint, svelte-check, vitest, production build |
| `gates/secrets.sh` | gitleaks over commits and the working tree, or the staged patch |
| `gates/contract-drift.sh` | SPEC-API §8 error codes against the panel's closed enum |
| `gates/all.sh` | every gate above, with a summary |

Run everything with:

```bash
scrypts/gates/all.sh
SKIP_PANEL_BUILD=1 scrypts/gates/all.sh   # skip the slowest panel step
```

## Hooks

| Hook | Runs | Why this split |
|---|---|---|
| `pre-commit` | gofmt on staged Go files, the header contract, gitleaks on the staged patch, contract drift when the spec or the error enum changed | These must not be deferred: a secret in history cannot be undone by deleting the file, and a stray formatting commit is noise. Everything here is fast, because a hook that takes minutes gets bypassed. |
| `pre-push` | the full gate set | A push is what other people see, so the slow checks (test suite, staticcheck, panel build) belong here. |

Bypass with `git commit --no-verify` / `git push --no-verify`. That should be
rare and worth explaining in the pull request.

## Configuration

- `.gitleaks.toml` (repository root) extends the default ruleset with an
  allowlist for two values that the `generic-api-key` heuristic flags: a masking
  test fixture and the key-generation alphabet. Both are named by literal value,
  not by path, so a real credential in those same files is still caught.
- `golangci-lint` runs only when installed. Its absence is reported as a skip
  rather than a pass, because a gate that silently succeeds without its tool
  reports safety it never checked.

## Bun on CPUs without AVX2

Bun ships two x86-64 builds, and only the `baseline` one runs on a CPU without
AVX2. The default build dies with SIGILL in that case, even for `bun --version`.
This is a property of the installed binary, not of the machine:

```bash
curl -fsSL -o bun.zip \
  https://github.com/oven-sh/bun/releases/latest/download/bun-linux-x64-baseline.zip
unzip bun.zip && install -m755 bun-linux-x64-baseline/bun ~/.local/bin/bun
```

`gates/panel-check.sh` probes Bun by executing it and searches `PATH` plus
`~/.local/bin`, so a baseline build there is picked up automatically. When no
working Bun is found, the gate falls back to npm/node and reports the fallback
as a skip, because that leaves the production `bun --preload` start path
unverified.

## Requirements

| Tool | Needed by | Install |
|---|---|---|
| `go` 1.26+ | go-lint, go-test, go-headers | https://go.dev/dl/ |
| `gofmt` | go-lint, pre-commit | ships with Go |
| `staticcheck` | go-lint | `go install honnef.co/go/tools/cmd/staticcheck@latest` |
| `golangci-lint` | go-lint (optional) | https://golangci-lint.run/welcome/install/ |
| `gitleaks` 8.30+ | secrets, pre-commit, pre-push | https://github.com/gitleaks/gitleaks#installing |
| `bun` or `node`+`npm` | panel-check | https://bun.sh (or Node 22+) |
| `PANNELAI_TEST_POSTGRES_DSN` | go-test integration suite | point it at a throwaway database |
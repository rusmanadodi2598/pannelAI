# app-bot

Telegram media bot for pannelAI. Contract: `docs/SPEC-BOT/001-SPEC-BOT.md`.
Phase 1 carries the module, typed config, logger, router, health routes, and
the migration set. Telegram intake starts in P2.

## Run

```bash
export PATH=$HOME/sdk/go/bin:$PATH GOTOOLCHAIN=local
go run ./cmd/app-bot -migrate
go run ./cmd/app-bot
```

Required environment:

| Variable       | Notes                                              |
| -------------- | -------------------------------------------------- |
| `APP_ENV`      | `development`, `staging`, or `production`          |
| `DATABASE_URL` | PostgreSQL DSN with `search_path=bot`              |
| `REDIS_ADDR`   | Defaults to `127.0.0.1:6379`                       |
| `REDIS_PASSWORD` | Required on this host                            |
| `HTTP_ADDR`    | Defaults to `127.0.0.1:8090`                       |
| `GATEWAY_BASE_URL` | https base for the chat data plane                |
| `GATEWAY_KEY`  | Bearer for the gateway, required                   |
| `BUILDER_MODEL` | Chat model for the six-section builder             |
| `BUILDER_MAX_TOKENS` | Defaults to `700`                          |
| `BUILDER_TIMEOUT` | Defaults to `20s` per attempt                  |
| `BUILDER_DAILY_CAP` | Defaults to `60` calls per user per day        |
| `PROMPT_MAX_CHARS` | Defaults to `2000`, the pre-builder cap            |

Routes: `GET /healthz`, `GET /readyz`, `GET /metrics` (loopback only).

## Gates

```bash
go vet ./... && go test -race ./...
bash ../scrypts/gates/antislop.sh
```

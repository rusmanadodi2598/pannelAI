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

Routes: `GET /healthz`, `GET /readyz`, `GET /metrics` (loopback only).

## Gates

```bash
go vet ./... && go test -race ./...
bash ../scrypts/gates/antislop.sh
```

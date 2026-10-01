# storeit

Asset management app: `backend/` (Go modular monolith: chi, oapi-codegen strict
server, pgx, River, sqlc, goose) and `frontend/`.

## Backend

- Commands (run in `backend/`): `make test`, `make check` (sqlc-compile, fmt, vet, lint,
  test), `make generate` (sqlc + oapi-codegen; never hand-edit `*.gen.go`,
  `events/db`). `CI=true go test ./...` makes Docker-backed tests fail instead of skip.
- **Platform packages** (`backend/internal/platform/*`: config, logger, server,
  middleware, web, errs, auth, jwt, database, events, jobs, storage): read
  `backend/docs/platform.md` before using or changing them or wiring a module. It is
  the API reference and lists the rules that are easy to get wrong. Update it in the
  same change when a platform API changes.
- Rules that bite most often:
  - Config: each package owns its env names and `Validate()`. Binaries list only the
    blocks they use in `cmd/<bin>/config.go`, without `envPrefix`.
  - Module routes are mounted via oapi-codegen `Middlewares`, **last entry runs first**:
    `{web.ValidateRequests(spec, base), <ForOperations overrides>, authMW}`. Wrap
    operation lists in `web.MustOperations`.
  - The validator does not apply schema defaults; handle defaults in Go.
  - Transactions (`database.WithTx`), outbox `Append` and `EnqueueTx` live in
    repositories; services never see a `pgx.Tx`.
- Code comments and package docs are written in Vietnamese; keep that style.
- Files use LF line endings. When scripting edits with Python on this Windows machine,
  open files with `newline=''`, or CRLF gets written.

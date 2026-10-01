# Platform packages: API reference

Quick reference for `backend/internal/platform/*`: what each package offers, how a
module is wired, and the rules that are easy to get wrong. Package `doc.go` files
(Vietnamese) hold the per-package examples; `go doc ./internal/platform/<pkg>` lists
exact signatures. Keep this file in sync when a platform API changes.

## Package map

| Package | Use it for |
|---|---|
| `config` | `config.Load(&cfg)`: env into the binary's config struct, then `Validate()` on every block |
| `logger` | `slog` setup (JSON/pretty), request-scoped attrs in ctx |
| `server` | `http.Server` with timeouts, the shared middleware chain, graceful shutdown |
| `middleware` | Body limit, client IP, request ID, access log, panic recovery, per-operation middleware |
| `web` | problem+json errors, OpenAPI request validation, JSON helpers for hand-written handlers |
| `errs` | Typed errors carrying HTTP status, type URI and title |
| `auth` | `Actor` in ctx, bearer-token middleware, `Require(perm)` |
| `jwt` | Issue and verify HS256 access tokens with a key ring |
| `database` | `pgxpool` from config, `WithTx` |
| `database/dbtest` | Real Postgres (testcontainers) for integration tests |
| `events` | Transactional outbox, subscriber registry, delivery worker (River) |
| `jobs` | Background jobs on River: `Enqueuer`, client constructors, actor propagation |
| `storage` | File storage (`Store` interface, `Local` implementation) |
| `web/apicommon` | Generated types from `api/common.yaml` (`Problem`, `FieldError`, `ID`, `Paged`) |

## Configuration

Each package owns its config block: full env var names, `envDefault`, `Validate()`.
A binary lists only the blocks it uses in `cmd/<bin>/config.go`, with **no
`envPrefix`**. `config.Load` validates every nested block (children first, then the
parent) and joins all errors with field paths (`config: DB: db: ...`). A
`Validate()` checks only its own fields, never calls children's. Zero means "use
the default" (configs built by hand in tests); negative values are rejected.

| Block | Env vars (default) |
|---|---|
| `logger.Config` | `LOG_LEVEL` (info), `LOG_FORMAT` (json \| pretty) |
| `server.Config` | `HTTP_ADDR` (:8080), `HTTP_READ_HEADER_TIMEOUT` (10s), `HTTP_READ_TIMEOUT` (30s), `HTTP_IDLE_TIMEOUT` (60s), `HTTP_SHUTDOWN_TIMEOUT` (15s), `HTTP_MAX_BODY_BYTES` (1048576), `HTTP_TRUSTED_PROXIES` (empty; CIDRs, comma-separated) |
| `database.Config` | `DB_URL` (empty: pgx reads `PG*`), `DB_MAX_CONNS` (25), `DB_MIN_CONNS` (0), `DB_MAX_CONN_LIFETIME` (1h), `DB_MAX_CONN_LIFETIME_JITTER` (5m), `DB_MAX_CONN_IDLE_TIME` (30m), `DB_HEALTH_CHECK_PERIOD` (1m), `DB_CONNECT_TIMEOUT` (5s). Also `PGPASSWORD_FILE` (Docker secret) |
| `jwt.Config` | `JWT_KEYS_FILE` (required; file of `kid:base64`, comma or newline separated, >= 32-byte secrets), `JWT_ACTIVE_KID` (required), `JWT_ISSUER` (required), `JWT_AUDIENCE` (storeit-api), `JWT_ACCESS_TOKEN_TTL` (15m) |
| `storage.Config` | `STORAGE_DIR` (./tmp/storage) |
| `jobs.Config` | `JOBS_DEFAULT_MAX_WORKERS` (10), `JOBS_EVENTS_MAX_WORKERS` (10). Worker only |

Compose files set `HTTP_ADDR`, `LOG_*`, `JWT_*` and `PG*`. In production behind the
reverse proxy, set `HTTP_TRUSTED_PROXIES` to the Docker network CIDR, otherwise the
access log records the gateway IP.

## HTTP request pipeline

Server level (`server.New`, in order):

```
BodyLimit(HTTP_MAX_BODY_BYTES) -> NoSniff -> ClientIP(trusted) -> RequestID
  -> RequestLogger(log) -> Recoverer() -> chi routing
```

Module level (oapi-codegen `ChiServerOptions.Middlewares`, run after routing). The
**last entry runs first**:

```go
Middlewares: []api.MiddlewareFunc{
    web.ValidateRequests(spec, "/api/v1"), // runs third: validate against the spec
    upload,                                // runs second: per-operation overrides
    authMW,                                // runs first: 401 before 422
}
```

The generated wrapper parses typed path/query params **before** these middlewares;
its errors go to `web.RequestError`.

## Wiring a module (strict server)

`oapi.yaml` must have `strict-server: true`, `chi-server: true`,
`embedded-spec: true` (for `api.GetSwagger()`), and
`import-mapping: {../../../api/common.yaml: storeit/internal/platform/web/apicommon}`.

```go
func (h *Handler) Mount(r chi.Router, tokens *jwt.Provider) error {
    spec, err := api.GetSwagger()
    if err != nil {
        return err
    }
    const base = "/api/v1" // must match `servers` in the spec, else ValidateRequests panics

    authMW := auth.Middleware(tokens,
        auth.Public(web.MustOperations(spec, base, "POST /api/v1/auth/login")...))
    upload := middleware.ForOperations(web.MustOperations(spec, base, "POST /api/v1/imports"),
        middleware.BodyLimit(50<<20), middleware.ReadTimeout(10*time.Minute))

    strict := api.NewStrictHandlerWithOptions(h, nil, api.StrictHTTPServerOptions{
        RequestErrorHandlerFunc:  web.RequestError,
        ResponseErrorHandlerFunc: web.WriteProblem,
    })
    api.HandlerWithOptions(strict, api.ChiServerOptions{
        BaseRouter:       r,
        BaseURL:          base,
        Middlewares:      []api.MiddlewareFunc{web.ValidateRequests(spec, base), upload, authMW},
        ErrorHandlerFunc: web.RequestError,
    })
    return nil
}
```

Operation strings everywhere (`auth.Public`, `ForOperations`, `MustOperations`) are
`"METHOD <chi route pattern>"`: uppercase method, full path including `BaseURL`,
param names exactly as in the spec (`/things/{thingID}`). Wrap lists in
`web.MustOperations` so a typo panics at startup.

## Package details

### web

- `ValidateRequests(spec, baseURL)`: validates params, `Content-Type` and the body
  against the spec. Errors: params 400 `/errors/invalid-request` + `errors[]`; body
  schema 422 `/errors/validation-failed` + `errors[]` (paths like `address.city`);
  malformed JSON 400; wrong or missing `Content-Type` 415 (lists allowed types);
  body over the limit 413; route not in the spec 500 (misconfiguration).
  - Only JSON bodies kin-openapi can decode are read and schema-validated. Other
    types (uploads, custom `+json`) get a `Content-Type` check and a required-body
    check (`ContentLength`); the body stays unread for streaming.
  - Spec `security` is ignored (auth middleware does it); kin-openapi would
    otherwise read every body into memory.
  - Schema **defaults are not applied** (`SkipSettingDefaults`). Apply defaults
    in Go (`page`/`page_size` arrive as nil).
  - `Content-Type` is normalized to lowercase before checks and passed on that way.
  - Path params are unescaped before validation, matching what the handler gets.
  - `format: uuid` accepts any UUID version (the app uses UUIDv7).
- `MustOperations(spec, baseURL, ops...)`: panics if an op is not in the spec.
- `RequestError(w, r, err)`: for oapi `ErrorHandlerFunc` / `RequestErrorHandlerFunc`.
  Generated param errors become 400 with a field (`has an invalid format`,
  `is required`); body read errors map like `Decode`; anything else returns a fixed
  message, never internal text.
- `WriteProblem(w, r, err)`: problem+json; logs 5xx (and anything after the response
  started) with the ctx logger. `RenderProblem` writes without logging. Both write
  nothing once the response has started, drop `Content-Length`,
  `Content-Disposition`, `Content-Encoding`, `ETag`, `Last-Modified`,
  `Cache-Control`, and treat a nil error as 500.
- Hand-written handlers: `web.Handle(func(w, r) error)`, `web.Decode(r, &dst)`
  (JSON + `validate:` tags, size limited by `BodyLimit`), `web.Validate(v)`,
  `web.JSON`, `web.NoContent`, `web.Text`.
- `NotFoundHandler()`, `MethodNotAllowedHandler()` (sets `Allow`), installed by `server.New`.
- Error vars: `ErrMalformedJSON` 400, `ErrInvalidRequest` 400, `ErrValidation` 422,
  `ErrRequestTooLarge` 413, `ErrUnsupportedMediaType` 415, `ErrRouteNotFound` 404,
  `ErrMethodNotAllowed` 405.

### errs

- Declare once per domain: `var ErrAssetNotFound = errs.NotFound("/errors/asset-not-found", "Asset not found")`.
  Helpers: `Invalid` 400, `Unauthorized` 401, `Forbidden` 403, `NotFound` 404,
  `Conflict` 409, `Unprocessable` 422, `Internal` 500, `New(status, ...)`.
- Per occurrence: `ErrX.With(errs.WithDetailf(...), errs.WithCause(err), errs.WithFields(...))`
  returns a copy. `errors.Is` matches by type URI, so copies and `%w` wraps match.
- Non-`*errs.Error` errors become `errs.ErrInternal` (500, generic text). The cause
  is logged, never serialized.

### auth

- `Middleware(tokens, auth.Public(ops...))`: verifies the bearer token, puts
  `Actor{AccountID, Permissions}` in ctx, adds `actor_id` to the log scope. Public
  ops are matched by chi route pattern, so it must run after routing (oapi
  `Middlewares`); mounted with `r.Use` it fails closed (everything requires a token).
- In services: `actor, err := auth.Require(ctx, domain.PermX)` gives 401 without an
  actor, 403 without the permission. `auth.SystemActor` has every permission and no
  account (scheduled jobs). `auth.ActorIDFrom(ctx)` is `uuid.Nil` for the system or
  no actor.
- Errors: `ErrUnauthenticated`, `ErrForbidden`, `ErrMissingToken`, `ErrInvalidToken`.

### jwt

- `p, _ := jwt.New(cfg)`, `tok, _ := p.Issue(accountID, perms)`, `claims, err := p.Verify(raw)`,
  `claims.UserID()`, `claims.TokenID()`, `claims.Permissions`.
- Verify accepts only HS256, `typ: at+jwt`, matching iss/aud, with `exp`, 30s leeway.
  Errors (`errors.Is`): `ErrMalformed`, `ErrSignature` (also wrong algorithm),
  `ErrExpired`, `ErrNotYetValid`, `ErrClaims`, `ErrUnknownKID`, `ErrBadType`.
- Rotation: add the new key to the file, switch `JWT_ACTIVE_KID`; old tokens verify
  while their kid is still in the file. Permissions live in the token, so keep the
  TTL short.

### database

- `pool, err := database.Open(ctx, cfg.DB)` validates, applies pool settings
  (overriding `pool_*` in the URL), reads `PGPASSWORD_FILE`, then pings.
  `database.Close(pool)`.
- `database.WithTx(ctx, pool, func(tx pgx.Tx) error)`: commit on nil; roll back on
  error, panic or `runtime.Goexit`. The fn's error is returned intact (domain errors
  keep their status). Rollback uses a non-cancelled ctx. Only repositories use it;
  services never see transactions. `Beginner` also accepts `pgx.Tx` (savepoint).

### events

- Emit (in a repository, inside `WithTx`):
  `e, _ := events.New(contract.EventX, "asset", id, payload)` then
  `outbox.Append(ctx, tx, e)`. Append fills `ActorID` from ctx and inserts the event
  plus one `HandleEventArgs` job per subscriber, in the same tx.
- Subscribe: `r.On("module.reaction", handler, eventTypes...)` (one or more types),
  `r.OnAll(name, handler)`. Each name is registered once; it is the job key, so
  don't rename it while jobs are pending. Handlers must be idempotent.
- Read the payload: `p, err := events.Decode[contract.X](e)`.
- Worker: `events.RegisterWorker(workers, pool, registry, loadActor)`. It restores
  the original actor (`jobs.RestoreActor`). An unknown subscriber retries (API may
  deploy before the worker); a missing event or deleted actor cancels the job.
- API and worker must build the same registry. Construct the outbox with
  `events.NewOutbox(registry, insertClient)`.

### jobs

- Clients: `jobs.NewInsertClient(pool, log)` (API, insert only) and
  `jobs.NewWorkerClient(pool, log, cfg.Jobs, workers, periodic)` (worker; queues
  `QueueDefault` and `QueueEvents`). A new queue must be added in `workerConfig`, or
  its jobs wait forever.
- `enq := jobs.NewRiver(client)` implements `Enqueuer`:
  `Enqueue(ctx, job, opts...)` or `EnqueueTx(ctx, tx, job, opts...)` (inside
  `WithTx`; the job exists only if the tx commits). Options: `WithQueue`,
  `WithPriority` (1-4), `WithSchedule`, `WithMaxAttempts`, `WithUniqueArgs` (dedupe
  against unfinished jobs with the same args).
- Args embed `jobs.ActorArgs` (`ActorArgsFrom(ctx)`); a worker calls
  `ctx, err := jobs.RestoreActor(ctx, args.ActorID, loadActor)`. Permissions are
  reloaded at run time. A loader returning an error wrapping `jobs.ErrActorNotFound`
  cancels the job; other errors retry.
- `DefaultMaxAttempts = 10` (about 7h of retries). `Kind()` must never change.

### logger

- `slog.SetDefault(logger.New(os.Stdout, cfg.Log))` once in main; app code uses `slog.*Context(ctx, ...)`.
- `ctx = logger.With(ctx, attrs...)`: attrs on every log written with that ctx.
- `logger.WithScope(ctx)` plus `logger.AddToScope(ctx, attrs...)`: attrs added deep
  inside (`actor_id`) also show up in logs written with outer ctxs (access log).
- `logger.NewContext` / `logger.FromContext`: the request's logger (fallback
  `slog.Default()`). Platform code logs through `FromContext`.
- Ctx attrs always stay top level, even inside `log.WithGroup(...)`.

### middleware

`BodyLimit(n)` (n <= 0 means no limit; a later `BodyLimit` replaces the limit, it
doesn't stack), `ReadTimeout(d)` (d <= 0 removes the deadline),
`ForOperations(ops, mws...)`, `ClientIP(trusted)` / `ClientIPFrom(ctx)`
(rightmost-untrusted `X-Forwarded-For`, only from trusted proxies), `RequestID`
(accepts `X-Request-Id` matching `[A-Za-z0-9._-]{1,64}`, else a UUID; echoed in the
response), `RequestLogger(log)` (one line: method, path, client_ip, route, status,
bytes, dur, aborted), `Recoverer()`, `NoSniff`.

### storage

`store, _ := storage.NewLocal(cfg.Storage)`; `Put(ctx, key, r)` (atomic temp + rename,
fsync, stops when ctx is cancelled), `Get` (caller closes; `ErrNotFound`), `Delete`
(idempotent). Keys are code-generated relative paths (`imports/<id>/source.xlsx`);
invalid keys give `ErrInvalidKey`. All access goes through `os.Root`.

## Testing

- `go test ./...` (or `make test`). Database-backed tests use `dbtest.Pool(t)`: one
  Postgres container per test binary, goose and River migrations applied, shared
  between tests (use fresh IDs and table names, don't assume an empty DB).
- Without Docker these tests **skip** locally. With several packages starting
  containers at once the health check can skip them, or testcontainers can fail to
  create its Docker provider on Windows. Run `CI=true go test -p 1 ./...` (skips fail,
  one package at a time) and keep Docker Desktop running.
- `internal/platform/events/db` is sqlc output; fake its `DBTX` interface to unit-test
  worker logic without Postgres (see `events/worker_test.go`).

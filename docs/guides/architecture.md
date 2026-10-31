---
title: Architecture
description: Services, backend layers and the request pipeline.
---

## The stack

The repository root owns the Docker stack (`compose.yml` for development, `compose.prod.yml` for one server).

| Service | Role |
|---|---|
| `postgres` | The only datastore (Postgres 18) |
| `migrate` | Applies the goose migrations in `backend/migrations/` before the app starts |
| `app` | The HTTP API (`cmd/server`), Swagger UI at `/api/docs` in development |
| `worker` | Background jobs (`cmd/worker`): email, session pruning, event delivery |
| `web` | The Vue SPA. Vite with hot reload in development; nginx in production, serving the SPA and proxying `/api` to `app` |
| `mailpit` | Development mail inbox on port 8025 |
| `pgadmin`, `seed` | Optional, behind the `tools` profile |

In production only `web` is published, on `127.0.0.1:${WEB_PORT}`. A host reverse proxy provides HTTPS.

## Backend: a modular monolith

One Go program split into modules under `backend/internal/`. The current modules are **identity**
(accounts, sign-in, roles) and **inventory** (asset types, assets, export). Shared infrastructure lives in
`internal/platform/*`.

Libraries: chi (router), oapi-codegen (strict server generated from each module's `openapi.yaml`), pgx,
River (jobs), sqlc (typed queries) and goose (migrations).

Inside a module the layers only depend downwards:

| Package | Holds |
|---|---|
| `domain` | Entities, validation rules, errors, permission codes, repository interfaces |
| `service` | Use cases. Every call starts with `auth.Require` |
| `repository` | sqlc queries, transactions, outbox events |
| `handler` | The OpenAPI spec and generated strict server, cookie handling, `Mount` |
| `contract` | What other modules may use: DTOs, reader interfaces, event names |

Modules never reach into each other's tables. Other modules read accounts only through `identity/contract`.
A linter (depguard) enforces the import rules, including in test files.

## Platform packages

`config`, `logger`, `server`, `middleware`, `web`, `errs`, `auth`, `jwt`, `database`, `events`, `jobs`,
`storage` and `mail`. Highlights:

- **Config**: each package owns its environment names and a `Validate()`; each binary lists only the blocks it uses.
- **Errors**: typed errors carry the HTTP status and a problem+json type URI such as `/errors/invalid-refresh-token`.
- **Events**: modules append domain events to a transactional outbox in the same transaction as the change;
  a worker delivers them to subscribers.
- **Jobs**: River jobs run as the account that queued them (`jobs.RestoreActor`).

## Request pipeline

1. The server middleware chain: body limit, client IP, request ID, access log, panic recovery.
2. Module routes are mounted through oapi-codegen `Middlewares`. The last entry runs first, so the order is
   request validation against the OpenAPI spec, then any per-operation overrides, then the bearer-token middleware.
3. The handler calls a service, which checks the permission with `auth.Require` and runs repository calls.
   Transactions (`database.WithTx`), outbox appends and job enqueues live in repositories.
4. Errors become problem+json responses.

The request validator does not apply schema defaults, so defaults are handled in Go.

## Frontend

Vue 3 with TypeScript, PrimeVue and Vue Query. API types are generated from the backend OpenAPI specs
(`npm run gen:api`) and committed. The interface text is English.

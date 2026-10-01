# Identity module: reference

`backend/internal/identity`: sign-in, sessions with rotating refresh tokens, and
role-based access control. Design: `docs/superpowers/specs/2026-10-01-identity-module-design.md`.
Platform APIs it builds on: `docs/platform.md`.

## Layout

| Package | Holds |
|---|---|
| `identity` (root) | `Config`, `New(Deps)`, `Module` (`Mount`, `Bootstrap`, `LoadActor`, `AccountReader`, `RegisterWorkers`), `PeriodicJobs()` |
| `domain` | `Account`, `Role`, `RefreshState.Decide`, `ValidatePassword`, `NormalizeEmail`, errors, permission constants, fixed role IDs, repository interfaces |
| `service` | Use cases; every management call starts with `auth.Require` |
| `repository` | sqlc queries (`repository/queries/*.sql` → `repository/db`), transactions, outbox events |
| `handler` | `openapi.yaml` → `handler/api` (strict server), cookie handling, `Mount` |
| `contract` | `AccountReader`, `Account` DTO, `ErrAccountNotFound`, event names and payloads |
| `job`, `worker` | `PruneSessionsArgs` and its worker |

Tests that need Postgres or wire several layers live at the module root
(`*_db_test.go`, package `identity_test`): depguard also lints test files, and
`handler/` may not import the repository or `dbtest`.

## Wiring

```go
m, err := identity.New(identity.Deps{Pool: pool, Tokens: tokens, Outbox: outbox, Config: cfg.Identity})
m.Bootstrap(ctx)                                   // cmd/server: first Administrator if no accounts
m.Mount(srv.Router())                              // cmd/server: /api/v1 routes
events.RegisterWorker(workers, pool, registry, m.LoadActor) // cmd/worker
m.RegisterWorkers(workers)                         // cmd/worker, with identity.PeriodicJobs()
```

`Deps.Tokens` is optional (the worker passes nil); `Mount` needs it.

## Configuration

| Env | Default | |
|---|---|---|
| `IDENTITY_REFRESH_SLIDING_TTL` | 336h | Lifetime of each refresh token |
| `IDENTITY_REFRESH_ABSOLUTE_TTL` | 720h | Hard limit of one sign-in; must be >= sliding |
| `IDENTITY_REFRESH_GRACE_PERIOD` | 30s | A just-used token presented again within this window is a retry, not theft |
| `IDENTITY_REFRESH_RETENTION` | 720h | Dead sessions kept this long before the prune job deletes them |
| `IDENTITY_COOKIE_SECURE` | true | Dev compose and `.env.local` set false (http) |
| `IDENTITY_COOKIE_DOMAIN` | empty | |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | empty | Both or neither. Creates an Administrator only while the accounts table is empty |

## Sessions and token rotation

- Access token: the platform JWT (15 min) carrying the account's permissions.
- Refresh token: 32 random bytes in the `storeit_refresh` cookie (`HttpOnly`,
  `SameSite=Strict`, `Path=/api/v1/auth`). Only its SHA-256 is stored.
- A sign-in opens a **family**; each refresh marks the presented token used and issues
  its child. `RefreshState.Decide` (domain) decides, the repository applies it under
  `SELECT … FOR UPDATE OF t, f` in one transaction:

| Situation | Result |
|---|---|
| Family revoked, token unknown or past its own expiry | 401, nothing written |
| Account disabled | revoke family (`admin`), 401 |
| Past the family's absolute expiry | revoke (`expired`), 401 |
| Token already used, within grace | rotate the family's current tip (two tabs, retries) |
| Token already used, after grace | **reuse**: revoke the whole family (`reuse_detected`), WARN log, 401 |
| Otherwise | rotate; new token expires at min(now + sliding, absolute) |

- Every refresh failure is the same 401 (`/errors/invalid-refresh-token`) and clears the cookie.
- Refresh reloads permissions: role and permission changes reach users within one access-token TTL.
- Disabling an account and an admin password reset revoke all its sessions. Changing
  your own password (`PUT /auth/password`) keeps the current session, revokes the rest.
- `identity.prune_sessions` (hourly, worker) deletes dead families and old used tokens.

## API (`/api/v1`)

| Endpoint | Permission |
|---|---|
| `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout` | public |
| `PUT /auth/password`, `GET /me` | signed in |
| `GET /accounts`, `GET /accounts/{accountID}` | `identity.account.read` |
| `POST /accounts`, `PATCH /accounts/{accountID}`, `POST …/disable`, `POST …/enable`, `PUT …/password`, `PUT …/roles` | `identity.account.manage` |
| `GET /roles`, `GET /roles/{roleID}`, `GET /permissions` | `identity.role.read` |
| `POST /roles`, `PATCH /roles/{roleID}`, `PUT /roles/{roleID}/permissions`, `DELETE /roles/{roleID}` | `identity.role.manage` |

`PATCH /accounts/{id}` takes `version` (409 when stale) and `clear_member_id` to unlink a member.
`GET /accounts` defaults to page 1, size 50 (applied in the handler).

## RBAC

| | Administrator | Authorized Manager | Inventory Officer | Employee |
|---|---|---|---|---|
| `identity.account.read` | ✓ | ✓ | | |
| `identity.account.manage` | ✓ | | | |
| `identity.role.read` | ✓ | ✓ | | |
| `identity.role.manage` | ✓ | | | |

- Seeded in `migrations/00002_identity.sql` with fixed IDs (`domain.AdministratorRoleID`, …).
  New modules add their permission codes to `identity.permissions` and their grants in their own migration.
- System roles cannot be renamed or deleted (409); their permissions can change.
- Lock-out guards (409 `/errors/lockout`): you cannot disable yourself, drop your own
  Administrator role, or remove `identity.role.manage` from Administrator.
- A role still assigned to an account cannot be deleted (409).

## Events (`contract/events.go`)

`identity.account_created`, `account_updated` (field changes), `account_disabled`,
`account_enabled`, `password_reset`, `roles_assigned` (from/to), `role_created`,
`role_updated`, `role_permissions_updated` (from/to), `role_deleted`. No events for
sign-in, refresh, logout, or changing your own password. Payloads never carry hashes or tokens.

## For other modules

- Check permissions in your service with `auth.Require(ctx, "<module>.<thing>.<action>")`.
- Show "changed by": `contract.AccountReader` (`GetAccount`, `GetAccounts`).
- Jobs run as the requesting account: `jobs.RestoreActor(ctx, id, module.LoadActor)`.
  A deleted account cancels the job; a disabled one runs with no permissions.

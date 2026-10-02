# Identity module: design

Date: 2026-10-01 · Status: approved in conversation, awaiting spec review

## 1. Goal

Implement `backend/internal/identity`: sign-in, sign-out and protected access (US-14), and
role-based access control with role management (US-15), using refresh-token rotation ported
from mimir-2.0. Finish the binaries (`cmd/migrate`, `cmd/server`, `cmd/worker`) so the
module runs end to end.

Sources: `backend/docs/tasks/StoreIT-Main.md` (FR-09, roles), `StoreIT-acceptance-criteria.md`
(US14-AC1..7, US15-AC1..7), the Phase 1 Task List (Milestone 1), and
`mimir-2.0/apps/backend/internal/identity` (rotation design).

### Decisions taken

| Topic | Decision |
|---|---|
| Scope | Identity module plus runnable `cmd/migrate`, `cmd/server`, `cmd/worker`. No frontend. |
| Refresh transport | HttpOnly cookie, `SameSite=Strict`, `Path=/api/v1/auth` |
| System roles | Administrator, Inventory Officer, Employee, Authorized Manager (StoreIT-Main) |
| Revocation | Disabling an account or an admin password reset revokes all its sessions. Role changes apply at the next refresh (at most one access-token TTL). |
| Endpoints | All in `openapi.yaml`, including refresh and logout (cookie as `in: cookie` parameter, `Set-Cookie` as response header) |
| Session model | mimir's refresh-token families with single-use tokens, grace window and reuse detection |
| Accounts | Created by an administrator. No self-registration, no email verification. |
| Passwords | bcrypt; 12 to 72 bytes |

## 2. Data model

New schema `identity`, one goose migration (`migrations/00002_identity.sql`) including seed data.
Email uniqueness is case-insensitive through a unique index on `lower(email)`; emails are stored
lowercased.

| Table | Columns |
|---|---|
| `accounts` | `id uuid PK`, `email text`, `name text`, `password_hash text`, `member_id uuid NULL` (no FK until directory exists), `active bool`, `version int`, `created_at`, `updated_at` |
| `permissions` | `code text PK`, `description text`. Each module's migration inserts its own codes. |
| `roles` | `id uuid PK`, `name text UNIQUE`, `description text`, `is_system bool`, `created_at`, `updated_at` |
| `role_permissions` | `role_id → roles ON DELETE CASCADE`, `permission → permissions(code)`, PK both |
| `account_roles` | `account_id → accounts ON DELETE CASCADE`, `role_id → roles`, PK both |
| `refresh_families` | `id uuid PK`, `account_id → accounts ON DELETE CASCADE`, `user_agent text`, `ip text`, `created_at`, `absolute_expires_at`, `revoked_at NULL`, `revoked_reason NULL` (`logout`, `reuse_detected`, `admin`, `expired`) |
| `refresh_tokens` | `id uuid PK`, `family_id → refresh_families ON DELETE CASCADE`, `token_hash bytea UNIQUE`, `parent_id → refresh_tokens ON DELETE SET NULL`, `issued_at`, `expires_at`, `used_at NULL` |

Indexes: unique partial `refresh_tokens(family_id) WHERE used_at IS NULL` (one live token per
family), `refresh_families(account_id) WHERE revoked_at IS NULL`.

### Seed

Permissions: `identity.account.read`, `identity.account.manage`, `identity.role.read`,
`identity.role.manage`. All four roles are system roles (`is_system = true`).

| | Administrator | Authorized Manager | Inventory Officer | Employee |
|---|---|---|---|---|
| `identity.account.read` | ✓ | ✓ | | |
| `identity.account.manage` | ✓ | | | |
| `identity.role.read` | ✓ | ✓ | | |
| `identity.role.manage` | ✓ | | | |

Later milestones add their permission codes and grants in their own migrations.

## 3. Sessions and token rotation

`identity.Config` (package-owned env names):

| Env | Default | Meaning |
|---|---|---|
| `IDENTITY_REFRESH_SLIDING_TTL` | 336h | Lifetime of each refresh token |
| `IDENTITY_REFRESH_ABSOLUTE_TTL` | 720h | Hard limit of a family (one sign-in) |
| `IDENTITY_REFRESH_GRACE_PERIOD` | 30s | Window in which a just-used token is treated as a retry |
| `IDENTITY_REFRESH_RETENTION` | 720h | How long dead rows are kept before pruning |
| `IDENTITY_COOKIE_SECURE` | true | `Secure` flag; dev compose sets false (http) |
| `IDENTITY_COOKIE_DOMAIN` | empty | Cookie domain |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | empty | Bootstrap admin; skipped when unset |

`Validate`: all durations > 0, sliding <= absolute; admin email and password both set or both empty.

Refresh secrets are 32 random bytes, base64url; the database stores only their SHA-256 hash.

- **Login**: lowercase the email; load the account (unknown email still runs a bcrypt compare
  against a fixed dummy hash, so timing doesn't reveal which emails exist); wrong password or
  unknown email → `ErrBadCredentials` (401); disabled → `ErrAccountDisabled` (403). Create a
  family (absolute expiry = now + absolute TTL, user agent and client IP), a root token
  (expiry = now + sliding TTL), and an access token carrying the account's current permissions.
- **Refresh**: one transaction. `SELECT … FROM refresh_tokens t JOIN refresh_families f … WHERE
  token_hash = $1 FOR UPDATE OF t, f`. Not found → reject. The account must still be active
  (else revoke the family with `admin`, 401). `RefreshState.Decide(now, grace)`:
  - family revoked → **reject** (401)
  - family past absolute expiry → **expired**: revoke (`expired`), commit, 401
  - token already used: within grace → **grace**: rotate the family's tip; else → **reuse**:
    revoke (`reuse_detected`), log WARN with family and account IDs (never the token), commit, 401
  - token past its own expiry → **reject**
  - otherwise → **rotate**: mark it used (guard `used_at IS NULL`, 0 rows → reject), create the
    child token with expiry `min(now + sliding, family absolute expiry)`, issue a new access
    token with **freshly loaded permissions**.
- **Logout**: revoke the cookie's family (`logout`). Idempotent: missing, unknown or dead token
  still returns 204.
- **Disable account / admin password reset**: revoke all the account's live families (`admin`)
  in the same transaction.
- **Change own password** (`PUT /me/password`): requires the current password; revokes the
  account's other families, keeps the current one. The current family is identified from the
  refresh cookie when present; without it, all families are revoked.
- **Prune** (River periodic job `identity.prune_sessions`, hourly, in `cmd/worker`): delete
  families where `GREATEST(revoked_at, absolute_expires_at) < now - retention`, and used tokens
  with `expires_at < now - retention`.

All 401s from refresh are the same problem (`/errors/invalid-refresh-token`) and clear the
cookie; a missing cookie and a bad one are indistinguishable.

## 4. HTTP API

Server `/api/v1`, global `bearerAuth`. Public operations: login, refresh, logout.

| Endpoint | Permission | Body / result |
|---|---|---|
| `POST /auth/login` | public | `{email, password}` → `{access_token, expires_at, account}` + `Set-Cookie` |
| `POST /auth/refresh` | public (cookie) | → same as login + new cookie; 401 clears cookie |
| `POST /auth/logout` | public (cookie) | → 204, clears cookie |
| `GET /me` | signed in | account + roles + effective permissions |
| `PUT /me/password` | signed in | `{current_password, new_password}` → 204 |
| `GET /accounts` | `identity.account.read` | `page`, `page_size`, `q` (name/email), `active` → `{items, total}` |
| `POST /accounts` | `identity.account.manage` | `{email, name, password, role_ids, member_id?}` → 201 account |
| `GET /accounts/{accountID}` | `identity.account.read` | account with roles |
| `PATCH /accounts/{accountID}` | `identity.account.manage` | `{name?, member_id?, version}` → account; stale version → 409 |
| `POST /accounts/{accountID}/disable` | `identity.account.manage` | → account; cannot disable self |
| `POST /accounts/{accountID}/enable` | `identity.account.manage` | → account |
| `PUT /accounts/{accountID}/password` | `identity.account.manage` | `{password}` → 204; revokes sessions |
| `PUT /accounts/{accountID}/roles` | `identity.account.manage` | `{role_ids}` → account; replaces roles |
| `GET /roles` | `identity.role.read` | roles with permissions |
| `POST /roles` | `identity.role.manage` | `{name, description, permissions}` → 201 role |
| `GET /roles/{roleID}` | `identity.role.read` | role with permissions |
| `PATCH /roles/{roleID}` | `identity.role.manage` | `{name?, description?}` → role |
| `PUT /roles/{roleID}/permissions` | `identity.role.manage` | `{permissions}` → role |
| `DELETE /roles/{roleID}` | `identity.role.manage` | → 204; system or assigned role → 409 |
| `GET /permissions` | `identity.role.read` | `[{code, description}]` |

Errors use problem+json. Unknown role IDs or permission codes → 422 with fields. Duplicate email
or role name → 409.

### Lock-out guards

- System roles cannot be renamed or deleted (409); their description and permissions can change.
- An account cannot disable itself.
- An account cannot remove the Administrator role from itself.
- `identity.role.manage` cannot be removed from the Administrator role.

## 5. Events

Appended in the same transaction as the change (`identity/contract/events.go`):
`identity.account_created`, `identity.account_updated` (changed fields), `identity.account_disabled`,
`identity.account_enabled`, `identity.password_reset`, `identity.roles_assigned` (role IDs),
`identity.role_created`, `identity.role_updated`, `identity.role_permissions_updated`,
`identity.role_deleted`. Payloads never contain password hashes or tokens. Sign-in, refresh and
logout produce no events.

## 6. Module structure

Follows the existing skeleton and depguard layer rules.

| Package | Contents |
|---|---|
| `identity` (root) | `Config`, `New(Deps) (*Module, error)` returning `Handler`, `Accounts` (contract), `LoadActor`, `Bootstrap`, `RegisterWorkers`, `PeriodicJobs` |
| `domain` | `Account` (`CanLogin`, `Disable`, `Enable`, `Rename`), `Role` (system guards), `RefreshState.Decide`, `ValidatePassword`, errors, permission constants, repository interfaces |
| `service` | use cases; each starts with `auth.Require`; `Hasher` (bcrypt), secret minting, `jwt.Provider` for access tokens, injectable clock |
| `repository` | sqlc queries in `repository/queries`, generated `repository/db`; writes in `database.WithTx` with `Outbox.Append` |
| `handler` | strict-server implementation, cookie build/clear, `Mount` |
| `contract` | `AccountReader` (`GetAccount`, `GetAccounts`), `Account` DTO, `ErrAccountNotFound`, event types |
| `job`, `worker` | `PruneSessionsArgs` and its worker |

`Mount` wiring, in documented order:

```go
public := web.MustOperations(spec, "/api/v1",
    "POST /api/v1/auth/login", "POST /api/v1/auth/refresh", "POST /api/v1/auth/logout")
Middlewares: []api.MiddlewareFunc{web.ValidateRequests(spec, "/api/v1"),
    auth.Middleware(tokens, auth.Public(public...))}
```

## 7. Binaries

- `cmd/migrate`: goose up, River migrate, exit. Config: `DB`.
- `cmd/server`: config (`Log`, `HTTP`, `DB`, `JWT`, `Identity`); logger, pool, JWT provider,
  events registry and outbox, River insert client, `identity.New`, `Bootstrap`, mount under
  `/api/v1`, `GET /healthz`, run until SIGINT/SIGTERM.
- `cmd/worker`: config (`Log`, `DB`, `Jobs`, `Identity`); pool, registry,
  `events.RegisterWorker(…, identity.LoadActor)`, identity workers and periodic prune job,
  River worker client, run until signal.
- Compose and env examples gain the new variables; `sqlc.yml` enables the identity block.

## 8. Testing

- Domain: table tests for every `Decide` branch, password policy, `CanLogin`, system-role guards.
- Service (fake repositories, fake clock, real bcrypt at minimum cost): login success, unknown
  email, wrong password, disabled; each refresh branch; logout idempotence; permission denied for
  each use case; lock-out guards; revocation on disable and password reset; permissions reloaded
  on refresh.
- Repository (`dbtest`, real Postgres): concurrent refreshes of one token yield one rotation and
  one grace result, never two live tokens; reuse revokes the family; prune; unique email → 409;
  event and job written in the same transaction; role assignment and permission listing.
- Handler (`httptest` over the generated router): login sets the cookie; missing token 401;
  missing permission 403; refresh rotates the cookie; logout clears it; validation errors.
- End to end: bootstrap admin → login → `/me` → create account with Employee role → that user
  logs in and gets 403 on `GET /accounts` → refresh → logout → refresh fails.

## 9. Out of scope

Frontend (sign-in page, route guards, generated TypeScript client); rate limiting and lockout after
repeated failed sign-ins (follow-up); activity history consuming the events (Milestone 2); member
foreign key (Milestone 3).

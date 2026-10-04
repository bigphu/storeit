# Identity module: reference

`backend/internal/identity`: sign-in, sessions with rotating refresh tokens,
role-based access control, and emailed links for invitations and password resets.
Design: `docs/superpowers/specs/2026-10-01-identity-module-design.md` and
`docs/superpowers/specs/2026-10-01-account-invitations-design.md`.
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
| `job`, `worker` | `PruneSessionsArgs`, `SendAccountEmailArgs` and their workers |

Tests that need Postgres or wire several layers live at the module root
(`*_db_test.go`, package `identity_test`): depguard also lints test files, and
`handler/` may not import the repository or `dbtest`.

## Wiring

```go
enq := jobs.NewRiver(insertClient)
m, err := identity.New(identity.Deps{Pool: pool, Tokens: tokens, Outbox: outbox, Jobs: enq, Config: cfg.Identity})
m.Bootstrap(ctx)                                   // cmd/server: first Administrator if no accounts
m.Mount(srv.Router())                              // cmd/server: /api/v1 routes
// cmd/worker: Deps{..., Jobs: enq, Mail: sender} with sender from mail.New(cfg.Mail, log)
events.RegisterWorker(workers, pool, registry, m.LoadActor)
err = m.RegisterWorkers(workers)                   // with identity.PeriodicJobs()
doc, err := m.APIDoc()                             // cmd/server, HTTP_API_DOCS: web.MountDocs(r, doc)
```

`Deps.Jobs` is required. `Deps.Tokens` is needed only by `Mount` (the worker passes
nil). `Deps.Mail` is needed only by `RegisterWorkers`: the API never sends mail, it
queues `identity.send_account_email` in the same transaction.

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
| `IDENTITY_APP_URL` | http://localhost:3000 | Frontend base for emailed links; http(s), no query or fragment |
| `IDENTITY_INVITE_TTL` | 72h | Invitation link lifetime |
| `IDENTITY_RESET_TTL` | 1h | Password reset link lifetime |

The worker also needs `MAIL_*` (`platform/mail`, see `docs/platform.md`). Dev compose
sends to Mailpit (web inbox on http://localhost:8025) unless `.env` sets the Resend block
from `.env.example`. Production sends through Resend's SMTP (`smtp.resend.com:465`, TLS,
username `resend`) with the API key in `deploy/app/secrets/smtp_password.txt`
(`make init` creates it empty; the worker refuses to start while it is empty).

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
- A disabled account's access token stays valid until it expires (≤ 15 min). Refresh fails at
  once, and `PUT /auth/password` refuses a disabled account.
- Login and refresh responses carry `Cache-Control: no-store`.
- Frontend: run at most one refresh at a time, shared across tabs (a lock or
  `BroadcastChannel`). Two tabs refreshing the same cookie both succeed thanks to the
  grace window, but if the responses reach the browser out of order it keeps the older,
  already-used token, and the next refresh more than 30 s later is treated as reuse and
  ends the session.
- Disabling an account (reason `admin`) and completing a password reset (`password_reset`)
  revoke all its sessions. Changing your own password (`PUT /auth/password`) keeps the
  current session and revokes the rest (`password_change`).
- `identity.prune_sessions` (hourly, worker) deletes dead families and old used tokens.

## Invitations and password reset

- Accounts are created without a password: `status` is `invited` (`active` true,
  `password_hash` NULL). Signing in to one looks exactly like an unknown email.
- Links are one-time tokens in `identity.password_tokens`: 32 random bytes, SHA-256
  stored, one row per `(account, purpose)`, `purpose` `invite` (72h) or `reset` (1h).
  Issuing again overwrites the row, so the previous link dies.
- The request that issues a token also queues `identity.send_account_email` with the
  raw token in its args (the only way it reaches the email; River deletes finished jobs
  after 24h). The worker skips links that are used, superseded, expired or whose
  account is disabled, sends with idempotency key `identity/<purpose>/<token id>`, and
  cancels the job on a permanent mail error.
- Links: `{IDENTITY_APP_URL}/accept-invite#token=…` and `/reset-password#token=…`. The
  token sits in the fragment so it never reaches a server log or `Referer`. The page
  posts it to `POST /auth/password/set`.
- Using a link (`setPassword`) locks the account, deletes the row, then checks expiry and
  account state, sets the password and deletes the account's other links, in one
  transaction. Every token write locks the account row before the token row (the same
  order as disabling), so a simultaneous disable and link use cannot deadlock. Issuing a
  link for a disabled account fails with `ErrAccountInactive`.
  A reset also revokes every session. Concurrent submissions: exactly one wins.
- `POST /auth/password/forgot` always answers 202 and only queues
  `identity.forgot_password{email}` (unique per email per minute), so the response time is
  the same whether or not the email has an account. The worker looks the account up:
  unknown or disabled is silent, an invited account gets a fresh invite. The 60-second
  per-account cooldown is part of the token upsert (`ON CONFLICT … WHERE created_at <
  now() - min_age`), so simultaneous requests cannot both send. Admin actions pass no
  cooldown.
- Per-IP rate limits (`middleware.RateLimit`, in memory, 429 `/errors/rate-limited` with
  `Retry-After`): login 10 then 1 per 6 s, forgot 5 then 1 per minute, set password 10
  then 1 per 6 s. Refresh and logout are not limited. With several API replicas each
  counts separately.
- Admin "send reset link" on an invited account sends a fresh invite. Disabling an
  account deletes its links; enabling does not restore them.

## API (`/api/v1`)

| Endpoint | Permission |
|---|---|
| `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout`, `POST /auth/password/forgot` (202), `POST /auth/password/set` (204) | public |
| `PUT /auth/password`, `GET /me`, `PATCH /me` | signed in |
| `GET /accounts`, `GET /accounts/{accountID}` | `identity.account.read` |
| `POST /accounts`, `PATCH /accounts/{accountID}`, `POST …/disable`, `POST …/enable`, `POST …/sign-out`, `POST …/invitation` (202), `POST …/password-reset` (202), `PUT …/roles` | `identity.account.manage` |
| `GET /roles`, `GET /roles/{roleID}`, `GET /permissions` | `identity.role.read` |
| `POST /roles`, `PATCH /roles/{roleID}`, `PUT /roles/{roleID}/permissions`, `DELETE /roles/{roleID}` | `identity.role.manage` |

`PATCH /accounts/{id}` takes `version` (409 when stale) and `clear_member_id` to unlink a member.
`PATCH /me` lets any signed-in account change its own display name (`name`, `version`; 409 when
stale, 403 `/errors/account-disabled` once disabled); email, roles and member stay admin-only.
`POST /accounts` takes no password. Errors for links: 422 `/errors/invalid-password-token`
(field `token`) for every unusable link, 409 `/errors/not-invited` when resending to an
account that has a password, 409 `/errors/account-inactive` for a disabled account.
`GET /accounts` defaults to page 1, size 50 (applied in the handler); `page` ≤ 100000.
`q` matches name or email as a literal substring (`%`, `_` and `\` are not wildcards).
`status` (`invited`/`active`/`disabled`) and `role_id` filter the list. Each item carries its
roles and, while invited, `invite_expires_at`; `status_counts` counts accounts per status for
the same `q` and `role_id` (ignoring `active` and `status`), for the filter buttons.
`GET /accounts/{id}` adds `active_sessions` (not revoked, not past the absolute limit, tip
unexpired) and `invite_expires_at`. Every account has `last_sign_in_at`, written at login into
`identity.account_sign_ins` (not a column of `accounts`, so login never waits on the account
row lock admin operations hold; refresh doesn't update it).
`POST /accounts/{id}/sign-out` revokes every live session (reason `admin`) and returns how many;
like disable, you must hold all of that account's permissions (403 otherwise).
`PATCH /accounts/{id}` with both `member_id` and `clear_member_id` is 422
`/errors/member-conflict`. Names (accounts, roles) may not contain control characters.

## RBAC

| | Administrator | Authorized Manager | Inventory Officer | Employee |
|---|---|---|---|---|
| `identity.account.read` | ✓ | ✓ | | |
| `identity.account.manage` | ✓ | | | |
| `identity.role.read` | ✓ | ✓ | | |
| `identity.role.manage` | ✓ | | | |

- Seeded in `migrations/00002_identity.sql` with fixed IDs (`domain.AdministratorRoleID`, …).
  New modules add their permission codes to `identity.permissions` and their grants in their
  own migration, **including a grant to Administrator**: nobody can hand out a permission
  they don't hold (next point), so a code Administrator lacks can never be assigned.
- No granting beyond your own permissions (403 `/errors/exceeds-own-permissions`): roles
  added to or removed from an account, permissions added to or removed from a role, and
  the permissions of an account you disable or enable must all be ones you hold. Without
  it `identity.account.manage` alone could assign Administrator.
- System roles cannot be renamed or deleted (409); their permissions can change.
- Lock-out guards (409 `/errors/lockout`): you cannot disable yourself, drop your own
  Administrator role, or remove `identity.role.manage` from Administrator, and nobody can
  disable or demote the last active Administrator. Operations that could remove an admin
  lock the Administrator role row first, so two admins acting on each other at once
  cannot both pass.
- A role still assigned to an account cannot be deleted (409).

## Events (`contract/events.go`)

`identity.account_created`, `account_updated` (field changes), `account_disabled`,
`account_enabled`, `invitation_resent`, `invitation_accepted` (actor: the account
itself), `password_reset_sent` (admin), `roles_assigned` (from/to), `role_created`,
`role_updated`, `role_permissions_updated` (from/to), `role_deleted`, `account_signed_out`
(admin sign-out that ended at least one session). No events for
sign-in, refresh, logout, the public forgot request, completing a reset, or changing
your own password. Payloads never carry hashes or tokens.

## For other modules

- Check permissions in your service with `auth.Require(ctx, "<module>.<thing>.<action>")`.
- Show "changed by": `contract.AccountReader` (`GetAccount`, `GetAccounts`).
- Jobs run as the requesting account: `jobs.RestoreActor(ctx, id, module.LoadActor)`.
  A deleted account cancels the job; a disabled one runs with no permissions.

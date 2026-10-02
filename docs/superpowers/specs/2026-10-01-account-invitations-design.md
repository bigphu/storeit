# Account invitations and password reset: design

Date: 2026-10-01. Builds on the identity module (`2026-10-01-identity-module-design.md`).

## Goal

Administrators create accounts without choosing a password. The user receives a
one-time link by email and sets their own. The same mechanism serves "forgot
password" and admin-triggered resets. Backend only: the links point at frontend
pages that do not exist yet; those pages post the token to a public endpoint.

Decisions taken with the user:

- Scope: invitation **and** forgot password. The admin "set password" endpoint is
  replaced by "send reset link".
- Link lifetimes: invite 72h, reset 1h. Each link is single-use; a new link
  supersedes the previous one of the same purpose.
- The token is created in the request transaction and travels to the worker in the
  job args (approach A). Accepted cost: the raw token sits in `river_job.args` until
  River deletes the finished job (24h by default).
- Mail transport lives in a new `platform/mail` package with `smtp`, `resend` and
  `log` transports, chosen explicitly. Production uses the Resend HTTP API; dev
  compose uses SMTP to a Mailpit container.
- Migration files are edited in place (no ALTER migrations).
- The bootstrap Administrator still uses `ADMIN_EMAIL`/`ADMIN_PASSWORD`.

## 1. Data and tokens

`identity.accounts.password_hash` becomes nullable. NULL means the invitation has
not been accepted.

| Status | `active` | `password_hash` | Can sign in |
|---|---|---|---|
| `invited` | true | NULL | no |
| `active` | true | set | yes |
| `disabled` | false | any | no |

`domain.Account.Status()` derives it; the API exposes `status` beside `active`.
Signing in to an invited account is indistinguishable from an unknown email (same
401, same dummy bcrypt compare).

New table, in `00002_identity.sql`:

```sql
CREATE TABLE identity.password_tokens (
    account_id uuid        NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    purpose    text        NOT NULL,
    id         uuid        NOT NULL,   -- new on every issue; mail idempotency key
    token_hash bytea       NOT NULL,   -- SHA-256 of 32 random bytes
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (account_id, purpose),
    CONSTRAINT password_tokens_hash_key UNIQUE (token_hash),
    CONSTRAINT password_tokens_purpose_check CHECK (purpose IN ('invite', 'reset'))
);
```

- **Issue**: upsert on `(account_id, purpose)` with a fresh `id`, hash and expiry, so
  the previous link dies immediately.
- **Use**: `DELETE … WHERE token_hash = $1 RETURNING`, then lock the account, check
  expiry and state, set the password, all in one transaction. Two concurrent
  submissions: one wins, the other finds nothing.
- **Prune**: the hourly `identity.prune_sessions` job also deletes expired rows.

Rules:

- Creating an account issues an invite (72h). An admin may resend it while the
  account is still invited and active.
- A reset (1h) is issued by the public forgot request or by an admin. If the account
  is still invited, an invite is issued instead (fresh 72h link), for both paths.
- Completing either sets the password and returns 204; it does not sign in.
  Completing a reset revokes every session of the account (reason `admin`).
  Completing an invite also deletes any reset token (none can exist; defensive).
- Disabling an account deletes its password tokens. Enabling does not restore them.
- Public forgot request: always 202. Unknown email, disabled account and the
  60-second per-account cooldown (a token of that purpose created less than 60s
  ago) are silently ignored. The cooldown is not applied to admin actions.
- Sending a reset link revokes nothing; completing it does.

## 2. API, errors, events

New operations (all under `/api/v1`):

| Method | Path | Operation | Needs | Body | Response |
|---|---|---|---|---|---|
| POST | `/auth/password/forgot` | `forgotPassword` | public | `{email}` | 202 |
| POST | `/auth/password/set` | `setPassword` | public | `{token, new_password}` | 204 |
| POST | `/accounts/{accountID}/invitation` | `resendInvitation` | `identity.account.manage` | none | 202 |
| POST | `/accounts/{accountID}/password-reset` | `sendPasswordReset` | `identity.account.manage` | none | 202 |

Changed:

- `POST /accounts` drops `password`; returns 201 with `status: invited` and queues
  the invitation in the same transaction.
- `Account` gains `status` (`invited | active | disabled`, required).
- `PUT /accounts/{accountID}/password` (`resetPassword`) is removed.
- `forgotPassword` and `setPassword` join the public operation list.

New errors (`domain/errors.go`):

| Var | Status | Type | Notes |
|---|---|---|---|
| `ErrInvalidPasswordToken` | 422 | `/errors/invalid-password-token` | field `token`, detail "This link is invalid or has expired. Ask for a new one." Unknown, used, superseded, expired, account disabled |
| `ErrNotInvited` | 409 | `/errors/not-invited` | resend on an account that has a password |
| `ErrAccountInactive` | 409 | `/errors/account-inactive` | resend or reset link for a disabled account |

Events (`contract/events.go`, aggregate `account`, payload `{account_id}`):

| Event | When | Actor |
|---|---|---|
| `identity.account_created` | admin creates (invite implied) | admin |
| `identity.invitation_resent` | admin resends, or admin reset on an invited account | admin |
| `identity.invitation_accepted` | first password set; account becomes active | the account |
| `identity.password_reset_sent` | admin sends a reset link (replaces `identity.password_reset`) | admin |

The public forgot request and completing a reset emit no events. Payloads never
contain tokens.

## 3. Mail and jobs

### `platform/mail`

Transport only; no templates, no knowledge of modules.

```go
type Address struct{ Name, Email string }
type Message struct {
    To             Address
    Subject        string
    Text, HTML     string
    IdempotencyKey string // Resend header; ignored by smtp/log
}
type Sender interface{ Send(ctx context.Context, m Message) error }
func New(cfg Config, log *slog.Logger) (Sender, error)
func IsPermanent(err error) bool // retrying cannot help
```

- `resend`: `POST {base}/emails` with `Authorization: Bearer`, `Idempotency-Key`,
  `from`, `to`, `subject`, `text`, `html`, optional `reply_to`. Non-2xx returns
  `*StatusError{Status, Reason}`; `Reason` reads only `name` and `message` from the
  JSON body, is truncated, and has the API key redacted. Transport errors are
  replaced by a static message (the URL and key must not reach logs). 4xx except
  408/429 is permanent. HTTP client timeout 30s. Ported from mimir-2.0.
- `smtp`: `github.com/wneessen/go-mail`. TLS mode `starttls` (mandatory, default),
  `tls` (implicit, port 465) or `none` (dev only, Mailpit). Auth only when a username
  is set. Client timeout 30s (go-mail only passes ctx to dial). A go-mail
  `*SendError` with `IsTemp() == false` is permanent.
- `log`: logs the message (recipient, subject, text body) at WARN. For tests and
  running without compose; the text body contains the link.
- From and Reply-To come from config, parsed with `net/mail` at startup.

### Identity side

- `job.SendAccountEmailArgs{TokenID uuid, Purpose string, Token string}`, kind
  `identity.send_account_email`, queue default, default max attempts.
- Enqueued with `EnqueueTx` by the repository in the same transaction that issues
  the token (account create, resend, reset). `identity.Deps` gains
  `Jobs jobs.Enqueuer` (required).
- `worker.SendAccountEmail` calls `service.SendAccountEmail(ctx, args)`:
  1. Load the token row by `TokenID`. Missing (used or superseded) or expired:
     return nil without sending.
  2. Load the account. Disabled: return nil.
  3. Render the template for the purpose and send with idempotency key
     `identity/<purpose>/<token id>`.
  4. `mail.IsPermanent(err)` → `river.JobCancel(err)`; other errors retry.
- Templates: `service/templates/{invite,reset}.{txt,html}`, embedded,
  `text/template` and `html/template`. English copy. Links:
  `{IDENTITY_APP_URL}/accept-invite#token=…` and
  `{IDENTITY_APP_URL}/reset-password#token=…`. The token is in the fragment so it
  never reaches a server log or a `Referer` header.
- `identity.Deps` gains `Mail mail.Sender`, required by `RegisterWorkers` (which now
  returns an error). The API server does not need mail config.

## 4. Configuration and testing

`mail.Config` (worker only):

| Env | Default | |
|---|---|---|
| `MAIL_TRANSPORT` | required | `smtp`, `resend` or `log` |
| `MAIL_FROM` | required | `StoreIt <no-reply@example.com>` |
| `MAIL_REPLY_TO` | empty | |
| `MAIL_SMTP_HOST` | | required for smtp |
| `MAIL_SMTP_PORT` | 587 | |
| `MAIL_SMTP_USERNAME`, `MAIL_SMTP_PASSWORD_FILE` | empty | both or neither |
| `MAIL_SMTP_TLS` | `starttls` | `starttls`, `tls`, `none` |
| `MAIL_RESEND_API_KEY_FILE` | | required for resend (Docker secret) |

`identity.Config` gains `IDENTITY_APP_URL` (default `http://localhost:3000`),
`IDENTITY_INVITE_TTL` (72h), `IDENTITY_RESET_TTL` (1h).

Compose: a `mailpit` service (SMTP 1025, web UI on `127.0.0.1:8025`); the dev worker
uses `MAIL_TRANSPORT=smtp`, host `mailpit`, TLS `none`. Prod compose: `resend` with
the key as a Docker secret. `.env.local.example` uses `smtp` to `127.0.0.1:1025`.

Tests:

- `platform/mail`: config validation; resend against `httptest` (headers, body,
  status classification, reason filtering, key redaction); smtp against a Mailpit
  test container (message arrives; permanent classification); log transport.
- `identity/domain`: `Status()`.
- `identity/service` (fakes): create issues invite; resend rules; admin reset on
  invited sends invite; forgot ignores unknown/disabled/cooldown; set password for
  invite and reset; invalid token; login on invited account; send-email job skips
  stale tokens and maps permanent errors.
- `identity/repository` (Postgres): issue supersedes, consume is single-use under
  concurrency, expiry, disable deletes tokens, prune, job enqueued in the same tx.
- HTTP (`http_db_test.go`): new routes, status codes, public access, removed route.
- End to end: admin creates account → read the token from the queued job (as the
  user would from the email) → set password → sign in; forgot → reset → old sessions
  revoked.

## Out of scope

Rate limiting beyond the per-account cooldown, email change, frontend pages,
bounce/complaint webhooks, localisation of email copy.

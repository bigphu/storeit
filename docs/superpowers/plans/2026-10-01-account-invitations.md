# Account Invitations and Password Reset Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Admin-created accounts receive an emailed one-time link to set their password; the same tokens serve forgot-password and admin-triggered resets.

**Architecture:** A new `platform/mail` package sends mail (smtp, resend, log). Identity stores SHA-256 hashes of one-time tokens in `identity.password_tokens`, issues them in the request transaction together with a River job carrying the raw token, and the worker renders and sends the email.

**Tech Stack:** Go, pgx, sqlc, River, oapi-codegen strict server, `github.com/wneessen/go-mail`, Resend HTTP API, Mailpit (dev).

**Spec:** `docs/superpowers/specs/2026-10-01-account-invitations-design.md`

## Global Constraints

- Code comments and package docs in Vietnamese; LF line endings.
- Never hand-edit `*.gen.go` or `repository/db`; run `make generate` (sqlc in Docker `sqlc/sqlc:1.31.1`, oapi-codegen).
- Edit `migrations/00002_identity.sql` in place; no new ALTER migration.
- Invite TTL 72h, reset TTL 1h, forgot cooldown 60s per account.
- Token: 32 random bytes base64url, only SHA-256 stored. Link carries it in the fragment: `/accept-invite#token=`, `/reset-password#token=`.
- Every `setPassword` failure is 422 `/errors/invalid-password-token` field `token`.
- Public forgot request always returns 202.
- Transactions, outbox `Append` and `EnqueueTx` only in repositories.
- depguard: `platform/*` never imports modules; worker imports only service/domain/contract/job/platform.
- Test command: `CI=true go test -count=1 -p 1 ./...` in `backend/`.

## Review Focus

1. A link used twice at the same moment (two tabs) → exactly one succeeds, the other gets 422. Test: concurrent consume in the repository DB test (Task 4).
2. A job retried after the token was superseded or used → no email for the dead link. Test: `SendAccountEmail` skips missing/expired tokens (Task 5).
3. Resend returns 4xx (unverified domain) → job cancelled, not retried 10 times. Test: worker maps `mail.IsPermanent` to `river.JobCancel` (Task 5) and `StatusError.Permanent` table (Task 1).
4. Forgot password for an unknown, invited or disabled email → same 202, no leak; invited gets a new invite. Test: service forgot cases (Task 5) and HTTP 202 for unknown email (Task 6).
5. Disabling an account with a pending invite → the link stops working. Test: repository disable deletes tokens (Task 4) and set-password after disable is 422 (Task 5).

---

### Task 1: `platform/mail`

**Files:**
- Create: `backend/internal/platform/mail/{doc.go,config.go,mail.go,resend.go,smtp.go,log.go,errors.go}`
- Test: `backend/internal/platform/mail/{config_test.go,resend_test.go,log_test.go,smtp_db_test.go}`
- Modify: `backend/go.mod` (add `github.com/wneessen/go-mail`), `backend/docs/platform.md`

**Interfaces (produces):**
```go
type Address struct{ Name, Email string }
type Message struct {
    To             Address
    Subject        string
    Text, HTML     string
    IdempotencyKey string
}
type Sender interface{ Send(ctx context.Context, m Message) error }
type Transport string // "smtp" | "resend" | "log"
type Config struct {
    Transport          Transport `env:"MAIL_TRANSPORT,required"`
    From               string    `env:"MAIL_FROM,required"`
    ReplyTo            string    `env:"MAIL_REPLY_TO"`
    SMTPHost           string    `env:"MAIL_SMTP_HOST"`
    SMTPPort           int       `env:"MAIL_SMTP_PORT" envDefault:"587"`
    SMTPUsername       string    `env:"MAIL_SMTP_USERNAME"`
    SMTPPassword       string    `env:"MAIL_SMTP_PASSWORD_FILE,file"`
    SMTPTLS            string    `env:"MAIL_SMTP_TLS" envDefault:"starttls"`
    ResendAPIKey       string    `env:"MAIL_RESEND_API_KEY_FILE,file"`
    ResendBaseURL      string    // test only; empty = https://api.resend.com
}
func (c Config) Validate() error
func New(cfg Config, log *slog.Logger) (Sender, error)
type StatusError struct{ Status int; Reason string }
func (e *StatusError) Permanent() bool
func IsPermanent(err error) bool
```

- [ ] Step 1: Config tests — table: missing transport, unknown transport, bad From (`net/mail.ParseAddress`), smtp without host, username without password (and vice versa), bad TLS mode, resend without key, valid smtp/resend/log.
- [ ] Step 2: Resend tests against `httptest.Server`: request method/path/headers (`Authorization: Bearer k`, `Idempotency-Key`), JSON body (`from` formatted `StoreIt <a@b>`, `to`, `subject`, `text`, `html`, `reply_to` omitted when empty); 200 → nil; 422 → `*StatusError` permanent with reason `name: message`; 429 and 500 → not permanent; reason with API key echoed → `[redacted]`; non-JSON body → "no readable reason in response"; server closed → error string contains neither URL nor key.
- [ ] Step 3: `StatusError.Permanent` table (400,401,403,404,422 true; 408,429,500,503 false); `IsPermanent` unwraps `%w`.
- [ ] Step 4: Log transport test: writes WARN with `to`, `subject`, `text`.
- [ ] Step 5: SMTP test with Mailpit testcontainer (`axllent/mailpit`), TLS `none`: message arrives (query Mailpit `GET /api/v1/messages`), skip without Docker unless `CI=true` (follow `dbtest` skip helper behaviour).
- [ ] Step 6: Run, see failures; implement; run until green. Port resend from `C:\d-drive\code\mimir-2.0\apps\backend\internal\email\resend.go`.
- [ ] Step 7: Document `mail` in `docs/platform.md` (package map, config table, details). Commit `feat(platform): mail package with smtp, resend and log transports`.

### Task 2: Schema and queries

**Files:**
- Modify: `backend/migrations/00002_identity.sql`, `backend/internal/identity/repository/queries/accounts.sql`
- Create: `backend/internal/identity/repository/queries/password_tokens.sql`
- Generated: `backend/internal/identity/repository/db/*`
- Test: `backend/internal/identity/repository/schema_db_test.go`

**Produces (sqlc):**
```sql
-- name: UpsertPasswordToken :one
INSERT INTO identity.password_tokens (account_id, purpose, id, token_hash, expires_at)
VALUES (@account_id, @purpose, @id, @token_hash, @expires_at)
ON CONFLICT (account_id, purpose) DO UPDATE
SET id = EXCLUDED.id, token_hash = EXCLUDED.token_hash, created_at = now(), expires_at = EXCLUDED.expires_at
RETURNING *;
-- name: ConsumePasswordToken :one
DELETE FROM identity.password_tokens WHERE token_hash = @token_hash RETURNING *;
-- name: GetPasswordToken :one       (by id)
-- name: GetAccountPasswordToken :one (by account_id, purpose)
-- name: DeleteAccountPasswordTokens :exec
-- name: DeleteExpiredPasswordTokens :execrows  (expires_at < @cutoff)
```

- [ ] Step 1: Schema test: `password_hash` accepts NULL; `password_tokens` purpose check rejects `other`; unique `token_hash`; PK one row per (account, purpose); cascade on account delete.
- [ ] Step 2: Edit migration (nullable hash, new table, Down drops it), add queries, `make generate`, fix compile errors from `PasswordHash *string` in `map.go`/repository.
- [ ] Step 3: Run repository tests; commit `feat(identity): password_tokens table, nullable password hash`.

### Task 3: Domain and contract

**Files:**
- Modify: `domain/account.go`, `domain/errors.go`, `domain/repository.go`, `contract/events.go`, `domain/domain_test.go`
- Create: `domain/password_token.go`

**Produces:**
```go
type AccountStatus string // StatusInvited, StatusActive, StatusDisabled
func (a Account) Status() AccountStatus
func (a Account) HasPassword() bool
type TokenPurpose string // PurposeInvite "invite", PurposeReset "reset"
type IssuedToken struct { ID uuid.UUID; Purpose TokenPurpose; Raw string; Hash []byte; ExpiresAt time.Time }
type PasswordToken struct { ID, AccountID uuid.UUID; Purpose TokenPurpose; CreatedAt, ExpiresAt time.Time }
type UsedToken struct { AccountID uuid.UUID; Purpose TokenPurpose }
type TokenEvent int // TokenEventNone, TokenEventInvitationResent, TokenEventResetSent
type TokenRepository interface {
    // Issue: upsert + EnqueueTx(SendAccountEmailArgs) + event, một tx
    Issue(ctx context.Context, accountID uuid.UUID, t IssuedToken, ev TokenEvent) error
    // Use: consume + kiểm tra + đặt mật khẩu (+ thu hồi phiên nếu reset, event nếu invite); ErrInvalidPasswordToken
    Use(ctx context.Context, hash []byte, now time.Time, passwordHash string) (UsedToken, error)
    Get(ctx context.Context, id uuid.UUID) (PasswordToken, error)            // ErrInvalidPasswordToken nếu không có
    Latest(ctx context.Context, accountID uuid.UUID, p TokenPurpose) (*PasswordToken, error)
}
// NewAccount gains Invite *IssuedToken; PasswordHash "" means NULL
// AccountRepository.SetPassword loses the reset flag: SetPassword(ctx, id, hash, keepFamily)
// SessionRepository.Prune also deletes expired password tokens (returns tokens count)
var ErrInvalidPasswordToken, ErrNotInvited, ErrAccountInactive
// contract: EventInvitationResent, EventInvitationAccepted, EventPasswordResetSent (replaces EventPasswordReset)
// payloads InvitationResent, InvitationAccepted, PasswordResetSent {AccountID}
```

- [ ] Step 1: Tests for `Status()` (3 rows) and `HasPassword`.
- [ ] Step 2: Implement; build fails elsewhere until Tasks 4–5 — keep compile green by updating call sites minimally in the same commit.
- [ ] Step 3: Commit `feat(identity): domain for password tokens and account status`.

### Task 4: Repository

**Files:**
- Create: `repository/token_repository.go`, `repository/token_repository_db_test.go`
- Modify: `repository/account_repository.go` (Create with invite, SetActive deletes tokens, SetPassword signature), `repository/session_repository.go` (Prune), `repository/map.go`, `repository/helpers_db_test.go`
- Uses: `jobs.Enqueuer.EnqueueTx`, `job.SendAccountEmailArgs` (created here in `job/send_account_email.go`)

**Produces:** `NewTokenRepository(pool, outbox, enq jobs.Enqueuer)`, `NewAccountRepository(pool, outbox, enq)`.

```go
// job/send_account_email.go
type SendAccountEmailArgs struct {
    TokenID uuid.UUID `json:"token_id"`
    Purpose string    `json:"purpose"`
    Token   string    `json:"token"`
}
func (SendAccountEmailArgs) Kind() string { return "identity.send_account_email" }
func (SendAccountEmailArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{Queue: jobs.QueueDefault} }
```

- [ ] Step 1: DB tests: create with invite → token row + one `river_job` of kind `identity.send_account_email` whose args hold the token id; Issue twice → old hash unusable, new id; Use valid invite → password set, row gone, `invitation_accepted` event; Use twice → second `ErrInvalidPasswordToken`; concurrent Use (2 goroutines) → exactly one success; expired → invalid; account disabled → invalid; reset Use → all families revoked; disable deletes tokens; prune deletes expired tokens; `Issue` with `TokenEventResetSent` writes `password_reset_sent`.
- [ ] Step 2: Implement; run `CI=true go test -count=1 ./internal/identity/repository/`; commit `feat(identity): password token repository`.

### Task 5: Service, templates, job worker

**Files:**
- Create: `service/password_tokens.go`, `service/mail.go`, `service/templates/{invite,reset}.{txt,html}`, `worker/send_account_email.go`
- Modify: `service/service.go` (Deps: `Tokens` repo is named `PasswordTokens`, `Mail mail.Sender`, `AppURL string`, Settings `InviteTTL`, `ResetTTL`), `service/accounts.go` (CreateAccount without password; remove ResetPassword; add ResendInvitation, SendPasswordReset), `service/auth.go` (Login on invited), `service/contract.go` (Bootstrap uses new NewAccount), `service/fakes_test.go`, `service/service_test.go`

**Produces:**
```go
func (s *Service) CreateAccount(ctx, in CreateAccountInput) (AccountView, error) // no Password field
func (s *Service) ResendInvitation(ctx context.Context, id uuid.UUID) error
func (s *Service) SendPasswordReset(ctx context.Context, id uuid.UUID) error
func (s *Service) ForgotPassword(ctx context.Context, email string) error
func (s *Service) SetPassword(ctx context.Context, token, password string) error
func (s *Service) SendAccountEmail(ctx context.Context, args job.SendAccountEmailArgs) error
// worker.NewSendAccountEmail(svc) ; Work maps mail.IsPermanent → river.JobCancel
```

- [ ] Step 1: Service tests with fakes: create issues invite (purpose, TTL 72h, hash = sha256(raw)); resend: invited ok + event, active → ErrNotInvited, disabled → ErrAccountInactive, needs manage; admin reset: active → reset + event, invited → invite + resent event, disabled → ErrAccountInactive; forgot: unknown/malformed/disabled → nil no issue, active → reset, invited → invite no event, cooldown <60s → no issue, ≥60s → issue; set password: weak → ErrWeakPassword, invalid → ErrInvalidPasswordToken; login invited → ErrBadCredentials; SendAccountEmail: token missing → nil no send, expired → nil, account disabled → nil, ok → message to account email with link `http://app/accept-invite#token=RAW` in text and html, idempotency key `identity/invite/<id>`, sender error propagated.
- [ ] Step 2: Worker test (in `service` via fakes is not possible for river; put in module root `module_db_test.go` or a unit test with a fake sender through `identity.New`): permanent error → `*river.JobCancelError`.
- [ ] Step 3: Implement, green, commit `feat(identity): invitations, password reset and account email job`.

### Task 6: OpenAPI, handler, module, config

**Files:**
- Modify: `handler/openapi.yaml`, `handler/api/api.gen.go` (generated), `handler/accounts.go`, `handler/auth.go`, `handler/convert.go`, `handler/handler.go`, `module.go`, `config.go`, `config_test.go`, `http_db_test.go`, `module_db_test.go`

- [ ] Step 1: HTTP tests: create account without password → 201 `status: invited`; `PUT /accounts/{id}/password` → 404/405; forgot unknown email → 202 without token; forgot without auth allowed; set with bad token → 422 type invalid-password-token field token; resend on active → 409 not-invited; password-reset as Employee → 403.
- [ ] Step 2: Config tests for `IDENTITY_INVITE_TTL`, `IDENTITY_RESET_TTL` (negative rejected), `IDENTITY_APP_URL` (must parse as absolute http(s) URL).
- [ ] Step 3: Implement spec + handlers; `Deps{Jobs jobs.Enqueuer (required), Mail mail.Sender}`; `RegisterWorkers(workers) error` requires Mail; `make generate`; green; commit `feat(identity): invitation and password reset endpoints`.

### Task 7: Binaries, compose, e2e, docs

**Files:**
- Modify: `cmd/server/main.go`, `cmd/worker/{main.go,config.go}`, `compose.yml`, `compose.prod.yml`, `.env.local.example`, `.env.prod.example`, `.env.local` (user's file: add MAIL_* only), `e2e_db_test.go`, `backend/docs/identity.md`, `CLAUDE.md`, `.golangci.yml` if needed
- [ ] Step 1: e2e: admin creates account → read `args->>'token'` from `river_job` → `POST /auth/password/set` → login 200; forgot → token from job → set → old refresh cookie 401.
- [ ] Step 2: Wire binaries (server: `jobs.NewRiver(insertClient)`; worker: `mail.New(cfg.Mail, log)`), compose Mailpit, env examples.
- [ ] Step 3: `go vet ./... && go build ./...`, full suite `CI=true go test -count=1 -p 1 ./...`; docs; commit; update the identity artifact.

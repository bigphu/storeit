# Identity Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Sign-in/out, refresh-token rotation and RBAC in `backend/internal/identity`, runnable through `cmd/migrate`, `cmd/server`, `cmd/worker`.

**Architecture:** Layered module (domain → service → repository/handler) on the platform packages. Refresh tokens rotate inside families (port of mimir-2.0); the refresh decision is a pure domain function, the repository applies it under `FOR UPDATE` locks in one transaction. All endpoints are in `openapi.yaml` (strict server), validated by `web.ValidateRequests`.

**Tech Stack:** Go 1.26, chi, oapi-codegen v2.8 strict server, pgx v5, sqlc, goose, River, bcrypt (`golang.org/x/crypto/bcrypt`), testcontainers.

**Spec:** `docs/superpowers/specs/2026-10-01-identity-module-design.md`

## Global Constraints

- Layer rules in `backend/.golangci.yml`: domain/contract import only stdlib, uuid, `platform/errs`; service never imports pgx, net/http, chi, River, `platform/database`, `platform/web`; repository is the only user of `repository/db` and transactions.
- Every state change appends its event with `Outbox.Append` in the same `database.WithTx`; sign-in, refresh, logout append none.
- Every service use case except login/refresh/logout/me starts with `auth.Require(ctx, perm)`.
- Passwords: 12 to 72 bytes, bcrypt. Emails stored lowercased; uniqueness on `lower(email)`.
- Refresh secrets: 32 random bytes base64url; only SHA-256 hashes stored; never logged.
- Cookie: name `storeit_refresh`, `HttpOnly`, `SameSite=Strict`, `Path=/api/v1/auth`, `Secure` from `IDENTITY_COOKIE_SECURE`.
- Env names owned by `identity.Config` (`IDENTITY_*`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`), no `envPrefix`.
- Code comments and package docs in Vietnamese; files LF.
- Tests: `CI=true go test ./...` must pass with Docker running.

## Review Focus

1. Two refreshes of the same cookie at once (two browser tabs): exactly one rotates, the other gets a grace rotation of the tip; never two live tokens in a family. → Task 4 concurrency test.
2. A refresh token replayed after the grace window (stolen): the whole family is revoked and every token in it stops working. → Task 4 and Task 5 tests.
3. A disabled account holding a still-valid refresh cookie: refresh fails and the family is revoked. → Task 4 and Task 5 tests.
4. An administrator trying to lock everyone out (disable self, drop own Administrator role, strip `identity.role.manage` from Administrator): refused with 409. → Task 5 tests.
5. Email case (`Admin@X.com` vs `admin@x.com`): login works with any case; creating the second is a 409 conflict. → Task 4 and Task 5 tests.

---

### Task 1: Schema, seed and sqlc queries

**Files:**
- Create: `backend/migrations/00002_identity.sql`
- Create: `backend/internal/identity/repository/queries/accounts.sql`, `roles.sql`, `sessions.sql`
- Modify: `backend/sqlc.yml` (uncomment the M1 block)
- Generated: `backend/internal/identity/repository/db/*`
- Test: `backend/internal/identity/repository/schema_db_test.go`

**Interfaces:**
- Produces: sqlc package `storeit/internal/identity/repository/db` with `Queries` methods named exactly as the query names below.

- [ ] **Step 1: Failing test** — `TestSchema_SeedRoles` (package `repository_test`, uses `dbtest.Pool`): selects role names ordered → want `[Administrator Authorized Manager Employee Inventory Officer]`; selects Administrator's permissions → want the four `identity.*` codes; Authorized Manager → `identity.account.read`, `identity.role.read`.
- [ ] **Step 2: Run** `CI=true go test ./internal/identity/repository/ -run Schema` → FAIL (relation `identity.roles` does not exist).
- [ ] **Step 3: Migration** `00002_identity.sql` (goose Up/Down) creating schema `identity` and the tables, indexes and seed exactly as in spec §2. Constraints: `CHECK (revoked_reason IN ('logout','reuse_detected','admin','expired'))`, unique index `accounts_email_lower ON accounts (lower(email))`, unique partial `refresh_tokens_live ON refresh_tokens (family_id) WHERE used_at IS NULL`, `refresh_families_live ON refresh_families (account_id) WHERE revoked_at IS NULL`. Seed role IDs are fixed UUIDs so tests and code can reference Administrator:

```sql
-- Administrator: 00000000-0000-7000-8000-000000000001
-- Authorized Manager: ...0002, Inventory Officer: ...0003, Employee: ...0004
```

- [ ] **Step 4: Queries.** `accounts.sql`: `CreateAccount`, `GetAccount`, `GetAccountForUpdate` (`FOR UPDATE`), `GetAccountByEmail` (`WHERE lower(email) = lower($1)`), `ListAccounts` (filters `q` ILIKE on name/email, `active` via `sqlc.narg`, `LIMIT/OFFSET`, ordered by name), `CountAccounts` (same filters), `CountAllAccounts`, `UpdateAccountProfile` (name, member_id, `version = version + 1` WHERE id and version → `:execrows`), `SetAccountActive`, `SetAccountPassword`, `AccountPermissions` (DISTINCT codes via account_roles → role_permissions, ordered), `AccountRoles`, `DeleteAccountRoles`, `InsertAccountRole`, `GetAccountsByIDs`. `roles.sql`: `ListRoles`, `GetRole`, `GetRoleByName`, `CreateRole`, `UpdateRole`, `DeleteRole`, `RolePermissions` (for a role), `AllRolePermissions` (role_id, code for many roles), `DeleteRolePermissions`, `InsertRolePermission`, `CountRoleAssignments`, `ListPermissions`, `CountPermissions(codes)`, `CountRoles(ids)`. `sessions.sql`: `CreateFamily`, `CreateRefreshToken`, `GetRefreshForUpdate` (port of mimir's `GetRefreshTokenForUpdate`, plus `a.active AS account_active` joined from accounts, `FOR UPDATE OF t, f`), `GetFamilyTip`, `MarkRefreshTokenUsed :execrows`, `RevokeFamily`, `RevokeAccountFamilies` (optional `except` family via `sqlc.narg`), `DeleteDeadFamilies :execrows`, `DeleteUsedTokens :execrows`.
- [ ] **Step 5: Generate** `make sqlc` (or `go generate ./...`); `go build ./...`.
- [ ] **Step 6: Run** the schema test → PASS. Commit.

### Task 2: Domain

**Files:**
- Create/replace: `backend/internal/identity/domain/{account.go,role.go,session.go,password.go,permissions.go,errors.go,repository.go}`
- Test: `backend/internal/identity/domain/*_test.go`

**Interfaces (Produces):**

```go
// permissions.go
const (
	PermAccountRead   = "identity.account.read"
	PermAccountManage = "identity.account.manage"
	PermRoleRead      = "identity.role.read"
	PermRoleManage    = "identity.role.manage"
)
var AdministratorRoleID = uuid.MustParse("00000000-0000-7000-8000-000000000001")

// account.go
type Account struct {
	ID uuid.UUID; Email, Name, PasswordHash string; MemberID *uuid.UUID
	Active bool; Version int32; CreatedAt, UpdatedAt time.Time
}
func NormalizeEmail(s string) (string, error)  // trim + lower, must contain one '@' with text both sides → ErrInvalidEmail
func (a Account) CanLogin() error              // ErrAccountDisabled when !Active

// role.go
type Role struct { ID uuid.UUID; Name, Description string; IsSystem bool; Permissions []string }
func (r Role) CanRename() error                // ErrSystemRole when IsSystem
func (r Role) CanDelete(assigned int64) error  // ErrSystemRole / ErrRoleInUse
func CheckRolePermissions(roleID uuid.UUID, perms []string) error // Administrator must keep PermRoleManage → ErrLockout

// password.go
func ValidatePassword(p string) error          // 12..72 bytes → ErrWeakPassword (field "password")

// session.go
type RefreshState struct {
	TokenID, FamilyID, AccountID uuid.UUID
	UsedAt *time.Time; ExpiresAt, AbsoluteExpiresAt time.Time
	RevokedAt *time.Time; AccountActive bool
}
type RefreshDecision int
const ( RefreshReject RefreshDecision = iota; RefreshExpired; RefreshReuse; RefreshGrace; RefreshRotate; RefreshAccountDisabled )
func (s RefreshState) Decide(now time.Time, grace time.Duration) RefreshDecision
// order: revoked → Reject; !AccountActive → AccountDisabled; absolute passed → Expired;
// used: now-used <= grace → Grace, else Reuse; token expired → Reject; else Rotate
type RevokeReason string // "logout","reuse_detected","admin","expired"

// errors.go (errs.*)
ErrBadCredentials (401 /errors/bad-credentials), ErrAccountDisabled (403 /errors/account-disabled),
ErrInvalidRefreshToken (401 /errors/invalid-refresh-token), ErrInvalidEmail (422),
ErrWeakPassword (422), ErrEmailTaken (409 /errors/email-taken), ErrRoleNameTaken (409),
ErrAccountNotFound (404), ErrRoleNotFound (404), ErrAccountChanged (409 /errors/account-changed),
ErrSystemRole (409 /errors/system-role), ErrRoleInUse (409 /errors/role-in-use),
ErrLockout (409 /errors/lockout), ErrUnknownRoles (422), ErrUnknownPermissions (422)

// repository.go: interfaces consumed by service, implemented in Task 4 (see Task 4 Interfaces)
```

- [ ] **Step 1: Failing tests:** `TestDecide` table — revoked → Reject; inactive → AccountDisabled; absolute passed → Expired; used 10s ago, grace 30s → Grace; used 31s ago → Reuse; token expired unused → Reject; fresh → Rotate; revoked wins over everything. `TestValidatePassword` (11 bytes fail, 12 ok, 72 ok, 73 fail, multibyte counted in bytes). `TestNormalizeEmail` (`" Admin@X.com "` → `admin@x.com`; `"nope"`, `"a@"`, `"@b"` → ErrInvalidEmail). `TestCanLogin`, `TestRoleGuards` (system role rename/delete → ErrSystemRole; assigned → ErrRoleInUse; Administrator without role.manage → ErrLockout).
- [ ] **Step 2:** run → FAIL (undefined). **Step 3:** implement. **Step 4:** run → PASS. Commit.

### Task 3: Module config

**Files:** Create `backend/internal/identity/config.go`; test `config_test.go`.

**Interfaces (Produces):**

```go
type Config struct {
	RefreshSlidingTTL  time.Duration `env:"IDENTITY_REFRESH_SLIDING_TTL" envDefault:"336h"`
	RefreshAbsoluteTTL time.Duration `env:"IDENTITY_REFRESH_ABSOLUTE_TTL" envDefault:"720h"`
	RefreshGracePeriod time.Duration `env:"IDENTITY_REFRESH_GRACE_PERIOD" envDefault:"30s"`
	RefreshRetention   time.Duration `env:"IDENTITY_REFRESH_RETENTION" envDefault:"720h"`
	CookieSecure bool   `env:"IDENTITY_COOKIE_SECURE" envDefault:"true"`
	CookieDomain string `env:"IDENTITY_COOKIE_DOMAIN"`
	AdminEmail    string `env:"ADMIN_EMAIL"`
	AdminPassword string `env:"ADMIN_PASSWORD"`
}
func (c Config) Validate() error   // durations > 0 (after defaults), sliding <= absolute, admin both-or-neither
func (c Config) withDefaults() Config
```

- [ ] Tests: env parse defaults; sliding > absolute → error; admin email without password → error; zero Config valid. Fail → implement → pass → commit.

### Task 4: Repository and events

**Files:**
- Create: `backend/internal/identity/contract/events.go` (event type consts + payload structs)
- Create: `backend/internal/identity/repository/{account_repository.go,role_repository.go,session_repository.go,map.go}`
- Test: `backend/internal/identity/repository/*_db_test.go`

**Interfaces (Produces, declared in `domain/repository.go`):**

```go
type AccountFilter struct { Query string; Active *bool; Limit, Offset int32 }
type NewAccount struct { Email, Name, PasswordHash string; MemberID *uuid.UUID; RoleIDs []uuid.UUID }
type ProfileChange struct { Name *string; MemberID *uuid.UUID; ClearMember bool; Version int32 }

type AccountRepository interface {
	Create(ctx context.Context, in NewAccount) (Account, error)        // ErrEmailTaken, ErrUnknownRoles; event account_created
	Get(ctx context.Context, id uuid.UUID) (Account, error)             // ErrAccountNotFound
	GetByEmail(ctx context.Context, email string) (Account, error)
	List(ctx context.Context, f AccountFilter) ([]Account, int64, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, ch ProfileChange) (Account, error) // ErrAccountChanged; event account_updated
	SetActive(ctx context.Context, id uuid.UUID, active bool) (Account, error) // disabling revokes all families (admin); events
	SetPassword(ctx context.Context, id uuid.UUID, hash string, keepFamily *uuid.UUID, reset bool) error
	// revokes the account's families except keepFamily; reset=true appends password_reset
	ReplaceRoles(ctx context.Context, id uuid.UUID, roleIDs []uuid.UUID) (Account, error) // ErrUnknownRoles; event roles_assigned
	Roles(ctx context.Context, id uuid.UUID) ([]Role, error)
	Permissions(ctx context.Context, id uuid.UUID) ([]string, error)
	Count(ctx context.Context) (int64, error)
	GetMany(ctx context.Context, ids []uuid.UUID) ([]Account, error)
}
type RoleRepository interface {
	List(ctx context.Context) ([]Role, error)
	Get(ctx context.Context, id uuid.UUID) (Role, error)
	Create(ctx context.Context, r Role) (Role, error)                    // ErrRoleNameTaken, ErrUnknownPermissions
	Update(ctx context.Context, id uuid.UUID, name, description string) (Role, error)
	ReplacePermissions(ctx context.Context, id uuid.UUID, perms []string) (Role, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CountAssignments(ctx context.Context, id uuid.UUID) (int64, error)
	Permissions(ctx context.Context) ([]Permission, error)               // catalogue
}
type Permission struct { Code, Description string }

type NewSession struct { AccountID uuid.UUID; TokenHash []byte; UserAgent, IP string; ExpiresAt, AbsoluteExpiresAt time.Time }
type RefreshInput struct { TokenHash, NextHash []byte; Now time.Time; Grace, Sliding time.Duration }
type RefreshResult struct { Decision RefreshDecision; AccountID, FamilyID uuid.UUID; ExpiresAt time.Time }
type SessionRepository interface {
	Start(ctx context.Context, s NewSession) (familyID uuid.UUID, err error)
	Refresh(ctx context.Context, in RefreshInput) (RefreshResult, error) // one tx: lock, Decide, apply; not found → Decision Reject
	Revoke(ctx context.Context, tokenHash []byte, reason RevokeReason) (familyID *uuid.UUID, err error) // idempotent
	FamilyOf(ctx context.Context, tokenHash []byte) (*uuid.UUID, error)
	Prune(ctx context.Context, cutoff time.Time) (families, tokens int64, err error)
}
```

`Refresh` applies: Reject → nothing; AccountDisabled → revoke(admin); Expired → revoke(expired); Reuse → revoke(reuse_detected) + WARN log (family, account); Grace → mark tip used, insert child of tip with `NextHash`; Rotate → mark token used (0 rows → Reject), insert child. Child expiry `min(now+sliding, absolute)`. Revocations commit before returning.

- [ ] **Step 1: Failing DB tests:** account create/get/case-insensitive email/duplicate → ErrEmailTaken; update with stale version → ErrAccountChanged; disable revokes live families; ReplaceRoles unknown role → ErrUnknownRoles; Permissions union of roles; event rows written (`platform.events` type `identity.account_created`, actor from ctx); refresh rotate/grace/reuse/expired/disabled; **concurrent**: two goroutines refresh the same hash → decisions `{Rotate, Grace}` and exactly one unused token remains; prune removes dead families and old used tokens but keeps live tips; role create duplicate name; replace permissions unknown code → ErrUnknownPermissions.
- [ ] **Step 2:** run → FAIL. **Step 3:** implement (unique violation `23505` on `accounts_email_lower` → ErrEmailTaken, on `roles_name_key` → ErrRoleNameTaken; FK `23503` on role/permission → unknown). **Step 4:** PASS. Commit.

### Task 5: Service

**Files:**
- Create/replace: `backend/internal/identity/service/{service.go,auth.go,accounts.go,roles.go,contract.go,secret.go,hasher.go}`
- Test: `service/*_test.go` with fakes in `service/fakes_test.go`

**Interfaces (Produces):**

```go
type Hasher interface { Hash(pw string) (string, error); Compare(hash, pw string) bool }
func NewBcrypt(cost int) Hasher
type TokenIssuer interface { Issue(accountID uuid.UUID, perms []string) (jwt.Token, error) }
type Settings struct { SlidingTTL, AbsoluteTTL, Grace time.Duration }
type Deps struct {
	Accounts domain.AccountRepository; Roles domain.RoleRepository; Sessions domain.SessionRepository
	Hasher Hasher; Tokens TokenIssuer; Settings Settings; Now func() time.Time
}
func New(d Deps) *Service

type Device struct { UserAgent, IP string }
type Session struct { AccessToken string; AccessExpiresAt time.Time; RefreshToken string; RefreshExpiresAt time.Time; Account domain.Account }
func (s *Service) Login(ctx, email, password string, dev Device) (Session, error)
func (s *Service) Refresh(ctx, refreshToken string) (Session, error)     // any failure → ErrInvalidRefreshToken
func (s *Service) Logout(ctx, refreshToken string) error                  // idempotent
type Me struct { Account domain.Account; Roles []domain.Role; Permissions []string }
func (s *Service) Me(ctx) (Me, error)                                     // requires actor
func (s *Service) ChangePassword(ctx, current, next, refreshToken string) error
func (s *Service) CreateAccount(ctx, in CreateAccountInput) (AccountView, error)
func (s *Service) GetAccount(ctx, id) (AccountView, error)
func (s *Service) ListAccounts(ctx, f domain.AccountFilter) ([]AccountView, int64, error)
func (s *Service) UpdateAccount(ctx, id, ch domain.ProfileChange) (AccountView, error)
func (s *Service) DisableAccount(ctx, id) (AccountView, error)            // self → ErrLockout
func (s *Service) EnableAccount(ctx, id) (AccountView, error)
func (s *Service) ResetPassword(ctx, id, password string) error
func (s *Service) AssignRoles(ctx, id, roleIDs []uuid.UUID) (AccountView, error) // self without Administrator → ErrLockout
type AccountView struct { domain.Account; Roles []domain.Role }
func (s *Service) ListRoles/GetRole/CreateRole/UpdateRole/UpdateRolePermissions/DeleteRole/ListPermissions
func (s *Service) Bootstrap(ctx, email, password string) (created bool, err error) // only when Count()==0; Administrator role; runs as SystemActor
func (s *Service) LoadActor(ctx, id) (auth.Actor, error)                 // missing → jobs.ErrActorNotFound wrapped; disabled → Actor with no permissions
func (s *Service) Accounts() contract.AccountReader
```

- [ ] **Step 1: Failing tests:** login ok returns access + refresh, wrong password and unknown email → ErrBadCredentials (both call `Compare`), disabled → ErrAccountDisabled, email case-insensitive; refresh maps each decision (Rotate/Grace → session with **reloaded** permissions; others → ErrInvalidRefreshToken); logout empty/unknown token → nil; every managing use case without permission → 403 and without actor → 401; DisableAccount self → ErrLockout; AssignRoles dropping own Administrator → ErrLockout; UpdateRolePermissions stripping role.manage from Administrator → ErrLockout; ResetPassword passes `reset=true, keepFamily=nil`; ChangePassword wrong current → ErrBadCredentials, keeps current family; Bootstrap creates once, second call no-op; LoadActor unknown → wraps `jobs.ErrActorNotFound`.
- [ ] **Step 2–4:** fail → implement → pass. Commit.

### Task 6: OpenAPI spec and handler

**Files:**
- Replace: `backend/internal/identity/handler/openapi.yaml`, `handler.go`, `auth.go`, `accounts.go`, `roles.go`; create `cookie.go`, `convert.go`
- Generated: `handler/api/api.gen.go`
- Test: `handler/handler_test.go`

**Interfaces:** Produces `func New(svc *service.Service, cookie CookieSettings) *Handler`, `type CookieSettings struct{ Secure bool; Domain string }`, `func (h *Handler) Mount(r chi.Router, tokens *jwt.Provider) error`. Spec per design §4; refresh/logout declare `parameters: - {name: storeit_refresh, in: cookie, required: false}` and `Set-Cookie` response headers; refresh 401 declares `Set-Cookie` + problem body so errors clear the cookie. Schemas reference `../../../api/common.yaml` (`Problem`, `ID`, `Page`, `PageSize`, `Paged`).

- [ ] **Step 1: Failing tests** (fake-free: real service + DB via `dbtest`, router from `Mount`): login 200 sets `storeit_refresh` HttpOnly SameSite=Strict Path=/api/v1/auth; wrong password 401 problem; `GET /me` without token 401; Employee token on `GET /accounts` 403; refresh with cookie rotates (new cookie value differs) and 200; refresh with no cookie 401 + clearing cookie; logout 204 + clearing cookie; create account invalid email 422 from validator.
- [ ] **Step 2–4:** write spec, `go generate`, implement, pass. Commit.

### Task 7: Prune job, module assembly, LoadActor

**Files:** Create `backend/internal/identity/job/prune_sessions.go`, `worker/prune_sessions.go`, replace `module.go`; tests `worker/prune_sessions_test.go`, `module_test.go`.

**Interfaces (Produces):**

```go
// job
type PruneSessionsArgs struct{}
func (PruneSessionsArgs) Kind() string { return "identity.prune_sessions" }
// module.go
type Deps struct { Pool *pgxpool.Pool; Tokens *jwt.Provider; Outbox *events.Outbox; Config Config }
type Module struct { Handler *handler.Handler; Accounts contract.AccountReader; svc *service.Service }
func New(d Deps) (*Module, error)
func (m *Module) Mount(r chi.Router) error
func (m *Module) LoadActor(ctx context.Context, id uuid.UUID) (auth.Actor, error)
func (m *Module) Bootstrap(ctx context.Context) error
func RegisterWorkers(workers *river.Workers, pool *pgxpool.Pool, cfg Config)
func PeriodicJobs() []*river.PeriodicJob   // hourly prune
```

- [ ] Tests: worker prunes (DB); `New` + `Bootstrap` creates admin from config once. Fail → implement → pass. Commit.

### Task 8: Binaries and deployment files

**Files:** `backend/cmd/migrate/{main.go,config.go}`, `backend/cmd/server/{main.go,config.go}`, `backend/cmd/worker/{main.go,config.go}`, `compose.yml`, `compose.prod.yml`, `.env.local.example`, `.env.example`, `.env.prod.example`.

- [ ] `cmd/migrate`: `config.Load` (Log, DB) → `database.Open` → goose `Up` on `migrations.FS` → `rivermigrate` up → exit 0.
- [ ] `cmd/server`: config (Log, HTTP, DB, JWT, Identity) → logger → pool → `jwt.New` → `events.NewRegistry` → `jobs.NewInsertClient` → `events.NewOutbox` → `identity.New` → `Bootstrap` → `server.New`, `Router().Get("/healthz")`, `Mount` → `Run` with `signal.NotifyContext`.
- [ ] `cmd/worker`: config (Log, DB, Jobs, Identity) → pool → registry → `river.NewWorkers` → `events.RegisterWorker(…, module.LoadActor)` → `identity.RegisterWorkers` → `jobs.NewWorkerClient(…, identity.PeriodicJobs())` → `Start`, wait for signal, `Stop`.
- [ ] Compose: `IDENTITY_COOKIE_SECURE: "false"` in dev, `ADMIN_EMAIL` and `ADMIN_PASSWORD` from env; uncomment the worker service in `compose.yml`.
- [ ] Verify: `go build ./...`, `go vet ./...`.

### Task 9: End-to-end test and docs

**Files:** `backend/internal/identity/e2e_db_test.go`; `backend/docs/platform.md` (cross-reference), `CLAUDE.md` (identity note), package `doc.go`/comments.

- [ ] End-to-end per spec §8 against `server.New` + `Mount` + `dbtest`: bootstrap admin → login → `/me` lists `identity.role.manage` → create Employee account → employee login → `GET /accounts` 403 → refresh rotates → logout → refresh with old cookie 401.
- [ ] Full verification: `gofmt -l`, `go vet ./...`, `CI=true go test ./...`, `golangci-lint` if available (`make lint`).
- [ ] Publish the module artifact (separate step after implementation).

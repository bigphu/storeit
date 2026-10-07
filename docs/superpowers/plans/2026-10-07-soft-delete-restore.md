# Soft Delete and Restore Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every delete the UI offers can be undone. Roles and export profiles become soft-deleted, and roles, export profiles, attributes and options each get a restore endpoint.

**Architecture:**
- One migration adds `deleted_at` to `identity.roles` and `inventory.export_profiles`. Both name-uniqueness indexes become partial (only rows not deleted).
- Repositories turn the hard deletes into updates, hide deleted rows from every read, and add restore methods. A restore locks the row, clears the timestamp, maps a unique violation to the existing "name/label taken" error, and appends an event.
- Attributes and options are already soft-deleted (`removed_at`); they only gain restore.
- This is plan 1 of 2 for the spec. Plan 2 is the frontend.

**Tech Stack:** Go 1.26, pgx v5, sqlc 1.31.1 (Docker, `make sqlc`), goose, oapi-codegen strict server, kin-openapi.

**Spec:** `docs/superpowers/specs/2026-10-07-ui-patterns-design.md` (sections "Backend" and "Action map")

## Global Constraints

- Branch `feat/ui-patterns`. Commit after each task, stage explicit paths, and never stage `backend/docs/tasks/`. Leave the user's uncommitted frontend files alone (`TabBar.vue`, `AccountsPage.vue`, `ReportDialog.vue`). Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Code comments are in Vietnamese. Files use LF. Never hand-edit `*.gen.go` or `*/repository/db/*`; run `make generate` (or `make sqlc`) in `backend/`.
- Transactions, events and outbox stay in repositories. Services check permissions with `auth.Require`.
- **Permissions:**
  - restore role: `identity.role.manage`;
  - restore attribute or option: `inventory.type.manage`;
  - restore export profile: only someone who could delete it, i.e. the owner, or for shared profiles also a holder of `inventory.export_profile.manage`. Anyone else gets 404.
- **Restore results:**
  - Restoring something that isn't deleted is a no-op that returns it.
  - Restoring something that doesn't exist is a 404, with the module's existing not-found error.
- **Restore conflicts (409), reusing existing errors:**
  - role name taken again: `/errors/role-name-taken`;
  - profile name taken again by the same owner: `/errors/export-profile-name-taken`;
  - attribute label taken again in the type: `/errors/attribute-label-taken`;
  - option label taken again in the attribute: `/errors/option-label-taken`.
- **Keys:** an attribute's key is unique across removed attributes too (`asset_type_attributes_key_key`), so restoring can't conflict on key.
- **Roles:** a role can still be deleted only when it's a custom role that nobody holds. Deleted roles are hidden from list and get (404), can't be assigned (`GetRolesByIDs` ignores them), and keep their permissions.
- **Export profiles:** deleted profiles are hidden from list, get, update, delete and from export by `profile_id`, for everyone.
- **Events:**
  - `identity.role_restored` (aggregate `role`);
  - `inventory.export_profile_restored` (aggregate `export_profile`);
  - attribute and option restores record `asset_type_updated` with the change `"removed" → "active"`, mirroring how removal records `"active" → "removed"`.

## Review Focus

1. **A deleted role's name reused:** delete role "Kế toán", create a new "Kế toán" (must succeed), then restore the old one (must 409 `role-name-taken`, not 500). Test: `TestRoleSoftDeleteAndRestore` (Task 1).
2. **Assigning a deleted role:** `PUT /accounts/{id}/roles` with a deleted role ID must fail as an unknown role, never grant its permissions. Test: `TestRoleSoftDeleteAndRestore` (Task 1).
3. **Someone else's deleted private profile:** another user restoring it must get 404. A non-manager restoring someone else's deleted *shared* profile must get 403. Test: `TestExportProfileRestoreOverHTTP` (Task 2).
4. **Exporting with a deleted profile's ID:** `POST /assets/export` with `profile_id` of a deleted profile must 404 `export-profile-not-found`. Test: `TestExportProfileRestoreOverHTTP` (Task 2).
5. **Restoring an attribute after its label was reused:** remove "RAM", add a new "RAM" (allowed), restore the old one → 409 `attribute-label-taken`. Test: `TestRestoreAttributeAndOption` (Task 3).

---

### Task 1: Role soft delete and restore

**Files:**
- Create: `backend/migrations/00006_soft_delete.sql`, covering both tables; Task 2 relies on it.
- Modify:
  - `backend/internal/identity/repository/queries/roles.sql`
  - `backend/internal/identity/repository/role_repository.go`
  - `backend/internal/identity/domain/repository.go` (`RoleRepository` interface)
  - `backend/internal/identity/contract/events.go`
  - `backend/internal/identity/service/roles.go`
  - `backend/internal/identity/handler/openapi.yaml`
  - `backend/internal/identity/handler/roles.go`
  - the identity service fakes (`backend/internal/identity/service/fakes_test.go`): add `Restore`
- Test: `backend/internal/identity/http_db_test.go`

**Interfaces:**
- Produces:
  - `RoleRepository.Restore(ctx, id uuid.UUID) (domain.Role, error)`
  - `Service.RestoreRole(ctx, id uuid.UUID) (domain.Role, error)`
  - `POST /api/v1/roles/{roleID}/restore`, which returns 200 with the same body as `GET /roles/{roleID}`
  - `contract.EventRoleRestored = "identity.role_restored"`, `contract.RoleRestored{RoleID uuid.UUID; Name string}`
  - Migration columns `identity.roles.deleted_at` and `inventory.export_profiles.deleted_at`

- [ ] **Step 1: Write the failing HTTP test**

Append to `backend/internal/identity/http_db_test.go`:

```go
// Xoá role là xoá mềm: ẩn khỏi danh sách, không gán được, khôi phục được; tên của role đã
// xoá dùng lại được, và khi đó khôi phục báo trùng tên
func TestRoleSoftDeleteAndRestore(t *testing.T) {
	a := newApp(t)
	admin, _ := a.login(a.seed(domain.AdministratorRoleID))
	name := "Kế toán " + uuid.NewString()[:6]
	var role struct{ ID string `json:"id"` }
	rec := a.do(call{method: "POST", path: "/api/v1/roles", token: admin, body: map[string]any{"name": name, "permissions": []string{"inventory.asset.read"}}})
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &role) != nil {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(call{method: "DELETE", path: "/api/v1/roles/" + role.ID, token: admin}); rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(call{method: "GET", path: "/api/v1/roles/" + role.ID, token: admin}); rec.Code != 404 {
		t.Errorf("get deleted: %d", rec.Code)
	}
	if rec := a.do(call{method: "GET", path: "/api/v1/roles", token: admin}); strings.Contains(rec.Body.String(), role.ID) {
		t.Error("deleted role listed")
	}
	// gán role đã xoá: như role không tồn tại
	accID := a.accountID(admin, a.seed(domain.EmployeeRoleID))
	if rec := a.do(call{method: "PUT", path: "/api/v1/accounts/" + accID + "/roles", token: admin, body: map[string]any{"role_ids": []string{role.ID}, "version": 1}}); rec.Code < 400 {
		t.Errorf("assigning a deleted role: %d %s", rec.Code, rec.Body)
	}

	rec = a.do(call{method: "POST", path: "/api/v1/roles/" + role.ID + "/restore", token: admin})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "inventory.asset.read") {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(call{method: "GET", path: "/api/v1/roles/" + role.ID, token: admin}); rec.Code != 200 {
		t.Errorf("get restored: %d", rec.Code)
	}

	// xoá lại, tạo role mới cùng tên (được), khôi phục role cũ → 409
	a.do(call{method: "DELETE", path: "/api/v1/roles/" + role.ID, token: admin})
	if rec := a.do(call{method: "POST", path: "/api/v1/roles", token: admin, body: map[string]any{"name": name}}); rec.Code != 201 {
		t.Fatalf("reuse name of deleted role: %d %s", rec.Code, rec.Body)
	}
	rec = a.do(call{method: "POST", path: "/api/v1/roles/" + role.ID + "/restore", token: admin})
	if rec.Code != 409 || problemType(t, rec) != "/errors/role-name-taken" {
		t.Errorf("restore over a reused name: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(call{method: "POST", path: "/api/v1/roles/" + uuid.NewString() + "/restore", token: admin}); rec.Code != 404 {
		t.Errorf("restore unknown role: %d", rec.Code)
	}
}
```

Add this helper next to `seed` in the same file (`a.seed` returns the new account's email):

```go
// accountID: id của account theo email, tìm qua danh sách account
func (a *app) accountID(token, email string) string {
	a.t.Helper()
	rec := a.do(call{method: "GET", path: "/api/v1/accounts?q=" + url.QueryEscape(email), token: token})
	var page struct {
		Items []struct{ ID, Email string } `json:"items"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &page) != nil {
		a.t.Fatalf("list accounts: %d %s", rec.Code, rec.Body)
	}
	for _, it := range page.Items {
		if strings.EqualFold(it.Email, email) {
			return it.ID
		}
	}
	a.t.Fatalf("account %s not found", email)
	return ""
}
```

Then check three things in `openapi.yaml` and match them, keeping the assertions as written:
- the field names of the account list response (`items`, `id`, `email`);
- the body of `PUT /accounts/{id}/roles` (`role_ids`, `version`);
- the request fields of `CreateRole` (`permissions`).

Add `net/url` to the imports.

- [ ] **Step 2: Run it to see it fail**

Run: `cd backend && CI=true go test -p 1 ./internal/identity/ -run TestRoleSoftDeleteAndRestore`
Expected: FAIL. `GET` of the deleted role returns 404, but the restore route returns 404 or 405 because it doesn't exist yet.

- [ ] **Step 3: Migration**

`backend/migrations/00006_soft_delete.sql`:

```sql
-- Xoá mềm cho role và profile export: hàng giữ lại để khôi phục (Undo). Tên chỉ cần không
-- trùng giữa các hàng chưa xoá, nên ràng buộc tên thành index một phần cùng tên cũ (ánh xạ
-- lỗi trong repository không đổi).

-- +goose Up
ALTER TABLE identity.roles ADD COLUMN deleted_at timestamptz;
ALTER TABLE identity.roles DROP CONSTRAINT roles_name_key;
CREATE UNIQUE INDEX roles_name_key ON identity.roles (name) WHERE deleted_at IS NULL;

ALTER TABLE inventory.export_profiles ADD COLUMN deleted_at timestamptz;
DROP INDEX inventory.export_profiles_owner_name;
CREATE UNIQUE INDEX export_profiles_owner_name ON inventory.export_profiles (owner_id, lower(name)) WHERE deleted_at IS NULL;

-- +goose Down
DELETE FROM inventory.export_profiles WHERE deleted_at IS NOT NULL;
DROP INDEX inventory.export_profiles_owner_name;
CREATE UNIQUE INDEX export_profiles_owner_name ON inventory.export_profiles (owner_id, lower(name));
ALTER TABLE inventory.export_profiles DROP COLUMN deleted_at;

DELETE FROM identity.roles WHERE deleted_at IS NOT NULL;
DROP INDEX identity.roles_name_key;
ALTER TABLE identity.roles ADD CONSTRAINT roles_name_key UNIQUE (name);
ALTER TABLE identity.roles DROP COLUMN deleted_at;
```

(`role_permissions` cascades on delete, so removing deleted roles in Down is safe.)

- [ ] **Step 4: Queries**

In `backend/internal/identity/repository/queries/roles.sql`:

```sql
-- name: ListRoles :many
SELECT * FROM identity.roles WHERE deleted_at IS NULL ORDER BY name;

-- name: GetRole :one
SELECT * FROM identity.roles WHERE id = @id AND deleted_at IS NULL;

-- name: GetRolesByIDs :many
SELECT * FROM identity.roles WHERE id = ANY(@ids::uuid[]) AND deleted_at IS NULL ORDER BY name;
```

Replace `DeleteRole`, and add the restore queries:

```sql
-- Xoá mềm; không xoá nếu còn account giữ role (service đã kiểm tra, đây là lớp chặn cuối)
-- name: DeleteRole :execrows
UPDATE identity.roles SET deleted_at = now(), updated_at = now()
WHERE id = @id AND deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM identity.account_roles WHERE role_id = @id);

-- Kể cả role đã xoá (khôi phục)
-- name: GetRoleAnyForUpdate :one
SELECT * FROM identity.roles WHERE id = @id FOR UPDATE;

-- name: RestoreRole :one
UPDATE identity.roles SET deleted_at = NULL, updated_at = now()
WHERE id = @id
RETURNING *;
```

Run: `cd backend && make sqlc`
Expected: `db.IdentityRole` gains `DeletedAt *time.Time`, and the new methods are generated.

- [ ] **Step 5: Event, repository, service, handler**

`contract/events.go`: add `EventRoleRestored = "identity.role_restored"` next to `EventRoleDeleted`, and:

```go
type RoleRestored struct {
	RoleID uuid.UUID `json:"role_id"`
	Name   string    `json:"name"`
}
```

`domain/repository.go`, in `RoleRepository`, after `Delete`:

```go
	// Restore: khôi phục role đã xoá (không xoá thì trả nguyên); ErrRoleNotFound,
	// ErrRoleNameTaken; event role_restored
	Restore(ctx context.Context, id uuid.UUID) (Role, error)
```

`repository/role_repository.go`: replace `Delete` and add `Restore`:

```go
func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := getRole(ctx, q, id)
		if err != nil {
			return err
		}
		n, err := q.DeleteRole(ctx, id)
		if err != nil {
			return fmt.Errorf("identity: delete role: %w", err)
		}
		if n == 0 {
			// vừa có account được gán role này
			return domain.ErrRoleInUse
		}
		return r.append(ctx, tx, contract.EventRoleDeleted, id, contract.RoleDeleted{RoleID: id, Name: cur.Name})
	})
}

func (r *RoleRepository) Restore(ctx context.Context, id uuid.UUID) (domain.Role, error) {
	var out domain.Role
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		row, err := q.GetRoleAnyForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRoleNotFound
		}
		if err != nil {
			return fmt.Errorf("identity: lock role: %w", err)
		}
		if row.DeletedAt != nil {
			if _, err := q.RestoreRole(ctx, id); err != nil {
				return mapRoleErr(err)
			}
			if err := r.append(ctx, tx, contract.EventRoleRestored, id, contract.RoleRestored{RoleID: id, Name: row.Name}); err != nil {
				return err
			}
		}
		out, err = getRole(ctx, q, id)
		return err
	})
	return out, err
}
```

Use the real name of the function at `role_repository.go:~219` that maps `roles_name_key` to `ErrRoleNameTaken` (here called `mapRoleErr`). `getRole` now filters deleted rows, which is correct after restore.

`service/roles.go`:

```go
// RestoreRole: hoàn tác xoá role
func (s *Service) RestoreRole(ctx context.Context, id uuid.UUID) (domain.Role, error) {
	if _, err := auth.Require(ctx, domain.PermRoleManage); err != nil {
		return domain.Role{}, err
	}
	return s.roles.Restore(ctx, id)
}
```

Check what `GetRole` in the service returns. If it returns a view with a member count, return the same kind of value here, and mirror the handler's `GetRole` mapping in the new handler.

`handler/openapi.yaml`, after the `/roles/{roleID}/permissions` path:

```yaml
  /roles/{roleID}/restore:
    parameters:
      - $ref: '#/components/parameters/RoleID'
    post:
      operationId: restoreRole
      summary: Undo deleting a role (identity.role.manage)
      responses:
        "200":
          description: Restored role
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Role' }
        default: { $ref: '#/components/responses/Problem' }
```

Use the schema `getRole`'s 200 response uses (check the name), and the module's error-response reference (check how `deleteRole` declares its default response).

Run: `cd backend && make generate`

`handler/roles.go`: implement `RestoreRole` by calling `h.svc.RestoreRole` and building the response exactly as `GetRole` does.

Service fakes: add a `Restore` method to the fake role repository in `service/fakes_test.go` so the package compiles. It should clear a "deleted" flag if the fake keeps one, otherwise return the role.

- [ ] **Step 6: Run the tests**

Run: `cd backend && CI=true go test -p 1 ./internal/identity/... && make check`
Expected: PASS, including `TestRoleSoftDeleteAndRestore` and the existing `DELETE /roles/<Employee>` 409 check in `TestAccountAndRoleManagement`.

- [ ] **Step 7: Commit**

```bash
git add backend/migrations/00006_soft_delete.sql backend/internal/identity
git commit -m "feat(identity): soft-delete roles and restore them

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(If `make sqlc` touched generated `models.go` in other modules, because they read the schema, stage those changed files too.)

---

### Task 2: Export profile soft delete and restore

**Files:**
- Modify:
  - `backend/internal/inventory/repository/queries/export_profiles.sql`
  - `backend/internal/inventory/repository/export_profile_repository.go`
  - `backend/internal/inventory/domain/export_profile.go` (interface)
  - `backend/internal/inventory/contract/events.go`
  - `backend/internal/inventory/service/export_profiles.go`
  - `backend/internal/inventory/service/fakes_test.go`
  - `backend/internal/inventory/handler/openapi.yaml`
  - `backend/internal/inventory/handler/export_profiles.go`
- Test:
  - `backend/internal/inventory/repository/export_profile_repository_db_test.go`
  - `backend/internal/inventory/http_db_test.go`

**Interfaces:**
- Consumes: column `inventory.export_profiles.deleted_at` (Task 1 migration).
- Produces:
  - `ExportProfileRepository.GetAny(ctx, id) (ExportProfile, error)`, which includes deleted rows (`ExportProfile.DeletedAt *time.Time` is added)
  - `ExportProfileRepository.Restore(ctx, id) (ExportProfile, error)`
  - `Service.RestoreExportProfile(ctx, id) (ExportProfileView, error)`
  - `POST /api/v1/export-profiles/{profileID}/restore`, which returns 200 with an `ExportProfile`
  - `contract.EventExportProfileRestored = "inventory.export_profile_restored"`, `contract.ExportProfileRestored{ProfileID uuid.UUID}`

- [ ] **Step 1: Write the failing tests**

Append to `http_db_test.go` (package `inventory_test`):

```go
// Xoá profile là xoá mềm: ẩn với mọi người (kể cả export theo profile_id), khôi phục được
// bởi người xoá được nó
func TestExportProfileRestoreOverHTTP(t *testing.T) {
	a := newApp(t)
	owner := a.token(domain.PermAssetRead, domain.PermAssetExport)
	other := a.token(domain.PermAssetRead, domain.PermAssetExport)
	layout := map[string]any{
		"columns": []map[string]any{{"field": "tag"}}, "sheets": "single", "sheet_name": "Assets",
		"title_row": false, "summary": false, "header": "bold", "freeze": true, "filter": true, "stripes": false,
		"date_format": "dd/mm/yyyy", "bool_style": "yes_no", "status_as": "name", "unit_in": "header",
	}
	name := "Kiểm kê " + uuid.NewString()[:6]
	p := decode[struct{ ID string }](t, a.do("POST", "/api/v1/export-profiles", owner, map[string]any{"name": name, "shared": true, "layout": layout}), 201)

	if rec := a.do("DELETE", "/api/v1/export-profiles/"+p.ID, owner, nil); rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("GET", "/api/v1/export-profiles/"+p.ID, other, nil); rec.Code != 404 {
		t.Errorf("deleted shared profile visible to others: %d", rec.Code)
	}
	if rec := a.do("POST", "/api/v1/assets/export", owner, map[string]any{"mode": "report", "profile_id": p.ID}); rec.Code != 404 {
		t.Errorf("export with a deleted profile: %d", rec.Code)
	} else if typ, _ := problem(t, rec); typ != "/errors/export-profile-not-found" {
		t.Errorf("problem type = %s", typ)
	}
	// không phải chủ, không có quyền quản lý: không khôi phục được profile chia sẻ của người khác
	if rec := a.do("POST", "/api/v1/export-profiles/"+p.ID+"/restore", other, nil); rec.Code != 403 {
		t.Errorf("non-manager restores someone else's shared profile: %d", rec.Code)
	}
	if rec := a.do("POST", "/api/v1/export-profiles/"+p.ID+"/restore", owner, nil); rec.Code != 200 {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("GET", "/api/v1/export-profiles/"+p.ID, other, nil); rec.Code != 200 {
		t.Errorf("restored shared profile: %d", rec.Code)
	}

	// profile riêng của người khác: 404 (không lộ là nó tồn tại)
	priv := decode[struct{ ID string }](t, a.do("POST", "/api/v1/export-profiles", owner, map[string]any{"name": "Riêng " + uuid.NewString()[:6], "layout": layout}), 201)
	a.do("DELETE", "/api/v1/export-profiles/"+priv.ID, owner, nil)
	if rec := a.do("POST", "/api/v1/export-profiles/"+priv.ID+"/restore", other, nil); rec.Code != 404 {
		t.Errorf("restore someone else's private profile: %d", rec.Code)
	}

	// tên dùng lại sau khi xoá: được; khôi phục bản cũ → 409
	a.do("DELETE", "/api/v1/export-profiles/"+p.ID, owner, nil)
	if rec := a.do("POST", "/api/v1/export-profiles", owner, map[string]any{"name": name, "layout": layout}); rec.Code != 201 {
		t.Fatalf("reuse name: %d %s", rec.Code, rec.Body)
	}
	rec := a.do("POST", "/api/v1/export-profiles/"+p.ID+"/restore", owner, nil)
	if rec.Code != 409 {
		t.Errorf("restore over reused name: %d", rec.Code)
	} else if typ, _ := problem(t, rec); typ != "/errors/export-profile-name-taken" {
		t.Errorf("problem type = %s", typ)
	}
}
```

In `export_profile_repository_db_test.go`, `TestExportProfiles_CRUD` asserts that `Get` after `Delete` returns `ErrExportProfileNotFound`. Keep that, and append:

```go
	// xoá mềm: GetAny vẫn thấy, Restore đưa về
	got, err := r.profiles.GetAny(context.Background(), p.ID)
	if err != nil || got.DeletedAt == nil {
		t.Errorf("GetAny after delete = %+v, %v", got, err)
	}
	back, err := r.profiles.Restore(ctx, p.ID)
	if err != nil || back.DeletedAt != nil {
		t.Fatalf("restore = %+v, %v", back, err)
	}
	if countEvents(t, r, contract.EventExportProfileRestored, p.ID) != 1 {
		t.Error("want one export_profile_restored event")
	}
```

The existing test checks that each event type has exactly one row. Keep that, and don't delete the profile again after this block.

- [ ] **Step 2: Run them to see them fail**

Run: `cd backend && go vet ./internal/inventory/...`
Expected: FAIL (`GetAny`, `Restore` and `EventExportProfileRestored` are undefined).

- [ ] **Step 3: Queries**

In `queries/export_profiles.sql`, add `deleted_at IS NULL` to `GetExportProfile`, `GetExportProfileForUpdate` and `UpdateExportProfile`. For `ListExportProfiles` use `WHERE (owner_id = @owner_id OR shared) AND deleted_at IS NULL`. Then replace the delete and add:

```sql
-- Xoá mềm
-- name: DeleteExportProfile :execrows
UPDATE inventory.export_profiles SET deleted_at = now(), updated_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- Kể cả profile đã xoá (khôi phục, kiểm tra quyền khôi phục)
-- name: GetExportProfileAny :one
SELECT * FROM inventory.export_profiles WHERE id = @id;

-- name: GetExportProfileAnyForUpdate :one
SELECT * FROM inventory.export_profiles WHERE id = @id FOR UPDATE;

-- name: RestoreExportProfile :one
UPDATE inventory.export_profiles SET deleted_at = NULL, updated_at = now()
WHERE id = @id
RETURNING *;
```

Run: `cd backend && make sqlc`

- [ ] **Step 4: Domain, events, repository**

`domain/export_profile.go`:
- add `DeletedAt *time.Time` to `ExportProfile`;
- add these to the interface:

```go
	// GetAny: kể cả đã xoá (để kiểm tra quyền khôi phục); ErrExportProfileNotFound
	GetAny(ctx context.Context, id uuid.UUID) (ExportProfile, error)
	// Restore: khôi phục profile đã xoá (chưa xoá thì trả nguyên); ErrExportProfileNotFound,
	// ErrExportProfileNameTaken; event export_profile_restored
	Restore(ctx context.Context, id uuid.UUID) (ExportProfile, error)
```

`contract/events.go`: add `EventExportProfileRestored = "inventory.export_profile_restored"` and `type ExportProfileRestored struct { ProfileID uuid.UUID `json:"profile_id"` }`.

`repository/export_profile_repository.go`:
- set `DeletedAt: row.DeletedAt` in `toExportProfile`;
- add:

```go
func (r *ExportProfileRepository) GetAny(ctx context.Context, id uuid.UUID) (domain.ExportProfile, error) {
	return profileOrNotFound(r.q.GetExportProfileAny(ctx, id))
}

func (r *ExportProfileRepository) Restore(ctx context.Context, id uuid.UUID) (domain.ExportProfile, error) {
	var out domain.ExportProfile
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := profileOrNotFound(q.GetExportProfileAnyForUpdate(ctx, id))
		if err != nil {
			return err
		}
		if cur.DeletedAt == nil {
			out = cur
			return nil
		}
		row, err := q.RestoreExportProfile(ctx, id)
		if err != nil {
			return mapWriteErr(err, "restore export profile")
		}
		if out, err = toExportProfile(row); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventExportProfileRestored, contract.AggregateExportProfile, id,
			contract.ExportProfileRestored{ProfileID: id})
	})
	return out, err
}
```

`mapWriteErr` already maps `export_profiles_owner_name` to `ErrExportProfileNameTaken`. The partial index keeps the same name.

- [ ] **Step 5: Service, handler**

`service/export_profiles.go`:

```go
// RestoreExportProfile: hoàn tác xoá. Ai xoá được thì khôi phục được; profile riêng của
// người khác (kể cả đã xoá) như không tồn tại
func (s *Service) RestoreExportProfile(ctx context.Context, id uuid.UUID) (ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportProfileView{}, err
	}
	cur, err := s.profiles.GetAny(ctx, id)
	if err != nil {
		return ExportProfileView{}, err
	}
	if cur.OwnerID != actor.AccountID && !cur.Shared {
		return ExportProfileView{}, domain.ErrExportProfileNotFound
	}
	if !canEditProfile(actor, cur) {
		return ExportProfileView{}, domain.ErrExportProfileForbidden
	}
	p, err := s.profiles.Restore(ctx, id)
	if err != nil {
		return ExportProfileView{}, err
	}
	return s.profileView(ctx, actor, p)
}
```

In `service/fakes_test.go`, add `GetAny` and `Restore` to `fakeProfiles`. Make the fake's `Delete` mark the profile deleted (set `DeletedAt`) instead of removing it, and filter deleted profiles in `Get` and `List`. Existing service tests must still pass.

`handler/openapi.yaml`, after `/export-profiles/{profileID}`:

```yaml
  /export-profiles/{profileID}/restore:
    parameters:
      - name: profileID
        in: path
        required: true
        schema: { $ref: '../../../api/common.yaml#/components/schemas/ID' }
    post:
      operationId: restoreExportProfile
      summary: Undo deleting a profile (owner; shared ones also inventory.export_profile.manage)
      responses:
        "200":
          description: Restored
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ExportProfile' }
        default: { $ref: '#/components/responses/Problem' }
```

(Copy the exact `profileID` parameter and default-response forms from the existing `/export-profiles/{profileID}` path.)

Run: `cd backend && make generate`

`handler/export_profiles.go`:

```go
func (h *Handler) RestoreExportProfile(ctx context.Context, req api.RestoreExportProfileRequestObject) (api.RestoreExportProfileResponseObject, error) {
	p, err := h.svc.RestoreExportProfile(ctx, req.ProfileID)
	if err != nil {
		return nil, err
	}
	return api.RestoreExportProfile200JSONResponse(toAPIProfile(p)), nil
}
```

- [ ] **Step 6: Run the tests**

Run: `cd backend && CI=true go test -p 1 ./internal/inventory/... && make check`
Expected: PASS, including `TestExportProfileRestoreOverHTTP`, `TestExportProfiles_CRUD` and every existing export test.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/inventory
git commit -m "feat(inventory): soft-delete export profiles and restore them

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Restore attributes and options

**Files:**
- Modify:
  - `backend/internal/inventory/repository/queries/asset_types.sql`
  - `backend/internal/inventory/repository/type_repository.go`
  - `backend/internal/inventory/domain/repository.go` (`TypeRepository`)
  - `backend/internal/inventory/service/asset_types.go`
  - `backend/internal/inventory/service/fakes_test.go` (fake types)
  - `backend/internal/inventory/handler/openapi.yaml`
  - `backend/internal/inventory/handler/asset_types.go`
- Test: `backend/internal/inventory/http_db_test.go`

**Interfaces:**
- Produces:
  - `TypeRepository.RestoreAttribute(ctx, typeID, attrID uuid.UUID) (domain.Attribute, error)`
  - `TypeRepository.RestoreOption(ctx, typeID, attrID, optID uuid.UUID) (domain.Option, error)`
  - service methods of the same names
  - `POST /api/v1/asset-types/{typeID}/attributes/{attributeID}/restore`, which returns 200 with an `Attribute`
  - `POST /api/v1/asset-types/{typeID}/attributes/{attributeID}/options/{optionID}/restore`, which returns 200 with an `Option`

- [ ] **Step 1: Write the failing test**

Append to `http_db_test.go`:

```go
// Bỏ thuộc tính / option đã là xoá mềm; khôi phục đưa lại, trùng nhãn với cái đang dùng thì 409
func TestRestoreAttributeAndOption(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "RS" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Restore " + code,
		"attributes": []map[string]any{
			{"key": "ram", "label": "RAM", "data_type": "number", "position": 1},
			{"key": "os", "label": "OS", "data_type": "select", "position": 2, "options": []string{"Windows", "macOS"}},
		},
	}), 201)
	base := "/api/v1/asset-types/" + typ.ID + "/attributes/"
	ram, osAttr := typ.attr("ram"), typ.attr("os")

	if rec := a.do("DELETE", base+ram.ID, tok, nil); rec.Code != 204 {
		t.Fatalf("remove attribute: %d %s", rec.Code, rec.Body)
	}
	rec := a.do("POST", base+ram.ID+"/restore", tok, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"key":"ram"`) {
		t.Fatalf("restore attribute: %d %s", rec.Code, rec.Body)
	}
	// khôi phục cái chưa bỏ: không đổi gì
	if rec := a.do("POST", base+ram.ID+"/restore", tok, nil); rec.Code != 200 {
		t.Errorf("restore active attribute: %d", rec.Code)
	}

	// bỏ, thêm thuộc tính mới cùng nhãn "RAM" (khoá khác), khôi phục cái cũ → 409
	a.do("DELETE", base+ram.ID, tok, nil)
	if rec := a.do("POST", "/api/v1/asset-types/"+typ.ID+"/attributes", tok, map[string]any{"key": "ram2", "label": "RAM", "data_type": "number", "position": 3}); rec.Code != 201 {
		t.Fatalf("add attribute with the removed label: %d %s", rec.Code, rec.Body)
	}
	rec = a.do("POST", base+ram.ID+"/restore", tok, nil)
	if rec.Code != 409 {
		t.Errorf("restore over a reused label: %d", rec.Code)
	} else if typ, _ := problem(t, rec); typ != "/errors/attribute-label-taken" {
		t.Errorf("problem type = %s", typ)
	}

	win := osAttr.option("Windows")
	optBase := base + osAttr.ID + "/options/"
	if rec := a.do("DELETE", optBase+win, tok, nil); rec.Code != 204 {
		t.Fatalf("remove option: %d", rec.Code)
	}
	if rec := a.do("POST", optBase+win+"/restore", tok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Windows") {
		t.Errorf("restore option: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("POST", optBase+uuid.NewString()+"/restore", tok, nil); rec.Code != 404 {
		t.Errorf("restore unknown option: %d", rec.Code)
	}
	reader := a.token(domain.PermAssetRead)
	if rec := a.do("POST", base+ram.ID+"/restore", reader, nil); rec.Code != 403 {
		t.Errorf("restore without type manage: %d", rec.Code)
	}
}
```

`typeDetail` exists in the test file. If it has no `attr(key)` or `option(label)` helpers, add small ones that look up attribute and option IDs from the decoded JSON (`attributes[].{id,key,options[].{id,label}}`). Check the field names in the file and adapt.

- [ ] **Step 2: Run it to see it fail**

Run: `cd backend && CI=true go test -p 1 ./internal/inventory/ -run TestRestoreAttributeAndOption`
Expected: FAIL. The restore route is missing (404 or 405).

- [ ] **Step 3: Queries**

In `queries/asset_types.sql`:

```sql
-- name: RestoreAttribute :one
UPDATE inventory.asset_type_attributes SET removed_at = NULL, updated_at = now()
WHERE id = @id AND asset_type_id = @asset_type_id
RETURNING *;

-- name: RestoreOption :one
UPDATE inventory.asset_attribute_options SET removed_at = NULL, updated_at = now()
WHERE id = @id AND attribute_id = @attribute_id
RETURNING *;
```

Run: `cd backend && make sqlc`

- [ ] **Step 4: Repository, domain, service**

`domain/repository.go`, in `TypeRepository` after `RemoveAttribute` and `RemoveOption` respectively:

```go
	// RestoreAttribute: hoàn tác bỏ thuộc tính (chưa bỏ thì trả nguyên); ErrAttributeNotFound,
	// ErrAttributeLabelTaken
	RestoreAttribute(ctx context.Context, typeID, attrID uuid.UUID) (Attribute, error)
	// RestoreOption: hoàn tác bỏ option; thuộc tính phải còn dùng; ErrOptionNotFound,
	// ErrOptionLabelTaken
	RestoreOption(ctx context.Context, typeID, attrID, optID uuid.UUID) (Option, error)
```

`repository/type_repository.go`:

```go
func (r *TypeRepository) RestoreAttribute(ctx context.Context, typeID, attrID uuid.UUID) (domain.Attribute, error) {
	var out domain.Attribute
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		row, err := q.GetAttributeForUpdate(ctx, db.GetAttributeForUpdateParams{ID: attrID, AssetTypeID: typeID})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrAttributeNotFound
		}
		if err != nil {
			return fmt.Errorf("inventory: lock attribute: %w", err)
		}
		if row.RemovedAt == nil {
			out = toAttribute(row)
			return nil
		}
		restored, err := q.RestoreAttribute(ctx, db.RestoreAttributeParams{ID: attrID, AssetTypeID: typeID})
		if err != nil {
			return mapWriteErr(err, "restore attribute")
		}
		out = toAttribute(restored)
		return r.updated(ctx, tx, typeID, []contract.FieldChange{change("attributes."+row.Key, "removed", "active")})
	})
	if err != nil {
		return domain.Attribute{}, err
	}
	// kèm option như các chỗ trả thuộc tính khác
	return r.attributeWithOptions(ctx, typeID, out.ID)
}

func (r *TypeRepository) RestoreOption(ctx context.Context, typeID, attrID, optID uuid.UUID) (domain.Option, error) {
	var out domain.Option
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		a, err := lockAttribute(ctx, q, typeID, attrID)
		if err != nil {
			return err
		}
		row, err := q.GetOptionForUpdate(ctx, db.GetOptionForUpdateParams{ID: optID, AttributeID: attrID})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrOptionNotFound
		}
		if err != nil {
			return fmt.Errorf("inventory: lock option: %w", err)
		}
		if row.RemovedAt == nil {
			out = toOption(row)
			return nil
		}
		restored, err := q.RestoreOption(ctx, db.RestoreOptionParams{ID: optID, AttributeID: attrID})
		if err != nil {
			return mapWriteErr(err, "restore option")
		}
		out = toOption(restored)
		return r.updated(ctx, tx, typeID, []contract.FieldChange{change("attributes."+a.Key+".options."+row.Label, "removed", "active")})
	})
	return out, err
}
```

Check how `UpdateAttribute` in the repository builds the `domain.Attribute` it returns (with options). Reuse that same helper in place of `attributeWithOptions`; that name is a placeholder for whatever the file uses. If the attribute is returned without options elsewhere, return `out` directly.

`service/asset_types.go`:

```go
// RestoreAttribute, RestoreOption: hoàn tác bỏ thuộc tính / option
func (s *Service) RestoreAttribute(ctx context.Context, typeID, attrID uuid.UUID) (domain.Attribute, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.Attribute{}, err
	}
	return s.types.RestoreAttribute(ctx, typeID, attrID)
}

func (s *Service) RestoreOption(ctx context.Context, typeID, attrID, optID uuid.UUID) (domain.Option, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.Option{}, err
	}
	return s.types.RestoreOption(ctx, typeID, attrID, optID)
}
```

Add `RestoreAttribute` and `RestoreOption` to the fake types repository in `service/fakes_test.go`, clearing `RemovedAt` on the stored attribute or option. Add both methods to the `TestPermissionChecks` table with `domain.PermTypeManage`, following the pattern of the existing rows.

- [ ] **Step 5: API**

`handler/openapi.yaml`, after `/asset-types/{typeID}/attributes/{attributeID}` and after `.../options/{optionID}` respectively. Copy each path's `parameters` block from its sibling path:

```yaml
  /asset-types/{typeID}/attributes/{attributeID}/restore:
    parameters: # same three path parameters as /asset-types/{typeID}/attributes/{attributeID}
    post:
      operationId: restoreAttribute
      summary: Undo removing an attribute (inventory.type.manage)
      responses:
        "200":
          description: Restored attribute
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Attribute' }
        default: { $ref: '#/components/responses/Problem' }

  /asset-types/{typeID}/attributes/{attributeID}/options/{optionID}/restore:
    parameters: # same as /asset-types/{typeID}/attributes/{attributeID}/options/{optionID}
    post:
      operationId: restoreOption
      summary: Undo removing an option (inventory.type.manage)
      responses:
        "200":
          description: Restored option
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Option' }
        default: { $ref: '#/components/responses/Problem' }
```

Run: `cd backend && make generate`

`handler/asset_types.go`: implement `RestoreAttribute` and `RestoreOption`. Map the results with the same converters `UpdateAttribute` and `UpdateOption` use (`toAPIAttribute` / `toAPIOption`; check the names).

- [ ] **Step 6: Run the tests**

Run: `cd backend && CI=true go test -p 1 ./internal/inventory/... && make check`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/inventory
git commit -m "feat(inventory): restore removed attributes and options

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Frontend API types and docs

**Files:**
- Modify:
  - `frontend/src/lib/api/identity.d.ts`, `frontend/src/lib/api/inventory.d.ts` (generated)
  - `backend/docs/identity.md`, `backend/docs/inventory.md`

- [ ] **Step 1: Regenerate the frontend types**

Run: `cd frontend && npm run gen:api && npx vue-tsc --noEmit -p tsconfig.app.json`
Expected: the `.d.ts` files gain `/roles/{roleID}/restore`, `/export-profiles/{profileID}/restore` and the two attribute and option restore paths; the type-check passes.

- [ ] **Step 2: Docs**

- **`identity.md`:** in the roles section, say:
  - deleting a role is a soft delete (`deleted_at`) and is allowed only for custom roles nobody holds;
  - deleted roles are hidden and can't be assigned;
  - `POST /roles/{roleID}/restore` (`identity.role.manage`) undoes it, with 409 `role-name-taken` if the name was reused;
  - `identity.role_restored`.
- **`inventory.md`:**
  - In the asset types section, add the two restore endpoints (`inventory.type.manage`), the label conflict (409), and that the change is recorded on `asset_type_updated` as `removed → active`.
  - In the export section, say deleting a profile is a soft delete, deleted profiles act as not found everywhere, and `POST /export-profiles/{profileID}/restore` is allowed for whoever could delete it, with 409 `export-profile-name-taken` and `inventory.export_profile_restored`.
  - Update the endpoint tables and event lists.

- [ ] **Step 3: Full check and commit**

Run: `cd backend && make check && CI=true go test -p 1 ./... && cd ../frontend && npm run check`
Expected: all pass.

```bash
git add frontend/src/lib/api/identity.d.ts frontend/src/lib/api/inventory.d.ts backend/docs/identity.md backend/docs/inventory.md
git commit -m "docs: soft delete and restore endpoints; regenerate API types

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(The dev database needs migration `00006`. Ask the user before running `docker compose run --rm migrate`.)

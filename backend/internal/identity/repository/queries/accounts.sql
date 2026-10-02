-- name: CreateAccount :one
INSERT INTO identity.accounts (id, email, name, password_hash, member_id)
VALUES (@id, @email, @name, @password_hash, sqlc.narg('member_id'))
RETURNING *;

-- name: GetAccount :one
SELECT * FROM identity.accounts WHERE id = @id;

-- Khoá hàng để đổi trạng thái/mật khẩu/role không chen nhau. NO KEY UPDATE:
-- không bao giờ đổi khoá chính, nên không chặn insert tham chiếu account (FK
-- lấy KEY SHARE), vd đăng nhập tạo refresh_families
-- name: GetAccountForUpdate :one
SELECT * FROM identity.accounts WHERE id = @id FOR NO KEY UPDATE;

-- Khoá hàng role Administrator: mọi thao tác có thể làm mất một admin (khoá
-- account, đổi role) chạy lần lượt, để kiểm tra "còn admin" không bị hai
-- transaction cùng lọt. Luôn khoá trước hàng account.
-- name: LockRole :one
SELECT id FROM identity.roles WHERE id = @id FOR NO KEY UPDATE;

-- name: CountActiveAccountsWithRole :one
SELECT count(*) FROM identity.accounts a
JOIN identity.account_roles ar ON ar.account_id = a.id
WHERE ar.role_id = @role_id AND a.active;

-- name: GetAccountByEmail :one
SELECT * FROM identity.accounts WHERE lower(email) = lower(@email);

-- name: GetAccountsByIDs :many
SELECT * FROM identity.accounts WHERE id = ANY(@ids::uuid[]) ORDER BY name, id;

-- name: ListAccounts :many
SELECT * FROM identity.accounts
WHERE (sqlc.narg('q')::text IS NULL
       OR name ILIKE '%' || sqlc.narg('q')::text || '%'
       OR email ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('active')::boolean IS NULL OR active = sqlc.narg('active')::boolean)
ORDER BY name, id
LIMIT @lim OFFSET @off;

-- name: CountAccounts :one
SELECT count(*) FROM identity.accounts
WHERE (sqlc.narg('q')::text IS NULL
       OR name ILIKE '%' || sqlc.narg('q')::text || '%'
       OR email ILIKE '%' || sqlc.narg('q')::text || '%')
  AND (sqlc.narg('active')::boolean IS NULL OR active = sqlc.narg('active')::boolean);

-- name: CountAllAccounts :one
SELECT count(*) FROM identity.accounts;

-- Optimistic locking: 0 hàng nghĩa là version đã đổi (hoặc không có account)
-- name: UpdateAccountProfile :one
UPDATE identity.accounts
SET name = @name, member_id = sqlc.narg('member_id'), version = version + 1, updated_at = now()
WHERE id = @id AND version = @version
RETURNING *;

-- name: SetAccountActive :one
UPDATE identity.accounts
SET active = @active, version = version + 1, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: SetAccountPassword :exec
UPDATE identity.accounts SET password_hash = @password_hash, updated_at = now() WHERE id = @id;

-- name: AccountPermissions :many
SELECT DISTINCT rp.permission
FROM identity.account_roles ar
JOIN identity.role_permissions rp ON rp.role_id = ar.role_id
WHERE ar.account_id = @account_id
ORDER BY rp.permission;

-- name: AccountRoleIDs :many
SELECT role_id FROM identity.account_roles WHERE account_id = @account_id;

-- name: DeleteAccountRoles :exec
DELETE FROM identity.account_roles WHERE account_id = @account_id;

-- name: InsertAccountRole :exec
INSERT INTO identity.account_roles (account_id, role_id) VALUES (@account_id, @role_id);

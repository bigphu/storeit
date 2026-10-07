-- Role đã xoá (xoá mềm) không hiện ở đâu, trừ khi khôi phục
-- name: ListRoles :many
SELECT * FROM identity.roles WHERE deleted_at IS NULL ORDER BY name;

-- name: GetRole :one
SELECT * FROM identity.roles WHERE id = @id AND deleted_at IS NULL;

-- name: GetRolesByIDs :many
SELECT * FROM identity.roles WHERE id = ANY(@ids::uuid[]) AND deleted_at IS NULL ORDER BY name;

-- name: CreateRole :one
INSERT INTO identity.roles (id, name, description) VALUES (@id, @name, @description)
RETURNING *;

-- name: UpdateRole :one
UPDATE identity.roles SET name = @name, description = @description, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: TouchRole :exec
UPDATE identity.roles SET updated_at = now() WHERE id = @id;

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

-- name: RolePermissions :many
SELECT permission FROM identity.role_permissions WHERE role_id = @role_id ORDER BY permission;

-- Quyền của nhiều role một lần, để dựng danh sách role không N+1
-- name: RolesPermissions :many
SELECT role_id, permission FROM identity.role_permissions
WHERE role_id = ANY(@role_ids::uuid[])
ORDER BY role_id, permission;

-- name: DeleteRolePermissions :exec
DELETE FROM identity.role_permissions WHERE role_id = @role_id;

-- name: InsertRolePermission :exec
INSERT INTO identity.role_permissions (role_id, permission) VALUES (@role_id, @permission);

-- name: CountRoleAssignments :one
SELECT count(*) FROM identity.account_roles WHERE role_id = @role_id;

-- name: ListPermissions :many
SELECT * FROM identity.permissions ORDER BY code;

-- Số account chưa bị khoá (kể cả đang được mời) giữ mỗi role (trang vai trò)
-- name: CountActiveMembersByRole :many
SELECT ar.role_id, count(*)::bigint AS n
FROM identity.account_roles ar
JOIN identity.accounts a ON a.id = ar.account_id
WHERE a.active
GROUP BY ar.role_id;

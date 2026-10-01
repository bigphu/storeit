-- name: ListRoles :many
SELECT * FROM identity.roles ORDER BY name;

-- name: GetRole :one
SELECT * FROM identity.roles WHERE id = @id;

-- name: GetRolesByIDs :many
SELECT * FROM identity.roles WHERE id = ANY(@ids::uuid[]) ORDER BY name;

-- name: CreateRole :one
INSERT INTO identity.roles (id, name, description) VALUES (@id, @name, @description)
RETURNING *;

-- name: UpdateRole :one
UPDATE identity.roles SET name = @name, description = @description, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: TouchRole :exec
UPDATE identity.roles SET updated_at = now() WHERE id = @id;

-- name: DeleteRole :execrows
DELETE FROM identity.roles WHERE id = @id;

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

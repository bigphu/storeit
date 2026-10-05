-- name: ListStatuses :many
SELECT * FROM inventory.asset_statuses
WHERE @include_archived::boolean OR archived_at IS NULL
ORDER BY position, lower(name), id;

-- Khoá mọi status đang dùng khi đổi thứ tự
-- name: ListActiveStatusesForUpdate :many
SELECT * FROM inventory.asset_statuses
WHERE archived_at IS NULL
ORDER BY position, lower(name), id
FOR NO KEY UPDATE;

-- name: SetStatusPosition :exec
UPDATE inventory.asset_statuses SET position = @position, updated_at = now()
WHERE id = @id;

-- name: GetStatus :one
SELECT * FROM inventory.asset_statuses WHERE id = @id;

-- name: GetStatusForUpdate :one
SELECT * FROM inventory.asset_statuses WHERE id = @id FOR NO KEY UPDATE;

-- name: GetDefaultStatus :one
SELECT * FROM inventory.asset_statuses WHERE kind = @kind AND is_default;

-- name: CreateStatus :one
INSERT INTO inventory.asset_statuses (id, name, kind, position)
VALUES (@id, @name, @kind, @position)
RETURNING *;

-- name: UpdateStatus :one
UPDATE inventory.asset_statuses
SET name = @name, position = @position, updated_at = now()
WHERE id = @id
RETURNING *;

-- Chuyển cờ mặc định: bỏ cờ của kind trước, rồi đặt cho status mới (cùng tx)
-- name: ClearDefaultStatus :exec
UPDATE inventory.asset_statuses SET is_default = false, updated_at = now()
WHERE kind = @kind AND is_default;

-- name: SetDefaultStatus :one
UPDATE inventory.asset_statuses SET is_default = true, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: ArchiveStatus :one
UPDATE inventory.asset_statuses SET archived_at = now(), updated_at = now()
WHERE id = @id
RETURNING *;

-- name: RestoreStatus :one
UPDATE inventory.asset_statuses SET archived_at = NULL, updated_at = now()
WHERE id = @id
RETURNING *;

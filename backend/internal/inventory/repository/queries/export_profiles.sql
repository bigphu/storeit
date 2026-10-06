-- name: CreateExportProfile :one
INSERT INTO inventory.export_profiles (id, owner_id, name, shared, layout)
VALUES (@id, @owner_id, @name, @shared, @layout)
RETURNING *;

-- name: GetExportProfile :one
SELECT * FROM inventory.export_profiles WHERE id = @id;

-- name: GetExportProfileForUpdate :one
SELECT * FROM inventory.export_profiles WHERE id = @id FOR UPDATE;

-- Của mình và mọi profile được chia sẻ
-- name: ListExportProfiles :many
SELECT * FROM inventory.export_profiles
WHERE owner_id = @owner_id OR shared
ORDER BY lower(name), id;

-- Optimistic locking: 0 hàng là version đã đổi
-- name: UpdateExportProfile :one
UPDATE inventory.export_profiles
SET name = @name, shared = @shared, layout = @layout, version = version + 1, updated_at = now()
WHERE id = @id AND version = @version
RETURNING *;

-- name: DeleteExportProfile :execrows
DELETE FROM inventory.export_profiles WHERE id = @id;

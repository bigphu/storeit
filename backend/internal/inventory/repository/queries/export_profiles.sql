-- name: CreateExportProfile :one
INSERT INTO inventory.export_profiles (id, owner_id, name, shared, layout)
VALUES (@id, @owner_id, @name, @shared, @layout)
RETURNING *;

-- name: GetExportProfile :one
SELECT * FROM inventory.export_profiles WHERE id = @id AND deleted_at IS NULL;

-- name: GetExportProfileForUpdate :one
SELECT * FROM inventory.export_profiles WHERE id = @id AND deleted_at IS NULL FOR UPDATE;

-- Của mình và mọi profile được chia sẻ
-- name: ListExportProfiles :many
SELECT * FROM inventory.export_profiles
WHERE (owner_id = @owner_id OR shared) AND deleted_at IS NULL
ORDER BY lower(name), id;

-- Optimistic locking: 0 hàng là version đã đổi
-- name: UpdateExportProfile :one
UPDATE inventory.export_profiles
SET name = @name, shared = @shared, layout = @layout, version = version + 1, updated_at = now()
WHERE id = @id AND version = @version AND deleted_at IS NULL
RETURNING *;

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

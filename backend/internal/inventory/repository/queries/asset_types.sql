-- name: CreateAssetType :one
INSERT INTO inventory.asset_types (id, code, name, description)
VALUES (@id, @code, @name, @description)
RETURNING *;

-- name: GetAssetType :one
SELECT * FROM inventory.asset_types WHERE id = @id;

-- name: GetAssetTypeForUpdate :one
SELECT * FROM inventory.asset_types WHERE id = @id FOR NO KEY UPDATE;

-- Loại hệ thống (GENERAL) đứng đầu, rồi theo tên
-- name: ListAssetTypes :many
SELECT * FROM inventory.asset_types
WHERE @include_archived::boolean OR archived_at IS NULL
ORDER BY is_system DESC, lower(name), id;

-- Optimistic locking: 0 hàng là version đã đổi
-- name: UpdateAssetType :one
UPDATE inventory.asset_types
SET name = @name, description = @description, version = version + 1, updated_at = now()
WHERE id = @id AND version = @version
RETURNING *;

-- name: SetAssetTypeArchived :one
UPDATE inventory.asset_types
SET archived_at = CASE WHEN @archived::boolean THEN coalesce(archived_at, now()) END,
    version = version + 1, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: CreateAttribute :one
INSERT INTO inventory.asset_type_attributes (id, asset_type_id, key, label, data_type, unit, is_required, position)
VALUES (@id, @asset_type_id, @key, @label, @data_type, sqlc.narg('unit'), @is_required, @position)
RETURNING *;

-- Mọi thuộc tính của loại, kể cả đã gỡ, theo thứ tự hiển thị
-- name: ListAttributes :many
SELECT * FROM inventory.asset_type_attributes
WHERE asset_type_id = @asset_type_id
ORDER BY position, lower(label), id;

-- Nhãn thuộc tính đang dùng của mọi loại, theo thứ tự hiển thị (thẻ ở trang loại)
-- name: ListActiveAttributeLabels :many
SELECT asset_type_id, label FROM inventory.asset_type_attributes
WHERE removed_at IS NULL
ORDER BY asset_type_id, position, lower(label), id;

-- name: GetAttributeForUpdate :one
SELECT * FROM inventory.asset_type_attributes
WHERE id = @id AND asset_type_id = @asset_type_id
FOR NO KEY UPDATE;

-- name: UpdateAttribute :one
UPDATE inventory.asset_type_attributes
SET label = @label, unit = sqlc.narg('unit'), data_type = @data_type,
    is_required = @is_required, position = @position, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: RemoveAttribute :execrows
UPDATE inventory.asset_type_attributes SET removed_at = now(), updated_at = now()
WHERE id = @id AND asset_type_id = @asset_type_id AND removed_at IS NULL;

-- Khôi phục về cuối danh sách: vị trí cũ có thể đã thuộc về thuộc tính thêm sau khi xoá
-- name: RestoreAttribute :one
UPDATE inventory.asset_type_attributes AS t SET removed_at = NULL, updated_at = now(),
    position = (SELECT COALESCE(MAX(a.position) + 1, 0) FROM inventory.asset_type_attributes a
                WHERE a.asset_type_id = @asset_type_id AND a.removed_at IS NULL)
WHERE t.id = @id AND t.asset_type_id = @asset_type_id
RETURNING t.*;

-- name: AttributeHasValues :one
SELECT EXISTS (SELECT 1 FROM inventory.asset_attribute_values WHERE attribute_id = @attribute_id);

-- name: DeleteAttributeOptions :exec
DELETE FROM inventory.asset_attribute_options WHERE attribute_id = @attribute_id;

-- name: CreateOption :one
INSERT INTO inventory.asset_attribute_options (id, attribute_id, label, position)
VALUES (@id, @attribute_id, @label, @position)
RETURNING *;

-- name: ListOptionsForType :many
SELECT o.* FROM inventory.asset_attribute_options o
JOIN inventory.asset_type_attributes a ON a.id = o.attribute_id
WHERE a.asset_type_id = @asset_type_id
ORDER BY o.position, lower(o.label), o.id;

-- name: GetOptionForUpdate :one
SELECT * FROM inventory.asset_attribute_options
WHERE id = @id AND attribute_id = @attribute_id
FOR NO KEY UPDATE;

-- Sắp xếp lại (kéo thả): chỉ đổi vị trí
-- name: SetAttributePosition :exec
UPDATE inventory.asset_type_attributes SET position = @position, updated_at = now() WHERE id = @id;

-- name: SetOptionPosition :exec
UPDATE inventory.asset_attribute_options SET position = @position, updated_at = now() WHERE id = @id;

-- name: UpdateOption :one
UPDATE inventory.asset_attribute_options
SET label = @label, position = @position, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: RemoveOption :execrows
UPDATE inventory.asset_attribute_options SET removed_at = now(), updated_at = now()
WHERE id = @id AND attribute_id = @attribute_id AND removed_at IS NULL;

-- Khôi phục về cuối danh sách, như thuộc tính
-- name: RestoreOption :one
UPDATE inventory.asset_attribute_options AS t SET removed_at = NULL, updated_at = now(),
    position = (SELECT COALESCE(MAX(o.position) + 1, 0) FROM inventory.asset_attribute_options o
                WHERE o.attribute_id = @attribute_id AND o.removed_at IS NULL)
WHERE t.id = @id AND t.attribute_id = @attribute_id
RETURNING t.*;

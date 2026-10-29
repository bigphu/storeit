-- name: InsertReplacement :exec
INSERT INTO inventory.asset_replacements (old_asset_id, new_asset_id, replaced_on, reason, created_by)
VALUES (@old_asset_id, @new_asset_id, @replaced_on, @reason, sqlc.narg('created_by'));

-- Liên kết của tài sản cũ, kèm tag và tên tài sản thay nó
-- name: GetReplacementByOld :one
SELECT r.old_asset_id, r.new_asset_id, r.replaced_on, r.reason, a.tag AS new_tag, a.name AS new_name
FROM inventory.asset_replacements r
JOIN inventory.assets a ON a.id = r.new_asset_id
WHERE r.old_asset_id = @old_asset_id;

-- Các tài sản mà tài sản mới đã thay, mới nhất trước
-- name: ListReplacementsByNew :many
SELECT r.old_asset_id, r.replaced_on, r.reason, a.tag AS old_tag, a.name AS old_name
FROM inventory.asset_replacements r
JOIN inventory.assets a ON a.id = r.old_asset_id
WHERE r.new_asset_id = @new_asset_id
ORDER BY r.replaced_on DESC, r.created_at DESC;

-- name: DeleteReplacement :exec
DELETE FROM inventory.asset_replacements WHERE old_asset_id = @old_asset_id;

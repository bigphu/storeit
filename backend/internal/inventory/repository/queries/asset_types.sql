-- name: GetAssetType :one
SELECT * FROM inventory.asset_types WHERE id = @id;

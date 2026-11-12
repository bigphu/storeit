-- Đặt chỗ cho một lần import trước khi xử lý dòng nào. Chỉ mục duy nhất (file_sha256) WHERE
-- has_creates làm lần thứ hai của cùng file có dòng tạo mới bị từ chối (23505)
-- name: ClaimImport :exec
INSERT INTO inventory.asset_imports (id, account_id, file_name, file_sha256, has_creates, row_count)
VALUES (@id, @account_id, @file_name, @file_sha256, @has_creates, @row_count);

-- name: FinishImport :exec
UPDATE inventory.asset_imports
SET created_count = @created_count, updated_count = @updated_count,
    skipped_count = @skipped_count, failed_count = @failed_count, finished_at = now()
WHERE id = @id;

-- Lần import có dòng tạo mới đã giữ chỗ cho file này (để báo ngày giờ và người làm)
-- name: GetCreatingImportBySHA :one
SELECT * FROM inventory.asset_imports WHERE file_sha256 = @file_sha256 AND has_creates;

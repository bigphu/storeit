-- Import tài sản từ Excel (US-06): mỗi lần xác nhận import ghi một dòng lịch sử (ai, file nào,
-- kết quả). Dòng tạo tài sản mới không có tag trong file nên nhập lại cùng một file sẽ tạo trùng
-- tài sản; vì vậy một file (SHA-256) có dòng tạo mới chỉ được xác nhận một lần.

-- +goose Up
CREATE TABLE inventory.asset_imports (
    id            uuid        PRIMARY KEY,
    account_id    uuid        NOT NULL,           -- tài khoản của identity, không có foreign key
    file_name     text        NOT NULL,
    file_sha256   text        NOT NULL,
    has_creates   boolean     NOT NULL,           -- file có dòng tạo tài sản mới
    row_count     integer     NOT NULL,
    created_count integer     NOT NULL DEFAULT 0,
    updated_count integer     NOT NULL DEFAULT 0,
    skipped_count integer     NOT NULL DEFAULT 0,
    failed_count  integer     NOT NULL DEFAULT 0,
    created_at    timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,                    -- NULL: đang chạy, hay bị ngắt giữa chừng
    CONSTRAINT asset_imports_sha_check CHECK (file_sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT asset_imports_file_name_check CHECK (char_length(file_name) BETWEEN 1 AND 255),
    CONSTRAINT asset_imports_counts_check CHECK (row_count >= 0 AND created_count >= 0 AND updated_count >= 0
        AND skipped_count >= 0 AND failed_count >= 0)
);
CREATE UNIQUE INDEX asset_imports_file_with_creates ON inventory.asset_imports (file_sha256) WHERE has_creates;
CREATE INDEX asset_imports_created_at ON inventory.asset_imports (created_at DESC);

-- +goose Down
DROP TABLE inventory.asset_imports;

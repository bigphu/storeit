-- Thay thế tài sản (US-13): liên kết hiện tại giữa tài sản cũ (đã retire) và tài sản thay
-- nó. Mỗi tài sản cũ một dòng; hoàn tác thì xoá dòng, lịch sử nằm ở platform.events.

-- +goose Up
CREATE TABLE inventory.asset_replacements (
    old_asset_id uuid        PRIMARY KEY REFERENCES inventory.assets (id),
    new_asset_id uuid        NOT NULL REFERENCES inventory.assets (id),
    replaced_on  date        NOT NULL,
    reason       text        NOT NULL,
    -- account của identity, không khoá ngoại sang module khác; NULL là hệ thống
    created_by   uuid,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_replacements_distinct CHECK (old_asset_id <> new_asset_id)
);
CREATE INDEX asset_replacements_new_idx ON inventory.asset_replacements (new_asset_id);

-- +goose Down
DROP TABLE inventory.asset_replacements;

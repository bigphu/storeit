-- Profile export (US-09): bố cục báo cáo Excel đã lưu, của riêng người tạo hoặc chia sẻ
-- cho mọi người được export. owner_id là account của identity, không khoá ngoại sang
-- module khác (như member_id). Kèm hai quyền mới và phân quyền cho role hệ thống.

-- +goose Up
CREATE TABLE inventory.export_profiles (
    id         uuid        PRIMARY KEY,
    owner_id   uuid        NOT NULL,
    name       text        NOT NULL,
    shared     boolean     NOT NULL DEFAULT false,
    layout     jsonb       NOT NULL,
    version    integer     NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT export_profiles_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100)
);
CREATE UNIQUE INDEX export_profiles_owner_name ON inventory.export_profiles (owner_id, lower(name));
CREATE INDEX export_profiles_shared ON inventory.export_profiles (owner_id) WHERE shared;

-- Mọi role export được; sửa profile người khác chia sẻ: Administrator, Authorized Manager
INSERT INTO identity.permissions (code, description) VALUES
    ('inventory.asset.export',          'Export assets to Excel and save export profiles'),
    ('inventory.export_profile.manage', 'Change or delete export profiles other people shared');

INSERT INTO identity.role_permissions (role_id, permission) VALUES
    ('00000000-0000-7000-8000-000000000001', 'inventory.asset.export'),
    ('00000000-0000-7000-8000-000000000001', 'inventory.export_profile.manage'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.asset.export'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.export_profile.manage'),
    ('00000000-0000-7000-8000-000000000003', 'inventory.asset.export'),
    ('00000000-0000-7000-8000-000000000004', 'inventory.asset.export');

-- +goose Down
DELETE FROM identity.role_permissions WHERE permission IN ('inventory.asset.export', 'inventory.export_profile.manage');
DELETE FROM identity.permissions WHERE code IN ('inventory.asset.export', 'inventory.export_profile.manage');
DROP TABLE inventory.export_profiles;

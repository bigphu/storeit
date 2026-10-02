-- Module inventory: loại tài sản và thuộc tính riêng, status, tài sản.
-- Giá trị thuộc tính là hàng có kiểu, khoá ngoại ghép giữ đúng kiểu và đúng loại.

-- +goose Up
CREATE SCHEMA IF NOT EXISTS inventory;

CREATE TABLE inventory.asset_types (
    id          uuid        PRIMARY KEY,
    code        text        NOT NULL,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    is_system   boolean     NOT NULL DEFAULT false,
    archived_at timestamptz,
    version     integer     NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_types_code_key UNIQUE (code),
    CONSTRAINT asset_types_code_check CHECK (code ~ '^[A-Z0-9_-]{1,32}$'),
    CONSTRAINT asset_types_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    CONSTRAINT asset_types_description_check CHECK (char_length(description) <= 500),
    CONSTRAINT asset_types_system_active CHECK (NOT (is_system AND archived_at IS NOT NULL))
);
CREATE UNIQUE INDEX asset_types_name_lower ON inventory.asset_types (lower(name));

CREATE TABLE inventory.asset_type_attributes (
    id            uuid        PRIMARY KEY,
    asset_type_id uuid        NOT NULL REFERENCES inventory.asset_types (id),
    key           text        NOT NULL,
    label         text        NOT NULL,
    data_type     text        NOT NULL,
    unit          text,
    is_required   boolean     NOT NULL DEFAULT false,
    position      integer     NOT NULL DEFAULT 0,
    removed_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    -- Key không bao giờ dùng lại trong một loại, kể cả khi thuộc tính đã gỡ
    CONSTRAINT asset_type_attributes_key_key UNIQUE (asset_type_id, key),
    -- Đích của khoá ngoại ghép từ giá trị và option
    CONSTRAINT asset_type_attributes_ref_key UNIQUE (id, asset_type_id, data_type),
    CONSTRAINT asset_type_attributes_type_ref_key UNIQUE (id, data_type),
    CONSTRAINT asset_type_attributes_key_check CHECK (key ~ '^[a-z][a-z0-9_]{0,31}$'),
    CONSTRAINT asset_type_attributes_label_check CHECK (char_length(btrim(label)) BETWEEN 1 AND 100),
    CONSTRAINT asset_type_attributes_data_type_check
        CHECK (data_type IN ('text', 'number', 'date', 'boolean', 'select')),
    CONSTRAINT asset_type_attributes_unit_check
        CHECK (unit IS NULL OR (data_type = 'number' AND char_length(btrim(unit)) BETWEEN 1 AND 16))
);
CREATE UNIQUE INDEX asset_type_attributes_label_lower
    ON inventory.asset_type_attributes (asset_type_id, lower(label)) WHERE removed_at IS NULL;

CREATE TABLE inventory.asset_attribute_options (
    id           uuid        PRIMARY KEY,
    attribute_id uuid        NOT NULL,
    -- Chỉ thuộc tính kiểu select mới có option
    data_type    text        NOT NULL DEFAULT 'select',
    label        text        NOT NULL,
    position     integer     NOT NULL DEFAULT 0,
    removed_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_attribute_options_attribute_fkey FOREIGN KEY (attribute_id, data_type)
        REFERENCES inventory.asset_type_attributes (id, data_type),
    CONSTRAINT asset_attribute_options_ref_key UNIQUE (id, attribute_id),
    CONSTRAINT asset_attribute_options_data_type_check CHECK (data_type = 'select'),
    CONSTRAINT asset_attribute_options_label_check CHECK (char_length(btrim(label)) BETWEEN 1 AND 100)
);
CREATE UNIQUE INDEX asset_attribute_options_label_lower
    ON inventory.asset_attribute_options (attribute_id, lower(label)) WHERE removed_at IS NULL;

CREATE TABLE inventory.asset_statuses (
    id          uuid        PRIMARY KEY,
    name        text        NOT NULL,
    kind        text        NOT NULL,
    is_default  boolean     NOT NULL DEFAULT false,
    is_system   boolean     NOT NULL DEFAULT false,
    position    integer     NOT NULL DEFAULT 0,
    archived_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_statuses_kind_check CHECK (kind IN ('available', 'in_use', 'unavailable', 'retired')),
    CONSTRAINT asset_statuses_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    CONSTRAINT asset_statuses_default_active CHECK (NOT (is_default AND archived_at IS NOT NULL)),
    CONSTRAINT asset_statuses_system_active CHECK (NOT (is_system AND archived_at IS NOT NULL))
);
CREATE UNIQUE INDEX asset_statuses_name_lower ON inventory.asset_statuses (lower(name));
CREATE UNIQUE INDEX asset_statuses_default_per_kind ON inventory.asset_statuses (kind) WHERE is_default;

CREATE TABLE inventory.assets (
    id               uuid        PRIMARY KEY,
    tag              text        NOT NULL,
    name             text        NOT NULL,
    description      text        NOT NULL DEFAULT '',
    asset_type_id    uuid        NOT NULL REFERENCES inventory.asset_types (id),
    status_id        uuid        NOT NULL REFERENCES inventory.asset_statuses (id),
    -- directory làm sau: chưa có khoá ngoại (giống identity.accounts.member_id)
    location_id      uuid,
    holder_member_id uuid,
    purchase_date    date,
    retired_at       timestamptz,
    retired_reason   text,
    version          integer     NOT NULL DEFAULT 1,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    -- Tag viết hoa sẵn (CHECK) nên UNIQUE thường cũng không phân biệt hoa thường;
    -- giữ cả tài sản đã retire: tag không bao giờ dùng lại
    CONSTRAINT assets_tag_key UNIQUE (tag),
    CONSTRAINT assets_ref_key UNIQUE (id, asset_type_id),
    CONSTRAINT assets_tag_check CHECK (tag ~ '^[A-Z0-9][A-Z0-9._-]{0,63}$'),
    CONSTRAINT assets_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 200),
    CONSTRAINT assets_description_check CHECK (char_length(description) <= 2000),
    CONSTRAINT assets_retired_reason_check
        CHECK (retired_reason IS NULL OR (retired_at IS NOT NULL AND char_length(retired_reason) <= 500))
);
CREATE INDEX assets_type_idx ON inventory.assets (asset_type_id) WHERE retired_at IS NULL;
CREATE INDEX assets_status_idx ON inventory.assets (status_id) WHERE retired_at IS NULL;
CREATE INDEX assets_location_idx ON inventory.assets (location_id) WHERE retired_at IS NULL;
CREATE INDEX assets_holder_idx ON inventory.assets (holder_member_id) WHERE retired_at IS NULL;

CREATE TABLE inventory.asset_attribute_values (
    asset_id        uuid NOT NULL,
    attribute_id    uuid NOT NULL,
    asset_type_id   uuid NOT NULL,
    data_type       text NOT NULL,
    value_text      text,
    value_number    numeric,
    value_date      date,
    value_bool      boolean,
    value_option_id uuid,
    PRIMARY KEY (asset_id, attribute_id),
    -- Giá trị thuộc đúng loại hiện tại của tài sản
    CONSTRAINT asset_attribute_values_asset_fkey FOREIGN KEY (asset_id, asset_type_id)
        REFERENCES inventory.assets (id, asset_type_id),
    -- Thuộc tính cùng loại, cùng kiểu dữ liệu: đổi kiểu khi còn giá trị bị chặn
    CONSTRAINT asset_attribute_values_attribute_fkey FOREIGN KEY (attribute_id, asset_type_id, data_type)
        REFERENCES inventory.asset_type_attributes (id, asset_type_id, data_type),
    -- Option phải là của chính thuộc tính này
    CONSTRAINT asset_attribute_values_option_fkey FOREIGN KEY (value_option_id, attribute_id)
        REFERENCES inventory.asset_attribute_options (id, attribute_id),
    CONSTRAINT asset_attribute_values_one_value
        CHECK (num_nonnulls(value_text, value_number, value_date, value_bool, value_option_id) = 1),
    CONSTRAINT asset_attribute_values_typed CHECK (
        (data_type = 'text' AND value_text IS NOT NULL) OR
        (data_type = 'number' AND value_number IS NOT NULL) OR
        (data_type = 'date' AND value_date IS NOT NULL) OR
        (data_type = 'boolean' AND value_bool IS NOT NULL) OR
        (data_type = 'select' AND value_option_id IS NOT NULL)),
    CONSTRAINT asset_attribute_values_text_check
        CHECK (value_text IS NULL OR char_length(btrim(value_text)) BETWEEN 1 AND 1000)
);
CREATE INDEX asset_attribute_values_attribute_idx ON inventory.asset_attribute_values (attribute_id);
CREATE INDEX asset_attribute_values_option_idx
    ON inventory.asset_attribute_values (value_option_id) WHERE value_option_id IS NOT NULL;

-- Seed: loại chung và bốn status mặc định, ID cố định để code tham chiếu
INSERT INTO inventory.asset_types (id, code, name, description, is_system) VALUES
    ('00000000-0000-7000-8000-000000000101', 'GENERAL', 'General', 'Assets without specific attributes', true);

INSERT INTO inventory.asset_statuses (id, name, kind, is_default, is_system, position) VALUES
    ('00000000-0000-7000-8000-000000000201', 'Available',    'available',   true, true, 1),
    ('00000000-0000-7000-8000-000000000202', 'In use',       'in_use',      true, true, 2),
    ('00000000-0000-7000-8000-000000000203', 'Under repair', 'unavailable', true, true, 3),
    ('00000000-0000-7000-8000-000000000204', 'Retired',      'retired',     true, true, 4);

-- Quyền của inventory vào danh mục quyền của identity, kèm phân cho role hệ thống.
-- Administrator nhận tất cả: không ai trao được quyền mình không có.
INSERT INTO identity.permissions (code, description) VALUES
    ('inventory.asset.read',    'View assets, asset types and statuses'),
    ('inventory.asset.manage',  'Create, update, retire and restore assets'),
    ('inventory.type.manage',   'Manage asset types, custom attributes and options'),
    ('inventory.status.manage', 'Manage asset statuses');

INSERT INTO identity.role_permissions (role_id, permission) VALUES
    ('00000000-0000-7000-8000-000000000001', 'inventory.asset.read'),
    ('00000000-0000-7000-8000-000000000001', 'inventory.asset.manage'),
    ('00000000-0000-7000-8000-000000000001', 'inventory.type.manage'),
    ('00000000-0000-7000-8000-000000000001', 'inventory.status.manage'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.asset.read'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.type.manage'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.status.manage'),
    ('00000000-0000-7000-8000-000000000003', 'inventory.asset.read'),
    ('00000000-0000-7000-8000-000000000003', 'inventory.asset.manage'),
    ('00000000-0000-7000-8000-000000000004', 'inventory.asset.read');

-- +goose Down
DELETE FROM identity.role_permissions WHERE permission LIKE 'inventory.%';
DELETE FROM identity.permissions WHERE code LIKE 'inventory.%';
DROP TABLE inventory.asset_attribute_values;
DROP TABLE inventory.assets;
DROP TABLE inventory.asset_statuses;
DROP TABLE inventory.asset_attribute_options;
DROP TABLE inventory.asset_type_attributes;
DROP TABLE inventory.asset_types;
DROP SCHEMA inventory;

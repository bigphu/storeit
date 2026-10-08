-- Module identity: tài khoản, role, quyền và phiên đăng nhập (refresh token xoay
-- vòng theo family, port từ mimir-2.0).

-- +goose Up
CREATE SCHEMA IF NOT EXISTS identity;

CREATE TABLE identity.accounts (
    id            uuid        PRIMARY KEY,
    -- Lưu chữ thường; unique theo lower(email) để chắc chắn không trùng hoa/thường
    email         text        NOT NULL,
    name          text        NOT NULL,
    -- NULL: account được mời nhưng chưa đặt mật khẩu (chưa đăng nhập được)
    password_hash text,
    -- Liên kết tới directory.members; chưa có FK vì module directory làm sau
    member_id     uuid,
    active        boolean     NOT NULL DEFAULT true,
    -- Optimistic locking cho cập nhật hồ sơ
    version       integer     NOT NULL DEFAULT 1,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT accounts_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX accounts_email_lower ON identity.accounts (lower(email));

-- Danh mục quyền. Mỗi module tự thêm mã quyền của mình bằng migration riêng;
-- FK từ role_permissions chặn gán mã không tồn tại.
CREATE TABLE identity.permissions (
    code        text PRIMARY KEY,
    description text NOT NULL
);

CREATE TABLE identity.roles (
    id          uuid        PRIMARY KEY,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    -- Role hệ thống không đổi tên, không xoá được (quyền thì sửa được)
    is_system   boolean     NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    -- Xoá mềm: giữ hàng và quyền để khôi phục; role đã xoá không cấp quyền, không gán được
    deleted_at  timestamptz,

    CONSTRAINT roles_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100)
);
-- Tên duy nhất trong các role chưa xoá (tên của role đã xoá dùng lại được)
CREATE UNIQUE INDEX roles_name_key ON identity.roles (name) WHERE deleted_at IS NULL;

CREATE TABLE identity.role_permissions (
    role_id    uuid NOT NULL REFERENCES identity.roles (id) ON DELETE CASCADE,
    permission text NOT NULL REFERENCES identity.permissions (code),
    PRIMARY KEY (role_id, permission)
);

CREATE TABLE identity.account_roles (
    account_id uuid NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    role_id    uuid NOT NULL REFERENCES identity.roles (id),
    PRIMARY KEY (account_id, role_id)
);
CREATE INDEX account_roles_role_idx ON identity.account_roles (role_id);

-- Một family là một lần đăng nhập (một thiết bị). Mọi lần refresh chỉ xoay
-- token bên trong family; thu hồi family là đăng xuất thiết bị đó.
CREATE TABLE identity.refresh_families (
    id                  uuid        PRIMARY KEY,
    account_id          uuid        NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    user_agent          text        NOT NULL DEFAULT '',
    ip                  text        NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    absolute_expires_at timestamptz NOT NULL,
    revoked_at          timestamptz,
    revoked_reason      text,

    CONSTRAINT refresh_families_reason_check CHECK (
        revoked_reason IS NULL
        OR revoked_reason IN ('logout', 'reuse_detected', 'admin', 'expired', 'password_change', 'password_reset'))
);
CREATE INDEX refresh_families_live ON identity.refresh_families (account_id) WHERE revoked_at IS NULL;

-- Chỉ lưu SHA-256 của token, không bao giờ lưu token thô
CREATE TABLE identity.refresh_tokens (
    id         uuid        PRIMARY KEY,
    family_id  uuid        NOT NULL REFERENCES identity.refresh_families (id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL,
    parent_id  uuid        REFERENCES identity.refresh_tokens (id) ON DELETE SET NULL,
    issued_at  timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,

    CONSTRAINT refresh_tokens_hash_key UNIQUE (token_hash)
);
CREATE INDEX refresh_tokens_family_idx ON identity.refresh_tokens (family_id);
-- Bất biến của vòng xoay: mỗi family có đúng một token chưa dùng (ngọn).
-- Chẻ family thành hai nhánh sống là tắt phát hiện dùng lại.
CREATE UNIQUE INDEX refresh_tokens_live ON identity.refresh_tokens (family_id) WHERE used_at IS NULL;

-- Link một lần gửi qua email: invite (đặt mật khẩu lần đầu) và reset (quên
-- mật khẩu). Mỗi account có tối đa một token mỗi loại: phát token mới ghi đè
-- token cũ, nên link cũ chết ngay. Dùng xong thì xoá hàng.
CREATE TABLE identity.password_tokens (
    account_id uuid        NOT NULL REFERENCES identity.accounts (id) ON DELETE CASCADE,
    purpose    text        NOT NULL,
    -- Đổi mỗi lần phát; là idempotency key khi gửi thư
    id         uuid        NOT NULL,
    -- Chỉ lưu SHA-256, không bao giờ lưu token thô
    token_hash bytea       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,

    PRIMARY KEY (account_id, purpose),
    CONSTRAINT password_tokens_hash_key UNIQUE (token_hash),
    CONSTRAINT password_tokens_id_key UNIQUE (id),
    CONSTRAINT password_tokens_purpose_check CHECK (purpose IN ('invite', 'reset'))
);

-- Seed: quyền của identity và bốn role hệ thống (StoreIT-Main, mục 4).
-- ID role cố định để code và test tham chiếu được Administrator.
INSERT INTO identity.permissions (code, description) VALUES
    ('identity.account.read',   'View accounts'),
    ('identity.account.manage', 'Create, update, disable accounts, reset passwords and assign roles'),
    ('identity.role.read',      'View roles and their permissions'),
    ('identity.role.manage',    'Create, update and delete roles and change their permissions');

INSERT INTO identity.roles (id, name, description, is_system) VALUES
    ('00000000-0000-7000-8000-000000000001', 'Administrator',      'Manages users, roles and system access', true),
    ('00000000-0000-7000-8000-000000000002', 'Authorized Manager', 'Accesses asset information and reports according to assigned permissions', true),
    ('00000000-0000-7000-8000-000000000003', 'Inventory Officer',  'Manages assets, import/export, borrowing and asset lifecycle', true),
    ('00000000-0000-7000-8000-000000000004', 'Employee',           'Views relevant assets and borrows/returns assets', true);

INSERT INTO identity.role_permissions (role_id, permission) VALUES
    ('00000000-0000-7000-8000-000000000001', 'identity.account.read'),
    ('00000000-0000-7000-8000-000000000001', 'identity.account.manage'),
    ('00000000-0000-7000-8000-000000000001', 'identity.role.read'),
    ('00000000-0000-7000-8000-000000000001', 'identity.role.manage'),
    ('00000000-0000-7000-8000-000000000002', 'identity.account.read'),
    ('00000000-0000-7000-8000-000000000002', 'identity.role.read');

-- +goose Down
DROP TABLE identity.password_tokens;
DROP TABLE identity.refresh_tokens;
DROP TABLE identity.refresh_families;
DROP TABLE identity.account_roles;
DROP TABLE identity.role_permissions;
DROP TABLE identity.roles;
DROP TABLE identity.permissions;
DROP TABLE identity.accounts;
DROP SCHEMA identity;

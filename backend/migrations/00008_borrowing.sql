-- Mượn tài sản (US-10): công tắc "Can be borrowed" trên loại tài sản, hai status hệ thống do
-- việc mượn đặt (Reserved, On loan), phiếu mượn và khoản mượn, hai quyền mới. Phiếu và
-- khoản mượn giữ mãi, không xoá.

-- +goose Up
-- Tắt cho mọi loại có sẵn: chưa ai bật thì chưa có gì cho mượn
ALTER TABLE inventory.asset_types ADD COLUMN can_borrow boolean NOT NULL DEFAULT false;

-- Status người dùng tự tạo trùng tên (vd "On loan") đổi tên để không chặn migration; tài sản
-- của nó giữ nguyên status đó
UPDATE inventory.asset_statuses SET name = name || ' (custom)', updated_at = now()
    WHERE lower(name) IN ('reserved', 'on loan');

INSERT INTO inventory.asset_statuses (id, name, kind, is_default, is_system, position) VALUES
    ('00000000-0000-7000-8000-000000000205', 'Reserved', 'unavailable', false, true,
        (SELECT coalesce(max(position), 0) + 1 FROM inventory.asset_statuses)),
    ('00000000-0000-7000-8000-000000000206', 'On loan',  'in_use',      false, true,
        (SELECT coalesce(max(position), 0) + 2 FROM inventory.asset_statuses));

CREATE TABLE inventory.borrow_tickets (
    id                 uuid        PRIMARY KEY,
    number             bigint      GENERATED ALWAYS AS IDENTITY UNIQUE,
    -- account của identity, không khoá ngoại sang module khác
    requester_id       uuid        NOT NULL,
    asset_type_id      uuid        NOT NULL REFERENCES inventory.asset_types (id),
    -- điều kiện thuộc tính "<key>:<op>:<value>", như bộ lọc của danh sách tài sản
    requirements       text[]      NOT NULL DEFAULT '{}',
    suggested_asset_id uuid        REFERENCES inventory.assets (id),
    needed_from        date        NOT NULL,
    return_by          date,
    note               text        NOT NULL DEFAULT '',
    state              text        NOT NULL DEFAULT 'waiting',
    asset_id           uuid        REFERENCES inventory.assets (id),
    decided_by         uuid,
    decided_at         timestamptz,
    reason             text        NOT NULL DEFAULT '',
    cancelled_by       uuid,
    cancelled_at       timestamptz,
    version            integer     NOT NULL DEFAULT 1,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT borrow_tickets_state_check
        CHECK (state IN ('waiting', 'approved', 'rejected', 'cancelled', 'handed_over')),
    CONSTRAINT borrow_tickets_return_by_check CHECK (return_by IS NULL OR return_by >= needed_from),
    CONSTRAINT borrow_tickets_requirements_check CHECK (cardinality(requirements) <= 10),
    -- đã duyệt hay đã giao thì phải có tài sản
    CONSTRAINT borrow_tickets_asset_check
        CHECK (state NOT IN ('approved', 'handed_over') OR asset_id IS NOT NULL)
);
-- Một tài sản chỉ được giữ chỗ cho một phiếu
CREATE UNIQUE INDEX borrow_tickets_one_approved_per_asset ON inventory.borrow_tickets (asset_id)
    WHERE state = 'approved';
CREATE INDEX borrow_tickets_requester_idx ON inventory.borrow_tickets (requester_id, created_at DESC);
CREATE INDEX borrow_tickets_state_idx ON inventory.borrow_tickets (state, created_at DESC);

CREATE TABLE inventory.loans (
    id                  uuid        PRIMARY KEY,
    asset_id            uuid        NOT NULL REFERENCES inventory.assets (id),
    borrower_id         uuid        NOT NULL,
    -- NULL: cho mượn trực tiếp
    ticket_id           uuid        UNIQUE REFERENCES inventory.borrow_tickets (id),
    lent_by             uuid,
    started_on          date        NOT NULL,
    return_by           date,
    returned_on         date,
    overdue_notified_on date,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT loans_return_by_check CHECK (return_by IS NULL OR return_by >= started_on)
);
-- Một tài sản chỉ có một khoản mượn chưa trả (US10-AC5)
CREATE UNIQUE INDEX loans_one_active_per_asset ON inventory.loans (asset_id) WHERE returned_on IS NULL;
CREATE INDEX loans_borrower_idx ON inventory.loans (borrower_id, started_on DESC);

INSERT INTO identity.permissions (code, description) VALUES
    ('inventory.borrow.request', 'Ask to borrow assets and follow own tickets'),
    ('inventory.borrow.manage',  'Approve borrow tickets, hand over and lend assets');

INSERT INTO identity.role_permissions (role_id, permission) VALUES
    ('00000000-0000-7000-8000-000000000001', 'inventory.borrow.request'),
    ('00000000-0000-7000-8000-000000000001', 'inventory.borrow.manage'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.borrow.request'),
    ('00000000-0000-7000-8000-000000000003', 'inventory.borrow.request'),
    ('00000000-0000-7000-8000-000000000003', 'inventory.borrow.manage'),
    ('00000000-0000-7000-8000-000000000004', 'inventory.borrow.request');

-- +goose Down
DELETE FROM identity.role_permissions WHERE permission IN ('inventory.borrow.request', 'inventory.borrow.manage');
DELETE FROM identity.permissions WHERE code IN ('inventory.borrow.request', 'inventory.borrow.manage');
DROP TABLE inventory.loans;
DROP TABLE inventory.borrow_tickets;
-- Tài sản đang mang hai status này về status mặc định cùng kind trước khi xoá status
UPDATE inventory.assets SET status_id = (
    SELECT id FROM inventory.asset_statuses WHERE kind = 'available' AND is_default)
    WHERE status_id = '00000000-0000-7000-8000-000000000205';
UPDATE inventory.assets SET status_id = (
    SELECT id FROM inventory.asset_statuses WHERE kind = 'in_use' AND is_default)
    WHERE status_id = '00000000-0000-7000-8000-000000000206';
DELETE FROM inventory.asset_statuses
    WHERE id IN ('00000000-0000-7000-8000-000000000205', '00000000-0000-7000-8000-000000000206');
ALTER TABLE inventory.asset_types DROP COLUMN can_borrow;

-- Nhật ký event bền vững của outbox. Giữ mọi event kể cả sau khi River dọn
-- job xong, để replay cho subscriber mới (vd dựng lại activity).

-- +goose Up
CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE platform.events (
    id             uuid        PRIMARY KEY,
    type           text        NOT NULL,
    aggregate_type text        NOT NULL,
    aggregate_id   uuid        NOT NULL,
    -- NULL: SystemActor (scheduled job...)
    actor_id       uuid,
    occurred_at    timestamptz NOT NULL,
    payload        jsonb       NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

-- Lịch sử của một entity; replay theo loại event
CREATE INDEX events_aggregate_idx ON platform.events (aggregate_type, aggregate_id, occurred_at);
CREATE INDEX events_type_idx ON platform.events (type, occurred_at);

-- +goose Down
DROP TABLE platform.events;
DROP SCHEMA platform;

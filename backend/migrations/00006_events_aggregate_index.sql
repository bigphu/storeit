-- Lịch sử hoạt động (US-16): đọc event của một aggregate theo thời gian, mới nhất trước
-- (events.ListForAggregate). Bảng events giữ mọi event nên cần chỉ mục cho truy vấn này.

-- +goose Up
CREATE INDEX events_aggregate_timeline_idx
    ON platform.events (aggregate_type, aggregate_id, occurred_at DESC, id DESC);

-- +goose Down
DROP INDEX platform.events_aggregate_timeline_idx;

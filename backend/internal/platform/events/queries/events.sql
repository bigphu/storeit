-- name: InsertEvent :exec
INSERT INTO platform.events (id, type, aggregate_type, aggregate_id, actor_id, occurred_at, payload)
VALUES (@id, @type, @aggregate_type, @aggregate_id, sqlc.narg('actor_id'), @occurred_at, @payload);

-- name: GetEvent :one
SELECT id, type, aggregate_type, aggregate_id, actor_id, occurred_at, payload
FROM platform.events
WHERE id = @id;

-- Event của một aggregate, mới nhất trước. before: id event cuối của trang trước; chỉ lấy
-- event cũ hơn event đó
-- name: ListAggregateEvents :many
SELECT e.id, e.type, e.aggregate_type, e.aggregate_id, e.actor_id, e.occurred_at, e.payload
FROM platform.events e
WHERE e.aggregate_type = @aggregate_type
  AND e.aggregate_id = @aggregate_id
  AND (sqlc.narg('before')::uuid IS NULL
       OR (e.occurred_at, e.id) < (SELECT b.occurred_at, b.id FROM platform.events b WHERE b.id = sqlc.narg('before')::uuid))
ORDER BY e.occurred_at DESC, e.id DESC
LIMIT @lim;

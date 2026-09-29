-- name: InsertEvent :exec
INSERT INTO platform.events (id, type, aggregate_type, aggregate_id, actor_id, occurred_at, payload)
VALUES (@id, @type, @aggregate_type, @aggregate_id, sqlc.narg('actor_id'), @occurred_at, @payload);

-- name: GetEvent :one
SELECT id, type, aggregate_type, aggregate_id, actor_id, occurred_at, payload
FROM platform.events
WHERE id = @id;

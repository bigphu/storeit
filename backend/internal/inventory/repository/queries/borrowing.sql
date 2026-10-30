-- Mượn tài sản (US-10): phiếu mượn và khoản mượn

-- name: InsertBorrowTicket :one
INSERT INTO inventory.borrow_tickets
    (id, requester_id, asset_type_id, requirements, suggested_asset_id, needed_from, return_by, note)
VALUES (@id, @requester_id, @asset_type_id, @requirements::text[], sqlc.narg('suggested_asset_id'),
        @needed_from, sqlc.narg('return_by'), @note)
RETURNING *;

-- name: GetBorrowTicket :one
SELECT * FROM inventory.borrow_tickets WHERE id = @id;

-- name: GetBorrowTicketForUpdate :one
SELECT * FROM inventory.borrow_tickets WHERE id = @id FOR UPDATE;

-- Phiếu đang giữ chỗ tài sản (nhiều nhất một)
-- name: GetApprovedTicketForAsset :one
SELECT * FROM inventory.borrow_tickets WHERE asset_id = @asset_id AND state = 'approved';

-- Lọc: trạng thái (rỗng là mọi trạng thái), người gửi (nil là mọi người), từ khoá (service
-- đã dịch thành danh sách người gửi và loại khớp: phiếu khớp một trong hai)
-- name: ListBorrowTickets :many
SELECT * FROM inventory.borrow_tickets
WHERE (cardinality(@states::text[]) = 0 OR state = ANY(@states::text[]))
  AND (sqlc.narg('requester_id')::uuid IS NULL OR requester_id = sqlc.narg('requester_id')::uuid)
  AND (NOT @has_q::boolean OR requester_id = ANY(@q_requesters::uuid[]) OR asset_type_id = ANY(@q_types::uuid[]))
ORDER BY
  CASE WHEN @sort::text = 'number' THEN number END ASC,
  CASE WHEN @sort::text = 'needed_from' THEN needed_from END ASC,
  CASE WHEN @sort::text = '-needed_from' THEN needed_from END DESC,
  number DESC
LIMIT @lim OFFSET @off;

-- name: CountBorrowTickets :one
SELECT count(*) FROM inventory.borrow_tickets
WHERE (cardinality(@states::text[]) = 0 OR state = ANY(@states::text[]))
  AND (sqlc.narg('requester_id')::uuid IS NULL OR requester_id = sqlc.narg('requester_id')::uuid)
  AND (NOT @has_q::boolean OR requester_id = ANY(@q_requesters::uuid[]) OR asset_type_id = ANY(@q_types::uuid[]));

-- name: CountBorrowTicketsByState :many
SELECT state, count(*) AS n FROM inventory.borrow_tickets
WHERE state IN ('waiting', 'approved')
GROUP BY state;

-- name: ApproveBorrowTicket :one
UPDATE inventory.borrow_tickets
SET state = 'approved', asset_id = @asset_id, decided_by = sqlc.narg('decided_by'), decided_at = now(),
    version = version + 1, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: RejectBorrowTicket :one
UPDATE inventory.borrow_tickets
SET state = 'rejected', reason = @reason, decided_by = sqlc.narg('decided_by'), decided_at = now(),
    version = version + 1, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: CancelBorrowTicket :one
UPDATE inventory.borrow_tickets
SET state = 'cancelled', reason = @reason, cancelled_by = sqlc.narg('cancelled_by'), cancelled_at = now(),
    version = version + 1, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: HandOverBorrowTicket :one
UPDATE inventory.borrow_tickets
SET state = 'handed_over', version = version + 1, updated_at = now()
WHERE id = @id
RETURNING *;

-- name: InsertLoan :one
INSERT INTO inventory.loans (id, asset_id, borrower_id, ticket_id, lent_by, started_on, return_by)
VALUES (@id, @asset_id, @borrower_id, sqlc.narg('ticket_id'), sqlc.narg('lent_by'), @started_on, sqlc.narg('return_by'))
RETURNING *;

-- Khoản mượn kèm số phiếu (0: cho mượn trực tiếp)
-- name: GetLoan :one
SELECT l.*, coalesce(t.number, 0)::bigint AS ticket_number
FROM inventory.loans l
LEFT JOIN inventory.borrow_tickets t ON t.id = l.ticket_id
WHERE l.id = @id;

-- name: GetActiveLoanForAsset :one
SELECT l.*, coalesce(t.number, 0)::bigint AS ticket_number
FROM inventory.loans l
LEFT JOIN inventory.borrow_tickets t ON t.id = l.ticket_id
WHERE l.asset_id = @asset_id AND l.returned_on IS NULL;

-- Khoản đang mượn: theo cột sort nếu có; mặc định quá hạn trước (hạn sớm nhất trước), rồi
-- theo hạn, rồi mới bắt đầu trước
-- name: ListActiveLoans :many
SELECT l.*, coalesce(t.number, 0)::bigint AS ticket_number
FROM inventory.loans l
LEFT JOIN inventory.borrow_tickets t ON t.id = l.ticket_id
WHERE l.returned_on IS NULL
ORDER BY CASE WHEN @sort::text = 'started_on' THEN l.started_on END ASC,
         CASE WHEN @sort::text = '-started_on' THEN l.started_on END DESC,
         CASE WHEN @sort::text = 'return_by' THEN l.return_by END ASC NULLS LAST,
         CASE WHEN @sort::text = '-return_by' THEN l.return_by END DESC NULLS LAST,
         (l.return_by IS NOT NULL AND l.return_by < @today::date) DESC,
         l.return_by ASC NULLS LAST, l.started_on DESC, l.id
LIMIT @lim OFFSET @off;

-- name: CountActiveLoans :one
SELECT count(*) FROM inventory.loans WHERE returned_on IS NULL;

-- Khoản quá hạn chưa báo, khoá để hai lần quét cùng lúc không báo trùng
-- name: ListOverdueUnnotifiedLoans :many
SELECT * FROM inventory.loans
WHERE returned_on IS NULL AND return_by < @today::date AND overdue_notified_on IS NULL
ORDER BY return_by, id
LIMIT @lim
FOR UPDATE SKIP LOCKED;

-- name: MarkLoanOverdueNotified :exec
UPDATE inventory.loans SET overdue_notified_on = @today::date WHERE id = @id;

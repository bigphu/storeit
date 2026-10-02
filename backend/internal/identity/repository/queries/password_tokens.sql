-- Phát token: ghi đè token cùng loại của account (link cũ chết ngay).
-- min_age_seconds > 0 là cooldown: token cùng loại mới hơn thế thì không ghi,
-- không trả hàng nào. Kiểm tra nằm trong câu upsert (khoá hàng), nên hai
-- request song song không cùng lọt, kể cả khi chưa có hàng nào.
-- name: UpsertPasswordToken :one
INSERT INTO identity.password_tokens (account_id, purpose, id, token_hash, expires_at)
VALUES (@account_id, @purpose, @id, @token_hash, @expires_at)
ON CONFLICT (account_id, purpose) DO UPDATE
SET id = EXCLUDED.id, token_hash = EXCLUDED.token_hash, created_at = now(), expires_at = EXCLUDED.expires_at
WHERE sqlc.arg(min_age_seconds)::float8 = 0
   OR identity.password_tokens.created_at < now() - make_interval(secs => sqlc.arg(min_age_seconds)::float8)
RETURNING *;

-- Tra token không khoá, để biết account nào cần khoá trước
-- name: GetPasswordTokenByHash :one
SELECT * FROM identity.password_tokens WHERE token_hash = @token_hash;

-- Dùng token: xoá rồi mới kiểm tra, nên hai lần gửi cùng lúc chỉ một bên thấy hàng
-- name: ConsumePasswordToken :one
DELETE FROM identity.password_tokens WHERE token_hash = @token_hash RETURNING *;

-- name: GetPasswordToken :one
SELECT * FROM identity.password_tokens WHERE id = @id;

-- name: GetAccountPasswordToken :one
SELECT * FROM identity.password_tokens WHERE account_id = @account_id AND purpose = @purpose;

-- name: DeleteAccountPasswordTokens :exec
DELETE FROM identity.password_tokens WHERE account_id = @account_id;

-- name: DeleteExpiredPasswordTokens :execrows
DELETE FROM identity.password_tokens WHERE expires_at < @cutoff;

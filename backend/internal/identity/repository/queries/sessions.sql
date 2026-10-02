-- name: CreateFamily :exec
INSERT INTO identity.refresh_families (id, account_id, user_agent, ip, absolute_expires_at)
VALUES (@id, @account_id, @user_agent, @ip, @absolute_expires_at);

-- name: CreateRefreshToken :exec
INSERT INTO identity.refresh_tokens (id, family_id, token_hash, parent_id, expires_at)
VALUES (@id, @family_id, @token_hash, sqlc.narg('parent_id'), @expires_at);

-- Trái tim của vòng xoay (port từ mimir). Khoá cả hàng token lẫn hàng family:
-- hai lần refresh song song trên cùng family phải xếp hàng, để lần đến sau đọc
-- được used_at mới nhất và rẽ vào nhánh ân hạn thay vì chẻ family. Kèm trạng
-- thái account để account bị khoá không refresh được.
-- name: GetRefreshForUpdate :one
SELECT
  t.id                  AS token_id,
  t.family_id           AS family_id,
  t.used_at             AS used_at,
  t.expires_at          AS expires_at,
  f.account_id          AS account_id,
  f.revoked_at          AS revoked_at,
  f.absolute_expires_at AS absolute_expires_at,
  a.active              AS account_active
FROM identity.refresh_tokens t
JOIN identity.refresh_families f ON f.id = t.family_id
JOIN identity.accounts a ON a.id = f.account_id
WHERE t.token_hash = @token_hash
FOR UPDATE OF t, f;

-- Ngọn của family: token chưa dùng duy nhất (refresh_tokens_live đảm bảo)
-- name: GetFamilyTip :one
SELECT id FROM identity.refresh_tokens WHERE family_id = @family_id AND used_at IS NULL;

-- Guard used_at IS NULL: 0 hàng là có ai đó đánh dấu trước
-- name: MarkRefreshTokenUsed :execrows
UPDATE identity.refresh_tokens SET used_at = @used_at WHERE id = @id AND used_at IS NULL;

-- name: FamilyOfToken :one
SELECT family_id FROM identity.refresh_tokens WHERE token_hash = @token_hash;

-- name: RevokeFamily :exec
UPDATE identity.refresh_families
SET revoked_at = now(), revoked_reason = @reason
WHERE id = @id AND revoked_at IS NULL;

-- Thu hồi mọi phiên còn sống của account, trừ family đang dùng (nếu có)
-- name: RevokeAccountFamilies :execrows
UPDATE identity.refresh_families
SET revoked_at = now(), revoked_reason = @reason
WHERE account_id = @account_id AND revoked_at IS NULL
  AND (sqlc.narg('keep')::uuid IS NULL OR id <> sqlc.narg('keep')::uuid);

-- Dọn rác: family đã chết (thu hồi hoặc hết hạn tuyệt đối) quá retention
-- name: DeleteDeadFamilies :execrows
DELETE FROM identity.refresh_families
WHERE GREATEST(revoked_at, absolute_expires_at) < @cutoff::timestamptz;

-- Token đã dùng trong family còn sống, hết hạn quá retention. Không đụng ngọn.
-- name: DeleteUsedTokens :execrows
DELETE FROM identity.refresh_tokens
WHERE used_at IS NOT NULL AND expires_at < @cutoff::timestamptz;

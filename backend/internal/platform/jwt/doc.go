// Package jwt phát và kiểm tra access token (HS256, có kid để xoay key).
//
//	p, err := jwt.New(cfg)             // cfg đọc từ env JWT_*
//	tok, err := p.Issue(accountID, perms) // tok.Value gửi cho client
//	claims, err := p.Verify(raw)          // lỗi so được bằng errors.Is, vd jwt.ErrExpired
//	id, err := claims.UserID()
//	claims.Permissions                    // vd ["inventory.asset.read", ...]
//
// Quyền nằm luôn trong token nên đổi role chỉ có tác dụng khi token hết hạn;
// vì vậy giữ TTL ngắn (mặc định 15 phút).
//
// File JWT_KEYS_FILE chứa "v1:<base64>,v2:<base64>". Token mới ký bằng
// JWT_ACTIVE_KID; token cũ vẫn verify được khi key của nó còn trong file.
package jwt

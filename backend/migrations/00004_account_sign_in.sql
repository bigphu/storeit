-- Lần đăng nhập gần nhất của account (trang tài khoản), ghi lúc tạo phiên đăng nhập;
-- refresh không đổi nó. Bảng riêng chứ không phải cột của accounts: đăng nhập không
-- được chờ khoá hàng account (FOR NO KEY UPDATE) mà thao tác quản trị đang giữ. Không
-- lấy từ refresh_families vì phiên chết bị dọn sau retention.

-- +goose Up
CREATE TABLE identity.account_sign_ins (
    account_id uuid        PRIMARY KEY REFERENCES identity.accounts (id) ON DELETE CASCADE,
    last_at    timestamptz NOT NULL
);

-- +goose Down
DROP TABLE identity.account_sign_ins;

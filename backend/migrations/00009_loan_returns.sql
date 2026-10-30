-- Trả tài sản (US-11): khoản mượn ghi người nhận lại và ghi chú lúc trả; ngày trả không trước
-- ngày bắt đầu. Khoản mượn đã trả giữ nguyên, không xoá.

-- +goose Up
ALTER TABLE inventory.loans
    ADD COLUMN returned_by uuid,
    ADD COLUMN return_note text,
    ADD CONSTRAINT loans_returned_on_check CHECK (returned_on IS NULL OR returned_on >= started_on);

-- +goose Down
ALTER TABLE inventory.loans
    DROP CONSTRAINT loans_returned_on_check,
    DROP COLUMN return_note,
    DROP COLUMN returned_by;

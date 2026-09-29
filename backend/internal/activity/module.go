// Package activity là lịch sử thay đổi (audit trail): nghe mọi event, lưu thành
// các dòng chỉ thêm, không sửa.
//
// Module khác không gọi activity. Muốn có lịch sử thì ghi event vào outbox.
package activity

// TODO(M2): Module{Handler}, New(pool, accounts) và Subscribe để nghe mọi event

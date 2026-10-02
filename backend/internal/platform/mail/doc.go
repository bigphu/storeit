// Package mail gửi thư qua một transport chọn tường minh bằng MAIL_TRANSPORT:
// smtp (máy chủ SMTP bất kỳ, dev dùng Mailpit), resend (HTTP API của Resend)
// hoặc log (in thư ra log, cho test và chạy không có compose).
//
// Package chỉ lo đường ra. Nội dung thư (template, link) do module dựng.
//
//	sender, err := mail.New(cfg.Mail, log)   // cmd/worker, một lần
//	err = sender.Send(ctx, mail.Message{
//		To:             mail.Address{Name: a.Name, Email: a.Email},
//		Subject:        "...",
//		Text:           text,
//		HTML:           html,
//		IdempotencyKey: "identity/invite/" + tokenID.String(),
//	})
//	if mail.IsPermanent(err) {
//		return river.JobCancel(err)   // thử lại cũng không khác: huỷ job
//	}
//
// Gửi thư nên chạy trong job nền: job có thể chạy lại, nên đặt IdempotencyKey
// để Resend bỏ qua lần gửi trùng (giữ key 24 giờ). SMTP không có cơ chế này.
package mail

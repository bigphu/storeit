// Package events gửi event giữa các module qua outbox, River lo việc giao.
//
// Module phát event: repository đổi domain event sang kiểu công khai trong
// contract/events.go rồi Append trong cùng transaction với thay đổi:
//
//	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
//		... UPDATE inventory.assets ...
//		e, err := events.New(contract.EventAssetCheckedOut, "asset", a.ID, payload)
//		if err != nil {
//			return err
//		}
//		return r.outbox.Append(ctx, tx, e)
//	})
//
// Module nghe event: đăng ký subscriber trong module.go. Subscriber phải
// idempotent, không dựa vào thứ tự event:
//
//	func (m *Module) Subscribe(r *events.Registry) {
//		r.OnAll("activity.record", m.subscriber.Record)
//		r.On("notifications.email", m.subscriber.Email,
//			invcontract.EventAssetCheckedOut, invcontract.EventAssetReturned)
//	}
//
// Tên subscriber là khoá của job: mỗi tên đăng ký một lần, không đổi khi còn
// job đang chờ. Worker gặp tên chưa đăng ký thì retry (API có thể deploy trước
// worker), không huỷ job.
//
// Wiring: cmd/api và cmd/worker dựng cùng registry; worker thêm
// RegisterWorker. platform.events giữ mọi event sau khi River dọn job, để
// replay cho subscriber mới.
package events

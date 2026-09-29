// Package jobs đẩy việc chạy nền vào queue (River trên Postgres). Service và
// repository chỉ thấy Enqueuer, không import River.
//
// Định nghĩa job: một struct serialize được ra JSON, có Kind() không đổi qua
// các lần deploy (job cũ trong DB tìm worker theo kind):
//
//	type SendReceiptArgs struct {
//		OrderID string `json:"order_id"`
//	}
//
//	func (SendReceiptArgs) Kind() string { return "send_receipt" }
//
// Worker thì viết theo River (river.Worker[SendReceiptArgs]) và đăng ký lúc
// dựng client. cmd/api dùng client chỉ insert, cmd/worker dùng client chạy
// job; bọc client rồi truyền Enqueuer cho service/repository:
//
//	client, err := jobs.NewInsertClient(pool)                   // cmd/api
//	client, err := jobs.NewWorkerClient(pool, workers, periodic) // cmd/worker
//	enq := jobs.NewRiver(client)
//
// Job do người dùng yêu cầu nhúng ActorArgs; worker gọi RestoreActor trước khi
// gọi service, để job chạy với quyền của người đó (scheduled job: SystemActor).
//
// Job đi kèm một thay đổi dữ liệu thì repository enqueue bằng EnqueueTx, trong
// cùng database.WithTx với thay đổi đó: commit thì job mới tồn tại, rollback
// thì job mất theo (transactional outbox). Service không bao giờ thấy
// transaction, nó chỉ gọi method của repository:
//
//	func (r *OrderRepository) Create(ctx context.Context, o *domain.Order) error {
//		return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
//			if err := r.q.WithTx(tx).InsertOrder(ctx, ...); err != nil {
//				return err
//			}
//			return r.jobs.EnqueueTx(ctx, tx, SendReceiptArgs{OrderID: o.ID},
//				jobs.WithQueue("mail"), jobs.WithUniqueArgs())
//		})
//	}
//
// Job không gắn với thay đổi nào thì service gọi thẳng Enqueue, không có đảm
// bảo trên.
//
// Worker có thể chạy lại một job nhiều lần (retry, crash giữa chừng), nên
// việc nó làm phải idempotent.
package jobs

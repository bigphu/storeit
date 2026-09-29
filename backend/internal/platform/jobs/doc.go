// Package jobs đẩy việc chạy nền vào queue (River trên Postgres). Service và
// repository chỉ thấy Enqueuer, không import River.
//
// Args của job nằm trong package job/ của module sở hữu nó: chỗ enqueue
// (repository hoặc service) và worker đều import được, module khác thì không.
// Args là struct serialize được ra JSON, có Kind() không đổi qua các lần
// deploy (job cũ trong DB tìm worker theo kind). Job do người dùng yêu cầu
// nhúng ActorArgs:
//
//	// inventory/job/commit_import.go
//	type CommitImportArgs struct {
//		jobs.ActorArgs
//		ImportID uuid.UUID `json:"import_id"`
//	}
//
//	func (CommitImportArgs) Kind() string { return "inventory.commit_import" }
//
// cmd/api dùng client chỉ insert, cmd/worker dùng client chạy job; bọc client
// rồi truyền Enqueuer cho repository/service:
//
//	client, err := jobs.NewInsertClient(pool)                   // cmd/api
//	client, err := jobs.NewWorkerClient(pool, workers, periodic) // cmd/worker
//	enq := jobs.NewRiver(client)
//
// Job đi kèm một thay đổi dữ liệu thì repository enqueue bằng EnqueueTx, trong
// cùng database.WithTx với thay đổi đó: commit thì job mới tồn tại, rollback
// thì job mất theo (transactional outbox). Service quyết định có cần job hay
// không, nhưng không bao giờ thấy transaction:
//
//	// inventory/repository/import_repository.go
//	func (r *ImportRepository) SaveCommitting(ctx context.Context, im *domain.Import) error {
//		return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
//			if err := r.q.WithTx(tx).MarkImportCommitting(ctx, im.ID); err != nil {
//				return err
//			}
//			return r.jobs.EnqueueTx(ctx, tx, job.CommitImportArgs{
//				ActorArgs: jobs.ActorArgsFrom(ctx),
//				ImportID:  im.ID,
//			}, jobs.WithUniqueArgs())
//		})
//	}
//
// Worker (trong worker/ của module) đặt lại actor rồi gọi đúng một method của
// service:
//
//	func (w *CommitImportWorker) Work(ctx context.Context, j *river.Job[job.CommitImportArgs]) error {
//		ctx, err := jobs.RestoreActor(ctx, j.Args.ActorID, w.loadActor)
//		if err != nil {
//			return err
//		}
//		return w.svc.CommitImport(ctx, j.Args.ImportID)
//	}
//
// Job không gắn với thay đổi nào thì service gọi thẳng Enqueue, không có đảm
// bảo trên. Scheduled job chạy bằng SystemActor.
//
// Worker có thể chạy lại một job nhiều lần (retry, crash giữa chừng), nên
// việc nó làm phải idempotent.
package jobs

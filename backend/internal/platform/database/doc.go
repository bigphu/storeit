// Package database mở pool Postgres (pgxpool) dùng chung cho cả app.
//
// Mở một lần trong main, ping luôn nên sai URL hay DB chưa lên thì dừng ngay
// lúc khởi động, không đợi tới request đầu tiên:
//
//	pool, err := database.Open(ctx, cfg.DatabaseURL)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer database.Close(pool)
//
// Repository nhận *pgxpool.Pool, không tự mở kết nối. Method nào ghi thì làm
// hết thay đổi đó trong một WithTx, gồm cả ghi event vào outbox, để thay đổi
// và event cùng commit hoặc cùng mất. Service không bao giờ thấy transaction:
//
//	func (r *AssetRepository) SaveCheckOut(ctx context.Context, a *domain.Asset) error {
//		return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
//			n, err := r.q.WithTx(tx).UpdateAssetCustody(ctx, ...)
//			if err != nil {
//				return err
//			}
//			if n == 0 {
//				return domain.ErrAssetChanged // lỗi domain đi ra nguyên vẹn
//			}
//			return r.outbox.Append(ctx, tx, events...)
//		})
//	}
package database

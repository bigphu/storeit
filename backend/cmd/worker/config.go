package main

import (
	"storeit/internal/identity"
	"storeit/internal/inventory"
	"storeit/internal/platform/database"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/logger"
	"storeit/internal/platform/mail"
)

// workerConfig: worker không phát token nên không có JWT
type workerConfig struct {
	Log       logger.Config    // LOG_*
	DB        database.Config  // DB_*, PG*
	Jobs      jobs.Config      // JOBS_*
	Identity  identity.Config  // IDENTITY_* (thời hạn giữ phiên, link trong thư)
	Inventory inventory.Config // INVENTORY_* (link trong thư mượn, múi giờ của ngày quá hạn)
	Mail      mail.Config      // MAIL_* (thư mời, đặt lại mật khẩu, thư mượn tài sản)
}

package main

import (
	"storeit/internal/identity"
	"storeit/internal/platform/database"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/logger"
	"storeit/internal/platform/server"
)

// serverConfig chỉ liệt kê khối cấu hình binary này dùng. Tên biến env, mặc
// định và Validate nằm ở package sở hữu khối đó; config.Load tự gọi Validate.
type serverConfig struct {
	Log      logger.Config   // LOG_*
	HTTP     server.Config   // HTTP_*
	DB       database.Config // DB_*, PG*
	JWT      jwt.Config      // JWT_*
	Identity identity.Config // IDENTITY_*, ADMIN_*
}

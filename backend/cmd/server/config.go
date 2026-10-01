package main

import "storeit/internal/platform/logger"

// serverConfig chỉ liệt kê khối cấu hình binary này thật sự dùng. Tên biến env,
// mặc định và Validate nằm ở package sở hữu khối đó; config.Load tự gọi
// Validate của từng khối, nên ở đây không cần viết Validate.
//
// Khi dựng API server (M0) thì thêm đúng các khối được dùng:
//
//	HTTP    server.Config   // HTTP_*
//	DB      database.Config // DB_*, PG*
//	JWT     jwt.Config      // JWT_*
//	Storage storage.Config  // STORAGE_DIR
type serverConfig struct {
	Log logger.Config
}

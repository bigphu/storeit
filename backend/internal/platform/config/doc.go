// Package config đọc env vào struct cấu hình của binary lúc khởi động. Thiếu
// hoặc sai thì dừng luôn.
//
// Mỗi package (database, server, jwt...) sở hữu khối cấu hình của mình: tên
// biến env đầy đủ, mặc định và Validate. Binary chỉ liệt kê khối nó dùng, trong
// cmd/<binary>/config.go, không thêm envPrefix:
//
//	type serverConfig struct {
//		Log  logger.Config
//		HTTP server.Config
//		DB   database.Config
//		JWT  jwt.Config
//	}
//
//	var cfg serverConfig
//	if err := config.Load(&cfg); err != nil {
//		log.Fatal(err)
//	}
//
// Load tự gọi Validate của từng khối. Binary chỉ viết Validate khi cần ràng
// buộc giữa các khối với nhau.
package config

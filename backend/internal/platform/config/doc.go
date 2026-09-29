// Package config đọc env vào struct lúc khởi động. Thiếu hoặc sai thì dừng luôn.
//
//	type Config struct {
//		Log logger.Config `envPrefix:"LOG_"`
//		JWT jwt.Config
//	}
//
//	func (c *Config) Validate() error { return c.JWT.Validate() }
//
//	var cfg Config
//	if err := config.Load(&cfg); err != nil {
//		log.Fatal(err)
//	}
package config

package main

import (
	"storeit/internal/platform/database"
	"storeit/internal/platform/logger"
)

// migrateConfig chỉ cần log và DB
type migrateConfig struct {
	Log logger.Config
	DB  database.Config
}

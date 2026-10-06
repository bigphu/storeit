package main

import (
	"storeit/internal/identity"
	"storeit/internal/inventory"
	"storeit/internal/platform/database"
	"storeit/internal/platform/logger"
	"storeit/internal/seed"
)

// seedConfig chỉ liệt kê khối cấu hình binary này dùng
type seedConfig struct {
	Log       logger.Config    // LOG_*
	DB        database.Config  // DB_*, PG*
	Identity  identity.Config  // IDENTITY_* (module cần config hợp lệ)
	Inventory inventory.Config // INVENTORY_*
	Seed      seed.Config      // SEED_PASSWORD
}

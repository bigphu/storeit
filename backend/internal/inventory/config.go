package inventory

import "fmt"

// Config của inventory; binary liệt kê khối này trong cmd/<bin>/config.go
type Config struct {
	// Số dòng tối đa một lần export Excel; 0 là mặc định của service (50.000)
	ExportMaxRows int `env:"INVENTORY_EXPORT_MAX_ROWS" envDefault:"50000"`
}

func (c Config) Validate() error {
	if c.ExportMaxRows < 0 || c.ExportMaxRows > 1_000_000 {
		return fmt.Errorf("inventory: INVENTORY_EXPORT_MAX_ROWS must be between 1 and 1000000")
	}
	return nil
}

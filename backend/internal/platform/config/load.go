package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Validator interface {
	Validate() error
}

func Load(cfg Validator) error {
	if err := env.Parse(cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

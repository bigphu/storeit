package identity

import (
	"testing"
	"time"

	"github.com/caarlos0/env/v11"
)

func TestConfig_FromEnvDefaults(t *testing.T) {
	var cfg struct{ Identity Config }
	if err := env.ParseWithOptions(&cfg, env.Options{Environment: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	want := Config{
		RefreshSlidingTTL:  336 * time.Hour,
		RefreshAbsoluteTTL: 720 * time.Hour,
		RefreshGracePeriod: 30 * time.Second,
		RefreshRetention:   720 * time.Hour,
		CookieSecure:       true,
	}
	if cfg.Identity != want {
		t.Errorf("got %+v, want %+v", cfg.Identity, want)
	}
	// Dựng tay để trống thì giống hệt mặc định của env (trừ cờ bool)
	if got := (Config{CookieSecure: true}).withDefaults(); got != want {
		t.Errorf("withDefaults = %+v, want %+v", got, want)
	}
}

func TestConfig_Validate(t *testing.T) {
	if err := (Config{}).Validate(); err != nil {
		t.Errorf("zero config gets defaults, want valid: %v", err)
	}
	for name, cfg := range map[string]Config{
		"sliding beyond absolute": {RefreshSlidingTTL: 48 * time.Hour, RefreshAbsoluteTTL: 24 * time.Hour},
		"negative grace":          {RefreshGracePeriod: -time.Second},
		"admin email only":        {AdminEmail: "admin@storeit.example"},
		"admin password only":     {AdminPassword: "correct-horse-battery"},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

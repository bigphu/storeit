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
		AppURL:             "http://localhost:3000",
		InviteTTL:          72 * time.Hour,
		ResetTTL:           time.Hour,
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
		"negative invite ttl":     {InviteTTL: -time.Hour},
		"negative reset ttl":      {ResetTTL: -time.Minute},
		"app url without scheme":  {AppURL: "storeit.example"},
		"app url not http":        {AppURL: "ftp://storeit.example"},
		"app url with query":      {AppURL: "https://storeit.example/?x=1"},
		"app url with fragment":   {AppURL: "https://storeit.example/#a"},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestConfig_AppURL(t *testing.T) {
	for _, u := range []string{"https://storeit.example", "http://localhost:5173/", "https://example.com/storeit"} {
		if err := (Config{AppURL: u}).Validate(); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
}

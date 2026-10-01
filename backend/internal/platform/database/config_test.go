package database

import (
	"testing"
	"time"

	"github.com/caarlos0/env/v11"
)

type appConfig struct {
	DB Config // tên biến do package đặt, binary không thêm prefix
}

func TestConfig_FromEnv(t *testing.T) {
	var cfg appConfig
	err := env.ParseWithOptions(&cfg, env.Options{Environment: map[string]string{
		"DB_URL":       "postgres://app_user@db/storeit",
		"DB_MAX_CONNS": "40",
		"DB_MIN_CONNS": "2",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := defaultConfig()
	want.URL, want.MaxConns, want.MinConns = "postgres://app_user@db/storeit", 40, 2
	if cfg.DB != want {
		t.Errorf("got %+v, want %+v", cfg.DB, want)
	}
}

// envDefault trong tag và withDefaults phải cùng một bộ giá trị, nếu không
// chạy thật (đọc env) và dựng tay trong test sẽ ra pool khác nhau
func TestConfig_EnvDefaultsMatchZeroValue(t *testing.T) {
	var cfg appConfig
	if err := env.ParseWithOptions(&cfg, env.Options{Environment: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	if got := (Config{}).withDefaults(); cfg.DB != got {
		t.Errorf("env defaults %+v != withDefaults %+v", cfg.DB, got)
	}
}

func TestConfig_Validate(t *testing.T) {
	if err := (Config{MaxConns: 5, MinConns: 6}).Validate(); err == nil {
		t.Error("want error when MinConns > MaxConns")
	}
	if err := (Config{}).Validate(); err != nil {
		t.Errorf("zero Config gets defaults, want valid: %v", err)
	}
}

func TestParseConfig_AppliesPoolSettings(t *testing.T) {
	t.Setenv("PGPASSWORD_FILE", "")
	cfg := Config{
		// pool_max_conns trong URL bị Config đè
		URL:                   "postgres://app_user:pw@localhost/storeit?pool_max_conns=3",
		MaxConns:              12,
		MinConns:              1,
		MaxConnLifetime:       2 * time.Hour,
		MaxConnLifetimeJitter: time.Minute,
		MaxConnIdleTime:       10 * time.Minute,
		HealthCheckPeriod:     30 * time.Second,
		ConnectTimeout:        3 * time.Second,
	}

	pc, err := parseConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if pc.MaxConns != 12 || pc.MinConns != 1 {
		t.Errorf("conns = %d/%d, want 12/1", pc.MaxConns, pc.MinConns)
	}
	if pc.MaxConnLifetime != 2*time.Hour || pc.MaxConnLifetimeJitter != time.Minute ||
		pc.MaxConnIdleTime != 10*time.Minute || pc.HealthCheckPeriod != 30*time.Second {
		t.Errorf("durations = %v %v %v %v", pc.MaxConnLifetime, pc.MaxConnLifetimeJitter,
			pc.MaxConnIdleTime, pc.HealthCheckPeriod)
	}
	if pc.ConnConfig.ConnectTimeout != 3*time.Second {
		t.Errorf("ConnectTimeout = %v, want 3s", pc.ConnConfig.ConnectTimeout)
	}
}

// Config dựng tay chỉ có URL (vd trong test) vẫn có pool đúng mặc định
func TestParseConfig_ZeroFieldsGetDefaults(t *testing.T) {
	t.Setenv("PGPASSWORD_FILE", "")
	pc, err := parseConfig(Config{URL: "postgres://app_user:pw@localhost/storeit"})
	if err != nil {
		t.Fatal(err)
	}
	def := defaultConfig()
	if pc.MaxConns != def.MaxConns || pc.ConnConfig.ConnectTimeout != def.ConnectTimeout {
		t.Errorf("MaxConns=%d ConnectTimeout=%v, want %d %v",
			pc.MaxConns, pc.ConnConfig.ConnectTimeout, def.MaxConns, def.ConnectTimeout)
	}
}

func TestConfig_ValidateRejectsNegative(t *testing.T) {
	for name, cfg := range map[string]Config{
		"max conns":       {MaxConns: -1},
		"connect timeout": {ConnectTimeout: -time.Second},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: want error for negative value", name)
		}
	}
}

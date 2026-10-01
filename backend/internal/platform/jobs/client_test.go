package jobs

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/caarlos0/env/v11"
	"github.com/riverqueue/river"

	"storeit/internal/platform/logger"
)

// Logger truyền vào là logger River dùng; nil thì slog.Default() (đọc lúc dựng)
func TestClientConfig_Logger(t *testing.T) {
	log := logger.New(&bytes.Buffer{}, logger.Config{})
	workers := river.NewWorkers()

	if got := insertConfig(log).Logger; got != log {
		t.Error("insert: want the given logger")
	}
	if got := workerConfig(log, Config{}, workers, nil).Logger; got != log {
		t.Error("worker: want the given logger")
	}
	if got := insertConfig(nil).Logger; got != slog.Default() {
		t.Error("insert nil: want slog.Default()")
	}
}

// Queue nào job dùng mà worker không nghe thì job nằm chờ mãi
func TestWorkerConfig_ListensOnAppQueues(t *testing.T) {
	cfg := workerConfig(nil, Config{}, river.NewWorkers(), nil)
	for _, q := range []string{QueueDefault, QueueEvents} {
		if _, ok := cfg.Queues[q]; !ok {
			t.Errorf("worker does not listen on queue %q", q)
		}
	}
	if cfg.MaxAttempts != DefaultMaxAttempts || insertConfig(nil).MaxAttempts != DefaultMaxAttempts {
		t.Error("MaxAttempts must be set on both clients")
	}
}

func TestConfig_FromEnv(t *testing.T) {
	var cfg struct{ Jobs Config }
	err := env.ParseWithOptions(&cfg, env.Options{Environment: map[string]string{
		"JOBS_EVENTS_MAX_WORKERS": "4",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := Config{DefaultMaxWorkers: 10, EventsMaxWorkers: 4}
	if cfg.Jobs != want {
		t.Errorf("got %+v, want %+v", cfg.Jobs, want)
	}
	// Dựng tay để trống thì giống hệt mặc định của env
	if got := (Config{}).withDefaults(); got != (Config{DefaultMaxWorkers: 10, EventsMaxWorkers: 10}) {
		t.Errorf("withDefaults = %+v", got)
	}
}

// Số worker mỗi queue lấy từ Config
func TestWorkerConfig_UsesMaxWorkers(t *testing.T) {
	cfg := workerConfig(nil, Config{DefaultMaxWorkers: 3, EventsMaxWorkers: 7}, river.NewWorkers(), nil)
	if got := cfg.Queues[QueueDefault].MaxWorkers; got != 3 {
		t.Errorf("default queue MaxWorkers = %d, want 3", got)
	}
	if got := cfg.Queues[QueueEvents].MaxWorkers; got != 7 {
		t.Errorf("events queue MaxWorkers = %d, want 7", got)
	}
}

func TestConfig_ValidateRejectsNegative(t *testing.T) {
	if err := (Config{EventsMaxWorkers: -1}).Validate(); err == nil {
		t.Error("want error for negative worker count")
	}
	if err := (Config{}).Validate(); err != nil {
		t.Errorf("zero Config gets defaults, want valid: %v", err)
	}
}

package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/caarlos0/env/v11"
)

type appConfig struct {
	Log Config // tên biến do package đặt, binary không thêm prefix
}

func TestConfig_FromEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		// Map rỗng chứ không nil: nil thì env đọc biến môi trường thật
		{"defaults", map[string]string{}, Config{Level: slog.LevelInfo, Format: FormatJSON}, false},
		{"set", map[string]string{"LOG_LEVEL": "debug", "LOG_FORMAT": "Pretty"},
			Config{Level: slog.LevelDebug, Format: FormatPretty}, false},
		{"bad level", map[string]string{"LOG_LEVEL": "loud"}, Config{}, true},
		{"bad format", map[string]string{"LOG_FORMAT": "xml"}, Config{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg appConfig
			err := env.ParseWithOptions(&cfg, env.Options{Environment: tt.env})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %+v", cfg.Log)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Log != tt.want {
				t.Errorf("got %+v, want %+v", cfg.Log, tt.want)
			}
		})
	}
}

func TestNew_JSONWithContextAttrs(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Config{})

	ctx := With(context.Background(), slog.String("request_id", "req-1"))
	ctx = With(ctx, slog.String("actor_id", "acc-7"))
	log.InfoContext(ctx, "asset checked out", "asset_id", "a-3")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %q", buf.String())
	}
	for k, want := range map[string]string{
		"msg": "asset checked out", "asset_id": "a-3", "request_id": "req-1", "actor_id": "acc-7",
	} {
		if got[k] != want {
			t.Errorf("%s = %v, want %q", k, got[k], want)
		}
	}
}

func TestNew_Pretty(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	log := New(&buf, Config{Format: FormatPretty})

	ctx := With(context.Background(), slog.String("request_id", "req-1"))
	log.InfoContext(ctx, "hi", "k", "v")

	if got, want := dropTime(buf.String()), "INFO  hi k=v request_id=req-1"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Hai ctx con từ cùng một cha không được thấy attr của nhau
func TestWith_SiblingsIndependent(t *testing.T) {
	parent := With(context.Background(), slog.Int("a", 1), slog.Int("b", 2))
	c1 := With(parent, slog.String("who", "c1"))
	c2 := With(parent, slog.String("who", "c2"))

	if got := attrsFrom(c1)[2].Value.String(); got != "c1" {
		t.Errorf("c1 who = %q", got)
	}
	if got := attrsFrom(c2)[2].Value.String(); got != "c2" {
		t.Errorf("c2 who = %q", got)
	}
	if n := len(attrsFrom(parent)); n != 2 {
		t.Errorf("parent has %d attrs, want 2", n)
	}
}

func TestWith_NoCtxAttrs(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, Config{}).Info("plain")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil || got["msg"] != "plain" {
		t.Errorf("got %q", buf.String())
	}
}

// Attr thêm vào scope từ ctx con (vd auth gắn actor sau khi đọc token) phải
// hiện cả trong log ghi bằng ctx cha (vd dòng access log của RequestLogger)
func TestScope_AttrAddedByChildVisibleToParent(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Config{})

	parent := WithScope(With(context.Background(), slog.String("request_id", "req-1")))
	child := With(parent, slog.String("step", "auth"))
	AddToScope(child, slog.String("actor_id", "acc-7"))
	log.InfoContext(parent, "http request")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %q", buf.String())
	}
	if got["request_id"] != "req-1" || got["actor_id"] != "acc-7" {
		t.Errorf("request_id=%v actor_id=%v, want req-1 acc-7", got["request_id"], got["actor_id"])
	}
	if _, ok := got["step"]; ok {
		t.Error("attr from child With leaked to parent")
	}
}

// WithScope lần hai trên cùng nhánh dùng lại scope cũ, để attr không bị kẹt ở
// scope trong mà ctx ngoài không thấy
func TestScope_NestedReusesOuter(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Config{})

	outer := WithScope(context.Background())
	inner := WithScope(outer)
	AddToScope(inner, slog.String("actor_id", "acc-7"))
	log.InfoContext(outer, "done")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %q", buf.String())
	}
	if got["actor_id"] != "acc-7" {
		t.Errorf("actor_id = %v, want acc-7", got["actor_id"])
	}
}

// Không có scope (vd job chạy ngoài HTTP) thì AddToScope bỏ qua, không panic
func TestScope_AddWithoutScopeIsNoop(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Config{})

	ctx := context.Background()
	AddToScope(ctx, slog.String("actor_id", "acc-7"))
	log.InfoContext(ctx, "done")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %q", buf.String())
	}
	if _, ok := got["actor_id"]; ok {
		t.Errorf("actor_id = %v, want absent", got["actor_id"])
	}
}

// Attr của ctx là của cả dòng log, không được rơi vào group đang mở của logger
// (vd log.WithGroup("job")), nếu không tìm theo request_id sẽ không thấy
func TestNew_ContextAttrsStayTopLevelInGroup(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, Config{}).With("svc", "api").WithGroup("job").With("id", "j-1")

	ctx := WithScope(With(context.Background(), slog.String("request_id", "req-1")))
	AddToScope(ctx, slog.String("actor_id", "acc-7"))
	log.InfoContext(ctx, "done", "k", 1)

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %q", buf.String())
	}
	for k, want := range map[string]any{"request_id": "req-1", "actor_id": "acc-7", "svc": "api"} {
		if got[k] != want {
			t.Errorf("%s = %v, want %v (line: %s)", k, got[k], want, buf.String())
		}
	}
	job, _ := got["job"].(map[string]any)
	if job["id"] != "j-1" || job["k"] != float64(1) || len(job) != 2 {
		t.Errorf("job = %v, want {id:j-1 k:1}", job)
	}
}

// Logger của request đi theo ctx, để code platform (vd web.WriteProblem) không
// phải đọc slog.Default()
func TestFromContext(t *testing.T) {
	if got := FromContext(context.Background()); got != slog.Default() {
		t.Error("no logger in ctx: want slog.Default()")
	}
	log := New(&bytes.Buffer{}, Config{})
	if got := FromContext(NewContext(context.Background(), log)); got != log {
		t.Error("want the logger stored by NewContext")
	}
}

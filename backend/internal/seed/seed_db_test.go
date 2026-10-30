package seed_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/identity"
	idservice "storeit/internal/identity/service"
	"storeit/internal/inventory"
	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/seed"
)

const password = "seed-password-123"

// Administrator lấy từ ADMIN_EMAIL/ADMIN_PASSWORD như cmd/server
const adminEmail, adminPassword = "admin@storeit.test", "admin-password-123"

type env struct {
	identity  *identity.Module
	inventory *inventory.Module
	pool      *pgxpool.Pool
}

// newEnv dựng hai module như cmd/seed trên DB test của package (DB riêng, trống)
func newEnv(t *testing.T) env {
	t.Helper()
	pool := dbtest.Pool(t)
	client, err := jobs.NewInsertClient(pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	outbox := events.NewOutbox(events.NewRegistry(), client)
	tokens, err := jwt.New(jwt.Config{
		Keys:      "v1:" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))),
		ActiveKID: "v1", Issuer: "storeit", Audience: "storeit-api", AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	idm, err := identity.New(identity.Deps{
		Pool: pool, Tokens: tokens, Outbox: outbox, Jobs: jobs.NewRiver(client),
		Config: identity.Config{AdminEmail: adminEmail, AdminPassword: adminPassword},
	})
	if err != nil {
		t.Fatal(err)
	}
	invm, err := inventory.New(inventory.Deps{Pool: pool, Outbox: outbox, Jobs: jobs.NewRiver(client), Accounts: idm.AccountReader()})
	if err != nil {
		t.Fatal(err)
	}
	return env{identity: idm, inventory: invm, pool: pool}
}

func (e env) deps(pw string) seed.Deps {
	return seed.Deps{Accounts: e.identity, Inventory: e.inventory.Service(), Password: pw}
}

// Thứ tự quan trọng (DB dùng chung trong package): mật khẩu sai không ghi gì,
// rồi seed lần đầu, rồi lần hai bỏ qua
func TestRun_RejectsBadPasswordBeforeWriting(t *testing.T) {
	e := newEnv(t)
	if _, err := seed.Run(context.Background(), e.deps("short")); err == nil {
		t.Fatal("weak password accepted")
	}
	ctx := auth.WithActor(context.Background(), auth.SystemActor)
	ts, err := e.inventory.Service().ListAssetTypes(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range ts {
		if !x.IsSystem {
			t.Fatalf("type %s written despite a bad password", x.Code)
		}
	}
}

func TestRun_SeedsOnceThenSkips(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	sum, err := seed.Run(ctx, e.deps(password))
	if err != nil {
		t.Fatal(err)
	}
	want := seed.Summary{Accounts: 6, Statuses: 1, Types: 10, Assets: 300, Profiles: 3, Tickets: 6, Loans: 2}
	got := sum
	got.Retired = 0
	if got != want {
		t.Errorf("summary = %+v, want %+v (retired any)", sum, want)
	}
	if sum.Retired < 3 || sum.Retired > 20 {
		t.Errorf("retired = %d, want a handful", sum.Retired)
	}

	// Đăng nhập được bằng SEED_PASSWORD, tên tiếng Việt giữ nguyên
	sess, err := e.identity.Service().Login(ctx, "manager@storeit.test", password, idservice.Device{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if sess.Account.Name != "Trần Thị Mai" {
		t.Errorf("name = %q", sess.Account.Name)
	}

	// Seed trên DB chưa có account vẫn có Administrator (Bootstrap chạy trước)
	if _, err := e.identity.Service().Login(ctx, adminEmail, adminPassword, idservice.Device{}); err != nil {
		t.Errorf("bootstrap admin missing after seeding an empty DB: %v", err)
	}

	// Đúng 300 tài sản kể cả đã retire
	sys := auth.WithActor(ctx, auth.SystemActor)
	_, total, err := e.inventory.Service().ListAssets(sys, domain.AssetFilter{IncludeRetired: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if total != 300 {
		t.Errorf("assets in DB = %d, want 300", total)
	}

	// Mượn (US-10): phiếu ở mọi trạng thái, hai khoản đang mượn, bốn loại cho mượn được, không
	// còn status tự tạo "Cho mượn"
	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		if err := e.pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for state, want := range map[string]int{"waiting": 3, "approved": 1, "handed_over": 1, "rejected": 1} {
		if n := count(`SELECT count(*) FROM inventory.borrow_tickets WHERE state = $1`, state); n != want {
			t.Errorf("%s tickets = %d, want %d", state, n, want)
		}
	}
	if n := count(`SELECT count(*) FROM inventory.loans WHERE returned_on IS NULL`); n != 2 {
		t.Errorf("active loans = %d, want 2", n)
	}
	if n := count(`SELECT count(*) FROM inventory.asset_statuses WHERE name = 'Cho mượn'`); n != 0 {
		t.Errorf(`status "Cho mượn" still seeded`)
	}
	if n := count(`SELECT count(*) FROM inventory.asset_types WHERE can_borrow AND code IN ('LAPTOP', 'MONITOR', 'PROJECTOR', 'PHONE')`); n != 4 {
		t.Errorf("borrowable demo types = %d, want 4", n)
	}
	if n := count(`SELECT count(*) FROM inventory.asset_types WHERE can_borrow`); n != 4 {
		t.Errorf("all borrowable types = %d, want only the four", n)
	}

	// Lần hai: không ghi gì
	again, err := seed.Run(ctx, e.deps(password))
	if err != nil || !again.Skipped || again.Assets != 0 {
		t.Fatalf("second run = %+v, %v; want skipped", again, err)
	}
}

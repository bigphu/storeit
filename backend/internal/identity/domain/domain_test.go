package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDecide(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	grace := 30 * time.Second
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	live := func() RefreshState {
		return RefreshState{
			ExpiresAt:         now.Add(time.Hour),
			AbsoluteExpiresAt: now.Add(24 * time.Hour),
			AccountActive:     true,
		}
	}
	tests := []struct {
		name   string
		mutate func(*RefreshState)
		want   RefreshDecision
	}{
		{"fresh token rotates", func(*RefreshState) {}, RefreshRotate},
		{"revoked family rejects", func(s *RefreshState) { s.RevokedAt = ago(time.Minute) }, RefreshReject},
		// Thu hồi thắng mọi thứ khác: family đã chết thì không thu hồi lại
		{"revoked wins over reuse", func(s *RefreshState) {
			s.RevokedAt = ago(time.Minute)
			s.UsedAt = ago(time.Hour)
		}, RefreshReject},
		{"disabled account", func(s *RefreshState) { s.AccountActive = false }, RefreshAccountDisabled},
		{"absolute expiry passed", func(s *RefreshState) { s.AbsoluteExpiresAt = now }, RefreshExpired},
		{"used within grace", func(s *RefreshState) { s.UsedAt = ago(10 * time.Second) }, RefreshGrace},
		{"used exactly at grace edge", func(s *RefreshState) { s.UsedAt = ago(grace) }, RefreshGrace},
		{"used after grace is reuse", func(s *RefreshState) { s.UsedAt = ago(31 * time.Second) }, RefreshReuse},
		{"unused but expired", func(s *RefreshState) { s.ExpiresAt = now }, RefreshReject},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := live()
			tt.mutate(&s)
			if got := s.Decide(now, grace); got != tt.want {
				t.Errorf("Decide = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	for _, tt := range []struct {
		pw string
		ok bool
	}{
		{strings.Repeat("a", 11), false},
		{strings.Repeat("a", 12), true},
		{strings.Repeat("a", 72), true},
		{strings.Repeat("a", 73), false},
		// Đếm theo byte (giới hạn của bcrypt): 6 ký tự 2 byte = 12 byte
		{strings.Repeat("é", 6), true},
		{strings.Repeat("é", 37), false},
	} {
		err := ValidatePassword(tt.pw)
		if (err == nil) != tt.ok {
			t.Errorf("ValidatePassword(%d bytes) = %v, want ok=%v", len(tt.pw), err, tt.ok)
		}
		if err != nil && !errors.Is(err, ErrWeakPassword) {
			t.Errorf("err = %v, want ErrWeakPassword", err)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	got, err := NormalizeEmail("  Admin@StoreIT.Example ")
	if err != nil || got != "admin@storeit.example" {
		t.Errorf("NormalizeEmail = %q, %v", got, err)
	}
	for _, bad := range []string{"", "nope", "a@", "@b", "a@b@c", "a b@c.d"} {
		if _, err := NormalizeEmail(bad); !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("NormalizeEmail(%q) err = %v, want ErrInvalidEmail", bad, err)
		}
	}
}

func TestCanLogin(t *testing.T) {
	if err := (Account{Active: true}).CanLogin(); err != nil {
		t.Errorf("active: %v", err)
	}
	if err := (Account{Active: false}).CanLogin(); !errors.Is(err, ErrAccountDisabled) {
		t.Errorf("disabled: err = %v, want ErrAccountDisabled", err)
	}
}

func TestRoleGuards(t *testing.T) {
	system := Role{ID: AdministratorRoleID, Name: "Administrator", IsSystem: true}
	custom := Role{ID: uuid.New(), Name: "Auditor"}

	if err := system.CanRename(); !errors.Is(err, ErrSystemRole) {
		t.Errorf("rename system: %v", err)
	}
	if err := custom.CanRename(); err != nil {
		t.Errorf("rename custom: %v", err)
	}
	if err := system.CanDelete(0); !errors.Is(err, ErrSystemRole) {
		t.Errorf("delete system: %v", err)
	}
	if err := custom.CanDelete(2); !errors.Is(err, ErrRoleInUse) {
		t.Errorf("delete assigned: %v", err)
	}
	if err := custom.CanDelete(0); err != nil {
		t.Errorf("delete unassigned custom: %v", err)
	}

	// Administrator phải giữ quyền quản lý role, nếu không không ai sửa lại được
	if err := CheckRolePermissions(AdministratorRoleID, []string{PermAccountRead}); !errors.Is(err, ErrLockout) {
		t.Errorf("admin without role.manage: %v", err)
	}
	if err := CheckRolePermissions(AdministratorRoleID, []string{PermRoleManage}); err != nil {
		t.Errorf("admin with role.manage: %v", err)
	}
	if err := CheckRolePermissions(custom.ID, nil); err != nil {
		t.Errorf("custom role may have no permissions: %v", err)
	}
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
)

// Người có identity.account.manage qua một role tự tạo (vd "HR") không được
// gán cho ai (kể cả mình) quyền mình không có, cũng không được gỡ role, khoá
// hay mở khoá account nắm quyền mình không có
func TestCannotGrantOrActBeyondOwnPermissions(t *testing.T) {
	e := newEnv(t)
	bg := context.Background()
	hrRole, _ := e.roles.Create(bg, domain.Role{Name: "HR", Permissions: []string{domain.PermAccountRead, domain.PermAccountManage}})
	hr := e.seed(t, "hr@storeit.test", true, hrRole.ID)
	admin := e.seed(t, "admin@storeit.test", true, domain.AdministratorRoleID)
	admin2 := e.seed(t, "admin2@storeit.test", true, domain.AdministratorRoleID)
	emp := e.seed(t, "emp@storeit.test", true, domain.EmployeeRoleID)
	hrCtx := as(hr.ID, domain.PermAccountRead, domain.PermAccountManage)
	adminCtx := as(admin.ID, domain.PermAccountRead, domain.PermAccountManage, domain.PermRoleRead, domain.PermRoleManage)

	denied := map[string]func() error{
		"grant self Administrator": func() error {
			_, err := e.svc.AssignRoles(hrCtx, hr.ID, []uuid.UUID{hrRole.ID, domain.AdministratorRoleID})
			return err
		},
		"strip Administrator from an admin": func() error {
			_, err := e.svc.AssignRoles(hrCtx, admin.ID, []uuid.UUID{domain.EmployeeRoleID})
			return err
		},
		"disable an admin": func() error {
			_, err := e.svc.DisableAccount(hrCtx, admin.ID)
			return err
		},
		"sign an admin out everywhere": func() error {
			_, err := e.svc.SignOutEverywhere(hrCtx, admin.ID)
			return err
		},
	}
	for name, call := range denied {
		if err := call(); !errors.Is(err, domain.ErrExceedsOwnPermissions) || status(err) != 403 {
			t.Errorf("%s: %v, want 403 ErrExceedsOwnPermissions", name, err)
		}
	}

	// Mở khoá một admin đã bị khoá cũng là trao lại quyền đó
	if _, err := e.svc.DisableAccount(adminCtx, admin2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.EnableAccount(hrCtx, admin2.ID); !errors.Is(err, domain.ErrExceedsOwnPermissions) {
		t.Errorf("enable an admin: %v, want ErrExceedsOwnPermissions", err)
	}

	// Trong phạm vi quyền của mình thì được
	if _, err := e.svc.AssignRoles(hrCtx, emp.ID, []uuid.UUID{domain.EmployeeRoleID, hrRole.ID}); err != nil {
		t.Errorf("grant HR to an employee: %v", err)
	}
	if _, err := e.svc.SignOutEverywhere(hrCtx, emp.ID); err != nil {
		t.Errorf("sign an employee out: %v", err)
	}
	if _, err := e.svc.DisableAccount(hrCtx, emp.ID); err != nil {
		t.Errorf("disable an employee: %v", err)
	}
	// Administrator có đủ quyền nên làm được hết
	if _, err := e.svc.AssignRoles(adminCtx, emp.ID, []uuid.UUID{domain.AdministratorRoleID}); err != nil {
		t.Errorf("admin grants Administrator: %v", err)
	}
}

// identity.role.manage không đủ để tạo hay sửa role mang quyền mình không có
func TestCannotShapeRolesBeyondOwnPermissions(t *testing.T) {
	e := newEnv(t)
	bg := context.Background()
	rm := e.seed(t, "rm@storeit.test", true)
	rmCtx := as(rm.ID, domain.PermRoleRead, domain.PermRoleManage)

	if _, err := e.svc.CreateRole(rmCtx, "Escalate", "", []string{domain.PermAccountManage}); !errors.Is(err, domain.ErrExceedsOwnPermissions) {
		t.Errorf("create role with account.manage: %v", err)
	}
	r, err := e.svc.CreateRole(rmCtx, "Viewer", "", []string{domain.PermRoleRead})
	if err != nil {
		t.Fatalf("create role within own permissions: %v", err)
	}
	if _, err := e.svc.UpdateRolePermissions(rmCtx, r.ID, []string{domain.PermRoleRead, domain.PermAccountRead}); !errors.Is(err, domain.ErrExceedsOwnPermissions) {
		t.Errorf("add account.read: %v", err)
	}
	// Gỡ một quyền mình không có cũng không được
	other, _ := e.roles.Create(bg, domain.Role{Name: "Auditor", Permissions: []string{domain.PermAccountRead}})
	if _, err := e.svc.UpdateRolePermissions(rmCtx, other.ID, []string{}); !errors.Is(err, domain.ErrExceedsOwnPermissions) {
		t.Errorf("remove account.read: %v", err)
	}
	// Đổi trong phạm vi quyền của mình thì được
	if _, err := e.svc.UpdateRolePermissions(rmCtx, r.ID, []string{domain.PermRoleRead, domain.PermRoleManage}); err != nil {
		t.Errorf("add role.manage (held): %v", err)
	}
}

// Account bị khoá còn access token (tối đa 15 phút) cũng không đổi được mật khẩu
func TestChangePasswordDisabledAccount(t *testing.T) {
	e := newEnv(t)
	a := e.seed(t, "lan@storeit.test", false)
	if err := e.svc.ChangePassword(as(a.ID), goodPassword, "brand-new-password", ""); !errors.Is(err, domain.ErrAccountDisabled) {
		t.Errorf("disabled account changes password: %v, want ErrAccountDisabled", err)
	}
}

func TestUpdateAccountMemberConflict(t *testing.T) {
	e := newEnv(t)
	a := e.seed(t, "lan@storeit.test", true)
	ctx := as(uuid.New(), domain.PermAccountManage)
	member := uuid.New()
	_, err := e.svc.UpdateAccount(ctx, a.ID, domain.ProfileChange{MemberID: &member, ClearMember: true, Version: a.Version})
	if !errors.Is(err, domain.ErrMemberConflict) || status(err) != 422 {
		t.Errorf("member_id with clear_member_id: %v, want 422 ErrMemberConflict", err)
	}
}

// Ký tự điều khiển (xuống dòng, tab, NUL) trong tên đi vào thư và log
func TestNamesRejectControlCharacters(t *testing.T) {
	e := newEnv(t)
	a := e.seed(t, "lan@storeit.test", true)
	ctx := as(uuid.New(), domain.PermAccountManage, domain.PermRoleManage)
	for _, name := range []string{"Lan\nBcc: x@y", "A\tB", "nul\x00", "bell\x07"} {
		if _, err := e.svc.CreateAccount(ctx, CreateAccountInput{Email: "n-" + uuid.NewString()[:6] + "@storeit.test", Name: name}); !errors.Is(err, domain.ErrInvalidName) {
			t.Errorf("CreateAccount(%q): %v", name, err)
		}
		if _, err := e.svc.UpdateAccount(ctx, a.ID, domain.ProfileChange{Name: &name, Version: a.Version}); !errors.Is(err, domain.ErrInvalidName) {
			t.Errorf("UpdateAccount(%q): %v", name, err)
		}
		if _, err := e.svc.CreateRole(ctx, name, "", nil); !errors.Is(err, domain.ErrInvalidName) {
			t.Errorf("CreateRole(%q): %v", name, err)
		}
	}
	// Chữ có dấu và khoảng trắng giữa tên vẫn hợp lệ
	if _, err := e.svc.CreateAccount(ctx, CreateAccountInput{Email: "ok@storeit.test", Name: "Nguyễn Thị Lan"}); err != nil {
		t.Errorf("Vietnamese name: %v", err)
	}
}

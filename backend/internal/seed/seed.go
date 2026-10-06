package seed

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/google/uuid"

	iddomain "storeit/internal/identity/domain"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/service"
	"storeit/internal/platform/auth"
)

// Config của cmd/seed
type Config struct {
	// Mật khẩu chung của mọi account demo (bắt buộc; đặt trong .env)
	Password string `env:"SEED_PASSWORD"`
}

func (c Config) Validate() error {
	if c.Password == "" {
		return errors.New("seed: SEED_PASSWORD is required")
	}
	return nil
}

// AccountSeeder là phần identity.Module mà seed dùng
type AccountSeeder interface {
	// Bootstrap tạo Administrator từ ADMIN_EMAIL khi DB chưa có account nào
	Bootstrap(ctx context.Context) error
	SeedAccount(ctx context.Context, email, name, password string, roleIDs []uuid.UUID) (uuid.UUID, error)
}

type Deps struct {
	Accounts  AccountSeeder
	Inventory *service.Service
	Password  string
}

// Summary: số bản ghi đã tạo; Skipped khi DB đã có dữ liệu (không ghi gì)
type Summary struct {
	Skipped                                              bool
	Accounts, Statuses, Types, Assets, Retired, Profiles int
}

// Run ghi toàn bộ dữ liệu demo. DB đã có loại tài sản không phải hệ thống thì bỏ
// qua (Skipped). Mỗi lần gọi module tự commit; lỗi giữa chừng để lại dữ liệu dở,
// make db-reset để làm lại.
func Run(ctx context.Context, d Deps) (Summary, error) {
	if err := iddomain.ValidatePassword(d.Password); err != nil {
		return Summary{}, fmt.Errorf("seed: SEED_PASSWORD: %w", err)
	}
	// Administrator phải có trước account demo: Bootstrap chỉ tạo khi DB chưa có
	// account nào, mà seed có thể chạy trước lần khởi động đầu của server
	if err := d.Accounts.Bootstrap(ctx); err != nil {
		return Summary{}, err
	}
	sys := auth.WithActor(ctx, auth.SystemActor)
	existing, err := d.Inventory.ListAssetTypes(sys, true)
	if err != nil {
		return Summary{}, err
	}
	for _, t := range existing {
		if !t.IsSystem {
			return Summary{Skipped: true}, nil
		}
	}

	var sum Summary
	owners := map[string]uuid.UUID{}
	for _, a := range accounts {
		id, err := d.Accounts.SeedAccount(ctx, a.Email, a.Name, d.Password, []uuid.UUID{a.Role})
		if err != nil {
			return sum, fmt.Errorf("seed: account %s: %w", a.Email, err)
		}
		if a.Key != "" {
			owners[a.Key] = id
		}
		sum.Accounts++
	}

	st, err := seedStatuses(sys, d.Inventory)
	if err != nil {
		return sum, err
	}
	sum.Statuses = len(statuses)

	r := rand.New(rand.NewPCG(20261006, 1))
	for _, ts := range types {
		t, err := d.Inventory.CreateAssetType(sys, newAssetType(ts))
		if err != nil {
			return sum, fmt.Errorf("seed: type %s: %w", ts.Code, err)
		}
		sum.Types++
		made, retired, err := seedAssets(sys, d.Inventory, r, ts, t, st)
		sum.Assets += made
		sum.Retired += retired
		if err != nil {
			return sum, err
		}
	}

	for _, p := range profiles {
		owner := auth.WithActor(ctx, auth.Actor{AccountID: owners[p.Owner], Permissions: []string{domain.PermAssetExport}})
		if _, err := d.Inventory.CreateExportProfile(owner, service.ExportProfileInput{Name: p.Name, Shared: p.Shared, Layout: p.Layout}); err != nil {
			return sum, fmt.Errorf("seed: profile %s: %w", p.Name, err)
		}
		sum.Profiles++
	}
	return sum, nil
}

// statusIDs: status dùng khi sinh tài sản
type statusIDs struct {
	inUse, loan, repair uuid.UUID
}

func seedStatuses(ctx context.Context, svc *service.Service) (statusIDs, error) {
	cur, err := svc.ListStatuses(ctx, true)
	if err != nil {
		return statusIDs{}, err
	}
	var ids statusIDs
	for _, s := range cur {
		if s.Kind == domain.KindInUse && s.IsDefault {
			ids.inUse = s.ID
		}
	}
	for i, spec := range statuses {
		s, err := svc.CreateStatus(ctx, domain.NewStatus{Name: spec.Name, Kind: spec.Kind, Position: int32(len(cur) + i + 1)})
		if err != nil {
			return statusIDs{}, fmt.Errorf("seed: status %s: %w", spec.Name, err)
		}
		switch spec.Name {
		case loanStatus:
			ids.loan = s.ID
		case repairStatus:
			ids.repair = s.ID
		}
	}
	if ids.inUse == uuid.Nil {
		return statusIDs{}, errors.New("seed: no default in_use status")
	}
	return ids, nil
}

func newAssetType(ts typeSpec) domain.NewAssetType {
	out := domain.NewAssetType{Code: ts.Code, Name: ts.Name, Description: ts.Description}
	for i, a := range ts.Attrs {
		out.Attributes = append(out.Attributes, domain.NewAttribute{
			Key: a.Key, Label: a.Label, DataType: a.Type, Unit: a.Unit, Required: a.Required,
			Position: int32(i + 1), Options: a.Options,
		})
	}
	return out
}

// seedAssets tạo ts.Count tài sản của loại t; trả số đã tạo và số đã retire
func seedAssets(ctx context.Context, svc *service.Service, r *rand.Rand, ts typeSpec, t domain.AssetType, st statusIDs) (int, int, error) {
	options := map[string]map[string]string{} // key -> nhãn -> id option
	for _, a := range t.Attributes {
		options[a.Key] = map[string]string{}
		for _, o := range a.Options {
			options[a.Key][o.Label] = o.ID.String()
		}
	}
	made, retired := 0, 0
	for i := 1; i <= ts.Count; i++ {
		attrs := map[string]any{}
		for _, a := range ts.Attrs {
			// thuộc tính tuỳ chọn để trống khoảng 20%
			if !a.Required && r.IntN(5) == 0 {
				continue
			}
			v := a.Gen(r)
			if a.Type == domain.TypeSelect {
				v = options[a.Key][v.(string)]
			}
			attrs[a.Key] = v
		}
		bought := seedDay.AddDate(0, 0, -(30 + r.IntN(3*365)))
		in := service.AssetInput{
			Name: ts.Models[r.IntN(len(ts.Models))], TypeID: t.ID, PurchaseDate: &bought, Attributes: attrs,
		}
		roll := r.IntN(100)
		switch {
		case roll < 13:
			in.StatusID = &st.inUse
		case roll < 25:
			in.StatusID = &st.loan
		case roll < 30:
			in.StatusID = &st.repair
		}
		if r.IntN(4) == 0 {
			in.Description = "Cấp cho phòng " + []string{"Kế toán", "Kỹ thuật", "Kinh doanh", "Nhân sự"}[r.IntN(4)]
		}
		tag := fmt.Sprintf("%s-%04d", ts.Prefix, i)
		a, err := svc.CreateAsset(ctx, tag, in)
		if err != nil {
			return made, retired, fmt.Errorf("seed: asset %s: %w", tag, err)
		}
		made++
		if roll >= 97 {
			if _, err := svc.RetireAsset(ctx, a.ID, retireReason, a.Version); err != nil {
				return made, retired, fmt.Errorf("seed: retire %s: %w", tag, err)
			}
			retired++
		}
	}
	return made, retired, nil
}

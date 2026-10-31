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
	Tickets, Loans                                       int
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
	people := map[string]uuid.UUID{} // email -> account
	for _, a := range accounts {
		id, err := d.Accounts.SeedAccount(ctx, a.Email, a.Name, d.Password, []uuid.UUID{a.Role})
		if err != nil {
			return sum, fmt.Errorf("seed: account %s: %w", a.Email, err)
		}
		if a.Key != "" {
			owners[a.Key] = id
		}
		people[a.Email] = id
		sum.Accounts++
	}

	st, err := seedStatuses(sys, d.Inventory)
	if err != nil {
		return sum, err
	}
	sum.Statuses = len(statuses)

	r := rand.New(rand.NewPCG(20261006, 1))
	typeIDs := map[string]uuid.UUID{} // code -> loại
	for _, ts := range types {
		t, err := d.Inventory.CreateAssetType(sys, newAssetType(ts))
		if err != nil {
			return sum, fmt.Errorf("seed: type %s: %w", ts.Code, err)
		}
		typeIDs[ts.Code] = t.ID
		sum.Types++
		made, retired, err := seedAssets(sys, d.Inventory, r, ts, t, st)
		sum.Assets += made
		sum.Retired += retired
		if err != nil {
			return sum, err
		}
	}

	if sum.Tickets, sum.Loans, err = seedBorrowing(ctx, d.Inventory, people, typeIDs); err != nil {
		return sum, err
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
	inUse, repair uuid.UUID
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
		if spec.Name == repairStatus {
			ids.repair = s.ID
		}
	}
	if ids.inUse == uuid.Nil {
		return statusIDs{}, errors.New("seed: no default in_use status")
	}
	return ids, nil
}

func newAssetType(ts typeSpec) domain.NewAssetType {
	out := domain.NewAssetType{Code: ts.Code, TagPrefix: ts.Prefix, Name: ts.Name, Description: ts.Description, CanBorrow: ts.Borrow}
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
		case roll < 25:
			in.StatusID = &st.inUse
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

// seedBorrowing tạo phiếu mượn ở mọi trạng thái và hai khoản đang mượn (US-10), qua service như
// người dùng thật: nhân viên gửi phiếu, người quản lý kho duyệt, giao, từ chối, cho mượn trực
// tiếp. Thư được xếp như thường; địa chỉ @storeit.test bị mail guard bỏ.
func seedBorrowing(ctx context.Context, svc *service.Service, people, typeIDs map[string]uuid.UUID) (int, int, error) {
	as := func(email string, perms ...string) context.Context {
		return auth.WithActor(ctx, auth.Actor{AccountID: people[email], Permissions: perms})
	}
	officer := as("officer@storeit.test", domain.PermAssetRead, domain.PermBorrowRequest, domain.PermBorrowManage)
	today := svc.Today()
	week := today.AddDate(0, 0, 7)
	tickets, loans := 0, 0

	ask := func(email, code, note string, reqs ...string) (service.TicketView, error) {
		t, err := svc.CreateTicket(as(email, domain.PermBorrowRequest), service.TicketInput{
			TypeID: typeIDs[code], Requirements: reqs, NeededFrom: today, ReturnBy: &week, Note: note,
		})
		if err != nil {
			return t, fmt.Errorf("seed: ticket for %s by %s: %w", code, email, err)
		}
		tickets++
		return t, nil
	}
	// free: tài sản đầu tiên của loại đang cho mượn được
	free := func(code string) (domain.AssetListItem, error) {
		id := typeIDs[code]
		items, _, err := svc.ListAssets(officer, domain.AssetFilter{TypeID: &id, Borrowable: true, Sort: domain.SortTag, Limit: 1})
		if err != nil || len(items) == 0 {
			return domain.AssetListItem{}, fmt.Errorf("seed: no free %s: %w", code, err)
		}
		return items[0], nil
	}

	// ba phiếu đang chờ, một có yêu cầu thuộc tính
	if _, err := ask("employee@storeit.test", "LAPTOP", "Client demo in Đà Nẵng", "ram_gb:gte:16"); err != nil {
		return tickets, loans, err
	}
	if _, err := ask("nguyen.an@storeit.test", "MONITOR", ""); err != nil {
		return tickets, loans, err
	}
	if _, err := ask("do.linh@storeit.test", "PHONE", "Field trip next week"); err != nil {
		return tickets, loans, err
	}
	// đã duyệt, chờ nhận
	t, err := ask("vo.minh@storeit.test", "LAPTOP", "")
	if err != nil {
		return tickets, loans, err
	}
	a, err := free("LAPTOP")
	if err != nil {
		return tickets, loans, err
	}
	if _, err := svc.ApproveTicket(officer, t.ID, a.ID, t.Version); err != nil {
		return tickets, loans, fmt.Errorf("seed: approve: %w", err)
	}
	// đã giao: một khoản mượn theo phiếu
	if t, err = ask("employee@storeit.test", "MONITOR", "Second screen for the audit"); err != nil {
		return tickets, loans, err
	}
	if a, err = free("MONITOR"); err != nil {
		return tickets, loans, err
	}
	ap, err := svc.ApproveTicket(officer, t.ID, a.ID, t.Version)
	if err != nil {
		return tickets, loans, fmt.Errorf("seed: approve: %w", err)
	}
	if _, err := svc.HandOverTicket(officer, t.ID, ap.Version); err != nil {
		return tickets, loans, fmt.Errorf("seed: hand over: %w", err)
	}
	loans++
	// bị từ chối
	if t, err = ask("nguyen.an@storeit.test", "PROJECTOR", "Training on Friday"); err != nil {
		return tickets, loans, err
	}
	if _, err := svc.RejectTicket(officer, t.ID, "No spare projector this week", t.Version); err != nil {
		return tickets, loans, fmt.Errorf("seed: reject: %w", err)
	}
	// cho mượn trực tiếp, hạn hôm nay: từ mai thành quá hạn
	if a, err = free("PHONE"); err != nil {
		return tickets, loans, err
	}
	if _, err := svc.LendAsset(officer, a.ID, people["do.linh@storeit.test"], &today, a.Version); err != nil {
		return tickets, loans, fmt.Errorf("seed: lend: %w", err)
	}
	loans++
	return tickets, loans, nil
}

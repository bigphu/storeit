package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
)

var hcm = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		panic(err)
	}
	return loc
}()

type borrowEnv struct {
	*env
	borrowing *fakeBorrowing
	accounts  borrowAccounts
	mail      *fakeMail
	typeID    uuid.UUID
}

// newBorrowEnv: service với giờ cố định now, múi giờ Hồ Chí Minh, một loại cho mượn được
func newBorrowEnv(t *testing.T, now time.Time) *borrowEnv {
	t.Helper()
	e := &env{types: newFakeTypes(), statuses: newFakeStatuses(), assets: newFakeAssets(), profiles: newFakeProfiles()}
	b := &borrowEnv{env: e, borrowing: newFakeBorrowing(), accounts: borrowAccounts{inactive: map[uuid.UUID]bool{}, searchLimit: new(int)}, mail: &fakeMail{}}
	e.svc = New(Deps{
		Types: e.types, Statuses: e.statuses, Assets: e.assets, Profiles: e.profiles, Accounts: b.accounts,
		Borrowing: b.borrowing, Location: hcm, Now: func() time.Time { return now }, Mail: b.mail, AppURL: "http://app.test",
	})
	ty, err := e.svc.CreateAssetType(as(domain.PermTypeManage, domain.PermBorrowManage), domain.NewAssetType{Code: "PROJ", Name: "Projector", CanBorrow: true})
	if err != nil {
		t.Fatal(err)
	}
	b.typeID = ty.ID
	return b
}

// asAccount: actor là account id với các quyền perms
func asAccount(id uuid.UUID, perms ...string) context.Context {
	return auth.WithActor(context.Background(), auth.Actor{AccountID: id, Permissions: perms})
}

func fieldName(t *testing.T, err error, want error) string {
	t.Helper()
	var e *errs.Error
	if !errors.Is(err, want) || !errors.As(err, &e) || len(e.Fields()) == 0 {
		t.Fatalf("err = %v, want %v with a field", err, want)
	}
	return e.Fields()[0].Field
}

func day(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return d
}

// 00:30 ở Việt Nam là 17:30 UTC hôm trước: "hôm nay" là ngày Việt Nam
func TestService_BusinessDay(t *testing.T) {
	now := time.Date(2026, 10, 29, 17, 30, 0, 0, time.UTC)
	b := newBorrowEnv(t, now)
	if got := b.svc.today(); !got.Equal(day("2026-10-30")) {
		t.Fatalf("today() = %v, want 2026-10-30", got)
	}
	emp := asAccount(uuid.New(), domain.PermBorrowRequest)
	// Form phiếu và hộp Lend lấy "hôm nay" từ đây, không theo đồng hồ của trình duyệt
	if got, err := b.svc.BusinessDay(emp); err != nil || !got.Equal(day("2026-10-30")) {
		t.Errorf("BusinessDay = %v, %v; want 2026-10-30", got, err)
	}
	if _, err := b.svc.BusinessDay(as(domain.PermAssetRead)); status(err) != 403 {
		t.Errorf("BusinessDay without a borrowing permission = %v, want 403", err)
	}
	if _, err := b.svc.CreateTicket(emp, TicketInput{TypeID: b.typeID, NeededFrom: day("2026-10-30")}); err != nil {
		t.Errorf("needed from the Vietnamese today: %v", err)
	}
	_, err := b.svc.CreateTicket(emp, TicketInput{TypeID: b.typeID, NeededFrom: day("2026-10-29")})
	if f := fieldName(t, err, domain.ErrInvalidTicket); f != "needed_from" {
		t.Errorf("field = %q, want needed_from", f)
	}
}

// Account người gửi bị khoá giữa lúc duyệt và lúc giao: không giao, giữ nguyên chỗ
func TestBorrowing_HandOverDisabledRequester(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	requester := uuid.New()
	tk, err := b.svc.CreateTicket(asAccount(requester, domain.PermBorrowRequest), TicketInput{TypeID: b.typeID, NeededFrom: day("2026-10-30")})
	if err != nil {
		t.Fatal(err)
	}
	ap, err := b.borrowing.Approve(context.Background(), tk.ID, uuid.New(), tk.Version)
	if err != nil {
		t.Fatal(err)
	}
	b.accounts.inactive[requester] = true
	officer := asAccount(uuid.New(), domain.PermBorrowManage)
	_, err = b.svc.HandOverTicket(officer, tk.ID, ap.Version)
	if f := fieldName(t, err, domain.ErrInvalidTicket); f != "requester" {
		t.Errorf("field = %q, want requester", f)
	}
	if b.borrowing.handOvers != 0 {
		t.Errorf("repository HandOver called %d times, want 0", b.borrowing.handOvers)
	}
}

func TestBorrowingPermissions(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	in := TicketInput{TypeID: b.typeID, NeededFrom: day("2026-10-30")}

	if _, err := b.svc.CreateTicket(as(domain.PermAssetRead), in); status(err) != 403 {
		t.Errorf("create without borrow.request = %v, want 403", err)
	}
	alice, bob := uuid.New(), uuid.New()
	aliceCtx, bobCtx := asAccount(alice, domain.PermBorrowRequest), asAccount(bob, domain.PermBorrowRequest)
	tk, err := b.svc.CreateTicket(aliceCtx, in)
	if err != nil {
		t.Fatal(err)
	}
	if tk.RequesterID != alice {
		t.Errorf("requester = %v, want the actor", tk.RequesterID)
	}
	if _, err := b.svc.GetTicket(bobCtx, tk.ID); !errors.Is(err, domain.ErrTicketNotFound) {
		t.Errorf("someone else's ticket: %v, want ErrTicketNotFound", err)
	}
	if _, _, err := b.svc.ListTickets(bobCtx, domain.TicketFilter{}, false); err != nil {
		t.Fatal(err)
	}
	if f := b.borrowing.lastFilter.RequesterID; f == nil || *f != bob {
		t.Errorf("list without manage: requester filter = %v, want forced to the actor", f)
	}
	if _, err := b.svc.ApproveTicket(aliceCtx, tk.ID, uuid.New(), tk.Version); status(err) != 403 {
		t.Errorf("approve without borrow.manage = %v, want 403", err)
	}
	// Người gửi huỷ được phiếu của mình kể cả đã duyệt
	ap, err := b.borrowing.Approve(context.Background(), tk.ID, uuid.New(), tk.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.svc.CancelTicket(bobCtx, tk.ID, "", ap.Version); !errors.Is(err, domain.ErrTicketNotFound) {
		t.Errorf("cancel someone else's ticket: %v, want ErrTicketNotFound", err)
	}
	if got, err := b.svc.CancelTicket(aliceCtx, tk.ID, "", ap.Version); err != nil || got.State != domain.TicketCancelled {
		t.Errorf("cancel own approved ticket = %+v, %v", got, err)
	}
	// Người quản lý không huỷ được phiếu đang chờ của người khác (họ từ chối)
	w, err := b.svc.CreateTicket(aliceCtx, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.svc.CancelTicket(asAccount(uuid.New(), domain.PermBorrowManage), w.ID, "", w.Version); !errors.Is(err, domain.ErrTicketState) {
		t.Errorf("manager cancels a waiting ticket: %v, want ErrTicketState", err)
	}
}

// "Can be borrowed" thuộc về việc mượn: bật/tắt cần borrow.manage (người quản lý kho), đổi tên
// loại vẫn cần type.manage
func TestCanBorrowNeedsBorrowManage(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	ty, err := b.svc.CreateAssetType(manager, domain.NewAssetType{Code: "CAM", Name: "Camera"})
	if err != nil {
		t.Fatal(err)
	}
	on := true
	officer := as(domain.PermAssetRead, domain.PermBorrowManage)
	got, err := b.svc.UpdateAssetType(officer, ty.ID, nil, nil, &on, ty.Version)
	if err != nil || !got.CanBorrow {
		t.Fatalf("officer turns it on = %+v, %v", got, err)
	}
	typesOnly := as(domain.PermAssetRead, domain.PermTypeManage)
	off := false
	if _, err := b.svc.UpdateAssetType(typesOnly, ty.ID, nil, nil, &off, got.Version); status(err) != 403 {
		t.Errorf("type.manage alone toggles it: %v, want 403", err)
	}
	name := "Cameras"
	if _, err := b.svc.UpdateAssetType(officer, ty.ID, &name, nil, nil, got.Version); status(err) != 403 {
		t.Errorf("borrow.manage alone renames the type: %v, want 403", err)
	}
	if _, err := b.svc.UpdateAssetType(typesOnly, ty.ID, &name, nil, nil, got.Version); err != nil {
		t.Errorf("type.manage renames: %v", err)
	}
}

// Reserved và On loan do việc mượn đặt: không bao giờ là status mặc định của kind
func TestBorrowingStatusesNeverDefault(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	for _, id := range []uuid.UUID{domain.ReservedStatusID, domain.OnLoanStatusID} {
		if _, err := b.svc.UpdateStatus(manager, id, domain.StatusChange{MakeDefault: true}); !errors.Is(err, domain.ErrBorrowingStatus) {
			t.Errorf("make %v default: %v, want ErrBorrowingStatus", id, err)
		}
	}
}

// Danh sách phiếu đọc gộp: không đọc từng tài sản một (trước đây vài truy vấn mỗi dòng)
func TestListTickets_ReadsAssetsInOneGo(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	for i := range 3 {
		tk, _ := approvedTicketTagged(t, b, fmt.Sprintf("LAP-%04d", i+1))
		_ = tk
	}
	b.assets.gets = 0
	views, _, err := b.svc.ListTickets(as(domain.PermBorrowManage), domain.TicketFilter{}, false)
	if err != nil || len(views) != 3 {
		t.Fatalf("list = %d, %v", len(views), err)
	}
	for _, v := range views {
		if v.Asset == nil || v.Asset.Tag == "" || v.TypeName != "Projector" {
			t.Errorf("view = %+v, want asset tag and type name", v)
		}
	}
	if b.assets.gets != 0 {
		t.Errorf("asset reads one by one = %d, want 0 (one batched list)", b.assets.gets)
	}
}

// Tìm phiếu theo tên người: xét mọi account khớp (tên phổ biến như "Nguyễn" khớp rất nhiều người)
func TestListTickets_SearchConsidersEveryone(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	if _, _, err := b.svc.ListTickets(as(domain.PermBorrowManage), domain.TicketFilter{Q: "nguyen"}, false); err != nil {
		t.Fatal(err)
	}
	if got := *b.accounts.searchLimit; got < 1000 {
		t.Errorf("people considered = %d, want at least 1000", got)
	}
}

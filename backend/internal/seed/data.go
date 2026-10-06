// Package seed ghi dữ liệu demo vào DB dev trống (make seed): account mỗi role,
// loại tài sản có thuộc tính, status, ~300 tài sản, profile export. Chỉ ghi qua
// API của module, với quyền hệ thống. Không bao giờ chạy ở production.
package seed

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"

	iddomain "storeit/internal/identity/domain"
	"storeit/internal/inventory/domain"
)

type accountSpec struct {
	Email, Name string
	Role        uuid.UUID
	Key         string // tên dùng trong profileSpec.Owner; "" là không sở hữu profile
}

var accounts = []accountSpec{
	{"manager@storeit.test", "Trần Thị Mai", iddomain.AuthorizedManagerRoleID, "manager"},
	{"officer@storeit.test", "Lê Văn Hùng", iddomain.InventoryOfficerRoleID, "officer"},
	{"employee@storeit.test", "Phạm Thu Hà", iddomain.EmployeeRoleID, "employee"},
	{"nguyen.an@storeit.test", "Nguyễn Văn An", iddomain.EmployeeRoleID, ""},
	{"do.linh@storeit.test", "Đỗ Khánh Linh", iddomain.EmployeeRoleID, ""},
	{"vo.minh@storeit.test", "Võ Quang Minh", iddomain.EmployeeRoleID, ""},
}

type statusSpec struct {
	Name string
	Kind domain.StatusKind
}

const (
	repairStatus = "Đang sửa chữa"
	loanStatus   = "Cho mượn"
	retireReason = "Hỏng, đã thanh lý"
)

var statuses = []statusSpec{
	{repairStatus, domain.KindUnavailable},
	{loanStatus, domain.KindInUse},
}

// attrSpec: Gen trả string (text, ngày YYYY-MM-DD), float64, bool, hoặc NHÃN
// option với select (Run đổi sang id option)
type attrSpec struct {
	Key, Label string
	Type       domain.DataType
	Unit       string
	Required   bool
	Options    []string
	Gen        func(r *rand.Rand) any
}

type typeSpec struct {
	Code, Name, Description string
	Prefix                  string // tag: <Prefix>-0001
	Count                   int
	Models                  []string
	Attrs                   []attrSpec
}

// Bộ sinh giá trị
func pick[T any](xs ...T) func(r *rand.Rand) any {
	return func(r *rand.Rand) any { return xs[r.IntN(len(xs))] }
}

func num(xs ...float64) func(r *rand.Rand) any { return pick(xs...) }

func coin(r *rand.Rand) any { return r.IntN(2) == 0 }

// daysAgo: ngày trong khoảng [min, max] ngày trước 2026-10-06 (cố định để chạy lại ra cùng dữ liệu)
func daysAgo(min, max int) func(r *rand.Rand) any {
	return func(r *rand.Rand) any {
		return seedDay.AddDate(0, 0, -(min + r.IntN(max-min+1))).Format(time.DateOnly)
	}
}

// daysAhead: ngày trong tương lai (vd hết hạn bảo hành)
func daysAhead(min, max int) func(r *rand.Rand) any {
	return func(r *rand.Rand) any {
		return seedDay.AddDate(0, 0, min+r.IntN(max-min+1)).Format(time.DateOnly)
	}
}

func serial(prefix string, digits int) func(r *rand.Rand) any {
	return func(r *rand.Rand) any { return fmt.Sprintf("%s%0*d", prefix, digits, r.IntN(pow10(digits))) }
}

func pow10(n int) int {
	p := 1
	for range n {
		p *= 10
	}
	return p
}

// Ngày gốc của dữ liệu demo
var seedDay = time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)

var types = []typeSpec{
	{
		Code: "LAPTOP", Name: "Laptop", Description: "Máy tính xách tay cấp cho nhân viên", Prefix: "LAP", Count: 90,
		Models: []string{"ThinkPad T14 Gen 3", "ThinkPad X1 Carbon Gen 11", "Dell Latitude 5440", "MacBook Air M2", "MacBook Pro 14 M3", "HP EliteBook 840 G10", "ASUS ExpertBook B9"},
		Attrs: []attrSpec{
			{Key: "cpu", Label: "CPU", Type: domain.TypeText, Required: true, Gen: pick[any]("Intel Core i5-1335U", "Intel Core i7-1365U", "Apple M2", "Apple M3 Pro", "AMD Ryzen 7 7840U")},
			{Key: "ram_gb", Label: "RAM", Type: domain.TypeNumber, Unit: "GB", Gen: num(8, 16, 32)},
			{Key: "storage_gb", Label: "Storage", Type: domain.TypeNumber, Unit: "GB", Gen: num(256, 512, 1024)},
			{Key: "touch", Label: "Touch screen", Type: domain.TypeBoolean, Gen: coin},
			{Key: "os", Label: "OS", Type: domain.TypeSelect, Options: []string{"Windows 11", "macOS", "Ubuntu"}, Gen: pick[any]("Windows 11", "Windows 11", "macOS", "Ubuntu")},
		},
	},
	{
		Code: "MONITOR", Name: "Monitor", Description: "Màn hình rời", Prefix: "MON", Count: 60,
		Models: []string{"Dell U2723QE", "Dell P2422H", "LG 27UL850", "Samsung ViewFinity S8", "ASUS ProArt PA278QV"},
		Attrs: []attrSpec{
			{Key: "size_in", Label: "Size", Type: domain.TypeNumber, Unit: "inch", Required: true, Gen: num(24, 27, 32, 34)},
			{Key: "resolution", Label: "Resolution", Type: domain.TypeSelect, Options: []string{"Full HD", "2K", "4K"}, Gen: pick[any]("Full HD", "2K", "4K")},
			{Key: "curved", Label: "Curved", Type: domain.TypeBoolean, Gen: coin},
		},
	},
	{
		Code: "PHONE", Name: "Điện thoại", Description: "Điện thoại công ty", Prefix: "PHN", Count: 40,
		Models: []string{"iPhone 14", "iPhone 15", "Samsung Galaxy A54", "Samsung Galaxy S23", "Xiaomi Redmi Note 13"},
		Attrs: []attrSpec{
			{Key: "imei", Label: "IMEI", Type: domain.TypeText, Required: true, Gen: serial("35", 13)},
			{Key: "os", Label: "OS", Type: domain.TypeSelect, Options: []string{"Android", "iOS"}, Gen: pick[any]("Android", "iOS")},
			{Key: "storage_gb", Label: "Storage", Type: domain.TypeNumber, Unit: "GB", Gen: num(128, 256, 512)},
		},
	},
	{
		Code: "PRINTER", Name: "Máy in", Description: "Máy in văn phòng", Prefix: "PRN", Count: 12,
		Models: []string{"HP LaserJet Pro M404dn", "Canon LBP2900", "Brother HL-L2321D", "Epson L3250"},
		Attrs: []attrSpec{
			{Key: "kind", Label: "Kind", Type: domain.TypeSelect, Options: []string{"Laser", "Inkjet"}, Gen: pick[any]("Laser", "Laser", "Inkjet")},
			{Key: "color", Label: "Color", Type: domain.TypeBoolean, Gen: coin},
			{Key: "ip", Label: "IP address", Type: domain.TypeText, Gen: func(r *rand.Rand) any { return fmt.Sprintf("10.0.%d.%d", 1+r.IntN(4), 10+r.IntN(200)) }},
		},
	},
	{
		Code: "DESK", Name: "Bàn làm việc", Description: "Bàn làm việc trong văn phòng", Prefix: "DSK", Count: 25,
		Models: []string{"Bàn chữ L Hòa Phát", "Bàn nâng hạ điện", "Bàn gỗ 1m2", "Bàn kính cường lực"},
		Attrs: []attrSpec{
			{Key: "width_cm", Label: "Width", Type: domain.TypeNumber, Unit: "cm", Gen: num(100, 120, 140, 160)},
			{Key: "material", Label: "Material", Type: domain.TypeSelect, Options: []string{"Gỗ", "Kính", "Thép"}, Gen: pick[any]("Gỗ", "Gỗ", "Kính", "Thép")},
			{Key: "price_vnd", Label: "Price", Type: domain.TypeNumber, Unit: "VND", Gen: num(1_800_000, 2_500_000, 4_200_000, 6_900_000)},
		},
	},
	{
		Code: "CHAIR", Name: "Ghế", Description: "Ghế văn phòng", Prefix: "CHR", Count: 30,
		Models: []string{"Ghế công thái học Sihoo M57", "Ghế xoay lưng lưới", "Ghế họp chân quỳ"},
		Attrs: []attrSpec{
			{Key: "ergonomic", Label: "Ergonomic", Type: domain.TypeBoolean, Gen: coin},
			{Key: "color", Label: "Color", Type: domain.TypeText, Gen: pick[any]("Đen", "Xám", "Xanh navy")},
		},
	},
	{
		Code: "MOTORBIKE", Name: "Xe máy", Description: "Xe máy giao hàng và công tác", Prefix: "MBK", Count: 8,
		Models: []string{"Honda Wave Alpha", "Honda Vision", "Yamaha Sirius", "VinFast Evo200"},
		Attrs: []attrSpec{
			{Key: "plate", Label: "Biển số", Type: domain.TypeText, Required: true, Gen: func(r *rand.Rand) any {
				return fmt.Sprintf("59-%c%d %03d.%02d", 'A'+rune(r.IntN(26)), 1+r.IntN(9), r.IntN(1000), r.IntN(100))
			}},
			{Key: "brand", Label: "Brand", Type: domain.TypeSelect, Options: []string{"Honda", "Yamaha", "VinFast"}, Gen: pick[any]("Honda", "Yamaha", "VinFast")},
			{Key: "registered_on", Label: "Registered on", Type: domain.TypeDate, Gen: daysAgo(30, 1000)},
		},
	},
	{
		Code: "SERVER", Name: "Server", Description: "Máy chủ trong phòng server", Prefix: "SRV", Count: 10,
		Models: []string{"Dell PowerEdge R650", "HPE ProLiant DL380 Gen10", "Lenovo ThinkSystem SR630"},
		Attrs: []attrSpec{
			{Key: "cpu_cores", Label: "CPU cores", Type: domain.TypeNumber, Gen: num(16, 24, 32, 64)},
			{Key: "ram_gb", Label: "RAM", Type: domain.TypeNumber, Unit: "GB", Gen: num(64, 128, 256)},
			{Key: "rack", Label: "Rack", Type: domain.TypeText, Gen: func(r *rand.Rand) any { return fmt.Sprintf("R%02d-U%02d", 1+r.IntN(4), 1+r.IntN(40)) }},
			{Key: "warranty_until", Label: "Warranty until", Type: domain.TypeDate, Gen: daysAhead(30, 900)},
		},
	},
	{
		Code: "PROJECTOR", Name: "Máy chiếu", Description: "Máy chiếu phòng họp", Prefix: "PRJ", Count: 10,
		Models: []string{"Epson EB-X51", "BenQ MW560", "ViewSonic PA503W"},
		Attrs: []attrSpec{
			{Key: "lumens", Label: "Brightness", Type: domain.TypeNumber, Unit: "lm", Gen: num(3600, 3800, 4000)},
			{Key: "hdmi", Label: "HDMI", Type: domain.TypeBoolean, Gen: coin},
		},
	},
	{
		Code: "NETWORK", Name: "Thiết bị mạng", Description: "Switch, router, access point", Prefix: "NET", Count: 15,
		Models: []string{"Cisco CBS350-24T", "TP-Link TL-SG1024", "UniFi U6 Lite", "MikroTik hEX S"},
		Attrs: []attrSpec{
			{Key: "kind", Label: "Kind", Type: domain.TypeSelect, Options: []string{"Switch", "Router", "Access point"}, Gen: pick[any]("Switch", "Router", "Access point")},
			{Key: "ports", Label: "Ports", Type: domain.TypeNumber, Gen: num(5, 8, 24, 48)},
			{Key: "poe", Label: "PoE", Type: domain.TypeBoolean, Gen: coin},
		},
	},
}

type profileSpec struct {
	Owner  string // accountSpec.Key
	Name   string
	Shared bool
	Layout domain.ExportLayout
}

func cols(fields ...string) []domain.ExportColumn {
	out := make([]domain.ExportColumn, len(fields))
	for i, f := range fields {
		out[i] = domain.ExportColumn{Field: f}
	}
	return out
}

var profiles = []profileSpec{
	{
		Owner: "manager", Name: "Kiểm kê quý", Shared: true,
		Layout: func() domain.ExportLayout {
			l := domain.DefaultReportLayout()
			l.Columns = cols("tag", "name", "status", "purchase_date")
			l.Sheets, l.EachTypeAttrs, l.TitleRow, l.Summary = domain.SheetPerType, true, true, true
			return l
		}(),
	},
	{
		Owner: "officer", Name: "Laptop chi tiết",
		Layout: func() domain.ExportLayout {
			l := domain.DefaultReportLayout()
			l.Columns = []domain.ExportColumn{
				{Field: "tag", Header: "Mã tài sản"}, {Field: "name", Header: "Tên"}, {Field: "attr:cpu"},
				{Field: "attr:ram_gb"}, {Field: "attr:storage_gb"}, {Field: "attr:os"}, {Field: "status", Header: "Trạng thái"},
			}
			l.SheetName, l.Sort = "Laptop", "-attributes.ram_gb"
			return l
		}(),
	},
	{
		Owner: "employee", Name: "Danh sách đơn giản",
		Layout: func() domain.ExportLayout {
			l := domain.DefaultReportLayout()
			l.Stripes = true
			return l
		}(),
	},
}

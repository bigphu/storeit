package inventory

import "testing"

func TestConfig_Validate(t *testing.T) {
	for _, n := range []int{0, 1, 50000, 1_000_000} {
		if err := (Config{ExportMaxRows: n}).Validate(); err != nil {
			t.Errorf("%d: %v", n, err)
		}
	}
	for _, n := range []int{-1, 1_000_001} {
		if err := (Config{ExportMaxRows: n}).Validate(); err == nil {
			t.Errorf("%d accepted", n)
		}
	}
}

// Mượn (US-10): link trong thư và múi giờ của ngày nghiệp vụ; rỗng là mặc định
func TestConfig_Borrowing(t *testing.T) {
	ok := []Config{{}, {AppURL: "https://storeit.bigphu.io.vn", TimeZone: "Asia/Ho_Chi_Minh"}, {TimeZone: "UTC"}}
	for _, c := range ok {
		if err := c.Validate(); err != nil {
			t.Errorf("%+v: %v", c, err)
		}
	}
	bad := []Config{{AppURL: "storeit.vn"}, {AppURL: "https://storeit.vn/?x=1"}, {TimeZone: "Mars/Olympus"}}
	for _, c := range bad {
		if err := c.Validate(); err == nil {
			t.Errorf("%+v accepted", c)
		}
	}
	if loc := (Config{}).Location(); loc.String() != "Asia/Ho_Chi_Minh" {
		t.Errorf("default location = %s, want Asia/Ho_Chi_Minh", loc)
	}
	if u := (Config{}).appURL(); u != "http://localhost:3000" {
		t.Errorf("default app URL = %q", u)
	}
}

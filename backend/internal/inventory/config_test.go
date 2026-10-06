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

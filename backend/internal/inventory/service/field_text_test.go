package service

import "testing"

func TestFieldFilterText(t *testing.T) {
	cases := map[string]string{
		"purchase_date:gte:2026-01-01": "Purchase date ≥ 01/01/2026",
		"created_at:eq:2026-10-08":     "Created = 08/10/2026",
		"updated_at:lt:2026-10-08":     "Updated < 08/10/2026",
		"description:contains:dock":    `Description contains "dock"`,
	}
	for cond, want := range cases {
		if got := fieldFilterText(cond, "02/01/2006"); got != want {
			t.Errorf("%s → %q, want %q", cond, got, want)
		}
	}
}

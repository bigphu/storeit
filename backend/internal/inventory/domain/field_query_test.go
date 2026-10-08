package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"storeit/internal/platform/errs"
)

func TestResolveFieldQuery(t *testing.T) {
	hcm, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	f := func(field BuiltinField, op AttrOp, v string) FieldFilter {
		return FieldFilter{Field: field, Op: op, Value: v}
	}
	cases := []struct {
		name string
		cond string
		loc  *time.Location
		want []FieldFilter
	}{
		{"purchase date keeps day and op", "purchase_date:gte:2026-01-01", time.UTC, []FieldFilter{f(FieldPurchaseDate, OpGte, "2026-01-01")}},
		{"created eq is one local day (HCM)", "created_at:eq:2026-10-08", hcm,
			[]FieldFilter{f(FieldCreatedAt, OpGte, "2026-10-07T17:00:00Z"), f(FieldCreatedAt, OpLt, "2026-10-08T17:00:00Z")}},
		{"created eq in UTC", "created_at:eq:2026-10-08", time.UTC,
			[]FieldFilter{f(FieldCreatedAt, OpGte, "2026-10-08T00:00:00Z"), f(FieldCreatedAt, OpLt, "2026-10-09T00:00:00Z")}},
		{"updated gt starts next day", "updated_at:gt:2026-10-08", hcm, []FieldFilter{f(FieldUpdatedAt, OpGte, "2026-10-08T17:00:00Z")}},
		{"updated gte starts that day", "updated_at:gte:2026-10-08", hcm, []FieldFilter{f(FieldUpdatedAt, OpGte, "2026-10-07T17:00:00Z")}},
		{"updated lt ends before that day", "updated_at:lt:2026-10-08", hcm, []FieldFilter{f(FieldUpdatedAt, OpLt, "2026-10-07T17:00:00Z")}},
		{"updated lte ends after that day", "updated_at:lte:2026-10-08", hcm, []FieldFilter{f(FieldUpdatedAt, OpLt, "2026-10-08T17:00:00Z")}},
		{"description trimmed", "description:contains:  dock 100% ", time.UTC, []FieldFilter{f(FieldDescription, OpContains, "dock 100%")}},
		{"value may contain colons", "description:contains:a:b", time.UTC, []FieldFilter{f(FieldDescription, OpContains, "a:b")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ResolveFieldQuery([]string{c.cond}, c.loc)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestResolveFieldQuery_Errors(t *testing.T) {
	conds := []string{
		"purchase_date:gte:2026-01-01",                     // 0 ok
		"location:eq:x",                                    // 1 unknown field
		"description:eq:dock",                              // 2 operator not allowed
		"created_at:gte:08/10/2026",                        // 3 bad date
		"description:contains:" + strings.Repeat("x", 201), // 4 too long
		"purchase_date",                                    // 5 malformed
	}
	_, err := ResolveFieldQuery(conds, time.UTC)
	if !errors.Is(err, ErrInvalidFieldQuery) {
		t.Fatalf("err = %v, want ErrInvalidFieldQuery", err)
	}
	var e *errs.Error
	if !errors.As(err, &e) {
		t.Fatalf("not an errs.Error: %v", err)
	}
	var got []string
	for _, fe := range e.Fields() {
		got = append(got, fe.Field)
	}
	want := []string{"field[1]", "field[2]", "field[3]", "field[4]", "field[5]"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fields = %v, want %v", got, want)
	}
}

package domain

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

func TestAssetSortAttribute(t *testing.T) {
	for in, want := range map[AssetSort]struct {
		key  string
		desc bool
		ok   bool
	}{
		"attributes.ram_gb":  {"ram_gb", false, true},
		"-attributes.ram_gb": {"ram_gb", true, true},
		"tag":                {"", false, false},
		"-name":              {"", false, false},
		"":                   {"", false, false},
	} {
		key, desc, ok := in.Attribute()
		if key != want.key || desc != want.desc || ok != want.ok {
			t.Errorf("%q.Attribute() = %q, %v, %v; want %+v", in, key, desc, ok, want)
		}
	}
}

func TestResolveAttrQuery(t *testing.T) {
	typ, windows, legacy := laptopType()
	byKey := map[string]Attribute{}
	for _, a := range typ.Attributes {
		byKey[a.Key] = a
	}

	filters, order, err := ResolveAttrQuery(typ, []string{
		"ram_gb:gte:8", "ram_gb:lt:32.50", "serial:contains: sn:1 ", "warranty_end:lte:2027-06-30",
		"has_dock:eq:true", "os:eq:" + windows.String(), "os:in:" + windows.String() + "," + legacy.String(),
	}, "-attributes.warranty_end")
	if err != nil {
		t.Fatal(err)
	}
	want := []AttrFilter{
		{AttributeID: byKey["ram_gb"].ID, DataType: TypeNumber, Op: OpGte, Value: "8"},
		{AttributeID: byKey["ram_gb"].ID, DataType: TypeNumber, Op: OpLt, Value: "32.5"},
		// dấu ':' trong giá trị là chữ; khoảng trắng đầu cuối bị bỏ
		{AttributeID: byKey["serial"].ID, DataType: TypeText, Op: OpContains, Value: "sn:1"},
		{AttributeID: byKey["warranty_end"].ID, DataType: TypeDate, Op: OpLte, Value: "2027-06-30"},
		{AttributeID: byKey["has_dock"].ID, DataType: TypeBoolean, Op: OpEq, Value: "true"},
		{AttributeID: byKey["os"].ID, DataType: TypeSelect, Op: OpEq, Value: windows.String()},
		// option đã gỡ vẫn lọc được: tài sản cũ còn giữ giá trị đó
		{AttributeID: byKey["os"].ID, DataType: TypeSelect, Op: OpIn, Value: windows.String() + "," + legacy.String()},
	}
	if !slices.Equal(filters, want) {
		t.Errorf("filters =\n%+v\nwant\n%+v", filters, want)
	}
	if order == nil || *order != (AttrOrder{AttributeID: byKey["warranty_end"].ID, DataType: TypeDate, Desc: true}) {
		t.Errorf("order = %+v", order)
	}

	// Không có điều kiện và sắp theo cột thường: không có gì
	filters, order, err = ResolveAttrQuery(typ, nil, "name")
	if err != nil || filters != nil || order != nil {
		t.Errorf("plain query = %v, %v, %v", filters, order, err)
	}
}

func TestResolveAttrQuery_Errors(t *testing.T) {
	typ, windows, _ := laptopType()
	_, _, err := ResolveAttrQuery(typ, []string{
		"ram_gb:gte:abc",                     // 0: không phải số
		"ram_gb:gte:1.1234567",               // 1: quá 6 chữ số thập phân
		"ram_gb:contains:8",                  // 2: toán tử không hợp kiểu
		"serial:eq:",                         // 3: giá trị rỗng
		"nope:eq:1",                          // 4: không có thuộc tính
		"old:eq:x",                           // 5: thuộc tính đã gỡ
		"warranty_end:eq:30/06/2027",         // 6: ngày sai dạng
		"has_dock:eq:yes",                    // 7: bool sai
		"os:eq:" + uuid.NewString(),          // 8: option của thuộc tính khác
		"os:in:" + windows.String() + ",bad", // 9: một id hỏng
		"ram_gb",                             // 10: thiếu toán tử và giá trị
		"ram_gb:between:1",                   // 11: toán tử lạ
	}, "attributes.nope")
	if !errors.Is(err, ErrInvalidAttributeQuery) {
		t.Fatalf("err = %v, want ErrInvalidAttributeQuery", err)
	}
	var e *errs.Error
	errors.As(err, &e)
	var fields []string
	for _, f := range e.Fields() {
		fields = append(fields, f.Field)
	}
	want := []string{"attr[0]", "attr[1]", "attr[2]", "attr[3]", "attr[4]", "attr[5]", "attr[6]", "attr[7]",
		"attr[8]", "attr[9]", "attr[10]", "attr[11]", "sort"}
	if !slices.Equal(fields, want) {
		t.Errorf("fields = %v\nwant     %v", fields, want)
	}
}

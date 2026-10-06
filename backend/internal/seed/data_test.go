package seed

import (
	"math/rand/v2"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
)

// toType dựng domain.AssetType từ spec (id giả cho thuộc tính và option) để
// chạy kiểm tra giá trị thật của inventory
func toType(ts typeSpec) (domain.AssetType, map[string]map[string]uuid.UUID) {
	t := domain.AssetType{ID: uuid.New(), Code: ts.Code, Name: ts.Name}
	opts := map[string]map[string]uuid.UUID{}
	for i, a := range ts.Attrs {
		attr := domain.Attribute{ID: uuid.New(), Key: a.Key, Label: a.Label, DataType: a.Type, Unit: a.Unit, Required: a.Required, Position: int32(i + 1)}
		opts[a.Key] = map[string]uuid.UUID{}
		for j, o := range a.Options {
			id := uuid.New()
			attr.Options = append(attr.Options, domain.Option{ID: id, AttributeID: attr.ID, Label: o, Position: int32(j + 1)})
			opts[a.Key][o] = id
		}
		t.Attributes = append(t.Attributes, attr)
	}
	return t, opts
}

func TestData_Shape(t *testing.T) {
	if len(types) != 10 {
		t.Fatalf("types = %d, want 10", len(types))
	}
	total, codes := 0, map[string]bool{}
	for _, ts := range types {
		total += ts.Count
		if codes[ts.Code] {
			t.Errorf("duplicate code %s", ts.Code)
		}
		codes[ts.Code] = true
		if len(ts.Models) == 0 || ts.Prefix == "" {
			t.Errorf("%s: needs models and a tag prefix", ts.Code)
		}
		for _, a := range ts.Attrs {
			if (a.Type == domain.TypeSelect) != (len(a.Options) > 0) {
				t.Errorf("%s.%s: options only and always for select", ts.Code, a.Key)
			}
			if a.Gen == nil {
				t.Errorf("%s.%s: no generator", ts.Code, a.Key)
			}
		}
	}
	if total != 300 {
		t.Errorf("asset count = %d, want 300", total)
	}
	if len(accounts) != 6 || len(statuses) != 2 || len(profiles) != 3 {
		t.Errorf("accounts %d, statuses %d, profiles %d", len(accounts), len(statuses), len(profiles))
	}
	for _, p := range profiles {
		if err := p.Layout.WithDefaults().Validate(); err != nil {
			t.Errorf("profile %q layout: %v", p.Name, err)
		}
	}
}

// Mọi giá trị sinh ra phải qua được domain.ValidateValues (key hợp lệ, đúng kiểu)
func TestData_GeneratesValidValues(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, ts := range types {
		at, opts := toType(ts)
		for range 50 {
			in := map[string]any{}
			for _, a := range ts.Attrs {
				v := a.Gen(r)
				if a.Type == domain.TypeSelect {
					id, ok := opts[a.Key][v.(string)]
					if !ok {
						t.Fatalf("%s.%s: generator returned unknown option %v", ts.Code, a.Key, v)
					}
					v = id.String()
				}
				in[a.Key] = v
			}
			if _, err := domain.ValidateValues(at, in); err != nil {
				t.Fatalf("%s: %v (values %v)", ts.Code, err, in)
			}
		}
	}
}

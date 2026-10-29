package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	idcontract "storeit/internal/identity/contract"
	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/platform/events"
)

// ev dựng event như repository ghi, rồi gán actor và thời điểm như khi đọc lại
func ev(t *testing.T, typ string, payload any, actor uuid.UUID) events.Event {
	t.Helper()
	e, err := events.New(typ, contract.AggregateAsset, uuid.New(), payload)
	if err != nil {
		t.Fatal(err)
	}
	e.ActorID = actor
	e.OccurredAt = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	return e
}

func str(s string) *string { return &s }

func updated(t *testing.T, changes ...contract.FieldChange) events.Event {
	return ev(t, contract.EventAssetUpdated, contract.AssetUpdated{Changes: changes}, uuid.Nil)
}

func change(field string, from, to any) contract.FieldChange {
	return contract.FieldChange{Field: field, From: from, To: to}
}

func TestResolveActivity_Kinds(t *testing.T) {
	got := resolveActivity([]events.Event{
		ev(t, contract.EventAssetCreated, contract.AssetCreated{}, uuid.Nil),
		ev(t, contract.EventAssetRetired, contract.AssetRetired{Reason: "Broken screen"}, uuid.Nil),
		ev(t, contract.EventAssetRestored, contract.AssetRestored{}, uuid.Nil),
		ev(t, "inventory.something_new", struct{}{}, uuid.Nil),
	}, activityLookups{})
	if len(got) != 4 {
		t.Fatalf("entries = %+v", got)
	}
	if got[0].Kind != "created" || len(got[0].Changes) != 0 {
		t.Errorf("created = %+v", got[0])
	}
	if got[1].Kind != "retired" || got[1].Reason != "Broken screen" {
		t.Errorf("retired = %+v", got[1])
	}
	if got[2].Kind != "restored" {
		t.Errorf("restored = %+v", got[2])
	}
	if got[3].Kind != "other" || got[3].Type != "inventory.something_new" {
		t.Errorf("other = %+v", got[3])
	}
}

func TestResolveActivity_FieldValues(t *testing.T) {
	s1, s2, laptop := uuid.New(), uuid.New(), uuid.New()
	lk := activityLookups{
		statuses:    map[uuid.UUID]domain.Status{s1: {ID: s1, Name: "Available"}, s2: {ID: s2, Name: "In use"}},
		currentType: laptop,
		types: []domain.AssetType{{ID: laptop, Name: "Laptop", Attributes: []domain.Attribute{
			{Key: "ram_gb", Label: "RAM", DataType: domain.TypeNumber, Unit: "GB"},
			{Key: "has_dock", Label: "Dock", DataType: domain.TypeBoolean},
		}}},
	}
	got := resolveActivity([]events.Event{updated(t,
		change("status_id", s1.String(), s2.String()),
		change("attributes.ram_gb", "8", "16"),
		change("attributes.has_dock", "true", "false"),
		change("purchase_date", nil, "2026-01-10"),
		change("asset_type_id", uuid.New().String(), laptop.String()),
	)}, lk)
	if len(got) != 1 {
		t.Fatalf("entries = %+v", got)
	}
	want := []ActivityChange{
		{Field: "status_id", Label: "Status", From: str("Available"), To: str("In use")},
		{Field: "attributes.ram_gb", Label: "RAM", From: str("8 GB"), To: str("16 GB")},
		{Field: "attributes.has_dock", Label: "Dock", From: str("Yes"), To: str("No")},
		{Field: "purchase_date", Label: "Purchase date", From: nil, To: str("2026-01-10")},
		{Field: "asset_type_id", Label: "Type", From: str("(deleted)"), To: str("Laptop")},
	}
	assertChanges(t, got[0].Changes, want)
}

func TestResolveActivity_RemovedOption(t *testing.T) {
	typ, windows, macos := uuid.New(), uuid.New(), uuid.New()
	gone := time.Now()
	lk := activityLookups{currentType: typ, types: []domain.AssetType{{ID: typ, Attributes: []domain.Attribute{{
		Key: "os", Label: "OS", DataType: domain.TypeSelect,
		Options: []domain.Option{{ID: windows, Label: "Windows", RemovedAt: &gone}, {ID: macos, Label: "macOS"}},
	}}}}}
	got := resolveActivity([]events.Event{updated(t, change("attributes.os", windows.String(), macos.String()))}, lk)
	assertChanges(t, got[0].Changes, []ActivityChange{{Field: "attributes.os", Label: "OS", From: str("Windows"), To: str("macOS")}})
}

func TestResolveActivity_LabelFromOtherType(t *testing.T) {
	t1, t2 := uuid.New(), uuid.New()
	lk := activityLookups{currentType: t2, types: []domain.AssetType{
		{ID: t2, Attributes: []domain.Attribute{{Key: "size_in", Label: "Size", DataType: domain.TypeNumber}}},
		{ID: t1, Attributes: []domain.Attribute{{Key: "serial", Label: "Serial number", DataType: domain.TypeText}}},
	}}
	got := resolveActivity([]events.Event{updated(t, change("attributes.serial", "A1", "B2"))}, lk)
	assertChanges(t, got[0].Changes, []ActivityChange{{Field: "attributes.serial", Label: "Serial number", From: str("A1"), To: str("B2")}})
}

func TestResolveActivity_HiddenFields(t *testing.T) {
	loc := uuid.New().String()
	got := resolveActivity([]events.Event{
		updated(t, change("location_id", nil, loc), change("holder_member_id", nil, uuid.New().String())),
		updated(t, change("location_id", loc, nil), change("name", "Old", "New")),
	}, activityLookups{})
	if len(got) != 1 {
		t.Fatalf("entries = %+v, want only the name change", got)
	}
	assertChanges(t, got[0].Changes, []ActivityChange{{Field: "name", Label: "Name", From: str("Old"), To: str("New")}})
}

func TestResolveActivity_Actors(t *testing.T) {
	known, unknown := uuid.New(), uuid.New()
	lk := activityLookups{accounts: map[uuid.UUID]idcontract.Account{known: {ID: known, Name: "Lê Văn Hùng"}}}
	got := resolveActivity([]events.Event{
		ev(t, contract.EventAssetRestored, contract.AssetRestored{}, uuid.Nil),
		ev(t, contract.EventAssetRestored, contract.AssetRestored{}, known),
		ev(t, contract.EventAssetRestored, contract.AssetRestored{}, unknown),
	}, lk)
	if got[0].Actor != nil {
		t.Errorf("system actor = %+v, want nil", got[0].Actor)
	}
	if got[1].Actor == nil || *got[1].Actor != (ActivityActor{ID: known, Name: "Lê Văn Hùng"}) {
		t.Errorf("known actor = %+v", got[1].Actor)
	}
	if got[2].Actor == nil || *got[2].Actor != (ActivityActor{ID: unknown, Name: "Unknown user"}) {
		t.Errorf("unknown actor = %+v", got[2].Actor)
	}
}

func assertChanges(t *testing.T, got, want []ActivityChange) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("changes = %+v, want %+v", show(got), show(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Field != w.Field || g.Label != w.Label || deref(g.From) != deref(w.From) || deref(g.To) != deref(w.To) ||
			(g.From == nil) != (w.From == nil) || (g.To == nil) != (w.To == nil) {
			t.Errorf("change %d = %+v, want %+v", i, show([]ActivityChange{g}), show([]ActivityChange{w}))
		}
	}
}

func show(cs []ActivityChange) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Field+"|"+c.Label+"|"+ptrStr(c.From)+"→"+ptrStr(c.To))
	}
	return out
}

func ptrStr(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

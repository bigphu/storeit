package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	idcontract "storeit/internal/identity/contract"
	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/events"
)

// Lịch sử hoạt động của tài sản (US-16): đọc event đã lưu trong platform.events, đổi ID
// thành tên người đọc được. Không ghi gì.

// AssetEventReader đọc event của một tài sản, mới nhất trước; before là ID event cuối của
// trang trước. domain không được import platform/events nên interface nằm ở đây
type AssetEventReader interface {
	Events(ctx context.Context, id uuid.UUID, before *uuid.UUID, limit int) ([]events.Event, error)
}

type ActivityEntry struct {
	ID         uuid.UUID
	Kind       string // "created" | "updated" | "retired" | "restored" | "other"
	Type       string // event type gốc (để hiện "other")
	OccurredAt time.Time
	Actor      *ActivityActor // nil = hệ thống
	Changes    []ActivityChange
	Reason     string // chỉ retired
}

type ActivityActor struct {
	ID   uuid.UUID
	Name string
}

// ActivityChange: giá trị trước và sau đã đổi sang chữ; nil là trống
type ActivityChange struct {
	Field, Label string
	From, To     *string
}

// activityLookups: dữ liệu tra cứu đã nạp sẵn cho một trang lịch sử
type activityLookups struct {
	statuses    map[uuid.UUID]domain.Status
	types       []domain.AssetType // các loại tài sản từng thuộc, kèm thuộc tính và option đã gỡ
	currentType uuid.UUID          // loại hiện tại của tài sản
	accounts    map[uuid.UUID]idcontract.Account
}

const (
	unknownUser  = "Unknown user"
	deletedValue = "(deleted)"
)

// Trường chung: nhãn hiện cho người đọc. location_id, holder_member_id không hiện (chưa có
// danh mục địa điểm; người giữ do mượn/trả hiện riêng)
var activityFieldLabels = map[string]string{
	"name": "Name", "description": "Description", "asset_type_id": "Type",
	"status_id": "Status", "purchase_date": "Purchase date",
}

var hiddenFields = map[string]bool{"location_id": true, "holder_member_id": true}

// AssetActivity: một trang lịch sử, mới nhất trước. next là ID event cuối của trang (theo
// event gốc, kể cả event bị bỏ vì chỉ đổi trường ẩn) khi còn event cũ hơn.
func (s *Service) AssetActivity(ctx context.Context, assetID uuid.UUID, before *uuid.UUID, limit int) ([]ActivityEntry, *uuid.UUID, error) {
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return nil, nil, err
	}
	a, err := s.assets.Get(ctx, assetID)
	if err != nil {
		return nil, nil, err
	}
	evts, err := s.events.Events(ctx, assetID, before, limit+1)
	if err != nil {
		return nil, nil, err
	}
	var next *uuid.UUID
	if len(evts) > limit {
		evts = evts[:limit]
		id := evts[limit-1].ID
		next = &id
	}
	lk, err := s.activityLookups(ctx, a, evts)
	if err != nil {
		return nil, nil, err
	}
	return resolveActivity(evts, lk), next, nil
}

// activityLookups nạp status, các loại tài sản từng thuộc (loại hiện tại và mọi loại xuất
// hiện trong event) và tên người thực hiện, mỗi thứ một lần cho cả trang
func (s *Service) activityLookups(ctx context.Context, a domain.Asset, evts []events.Event) (activityLookups, error) {
	lk := activityLookups{statuses: map[uuid.UUID]domain.Status{}, currentType: a.TypeID, accounts: map[uuid.UUID]idcontract.Account{}}
	statuses, err := s.statuses.List(ctx, true)
	if err != nil {
		return lk, err
	}
	for _, st := range statuses {
		lk.statuses[st.ID] = st
	}

	typeIDs := []uuid.UUID{a.TypeID}
	seen := map[uuid.UUID]bool{a.TypeID: true}
	addType := func(v any) {
		if id, ok := idOf(v); ok && !seen[id] {
			seen[id] = true
			typeIDs = append(typeIDs, id)
		}
	}
	var actorIDs []uuid.UUID
	seenActor := map[uuid.UUID]bool{}
	for _, e := range evts {
		if e.ActorID != uuid.Nil && !seenActor[e.ActorID] {
			seenActor[e.ActorID] = true
			actorIDs = append(actorIDs, e.ActorID)
		}
		switch e.Type {
		case contract.EventAssetCreated:
			if p, err := events.Decode[contract.AssetCreated](e); err == nil {
				addType(p.TypeID.String())
			}
		case contract.EventAssetUpdated:
			if p, err := events.Decode[contract.AssetUpdated](e); err == nil {
				for _, c := range p.Changes {
					if c.Field == "asset_type_id" {
						addType(c.From)
						addType(c.To)
					}
				}
			}
		}
	}
	for _, id := range typeIDs {
		t, err := s.types.Get(ctx, id)
		if errors.Is(err, domain.ErrTypeNotFound) {
			continue
		}
		if err != nil {
			return lk, err
		}
		lk.types = append(lk.types, t)
	}
	// Thuộc tính của loại cũ mà trang này không có event đổi loại (trang sau của lịch sử):
	// tìm ở mọi loại có key đó, chỉ khi thiếu
	if err := s.loadTypesForMissingKeys(ctx, &lk, evts, seen); err != nil {
		return lk, err
	}

	if len(actorIDs) > 0 {
		accs, err := s.accounts.GetAccounts(ctx, actorIDs)
		if err != nil {
			return lk, err
		}
		for _, acc := range accs {
			lk.accounts[acc.ID] = acc
		}
	}
	return lk, nil
}

// loadTypesForMissingKeys nạp thêm các loại (kể cả đã lưu trữ) khi trang có thuộc tính mà
// các loại đã nạp không có key đó
func (s *Service) loadTypesForMissingKeys(ctx context.Context, lk *activityLookups, evts []events.Event, loaded map[uuid.UUID]bool) error {
	missing := false
	for _, e := range evts {
		if e.Type != contract.EventAssetUpdated {
			continue
		}
		p, err := events.Decode[contract.AssetUpdated](e)
		if err != nil {
			continue
		}
		for _, c := range p.Changes {
			if key, ok := strings.CutPrefix(c.Field, "attributes."); ok {
				if _, found := lk.attribute(key); !found {
					missing = true
				}
			}
		}
	}
	if !missing {
		return nil
	}
	all, err := s.types.List(ctx, true)
	if err != nil {
		return err
	}
	for _, t := range all {
		if loaded[t.ID] {
			continue
		}
		full, err := s.types.Get(ctx, t.ID)
		if errors.Is(err, domain.ErrTypeNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		loaded[t.ID] = true
		lk.types = append(lk.types, full)
	}
	return nil
}

// resolveActivity đổi event thành entry; event asset_updated chỉ đổi trường ẩn thì bỏ
func resolveActivity(evts []events.Event, lk activityLookups) []ActivityEntry {
	out := make([]ActivityEntry, 0, len(evts))
	for _, e := range evts {
		en := ActivityEntry{ID: e.ID, Type: e.Type, OccurredAt: e.OccurredAt, Actor: lk.actor(e.ActorID)}
		switch e.Type {
		case contract.EventAssetCreated:
			en.Kind = "created"
		case contract.EventAssetRetired:
			en.Kind = "retired"
			if p, err := events.Decode[contract.AssetRetired](e); err == nil {
				en.Reason = p.Reason
			}
		case contract.EventAssetRestored:
			en.Kind = "restored"
		case contract.EventAssetUpdated:
			en.Kind = "updated"
			p, err := events.Decode[contract.AssetUpdated](e)
			if err != nil {
				continue
			}
			for _, c := range p.Changes {
				if hiddenFields[c.Field] {
					continue
				}
				en.Changes = append(en.Changes, lk.change(c))
			}
			if len(en.Changes) == 0 {
				continue
			}
		default:
			en.Kind = "other"
		}
		out = append(out, en)
	}
	return out
}

func (lk activityLookups) actor(id uuid.UUID) *ActivityActor {
	if id == uuid.Nil {
		return nil
	}
	if acc, ok := lk.accounts[id]; ok {
		return &ActivityActor{ID: id, Name: acc.Name}
	}
	return &ActivityActor{ID: id, Name: unknownUser}
}

func (lk activityLookups) change(c contract.FieldChange) ActivityChange {
	out := ActivityChange{Field: c.Field, Label: c.Field}
	if key, ok := strings.CutPrefix(c.Field, "attributes."); ok {
		a, found := lk.attribute(key)
		if !found {
			out.Label = key
			out.From, out.To = text(c.From), text(c.To)
			return out
		}
		out.Label = a.Label
		out.From, out.To = attrValue(a, c.From), attrValue(a, c.To)
		return out
	}
	if l, ok := activityFieldLabels[c.Field]; ok {
		out.Label = l
	}
	switch c.Field {
	case "status_id":
		out.From, out.To = lk.statusName(c.From), lk.statusName(c.To)
	case "asset_type_id":
		out.From, out.To = lk.typeName(c.From), lk.typeName(c.To)
	default:
		out.From, out.To = text(c.From), text(c.To)
	}
	return out
}

// attribute: loại hiện tại trước, rồi loại khác từng thuộc có key đó
func (lk activityLookups) attribute(key string) (domain.Attribute, bool) {
	for _, current := range []bool{true, false} {
		for _, t := range lk.types {
			if (t.ID == lk.currentType) != current {
				continue
			}
			for _, a := range t.Attributes {
				if a.Key == key {
					return a, true
				}
			}
		}
	}
	return domain.Attribute{}, false
}

func (lk activityLookups) statusName(v any) *string {
	id, ok := idOf(v)
	if !ok {
		return text(v)
	}
	if st, found := lk.statuses[id]; found {
		return &st.Name
	}
	return strp(deletedValue)
}

func (lk activityLookups) typeName(v any) *string {
	id, ok := idOf(v)
	if !ok {
		return text(v)
	}
	for _, t := range lk.types {
		if t.ID == id {
			return strp(t.Name)
		}
	}
	return strp(deletedValue)
}

// attrValue định dạng giá trị thuộc tính theo kiểu: số kèm đơn vị, Yes/No, nhãn option
func attrValue(a domain.Attribute, v any) *string {
	s := text(v)
	if s == nil {
		return nil
	}
	switch a.DataType {
	case domain.TypeNumber:
		if a.Unit != "" {
			return strp(*s + " " + a.Unit)
		}
	case domain.TypeBoolean:
		switch *s {
		case "true":
			return strp("Yes")
		case "false":
			return strp("No")
		}
	case domain.TypeSelect:
		for _, o := range a.Options {
			if o.ID.String() == *s {
				return strp(o.Label)
			}
		}
		return strp(deletedValue)
	}
	return s
}

// text: giá trị trong payload (JSON) thành chữ; nil hay "" là trống
func text(v any) *string {
	if v == nil {
		return nil
	}
	s := fmt.Sprint(v)
	if s == "" {
		return nil
	}
	return &s
}

func idOf(v any) (uuid.UUID, bool) {
	s := text(v)
	if s == nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(*s)
	return id, err == nil
}

func strp(s string) *string { return &s }

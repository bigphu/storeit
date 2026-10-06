package spreadsheet

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
)

// Formatter đổi giá trị một trường của một tài sản thành ô theo bố cục. Types phải có
// thuộc tính và option của mọi loại có trong file.
type Formatter struct {
	Layout domain.ExportLayout
	Types  map[uuid.UUID]domain.AssetType
	Loc    *time.Location // múi giờ cho updated_at; nil là UTC
}

func (f Formatter) Cell(a domain.AssetListItem, c domain.SheetColumn) Cell {
	switch c.Field {
	case "tag":
		return TextCell(a.Tag)
	case "name":
		return TextCell(a.Name)
	case "description":
		return TextCell(a.Description)
	case "type":
		return TextCell(a.TypeName)
	case "status":
		if f.Layout.StatusAs == domain.StatusAsKind {
			return TextCell(a.StatusKind.Label())
		}
		return TextCell(a.StatusName)
	case "purchase_date":
		if a.PurchaseDate == nil {
			return TextCell("")
		}
		return f.date(*a.PurchaseDate)
	case "updated_at":
		loc := f.Loc
		if loc == nil {
			loc = time.UTC
		}
		t := a.UpdatedAt.In(loc)
		return Cell{Kind: Date, Time: time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC),
			NumFmt: string(f.Layout.DateFormat) + " hh:mm"}
	}
	return f.attribute(a, c.Key)
}

func (f Formatter) date(d time.Time) Cell {
	return Cell{Kind: Date, Time: d, NumFmt: string(f.Layout.DateFormat)}
}

func (f Formatter) attribute(a domain.AssetListItem, key string) Cell {
	t, ok := f.Types[a.TypeID]
	if !ok || key == "" {
		return TextCell("")
	}
	var attr *domain.Attribute
	for i := range t.Attributes {
		if t.Attributes[i].Key == key && t.Attributes[i].RemovedAt == nil {
			attr = &t.Attributes[i]
		}
	}
	if attr == nil {
		return TextCell("")
	}
	for _, v := range a.Values {
		if v.AttributeID != attr.ID {
			continue
		}
		switch {
		case v.Text != nil:
			return TextCell(*v.Text)
		case v.Number != nil:
			if f.Layout.UnitIn == domain.UnitInCell && attr.Unit != "" {
				return TextCell(*v.Number + " " + attr.Unit)
			}
			n, err := strconv.ParseFloat(*v.Number, 64)
			if err != nil {
				return TextCell(*v.Number)
			}
			return Cell{Kind: Number, Num: n}
		case v.Date != nil:
			return f.date(*v.Date)
		case v.Bool != nil:
			return f.boolean(*v.Bool)
		case v.OptionID != nil:
			for _, o := range attr.Options {
				if o.ID == *v.OptionID {
					return TextCell(o.Label)
				}
			}
		}
	}
	return TextCell("")
}

func (f Formatter) boolean(b bool) Cell {
	switch f.Layout.BoolStyle {
	case domain.BoolTrueFalse:
		return Cell{Kind: Bool, Bool: b}
	case domain.BoolCheck:
		if b {
			return TextCell("✓")
		}
		return TextCell("–")
	}
	if b {
		return TextCell("Yes")
	}
	return TextCell("No")
}

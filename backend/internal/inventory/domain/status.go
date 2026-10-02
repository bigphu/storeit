package domain

import (
	"time"

	"github.com/google/uuid"
)

// StatusKind quyết định hành vi; tên status chỉ để hiển thị. Mượn tài sản sẽ
// đòi kind available; retire dùng status mặc định của kind retired.
type StatusKind string

const (
	KindAvailable   StatusKind = "available"
	KindInUse       StatusKind = "in_use"
	KindUnavailable StatusKind = "unavailable"
	KindRetired     StatusKind = "retired"
)

func (k StatusKind) Valid() bool {
	switch k {
	case KindAvailable, KindInUse, KindUnavailable, KindRetired:
		return true
	}
	return false
}

// Status là một trạng thái tài sản do người quản lý đặt ("Lost", "Being repaired")
type Status struct {
	ID         uuid.UUID
	Name       string
	Kind       StatusKind
	IsDefault  bool // mặc định của kind: mỗi kind đúng một
	IsSystem   bool // seed sẵn: không archive, không đổi kind
	Position   int32
	ArchivedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (s Status) Archived() bool { return s.ArchivedAt != nil }

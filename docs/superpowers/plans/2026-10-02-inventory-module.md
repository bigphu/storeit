# Inventory Module (Sprint 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Asset types with typed custom attributes (units, choice options), managed statuses, and assets with create/view/full-update/retire/restore/search, exposed under `/api/v1` with permissions.

**Architecture:** New Postgres schema `inventory` (migration `00003_inventory.sql`) with common asset fields as columns and custom values as typed rows protected by composite foreign keys. The module follows identity's layering: `domain` (rules, value validation, interfaces), `repository` (sqlc, transactions, outbox events), `service` (permission checks, orchestration), `handler` (OpenAPI strict server), module root (`New`, `Mount`, `APIDoc`).

**Tech Stack:** Go, pgx v5, sqlc 1.31.1, goose, oapi-codegen strict server, kin-openapi validation, platform packages (`errs`, `auth`, `database`, `events`, `web`, `middleware`).

**Spec:** `docs/superpowers/specs/2026-10-02-inventory-schema-design.md`

## Global Constraints

- Code comments and package docs in Vietnamese; files LF.
- Never hand-edit `*.gen.go` or `repository/db`; run sqlc (`make sqlc`, or `$(go env GOPATH)/bin/sqlc.exe generate` when Docker is down — same 1.31.1) and `go generate ./internal/inventory/handler/...`.
- Read `backend/docs/platform.md` before using platform packages. Transactions, outbox `Append` live in repositories; services never see `pgx.Tx`.
- Module routes: `Middlewares: {web.ValidateRequests(spec, base), …, auth.Middleware(tokens, …)}` (last runs first); operation lists via `web.MustOperations`; validator does not apply defaults.
- depguard lints tests: HTTP and DB-wiring tests live at the module root (`package inventory_test`), not in `handler/`.
- No cross-module foreign keys (`location_id`, `holder_member_id` are plain uuids).
- Every row lock on an asset uses `FOR NO KEY UPDATE`.
- Tag: trimmed, uppercased, `^[A-Z0-9][A-Z0-9._-]{0,63}$`, unique across all assets, immutable.
- Type code `^[A-Z0-9_-]{1,32}$` immutable; attribute key `^[a-z][a-z0-9_]{0,31}$` immutable, unique per type including removed ones; unit only for `number`, 1..16 chars.
- Inventory permissions are granted to Administrator in the migration (nobody can grant what they don't hold).
- Tests: `CI=true go test -count=1 -p 1 ./...` in `backend/`.

## Review Focus

1. A number with many digits or decimals (`1234567890123456.5`) → 422 "must be a number with at most 15 digits and 6 decimals", never silently rounded. Test: Task 2 `TestValidateValues` row.
2. Changing an attribute's data type away from `select` while it has options but no values → succeeds and drops the options; with values → 409 `attribute-in-use`. Test: Task 3 `TestTypes_ChangeDataType`.
3. Two assets created at once with the same tag (different case: `lap-1` / `LAP-1`) → exactly one 201, the other 409 `tag-taken`. Test: Task 3 `TestAssets_TagUniqueAnyCase`.
4. An asset whose current status or type was archived is still editable when status/type are unchanged; choosing an archived one is 422. Test: Task 4 `TestAssetRules_Archived`.
5. `PUT` with `attributes` omitted entirely clears all optional values and fails if the type has required attributes (full replacement semantics) → documented and tested. Test: Task 4 `TestUpdateAsset_FullReplacement`.

---

### Task 1: Migration, seeds, sqlc block

**Files:**
- Create: `backend/migrations/00003_inventory.sql`
- Modify: `backend/sqlc.yml` (uncomment the M4 block)
- Create: `backend/internal/inventory/repository/queries/asset_types.sql` (one trivial query so sqlc has a file; filled in Task 3)
- Test: `backend/internal/inventory/repository/schema_db_test.go`

**Produces:** tables in section 1–2 of the spec; fixed seed IDs:
`GENERAL` type `00000000-0000-7000-8000-000000000101`; statuses Available `…0201` (available), In use `…0202` (in_use), Under repair `…0203` (unavailable), Retired `…0204` (retired).

- [ ] **Step 1: Write the schema test** (`repository_test` package; `dbtest.Pool(t)`), one sub-test per guarantee, each inserting raw SQL rows with fresh uuids:
  - seeds: GENERAL type is system and active; four statuses, one default per kind; four inventory permissions exist; Administrator has all four, Inventory Officer has `asset.read`+`asset.manage`, Authorized Manager has `asset.read`+`type.manage`+`status.manage`, Employee has `asset.read`.
  - value of wrong column for type (`data_type='number'` with `value_text`) → check violation (`23514`).
  - two value columns filled → check violation.
  - value for an attribute of another type → FK violation (`23503`).
  - select value with an option of another attribute → FK violation.
  - `UPDATE asset_type_attributes SET data_type='text'` while a value exists → FK violation.
  - unit on a `text` attribute → check violation.
  - duplicate tag → unique violation (`23505`), lowercase tag → check violation.
  - two default statuses of one kind → unique violation; archived default → check violation.
  - option for a non-select attribute → FK violation.
- [ ] **Step 2: Run** `CI=true go test -count=1 -run TestSchema ./internal/inventory/repository/` → FAIL (`relation "inventory.assets" does not exist`).
- [ ] **Step 3: Write the migration** exactly:

```sql
-- Module inventory: loại tài sản và thuộc tính riêng, status, tài sản.
-- Giá trị thuộc tính là hàng có kiểu, khoá ngoại ghép giữ đúng kiểu và đúng loại.

-- +goose Up
CREATE SCHEMA IF NOT EXISTS inventory;

CREATE TABLE inventory.asset_types (
    id          uuid        PRIMARY KEY,
    code        text        NOT NULL,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    is_system   boolean     NOT NULL DEFAULT false,
    archived_at timestamptz,
    version     integer     NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_types_code_key UNIQUE (code),
    CONSTRAINT asset_types_code_check CHECK (code ~ '^[A-Z0-9_-]{1,32}$'),
    CONSTRAINT asset_types_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    CONSTRAINT asset_types_description_check CHECK (char_length(description) <= 500),
    CONSTRAINT asset_types_system_active CHECK (NOT (is_system AND archived_at IS NOT NULL))
);
CREATE UNIQUE INDEX asset_types_name_lower ON inventory.asset_types (lower(name));

CREATE TABLE inventory.asset_type_attributes (
    id            uuid        PRIMARY KEY,
    asset_type_id uuid        NOT NULL REFERENCES inventory.asset_types (id),
    key           text        NOT NULL,
    label         text        NOT NULL,
    data_type     text        NOT NULL,
    unit          text,
    is_required   boolean     NOT NULL DEFAULT false,
    position      integer     NOT NULL DEFAULT 0,
    removed_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    -- Key không bao giờ dùng lại trong một loại, kể cả khi thuộc tính đã gỡ
    CONSTRAINT asset_type_attributes_key_key UNIQUE (asset_type_id, key),
    -- Đích của khoá ngoại ghép từ giá trị và option
    CONSTRAINT asset_type_attributes_ref_key UNIQUE (id, asset_type_id, data_type),
    CONSTRAINT asset_type_attributes_type_ref_key UNIQUE (id, data_type),
    CONSTRAINT asset_type_attributes_key_check CHECK (key ~ '^[a-z][a-z0-9_]{0,31}$'),
    CONSTRAINT asset_type_attributes_label_check CHECK (char_length(btrim(label)) BETWEEN 1 AND 100),
    CONSTRAINT asset_type_attributes_data_type_check
        CHECK (data_type IN ('text', 'number', 'date', 'boolean', 'select')),
    CONSTRAINT asset_type_attributes_unit_check
        CHECK (unit IS NULL OR (data_type = 'number' AND char_length(btrim(unit)) BETWEEN 1 AND 16))
);
CREATE UNIQUE INDEX asset_type_attributes_label_lower
    ON inventory.asset_type_attributes (asset_type_id, lower(label)) WHERE removed_at IS NULL;

CREATE TABLE inventory.asset_attribute_options (
    id           uuid        PRIMARY KEY,
    attribute_id uuid        NOT NULL,
    -- Chỉ thuộc tính kiểu select mới có option
    data_type    text        NOT NULL DEFAULT 'select',
    label        text        NOT NULL,
    position     integer     NOT NULL DEFAULT 0,
    removed_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_attribute_options_attribute_fkey FOREIGN KEY (attribute_id, data_type)
        REFERENCES inventory.asset_type_attributes (id, data_type),
    CONSTRAINT asset_attribute_options_ref_key UNIQUE (id, attribute_id),
    CONSTRAINT asset_attribute_options_data_type_check CHECK (data_type = 'select'),
    CONSTRAINT asset_attribute_options_label_check CHECK (char_length(btrim(label)) BETWEEN 1 AND 100)
);
CREATE UNIQUE INDEX asset_attribute_options_label_lower
    ON inventory.asset_attribute_options (attribute_id, lower(label)) WHERE removed_at IS NULL;

CREATE TABLE inventory.asset_statuses (
    id          uuid        PRIMARY KEY,
    name        text        NOT NULL,
    kind        text        NOT NULL,
    is_default  boolean     NOT NULL DEFAULT false,
    is_system   boolean     NOT NULL DEFAULT false,
    position    integer     NOT NULL DEFAULT 0,
    archived_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT asset_statuses_kind_check CHECK (kind IN ('available', 'in_use', 'unavailable', 'retired')),
    CONSTRAINT asset_statuses_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    CONSTRAINT asset_statuses_default_active CHECK (NOT (is_default AND archived_at IS NOT NULL)),
    CONSTRAINT asset_statuses_system_active CHECK (NOT (is_system AND archived_at IS NOT NULL))
);
CREATE UNIQUE INDEX asset_statuses_name_lower ON inventory.asset_statuses (lower(name));
CREATE UNIQUE INDEX asset_statuses_default_per_kind ON inventory.asset_statuses (kind) WHERE is_default;

CREATE TABLE inventory.assets (
    id               uuid        PRIMARY KEY,
    tag              text        NOT NULL,
    name             text        NOT NULL,
    description      text        NOT NULL DEFAULT '',
    asset_type_id    uuid        NOT NULL REFERENCES inventory.asset_types (id),
    status_id        uuid        NOT NULL REFERENCES inventory.asset_statuses (id),
    -- directory làm sau: chưa có khoá ngoại (giống identity.accounts.member_id)
    location_id      uuid,
    holder_member_id uuid,
    purchase_date    date,
    retired_at       timestamptz,
    retired_reason   text,
    version          integer     NOT NULL DEFAULT 1,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    -- Tag viết hoa sẵn (CHECK) nên UNIQUE thường cũng không phân biệt hoa thường;
    -- giữ cả tài sản đã retire: tag không bao giờ dùng lại
    CONSTRAINT assets_tag_key UNIQUE (tag),
    CONSTRAINT assets_ref_key UNIQUE (id, asset_type_id),
    CONSTRAINT assets_tag_check CHECK (tag ~ '^[A-Z0-9][A-Z0-9._-]{0,63}$'),
    CONSTRAINT assets_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 200),
    CONSTRAINT assets_description_check CHECK (char_length(description) <= 2000),
    CONSTRAINT assets_retired_reason_check
        CHECK (retired_reason IS NULL OR (retired_at IS NOT NULL AND char_length(retired_reason) <= 500))
);
CREATE INDEX assets_type_idx ON inventory.assets (asset_type_id) WHERE retired_at IS NULL;
CREATE INDEX assets_status_idx ON inventory.assets (status_id) WHERE retired_at IS NULL;
CREATE INDEX assets_location_idx ON inventory.assets (location_id) WHERE retired_at IS NULL;
CREATE INDEX assets_holder_idx ON inventory.assets (holder_member_id) WHERE retired_at IS NULL;

CREATE TABLE inventory.asset_attribute_values (
    asset_id        uuid NOT NULL,
    attribute_id    uuid NOT NULL,
    asset_type_id   uuid NOT NULL,
    data_type       text NOT NULL,
    value_text      text,
    value_number    numeric,
    value_date      date,
    value_bool      boolean,
    value_option_id uuid,
    PRIMARY KEY (asset_id, attribute_id),
    -- Giá trị thuộc đúng loại hiện tại của tài sản
    CONSTRAINT asset_attribute_values_asset_fkey FOREIGN KEY (asset_id, asset_type_id)
        REFERENCES inventory.assets (id, asset_type_id),
    -- Thuộc tính cùng loại, cùng kiểu dữ liệu: đổi kiểu khi còn giá trị bị chặn
    CONSTRAINT asset_attribute_values_attribute_fkey FOREIGN KEY (attribute_id, asset_type_id, data_type)
        REFERENCES inventory.asset_type_attributes (id, asset_type_id, data_type),
    -- Option phải là của chính thuộc tính này
    CONSTRAINT asset_attribute_values_option_fkey FOREIGN KEY (value_option_id, attribute_id)
        REFERENCES inventory.asset_attribute_options (id, attribute_id),
    CONSTRAINT asset_attribute_values_one_value
        CHECK (num_nonnulls(value_text, value_number, value_date, value_bool, value_option_id) = 1),
    CONSTRAINT asset_attribute_values_typed CHECK (
        (data_type = 'text' AND value_text IS NOT NULL) OR
        (data_type = 'number' AND value_number IS NOT NULL) OR
        (data_type = 'date' AND value_date IS NOT NULL) OR
        (data_type = 'boolean' AND value_bool IS NOT NULL) OR
        (data_type = 'select' AND value_option_id IS NOT NULL)),
    CONSTRAINT asset_attribute_values_text_check
        CHECK (value_text IS NULL OR char_length(btrim(value_text)) BETWEEN 1 AND 1000)
);
CREATE INDEX asset_attribute_values_attribute_idx ON inventory.asset_attribute_values (attribute_id);
CREATE INDEX asset_attribute_values_option_idx
    ON inventory.asset_attribute_values (value_option_id) WHERE value_option_id IS NOT NULL;

-- Seed: loại chung và bốn status mặc định, ID cố định để code tham chiếu
INSERT INTO inventory.asset_types (id, code, name, description, is_system) VALUES
    ('00000000-0000-7000-8000-000000000101', 'GENERAL', 'General', 'Assets without specific attributes', true);

INSERT INTO inventory.asset_statuses (id, name, kind, is_default, is_system, position) VALUES
    ('00000000-0000-7000-8000-000000000201', 'Available',    'available',   true, true, 1),
    ('00000000-0000-7000-8000-000000000202', 'In use',       'in_use',      true, true, 2),
    ('00000000-0000-7000-8000-000000000203', 'Under repair', 'unavailable', true, true, 3),
    ('00000000-0000-7000-8000-000000000204', 'Retired',      'retired',     true, true, 4);

-- Quyền của inventory vào danh mục quyền của identity, kèm phân cho role hệ thống.
-- Administrator nhận tất cả: không ai trao được quyền mình không có.
INSERT INTO identity.permissions (code, description) VALUES
    ('inventory.asset.read',    'View assets, asset types and statuses'),
    ('inventory.asset.manage',  'Create, update, retire and restore assets'),
    ('inventory.type.manage',   'Manage asset types, custom attributes and options'),
    ('inventory.status.manage', 'Manage asset statuses');

INSERT INTO identity.role_permissions (role_id, permission) VALUES
    ('00000000-0000-7000-8000-000000000001', 'inventory.asset.read'),
    ('00000000-0000-7000-8000-000000000001', 'inventory.asset.manage'),
    ('00000000-0000-7000-8000-000000000001', 'inventory.type.manage'),
    ('00000000-0000-7000-8000-000000000001', 'inventory.status.manage'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.asset.read'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.type.manage'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.status.manage'),
    ('00000000-0000-7000-8000-000000000003', 'inventory.asset.read'),
    ('00000000-0000-7000-8000-000000000003', 'inventory.asset.manage'),
    ('00000000-0000-7000-8000-000000000004', 'inventory.asset.read');

-- +goose Down
DELETE FROM identity.role_permissions WHERE permission LIKE 'inventory.%';
DELETE FROM identity.permissions WHERE code LIKE 'inventory.%';
DROP TABLE inventory.asset_attribute_values;
DROP TABLE inventory.assets;
DROP TABLE inventory.asset_statuses;
DROP TABLE inventory.asset_attribute_options;
DROP TABLE inventory.asset_type_attributes;
DROP TABLE inventory.asset_types;
DROP SCHEMA inventory;
```

- [ ] **Step 4:** Uncomment the M4 block in `sqlc.yml`; add `queries/asset_types.sql` with `-- name: GetAssetType :one SELECT * FROM inventory.asset_types WHERE id = @id;`; run sqlc; `go build ./...`.
- [ ] **Step 5: Run** schema test → PASS; run identity repository tests (seed rows added to `identity.permissions` must not break `TestSchema_SeedRoles`, which checks only identity grants — adjust it to filter `permission LIKE 'identity.%'` if it fails, ledger the change).
- [ ] **Step 6: Commit** `feat(inventory): schema, seeds and permissions`.

### Task 2: Domain

**Files:**
- Delete placeholders: `internal/inventory/domain/category.go`
- Create/replace: `domain/asset_type.go`, `domain/status.go`, `domain/asset.go`, `domain/values.go`, `domain/errors.go`, `domain/permissions.go`, `domain/repository.go`
- Test: `domain/domain_test.go`

**Produces:**

```go
// permissions.go
const (
	PermAssetRead    = "inventory.asset.read"
	PermAssetManage  = "inventory.asset.manage"
	PermTypeManage   = "inventory.type.manage"
	PermStatusManage = "inventory.status.manage"
)
var GeneralTypeID = uuid.MustParse("00000000-0000-7000-8000-000000000101")

// asset_type.go
type DataType string // "text","number","date","boolean","select"; Valid()
type AssetType struct {
	ID uuid.UUID; Code, Name, Description string; IsSystem bool
	ArchivedAt *time.Time; Version int32; CreatedAt, UpdatedAt time.Time
	Attributes []Attribute // only from Get; ordered by position, label
}
type Attribute struct {
	ID, TypeID uuid.UUID; Key, Label string; DataType DataType
	Unit string // "" = none
	Required bool; Position int32; RemovedAt *time.Time; Options []Option
}
type Option struct { ID, AttributeID uuid.UUID; Label string; Position int32; RemovedAt *time.Time }
func NormalizeTypeCode(s string) (string, error)   // trim+upper, ErrInvalidTypeCode
func ValidateAttributeKey(s string) error          // ErrInvalidAttributeKey
func CleanLabel(s string, max int) (string, error) // trim, no control chars, 1..max; ErrInvalidLabel
func CheckUnit(dt DataType, unit string) (string, error) // trim; non-empty only for number, <=16; ErrInvalidUnit
func (t AssetType) ActiveAttributes() []Attribute

// status.go
type StatusKind string // available,in_use,unavailable,retired; Valid()
type Status struct { ID uuid.UUID; Name string; Kind StatusKind; IsDefault, IsSystem bool; Position int32; ArchivedAt *time.Time }

// asset.go
type Asset struct {
	ID uuid.UUID; Tag, Name, Description string; TypeID, StatusID uuid.UUID
	LocationID, HolderMemberID *uuid.UUID; PurchaseDate *time.Time
	RetiredAt *time.Time; RetiredReason string; Version int32; CreatedAt, UpdatedAt time.Time
	Values []Value
}
func NormalizeTag(s string) (string, error) // trim+upper, regexp, ErrInvalidTag
func (a Asset) Retired() bool

// values.go
type Value struct {
	AttributeID uuid.UUID; DataType DataType
	Text *string; Number *string /* canonical decimal */; Date *time.Time; Bool *bool; OptionID *uuid.UUID
}
// ValidateValues kiểm tra input (map key -> giá trị JSON đã decode: string, float64, bool)
// theo thuộc tính đang hoạt động của loại. Mọi lỗi gom vào một ErrInvalidAttributeValues
// với một FieldError mỗi key: "attributes.<key>".
func ValidateValues(t AssetType, in map[string]any) ([]Value, error)
```

Rules in `ValidateValues` (one field error per key, sorted by key):
- key not an active attribute → "unknown attribute"
- nil value → treated as absent
- required active attribute absent → "is required"
- text: string, trimmed non-empty, ≤ 1000 runes, no control chars except `\n` `\t` → else "must be text up to 1000 characters"
- number: float64 (JSON number), finite, formatted `strconv.FormatFloat(f, 'f', -1, 64)`, at most 15 significant digits and 6 decimals → else "must be a number with at most 15 digits and 6 decimals"
- date: string `2006-01-02` → else "must be a date YYYY-MM-DD"
- boolean: bool → else "must be true or false"
- select: string uuid of an active option of that attribute → else "is not an option of this attribute"

Errors (`errs` values, types under `/errors/…`): `ErrInvalidAttributeValues` 422, `ErrInvalidTag` 422 (field `tag`), `ErrInvalidTypeCode` 422 (`code`), `ErrInvalidAttributeKey` 422 (`key`), `ErrInvalidLabel` 422 (`label`/`name`), `ErrInvalidUnit` 422 (`unit`), `ErrInvalidDataType` 422 (`data_type`), `ErrInvalidStatusKind` 422 (`kind`), `ErrTypeArchived` 422, `ErrStatusArchived` 422, `ErrRetiredStatus` 422 ("use retire"), `ErrNotSelectAttribute` 422; 409: `ErrTagTaken`, `ErrTypeCodeTaken`, `ErrTypeNameTaken`, `ErrAttributeKeyTaken`, `ErrAttributeLabelTaken`, `ErrOptionLabelTaken`, `ErrStatusNameTaken`, `ErrAssetChanged`, `ErrAssetTypeChanged`, `ErrAssetRetired`, `ErrAttributeInUse`, `ErrSystemType`, `ErrSystemStatus`, `ErrStatusIsDefault` (archiving the default of a kind: make another status default first); 404: `ErrAssetNotFound`, `ErrTypeNotFound`, `ErrAttributeNotFound`, `ErrOptionNotFound`, `ErrStatusNotFound`.

Repository interfaces (`repository.go`):

```go
type NewAttribute struct { Key, Label string; DataType DataType; Unit string; Required bool; Position int32; Options []string }
type NewAssetType struct { Code, Name, Description string; Attributes []NewAttribute }
type AttributeChange struct { Label, Unit *string; DataType *DataType; Required *bool; Position *int32 }
type TypeRepository interface {
	List(ctx context.Context, includeArchived bool) ([]AssetType, error)
	Get(ctx context.Context, id uuid.UUID) (AssetType, error) // with attributes (incl. removed) and options
	Create(ctx context.Context, in NewAssetType) (AssetType, error)
	Update(ctx context.Context, id uuid.UUID, name, description *string, version int32) (AssetType, error)
	SetArchived(ctx context.Context, id uuid.UUID, archived bool) (AssetType, error)
	AddAttribute(ctx context.Context, typeID uuid.UUID, in NewAttribute) (Attribute, error)
	UpdateAttribute(ctx context.Context, typeID, attrID uuid.UUID, ch AttributeChange) (Attribute, error)
	RemoveAttribute(ctx context.Context, typeID, attrID uuid.UUID) error
	AddOption(ctx context.Context, typeID, attrID uuid.UUID, label string, position int32) (Option, error)
	UpdateOption(ctx context.Context, typeID, attrID, optID uuid.UUID, label *string, position *int32) (Option, error)
	RemoveOption(ctx context.Context, typeID, attrID, optID uuid.UUID) error
}
type NewStatus struct { Name string; Kind StatusKind; Position int32 }
type StatusChange struct { Name *string; Position *int32; MakeDefault bool }
type StatusRepository interface {
	List(ctx context.Context, includeArchived bool) ([]Status, error)
	Get(ctx context.Context, id uuid.UUID) (Status, error)
	Default(ctx context.Context, kind StatusKind) (Status, error)
	Create(ctx context.Context, in NewStatus) (Status, error)
	Update(ctx context.Context, id uuid.UUID, ch StatusChange) (Status, error)
	Archive(ctx context.Context, id uuid.UUID) (Status, error)
}
type AssetFields struct {
	Name, Description string; TypeID, StatusID uuid.UUID
	LocationID, HolderMemberID *uuid.UUID; PurchaseDate *time.Time; Values []Value
}
type AssetSort string // "tag","-tag","name","-name","purchase_date","-purchase_date","updated_at","-updated_at"
type AssetFilter struct {
	Query string; TypeID, StatusID, LocationID, HolderMemberID *uuid.UUID; StatusKind *StatusKind
	IncludeRetired bool; Sort AssetSort; Limit, Offset int32
}
type AssetListItem struct { Asset; TypeName, StatusName string; StatusKind StatusKind }
type AssetRepository interface {
	Create(ctx context.Context, tag string, f AssetFields) (Asset, error)
	Get(ctx context.Context, id uuid.UUID) (Asset, error) // with values
	List(ctx context.Context, f AssetFilter) ([]AssetListItem, int64, error)
	// Replace: khoá hàng (NO KEY UPDATE), ErrAssetRetired, ErrAssetChanged; đổi loại thì xoá giá trị cũ trước
	Replace(ctx context.Context, id uuid.UUID, f AssetFields, version int32) (Asset, error)
	Retire(ctx context.Context, id uuid.UUID, reason string, retiredStatus uuid.UUID, version int32) (Asset, error)
	Restore(ctx context.Context, id uuid.UUID, availableStatus uuid.UUID, version int32) (Asset, error)
}
```

- [ ] **Step 1: Tests** (`domain_test.go`): table tests for `NormalizeTag` (`" lap-1 "`→`LAP-1`; `"-x"`, `""`, 65 chars, `"A B"` invalid), `NormalizeTypeCode`, `ValidateAttributeKey` (`ram_gb` ok; `Ram`, `1x`, 33 chars invalid), `CleanLabel` (control chars rejected), `CheckUnit` (unit on text → error; `" GB "`→`GB`; 17 chars → error), and `TestValidateValues` with a type holding: required text `serial`, number `ram_gb` (unit GB), date `warranty_end`, boolean `has_dock`, select `os` (options Windows active, Legacy removed), removed text `old`:
  - all valid → 5 values with canonical number `"16"`/`"15.6"`, parsed date, option id
  - missing `serial` → field `attributes.serial` "is required"
  - `ram_gb: "16"` → "must be a number…"; `ram_gb: 1234567890123456.5` → same; `ram_gb: 0.1234567` → same
  - `warranty_end: "30/06/2027"` → date error; `has_dock: "yes"` → boolean error
  - `os: <Legacy id>` and `os: "not-a-uuid"` → option error
  - `old: "x"` and `nope: 1` → "unknown attribute"
  - several errors at once → one error, fields sorted by key
  - `serial: nil` with required → "is required"
- [ ] **Step 2:** run → FAIL (undefined). **Step 3:** implement. **Step 4:** run → PASS. **Step 5:** commit `feat(inventory): domain types, value validation, errors`.

### Task 3: Repository

**Files:**
- Delete placeholder: `repository/category_repository.go`
- Create: `repository/queries/{asset_types,statuses,assets}.sql`, `repository/{type_repository,status_repository,asset_repository,map}.go`
- Test: `repository/{helpers,type_repository,status_repository,asset_repository}_db_test.go`

**Interfaces:** consumes Task 2 interfaces; produces `NewTypeRepository(pool, outbox)`, `NewStatusRepository(pool, outbox)`, `NewAssetRepository(pool, outbox)`.

Key behaviour:
- Map pg errors: 23505 by constraint name → `ErrTagTaken` (`assets_tag_key`), `ErrTypeCodeTaken`, `ErrTypeNameTaken` (`asset_types_name_lower`), `ErrAttributeKeyTaken`, `ErrAttributeLabelTaken`, `ErrOptionLabelTaken`, `ErrStatusNameTaken`; 23503 on `asset_attribute_values_attribute_fkey` during attribute update → `ErrAttributeInUse`.
- `UpdateAttribute`: in a tx, lock the attribute; if `DataType` or `Unit` changes and any value exists (`SELECT EXISTS … WHERE attribute_id`) → `ErrAttributeInUse`; changing away from `select` deletes its options first.
- `Create` type with attributes and options in one tx; event `inventory.asset_type_created`.
- `Replace`: lock asset; retired → `ErrAssetRetired`; version mismatch → `ErrAssetChanged`; delete all values; if type changes update `asset_type_id` after deleting values; insert new values; bump version; event `inventory.asset_updated` with field changes and `{attribute changes: key, from, to}`.
- `Retire` / `Restore`: lock, version, set status and `retired_at`/reason; events.
- Numbers: `value_number` as `pgtype.Numeric` in sqlc; convert from canonical string with `Numeric.Scan(string)` and back with `Numeric.Value()` → string.
- `List`: `ILIKE` on tag and name with escaped `q` (copy identity's `likeEscaper`), filters, `ORDER BY` via a fixed `CASE` per sort key (no string-built SQL), count query with the same filters.
- Status `Update` with `MakeDefault`: in one tx clear `is_default` on the kind's current default, then set it. `Archive`: system → `ErrSystemStatus`, current default → `ErrStatusIsDefault`. Status writes emit `inventory.status_created`, `status_updated`, `status_archived`.
- Events use `events.New(type, aggregate, id, payload)` + `outbox.Append` like identity (`contract.AggregateAsset = "asset"`, `AggregateAssetType = "asset_type"`, `AggregateStatus = "asset_status"`).

- [ ] **Step 1: DB tests** (one function each):
  - `TestTypes_CreateWithAttributesAndOptions` (event written, `Get` returns ordered attributes with options)
  - `TestTypes_DuplicateCodeNameKeyLabel` (each maps to its domain error; label reusable after `RemoveAttribute`; key not reusable)
  - `TestTypes_ChangeDataType` (no values: number→text ok; select with options but no values → text ok and options gone; with a value → `ErrAttributeInUse`; unit change with values → `ErrAttributeInUse`)
  - `TestStatuses_DefaultMoves` (make another `available` status default → old one loses the flag; archiving a non-system status that is the default → `ErrStatusIsDefault`; events `inventory.status_created/updated/archived` written)
  - `TestAssets_CreateGetValues` (all five data types round-trip; numeric `"15.6"` exact)
  - `TestAssets_TagUniqueAnyCase` (concurrent create `LAP-1` twice → one `ErrTagTaken`)
  - `TestAssets_ReplaceChangesTypeAndValues` (type change drops old values; event lists changes)
  - `TestAssets_StaleVersionAndRetired` (`ErrAssetChanged`; replace after retire → `ErrAssetRetired`; restore works)
  - `TestAssets_ListFiltersSortSearch` (q matches tag and name case-insensitively and escapes `%`; filters by type/status/status kind/include_retired; sort `-tag`)
- [ ] **Step 2:** run → FAIL. **Step 3:** queries + sqlc + implementation. **Step 4:** `CI=true go test -count=1 ./internal/inventory/repository/` → PASS. **Step 5:** commit `feat(inventory): repositories`.

### Task 4: Service

**Files:**
- Delete placeholders: `service/categories.go`
- Create/replace: `service/service.go`, `service/asset_types.go`, `service/statuses.go`, `service/assets.go`
- Test: `service/{fakes_test,service_test}.go`

**Produces:**

```go
type Deps struct { Types domain.TypeRepository; Statuses domain.StatusRepository; Assets domain.AssetRepository }
func New(d Deps) *Service
// asset types (Require PermAssetRead for reads, PermTypeManage for writes)
func (s *Service) ListAssetTypes(ctx, includeArchived bool) ([]domain.AssetType, error)
func (s *Service) GetAssetType(ctx, id uuid.UUID) (domain.AssetType, error)
func (s *Service) CreateAssetType(ctx, in domain.NewAssetType) (domain.AssetType, error)
func (s *Service) UpdateAssetType(ctx, id uuid.UUID, name, description *string, version int32) (domain.AssetType, error)
func (s *Service) ArchiveAssetType(ctx, id uuid.UUID) (domain.AssetType, error) // system → ErrSystemType
func (s *Service) RestoreAssetType(ctx, id uuid.UUID) (domain.AssetType, error)
func (s *Service) AddAttribute(ctx, typeID uuid.UUID, in domain.NewAttribute) (domain.Attribute, error)
func (s *Service) UpdateAttribute(ctx, typeID, attrID uuid.UUID, ch domain.AttributeChange) (domain.Attribute, error)
func (s *Service) RemoveAttribute(ctx, typeID, attrID uuid.UUID) error
func (s *Service) AddOption(ctx, typeID, attrID uuid.UUID, label string, position int32) (domain.Option, error)
func (s *Service) UpdateOption(ctx, typeID, attrID, optID uuid.UUID, label *string, position *int32) (domain.Option, error)
func (s *Service) RemoveOption(ctx, typeID, attrID, optID uuid.UUID) error
// statuses (PermAssetRead / PermStatusManage)
func (s *Service) ListStatuses(ctx, includeArchived bool) ([]domain.Status, error)
func (s *Service) CreateStatus(ctx, in domain.NewStatus) (domain.Status, error)
func (s *Service) UpdateStatus(ctx, id uuid.UUID, ch domain.StatusChange) (domain.Status, error)
func (s *Service) ArchiveStatus(ctx, id uuid.UUID) (domain.Status, error) // system → ErrSystemStatus
// assets (PermAssetRead / PermAssetManage)
type AssetInput struct {
	Name, Description string; TypeID uuid.UUID; StatusID *uuid.UUID
	LocationID, HolderMemberID *uuid.UUID; PurchaseDate *time.Time; Attributes map[string]any
}
type AssetView struct { domain.Asset; Type domain.AssetType; Status domain.Status }
func (s *Service) ListAssets(ctx, f domain.AssetFilter) ([]domain.AssetListItem, int64, error)
func (s *Service) GetAsset(ctx, id uuid.UUID) (AssetView, error)
func (s *Service) CreateAsset(ctx, tag string, in AssetInput) (AssetView, error)
func (s *Service) UpdateAsset(ctx, id uuid.UUID, in AssetInput, version int32) (AssetView, error)
func (s *Service) RetireAsset(ctx, id uuid.UUID, reason string, version int32) (AssetView, error)
func (s *Service) RestoreAsset(ctx, id uuid.UUID, version int32) (AssetView, error)
```

Rules: `AttributeChange.Unit` of `""` clears the unit (stored NULL); names/labels via `CleanLabel`; type code via `NormalizeTypeCode`; attribute key via `ValidateAttributeKey`; unit via `CheckUnit`; options only on `select` (`ErrNotSelectAttribute`); create/update asset loads the type (`Get`), rejects archived type unless unchanged on update, loads status (default `available` when nil), rejects archived status unless unchanged, rejects `retired` kind (`ErrRetiredStatus`), runs `domain.ValidateValues(type, in.Attributes)`; `RetireAsset` uses `Statuses.Default(retired)`, `RestoreAsset` uses `Default(available)`.

- [ ] **Step 1: Tests with fakes**: `TestPermissionChecks` (every method: 401 without actor, 403 without permission), `TestAssetRules_Archived` (Review Focus 4), `TestAssetRules_RetiredStatus`, `TestCreateAsset_DefaultsAndValidation` (default status, invalid attributes → 422 with fields), `TestUpdateAsset_FullReplacement` (Review Focus 5), `TestAssetTypeRules` (system type archive → 409; option on non-select → 422; invalid code/key/unit/label → 422), `TestStatusRules` (system status archive → 409; invalid kind → 422).
- [ ] **Step 2:** FAIL. **Step 3:** implement. **Step 4:** `go test ./internal/inventory/service/` PASS. **Step 5:** commit `feat(inventory): service`.

### Task 5: OpenAPI and handlers

**Files:**
- Replace: `handler/openapi.yaml` (endpoints in spec section 4), `handler/handler.go`, `handler/assets.go`, `handler/asset_types.go` (rename from `categories.go`), `handler/statuses.go`, create `handler/convert.go`
- Generated: `handler/api/api.gen.go`
- Test: `internal/inventory/http_db_test.go` (module root, `package inventory_test`)

Spec details:
- Schemas: `AssetType`, `AssetTypeDetail` (with `attributes: [Attribute]`), `Attribute` (`id,key,label,data_type,unit,is_required,position,removed,options`), `Option` (`id,label,position,removed`), `Status`, `AssetListItem`, `AssetDetail` (common fields, `type`, `status`, `attributes: [AttributeValue]`), `AttributeValue` (`key,label,data_type,unit,value,option_label,option_removed`), request bodies with `maxLength` on every string (tag 64, name 200, description 2000, code 32, key 32, label 100, unit 16, reason 500), `attributes: {type: object, additionalProperties: true, maxProperties: 100}`.
- `value` in `AttributeValue`: `nullable: true`, no `type` (string, number or boolean).
- `GET /assets` params: `q` (maxLength 200), `type_id`, `status_id`, `status_kind` (enum), `location_id`, `holder_member_id`, `include_retired` (boolean), `sort` (enum of 8), shared `Page`, `PageSize`.
- Handler applies defaults (page 1, size 50, sort `tag`) like identity; `Mount(r, tokens)` with no public operations.

- [ ] **Step 1: HTTP tests** (token issued directly with `jwt.Provider.Issue(uuid.New(), perms)`): create type with attributes (201) → create asset with values (201) → GET detail shows labels/units/option label → PUT with stale version 409 → PUT changing type 200 → retire 200 → PUT after retire 409 → list with `status_kind=retired&include_retired=true` → restore. Plus: 403 for missing permission on each write group, 422 `invalid-attribute-values` field paths, 409 `tag-taken`, 404 unknown asset, `sort` invalid → 400.
- [ ] **Step 2:** FAIL. **Step 3:** spec + `go generate` + handlers. **Step 4:** PASS. **Step 5:** commit `feat(inventory): HTTP API`.

### Task 6: Module, contract, binaries, docs, end to end

**Files:**
- Replace: `internal/inventory/module.go`, `contract/events.go` (event names, aggregates, payloads), `contract/contract.go` (leave AssetsByMember TODO for borrowing; document)
- Modify: `cmd/server/main.go` (construct and mount inventory; add its `APIDoc()` to `web.MountDocs`), `.golangci.yml` only if a rule blocks a needed import (ledger it)
- Create: `backend/docs/inventory.md`; Modify: `CLAUDE.md` (inventory pointer)
- Test: `internal/inventory/e2e_db_test.go`, `module_db_test.go`

**Produces:** `inventory.New(Deps{Pool, Outbox}) (*Module, error)`, `(*Module).Mount(r chi.Router, tokens *jwt.Provider) error`, `(*Module).APIDoc() (web.APIDoc, error)`, `(*Module).Service() *service.Service`.

- [ ] **Step 1: e2e test**: type "Laptop" (code LAPTOP) with `ram_gb` number GB required, `os` select (Windows, macOS), `warranty_end` date → asset `LAP-0001` → read back → add option Linux → change `os` → change type to GENERAL (values dropped) → retire → list default hides it → restore. `TestModule_APIDoc` (paths `/assets`, `/asset-types` present).
- [ ] **Step 2:** FAIL. **Step 3:** implement wiring and docs. **Step 4:** full suite `CI=true go test -count=1 -p 1 ./...` and `make lint` → PASS / 0 issues. **Step 5:** commit `feat(inventory): module wiring, docs, end to end`.

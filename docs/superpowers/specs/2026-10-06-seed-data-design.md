# Demo seed data — design

Date: 2026-10-06. Status: approved in conversation (approach A).

## Goal

One command fills an empty **dev** database with realistic sample data, so the app
and its acceptance criteria can be tried by hand. That includes paging, filtering,
attribute filters, Excel export with more than 8 asset types, Vietnamese names, and
signing in as each role. The seed never runs in production.

Out of scope:
- bulk or performance data (e.g. 60,000 assets);
- production starter defaults;
- fixtures for automated tests.

## How it runs

- `make seed` runs `docker compose run --rm seed`. `seed` is a one-off compose
  service on the dev Go image, with the same settings as `migrate`. Its command is
  `go run ./cmd/seed`, it depends on `migrate` having completed, and it sits under
  `profiles: ["tools"]`, so `make up` never starts it. `compose.prod.yml` has no
  seed service.
- Running it on an already seeded or real database is refused. If any asset type
  other than the system "General" type exists, the command prints
  `already seeded — run "make db-reset CONFIRM=yes" to start over` and exits 0
  without writing anything. A second run is therefore a no-op.
- All writes go through the module services, acting as the system actor. Domain
  validation, attribute typing, outbox events and password hashing all apply.
  Each module call commits on its own; nothing is wrapped in one big transaction.
  A partial failure leaves partial data, which `make db-reset CONFIRM=yes` clears.
- Generated values come from a fixed random seed, so every run on an empty
  database produces the same data.

## Configuration

- `SEED_PASSWORD` is the password for every seeded account. It goes in `.env`,
  with an empty placeholder in `.env.example`, and is passed only to the `seed`
  service. It is required: if it is empty, or fails the identity password rules,
  the command fails before writing anything.
- The command reads the same DB settings as `migrate` and `server`. Its
  `cmd/seed/config.go` lists only the config blocks it uses.

## Identity

- New `identity.Module.SeedAccount(ctx, email, name, password string, roleIDs []uuid.UUID) (uuid.UUID, error)`; it returns the account ID, which export profile owners need.
  It follows the bootstrap path: normalize the email, validate and hash the
  password, then create the account with those roles, with the system actor as the
  event actor. An existing email is not an error, which keeps reruns safe. Its
  package comment states that it is for `cmd/seed` only.
- Accounts are created on top of the bootstrap Administrator from `ADMIN_EMAIL`.
  The domain is `storeit.test` (reserved), and each account has the role shown:

  | Email | Name | Role |
  |---|---|---|
  | `manager@storeit.test` | Trần Thị Mai | Authorized Manager |
  | `officer@storeit.test` | Lê Văn Hùng | Inventory Officer |
  | `employee@storeit.test` | Phạm Thu Hà | Employee |
  | `nguyen.an@storeit.test` | Nguyễn Văn An | Employee |
  | `do.linh@storeit.test` | Đỗ Khánh Linh | Employee |
  | `vo.minh@storeit.test` | Võ Quang Minh | Employee |

## Inventory

**Statuses.** The seeded statuses stay. Two custom statuses are added:
"Đang sửa chữa" (kind `unavailable`) and "Cho mượn" (kind `in_use`).

**Asset types.** Ten types with typed attributes. Units appear in brackets, and the
option lists for select attributes are listed under the table.

| Code | Name | Attributes |
|---|---|---|
| `LAPTOP` | Laptop | `cpu` text (required); `ram_gb` number [GB]; `storage_gb` number [GB]; `touch` boolean; `os` select |
| `MONITOR` | Monitor | `size_in` number [inch] (required); `resolution` select; `curved` boolean |
| `PHONE` | Điện thoại | `imei` text (required); `os` select; `storage_gb` number [GB] |
| `PRINTER` | Máy in | `kind` select; `color` boolean; `ip` text |
| `DESK` | Bàn làm việc | `width_cm` number [cm]; `material` select; `price_vnd` number [VND] |
| `CHAIR` | Ghế | `ergonomic` boolean; `color` text |
| `MOTORBIKE` | Xe máy | `plate` text (required); `brand` select; `registered_on` date |
| `SERVER` | Server | `cpu_cores` number; `ram_gb` number [GB]; `rack` text; `warranty_until` date |
| `PROJECTOR` | Máy chiếu | `lumens` number; `hdmi` boolean |
| `NETWORK` | Thiết bị mạng | `kind` select; `ports` number; `poe` boolean |

Select options:
- **Laptop `os`:** Windows 11, macOS, Ubuntu
- **Monitor `resolution`:** Full HD, 2K, 4K
- **Phone `os`:** Android, iOS
- **Printer `kind`:** Laser, Inkjet
- **Desk `material`:** Gỗ, Kính, Thép
- **Motorbike `brand`:** Honda, Yamaha, VinFast
- **Network `kind`:** Switch, Router, Access point

**Assets.** About 300, spread across all ten types with weights: Laptop ~90,
Monitor ~60, Phone ~40, Chair ~30, Desk ~25, and the rest ~5–15 each.
- Tags are per type, e.g. `LAP-0001`.
- Names are realistic models, e.g. "ThinkPad T14 Gen 3" or "Dell U2723QE",
  sometimes with Vietnamese descriptions.
- Purchase dates spread over the last three years.
- Every required attribute is filled. Optional attributes are filled about 80% of
  the time.
- Statuses are mostly Available. About 25% are In use or "Cho mượn", about 5% are
  "Đang sửa chữa", and about 3% are retired with a reason.

## Export profiles

Three profiles, created through the profile service acting as each owner:

1. "Kiểm kê quý" — owned by manager, shared. One sheet per type, each type's
   attributes, title row, summary sheet.
2. "Laptop chi tiết" — owned by officer, private. Laptop columns with renamed
   headers, sorted by `-attributes.ram_gb`.
3. "Danh sách đơn giản" — owned by employee, private. The default report columns
   with stripes.

## Code layout

| File | Responsibility |
|---|---|
| `backend/cmd/seed/main.go`, `config.go` | Wire config, DB, outbox and the identity and inventory modules; run the seed; log a summary |
| `backend/internal/seed/` (new package) | Seed data (the tables above as Go values) and `Run(ctx, Deps)`. Uses only module APIs, never another module's internals |
| `backend/internal/identity/module.go` (+ service) | `SeedAccount` |
| `backend/internal/inventory/module.go` | Exposes the service, or the narrow methods the seed needs, if not already reachable |
| `compose.yml`, `Makefile`, `.env.example` | `seed` service, `make seed`, `SEED_PASSWORD` |
| `README.md`, `backend/docs/platform.md` | How to seed, and the accounts list |

## Testing

- A DB test runs `seed.Run` against a test container:
  - counts: 10 types, about 300 assets, 6 accounts, 3 profiles;
  - a second `Run` writes nothing;
  - a seeded account can sign in with `SEED_PASSWORD`.
- Unit tests check that the data tables are valid: every attribute key passes the
  key pattern, every select attribute has options, and the generated required
  values match their data types.
- `make check` and `CI=true go test -p 1 ./...` pass.

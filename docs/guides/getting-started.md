---
title: Getting started
---

Asset management app. Monorepo: the root owns the Docker stack, each app has its own
folder and Dockerfile.

| Path | Holds |
|---|---|
| `backend/` | Go API, worker and migrations (`backend/docs/` for module references) |
| `frontend/` | Vue SPA (`frontend/README.md`) |
| `compose.yml` | Dev stack: postgres, migrate, app, worker, web, mailpit, pgadmin (profile `tools`) |
| `compose.prod.yml` | Single-server production stack |
| `deploy/` | Postgres initdb, pgAdmin config, `scripts/init.sh`; generated secrets and certs (git-ignored) |
| `Makefile` | Stack, database, codegen and checks for both apps |
| `docs/` | Design specs and plans |

## Development

Needs Docker, and for running checks on the host Go and Node 24.

```sh
make init     # once: secrets, TLS certs, .env, backend/.env.local
make dev      # build + start everything, follow logs (Ctrl-C leaves it running)
```

| URL | |
|---|---|
| http://localhost:3000 | Frontend (Vite, hot reload) |
| http://localhost:8080/api/docs | API, Swagger UI |
| http://localhost:8025 | Mailpit (invitation and reset emails) |

Set `ADMIN_EMAIL` and `ADMIN_PASSWORD` in `.env` before the first start to get an
Administrator account. Ports are in `.env` (`WEB_PORT`, `APP_PORT`, `PG_PORT`, …).

Common targets: `make up`, `make down`, `make logs-app`, `make logs-web`, `make psql`,
`make pgadmin`, `make db-reset CONFIRM=yes`, `make generate` (backend code + frontend API
types), `make check` (everything CI runs). Go-only targets: `make -C backend help`.

### Demo data

`make seed` fills an empty dev database with demo data. It creates:

- 10 asset types with typed attributes;
- two extra statuses;
- about 300 assets;
- three export profiles;
- one sign-in account per role, plus extra Employees.

Set `SEED_PASSWORD` in `.env` first (12-72 bytes); every demo account uses it.
`make seed` only runs on a database without custom asset types, so running it twice does nothing.
To start over, run `make db-reset CONFIRM=yes` and then `make seed`.

| Email | Name | Role |
|---|---|---|
| `manager@storeit.test` | Trần Thị Mai | Authorized Manager |
| `officer@storeit.test` | Lê Văn Hùng | Inventory Officer |
| `employee@storeit.test` | Phạm Thu Hà | Employee |
| `nguyen.an@storeit.test` | Nguyễn Văn An | Employee |
| `do.linh@storeit.test` | Đỗ Khánh Linh | Employee |
| `vo.minh@storeit.test` | Võ Quang Minh | Employee |

The Administrator is the `ADMIN_EMAIL` account.

## Production

StoreIt runs at https://storeit.bigphu.io.vn on one Ubuntu server: the repository is cloned to `/opt/storeit`,
the images are built there from a release tag, and host nginx terminates HTTPS in front of the `web` container.

```sh
git fetch --tags && git checkout v1.1.1               # the release tag
make images TAG=v1.1.1                                # storeit/storeit:v1.1.1 and storeit/storeit-web:v1.1.1
# first time only: sh deploy/scripts/init.sh, then cp .env.prod.example .env and fill it in
docker compose -f compose.prod.yml up -d              # migrate runs before app and worker
```

Every later release follows [Shipping features and migrations](/guides/deploying/).

# storeit

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

```sh
make images TAG=1.0.0 REGISTRY=registry.example.com   # then docker push both
cp .env.prod.example .env                             # set STOREIT_IMAGE, STOREIT_WEB_IMAGE, APP_URL, MAIL_FROM
sh deploy/scripts/init.sh                             # new secrets and certs for the server
docker compose -f compose.prod.yml up -d
```

`web` (nginx) is the only published service, on `127.0.0.1:${WEB_PORT}`: it serves the
SPA and proxies `/api` to the app. Put a reverse proxy with HTTPS in front of it.

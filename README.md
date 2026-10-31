# StoreIt

Asset management for a company's equipment: what you own, who has it, and when it comes back.

**Live:** https://storeit.bigphu.io.vn (reachable from Vietnam only; the hosting provider blocks traffic
from abroad). Releases and their notes: [GitHub releases](https://github.com/bigphu/storeit/releases).

## What it does

- **Inventory**: asset types with their own typed fields, statuses, and assets with tags generated per type
  (`LAP-0091`). Search and filter by any field, bulk actions, retire and restore, replacement records, and a
  full activity history per asset.
- **Borrowing**: employees request an asset type with the specs they need; the inventory team reserves a
  matching asset, hands it over and records the return. Overdue and status emails go out automatically.
- **Excel export**: plain data exports and formatted reports, with saved report layouts.
- **People**: invitations, roles and permissions, sign-in with rotating refresh tokens.

## Repository

Monorepo: the root owns the Docker stack, each app has its own folder and Dockerfile.

| Path | Holds |
|---|---|
| `backend/` | Go API, worker and migrations. Module references in `backend/docs/` |
| `frontend/` | Vue SPA (`frontend/README.md`) |
| `docs/guides/` | User and maintainer guides (start at `docs/index.mdx`) |
| `docs/StoreIT-Sprint3-*.md` | Sprint 3 requirements and planning (Vietnamese) |
| `compose.yml` | Dev stack: postgres, migrate, app, worker, web, mailpit, pgadmin (profile `tools`) |
| `compose.prod.yml` | Single-server production stack |
| `deploy/` | Postgres initdb, pgAdmin config, host nginx site, `scripts/init.sh`; generated secrets and certs (git-ignored) |
| `Makefile` | Stack, database, codegen and checks for both apps |

## Guides

| Guide | For |
|---|---|
| [Getting started](docs/guides/getting-started.md) | Run the stack, demo data and accounts |
| [Architecture](docs/guides/architecture.md) | Services, backend layers, how a request flows |
| [Inventory model](docs/guides/inventory-model.md) | Types, attributes, statuses, tags, replacement, history, search |
| [Borrowing](docs/guides/borrowing.md) | Requests, reservations, loans, returns and emails |
| [Exporting assets](docs/guides/exporting-assets.md) | Data exports, reports, export profiles |
| [Sign-in and sessions](docs/guides/authentication.md), [Roles and permissions](docs/guides/roles-and-permissions.md) | Identity |
| [Configuration and operations](docs/guides/operations.md) | Environment, `make` targets, checks |
| [Shipping features and migrations](docs/guides/deploying.md) | Branch → PR → release tag → server |
| [Adding a backend module](docs/guides/writing-a-module.md) | Checklist for a new module |

## Development

Needs Docker, and for running checks on the host Go and Node 24.

```sh
make init     # once: secrets, TLS certs, .env, backend/.env.local
make dev      # build + start everything, follow logs (Ctrl-C leaves it running)
make seed     # optional: demo data (set SEED_PASSWORD in .env first)
```

| URL | |
|---|---|
| http://localhost:3000 | Frontend (Vite, hot reload) |
| http://localhost:8080/api/docs | API, Swagger UI |
| http://localhost:8025 | Mailpit (emails in development) |

Set `ADMIN_EMAIL` and `ADMIN_PASSWORD` in `.env` before the first start to get an Administrator account. Demo
accounts and more targets are in [Getting started](docs/guides/getting-started.md) and
[Configuration and operations](docs/guides/operations.md).

## Releases

`main` is protected (linear history, pull request, required checks). A release is a `vX.Y.Z` tag on `main`;
the server checks out the tag, builds both images and starts them, and the `migrate` service applies new
migrations first. The full path is in [Shipping features and migrations](docs/guides/deploying.md).

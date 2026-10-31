---
title: Shipping features and migrations
description: From a feature branch to a running release, with database migrations done safely.
---

This is the path every change takes: build it on a branch, land it on `main`, tag a release, and roll it out
on the server. The database is in production since v1.0.0, so migrations get extra care.

## 1. Build on a branch

```sh
git switch main && git pull
git switch -c feat/<short-name>
```

Before you push, run what CI runs:

| Where | Command |
|---|---|
| `backend/` | `make generate` (after changing SQL queries or `openapi.yaml`), then `make check` |
| `backend/` | `CI=true go test -p 1 ./...` (database tests cannot skip) |
| `frontend/` | `npm run gen:api` (after changing a backend `openapi.yaml`), then `npm run check` |

Commit the generated files (`*.gen.go`, `repository/db/`, `frontend/src/lib/api/*.d.ts`). Never edit them by hand.
Run `npm run gen:api` on the host: the `web` container does not mount `backend/`.

## 2. Write a migration (when the schema changes)

Migrations live in `backend/migrations/` and are embedded into the binary, so a new file is picked up
automatically.

**Rules**

- **Never edit a migration that is already on `main`.** Production has applied it; goose will not run it
  again. Fix mistakes with a new migration.
- Name it with the next number: `00011_<what_it_does>.sql`. Check the last number with `ls backend/migrations`.
- Start with a comment explaining why, in Vietnamese like the rest of the code, then `-- +goose Up` and
  `-- +goose Down`. Wrap functions and `DO` blocks in `-- +goose StatementBegin` / `StatementEnd`.
- Write it so it works on a database that already has data:
  - add the column as nullable, **backfill** it with `UPDATE`, then `SET NOT NULL` and add constraints;
  - a new `UNIQUE` constraint fails if the backfill produced duplicates, so make the backfill deduplicate;
  - avoid rewriting large tables in one statement during busy hours.
- A new module's migration must also grant its permissions to Administrator.
- Keep `-- +goose Down` honest (drop what Up added). It is for development; in production we roll forward or
  restore a backup (see section 6).

**Example: `00010_asset_tag_prefix.sql` (v1.1.0)**

1. `ADD COLUMN tag_prefix text` (nullable).
2. Backfill from the prefix each type's existing tags use (`LAP-0001` → `LAP`).
3. Types without such tags get the first 3 letters of their code.
4. Duplicates get a number (`LAP2`).
5. `SET NOT NULL`, then `CHECK` and `UNIQUE`.

**Try it locally**

```sh
docker compose run --rm migrate      # applies new migrations to the dev database
make psql                            # look at the result
```

If the dev API still behaves like the old code afterwards, its hot reload is stuck; restart it with
`docker compose restart app`.

Add or update the database tests (`repository/*_db_test.go`, `schema_db_test.go`). Tests that insert rows with raw
SQL must include the new `NOT NULL` columns.

## 3. Land it on `main`

`main` requires linear history, a pull request and two green checks ("CI / Build and Test" and
"Security / SAST Scanner"). Rebase instead of merging, so the branch stays linear:

```sh
git fetch origin && git rebase origin/main
git push -u origin feat/<short-name>
gh pr create --base main --fill
gh pr checks --watch
```

When the checks are green, fast-forward `main` (the repository owner can bypass the approval rule):

```sh
git push origin feat/<short-name>:main
git switch main && git pull
git branch -d feat/<short-name> && git push origin --delete feat/<short-name>
```

In the PR description, list every migration under a **Deploy note** heading.

## 4. Tag the release

Versions follow `vMAJOR.MINOR.PATCH`: a patch for fixes only, a minor for new features or migrations.

```sh
git switch main && git pull
git tag -a v1.1.0 -m "v1.1.0: auto-generated asset tags, UI refresh"
git push origin v1.1.0
```

## 5. Roll out on the server

Back up the database first, every time a release contains a migration:

```sh
docker compose -f compose.prod.yml exec -T postgres \
  pg_dump -U "$(cat deploy/postgres/secrets/pg_user.txt)" -Fc storeit > backup-$(date +%F)-pre-v1.1.0.dump
```

Build and push the images (from the tagged commit):

```sh
make images TAG=1.1.0 REGISTRY=registry.example.com
docker push registry.example.com/storeit:1.1.0
docker push registry.example.com/storeit-web:1.1.0
```

On the server, set the new tags in `.env` (`STOREIT_IMAGE`, `STOREIT_WEB_IMAGE`), then:

```sh
docker compose -f compose.prod.yml pull
docker compose -f compose.prod.yml up -d
```

`up` runs the `migrate` service first. It applies the goose migrations and then River's own, and `app` and `worker`
only start if it succeeds. Check it:

```sh
docker compose -f compose.prod.yml logs migrate     # "applied migration … 00010_asset_tag_prefix.sql", "migrations up to date"
docker compose -f compose.prod.yml ps               # app, worker, web running
```

Then sign in and try the new feature once. For v1.1.0, create an asset and check that it gets the next tag
(for example `LAP-0091`).

## 6. When something goes wrong

| Problem | Do this |
|---|---|
| `migrate` failed | `app` and `worker` did not start, and the old containers keep running until replaced. Read `logs migrate`. Fix it with a **new** migration in a patch release. Goose runs each migration in a transaction, so a failed one leaves nothing half-applied |
| New code is broken, schema is fine | Set the previous image tags in `.env` and `up -d` again. Most migrations only add things, so the old code still runs on the new schema |
| Data is damaged | Stop `app` and `worker`, restore the backup (`pg_restore --clean -d storeit …`), and deploy the previous images |

Do not run `goose down` in production. Down migrations drop columns and the data in them.

## Checklist

- [ ] Branch from fresh `main`; `make check`, `CI=true go test -p 1 ./...`, `npm run check` pass
- [ ] Generated code regenerated and committed
- [ ] New migration file only (none edited), backfills existing data, has tests
- [ ] PR with a deploy note; checks green; `main` fast-forwarded
- [ ] Tag pushed; images built and pushed with the same version
- [ ] Backup taken; `.env` image tags updated; `up -d`; `logs migrate` clean; feature tried once

---
title: Adding a backend module
description: A checklist for a new module in the modular monolith.
---

Use the identity and inventory modules as the pattern. Read the platform reference
(`backend/docs/platform.md`) first; it lists the rules that are easy to get wrong.

## Checklist

1. **Layout**: `backend/internal/<module>/` with `domain`, `service`, `repository`, `handler`, `contract`, and a root
   package exposing `New(Deps)` and a `Module` with `Mount`.
2. **Migration**: a new goose file in `backend/migrations/` creating the module's schema. Insert its permission codes
   into `identity.permissions` and grant them to roles, **including Administrator**.
3. **Queries**: write SQL under `repository/queries/*.sql` and run `make generate`.
4. **API**: write `handler/openapi.yaml`, generate the strict server, and mount it with
   `{web.ValidateRequests(spec, base), <ForOperations overrides>, authMW}`. The last entry runs first. Wrap
   operation lists in `web.MustOperations`.
5. **Services**: start every public method with `auth.Require(ctx, "<module>.<thing>.<action>")`.
6. **Transactions**: use `database.WithTx` in repositories. Append outbox events and enqueue jobs in the same
   transaction. Services never see a `pgx.Tx`.
7. **Config**: the module owns its environment names and a `Validate()`. Each binary lists only the blocks it uses in
   `cmd/<bin>/config.go`.
8. **Wiring**: add the module in `cmd/server` (and `cmd/worker` if it has jobs or event subscribers).
9. **Defaults**: the request validator does not apply schema defaults, so apply them in Go.
10. **Tests**: HTTP and end-to-end tests at the module root (`*_db_test.go`, package `<module>_test`).
11. **Docs**: add `backend/docs/<module>.md`, and update `platform.md` if a platform API changed.

## Talking to other modules

- Read accounts only through `identity/contract` (`AccountReader`).
- Run jobs as the requesting account with `jobs.RestoreActor(ctx, id, module.LoadActor)`. A deleted account cancels
  the job; a disabled one runs with no permissions.

## Conventions

- Code comments and package docs are in Vietnamese.
- Files use LF line endings.

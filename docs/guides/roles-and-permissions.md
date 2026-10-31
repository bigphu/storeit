---
title: Roles and permissions
description: The four seeded roles and the permission codes they hold.
---

Permissions are codes of the form `module.thing.action`. A role is a set of permissions, an account holds roles,
and the access token carries the union. Services check them with `auth.Require`.

## Seeded roles

| Permission | Administrator | Authorized Manager | Inventory Officer | Employee |
|---|:-:|:-:|:-:|:-:|
| `identity.account.read` | ✓ | ✓ | | |
| `identity.account.manage` | ✓ | | | |
| `identity.role.read` | ✓ | ✓ | | |
| `identity.role.manage` | ✓ | | | |
| `inventory.asset.read` | ✓ | ✓ | ✓ | ✓ |
| `inventory.asset.manage` | ✓ | | ✓ | |
| `inventory.type.manage` | ✓ | ✓ | | |
| `inventory.status.manage` | ✓ | ✓ | | |
| `inventory.asset.export` | ✓ | ✓ | ✓ | ✓ |
| `inventory.export_profile.manage` | ✓ | ✓ | | |

Holders of `identity.role.manage` can create, edit, delete and restore roles.

## Rules that surprise people

- **No granting beyond your own permissions** (403 `/errors/exceeds-own-permissions`). Roles added to or removed
  from an account, permissions changed on a role, and the permissions of an account you disable or enable must all
  be permissions you hold.
- **A module's migration must grant its new permissions to Administrator.** Nobody can hand out a permission they
  lack, so a code the Administrator role does not hold could never be assigned.
- Role and permission changes reach users when their access token is next refreshed (within 15 minutes).
- Disabling an account revokes all its sessions.

## Endpoints by permission (identity)

| Needs | Endpoints |
|---|---|
| Nothing (public) | login, refresh, logout, forgot password, set password |
| Signed in | `PUT /auth/password`, `GET /me`, `PATCH /me` |
| `identity.account.read` | `GET /accounts`, `GET /accounts/{id}` |
| `identity.account.manage` | create, update, disable, enable, sign out, send invitation, send password reset, set roles |
| `identity.role.read` | `GET /roles`, `GET /roles/{id}`, `GET /permissions` |
| `identity.role.manage` | create, update, delete and restore roles; set a role's permissions |

For the inventory endpoints see [Inventory model](/guides/inventory-model/) and [Exporting assets](/guides/exporting-assets/).

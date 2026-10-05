# Frontend prototype (identity + inventory): design

Status: approved in conversation 2026-10-02, implemented on `feat/frontend-prototype`.
Reference for page structure: `C:\d-drive\assignments\asset-management\frontend` (React);
its export-profiles feature has no storeit backend and is left out.

## Goal

A Vue frontend in `frontend/` that covers every identity and inventory endpoint, kept as
the base to build on. Unstyled beyond PrimeVue's Aura theme; the user restyles later.

## Stack

Vite, Vue 3, TypeScript 6, Vue Router 5, Pinia 4, TanStack Vue Query 5, PrimeVue 4.5 (Aura
preset from `@primeuix/themes` 2, `primeicons` 7; all MIT. PrimeVue 5 and primeicons 8
moved to a commercial license that needs a key, so the project stays on the 4.x line), `openapi-typescript` + `openapi-fetch`, Vitest.
`openapi-typescript` declares a TypeScript 5 peer; `package.json` overrides it to the
project's TypeScript (it only generates types).

Layout: `src/app` (layouts, router, providers), `src/lib` (API clients, session, errors,
query helpers), `src/components` (shared), `src/features/<feature>/{api,components,pages}`
with features `auth`, `accounts`, `roles`, `assets`, `asset-types`, `statuses`.

## Dev setup

`npm run dev` serves on :3000 (the port emailed links use, `IDENTITY_APP_URL`) and
proxies `/api` to `http://localhost:8080`. Same origin, so the `SameSite=Strict` refresh
cookie on `/api/v1/auth` works and the backend needs no CORS. Production serving is out
of scope.

## Routes and access

| Path | Page | Needs |
|---|---|---|
| `/login`, `/forgot-password` | public | |
| `/accept-invite#token=`, `/reset-password#token=` | set password (one page) | public |
| `/` | redirect to `/assets` | signed in |
| `/assets`, `/assets/:id` | list, detail (retire/restore) | `inventory.asset.read` |
| `/assets/new`, `/assets/:id/edit` | form | `inventory.asset.manage` |
| `/asset-types`, `/asset-types/:id` | list, detail (attributes, options) | read; `inventory.type.manage` to edit |
| `/statuses` | statuses | read; `inventory.status.manage` to edit |
| `/accounts`, `/accounts/:id` | list, detail | `identity.account.read`; manage to edit |
| `/roles`, `/roles/:id` | list, detail (permissions) | `identity.role.read`; manage to edit |
| `/account/password` | change own password | signed in |

Public layout (centered card) and app layout (sidebar filtered by permission, top bar
with name, change password, sign out). A route needing a missing permission renders "No
access". Manage actions are hidden without the permission; the backend still enforces.
Sign-in returns to `?redirect=`.

## Identity screens

- Sign in (429 shows the wait from `Retry-After`), forgot password (always the same
  message), set password (reads `#token=`, strips it from the address bar at once; 422
  `invalid-password-token` → "invalid or expired" + link to forgot), change password
  (notes that other sessions end).
- Accounts: table (name, email, status tag, created), `q`, active filter, server paging,
  state in the URL; create dialog (email, name, roles) → "Invitation sent". Detail: edit
  name with `version` (409 → reload + message), roles multi-select (`PUT …/roles`),
  resend invitation (invited), send reset link (active), disable/enable, all confirmed;
  lockout / exceeds-own-permissions errors shown. `member_id` not shown (no directory yet).
- Roles: list (name, description, system, permission count), create dialog; detail edits
  name/description (read-only for system roles), permission checklist grouped by module
  prefix from `GET /permissions`, delete with confirm.

## Inventory screens

- Asset list: `q`, type, status, status kind, include retired; columns tag (link), name,
  type, status (tag by kind), purchase date, updated, plus one column per active attribute
  when a type is chosen (unit in header). Header click sorts on the server (`tag`, `name`,
  `purchase_date`, `updated_at`, `asset_type`, `status`, `attributes.<key>`), server
  paging. Filter rows (attribute, operator by data type, typed value) when a type is
  chosen, sent as `attr=<key>:<op>:<value>`, max 10. All state in the URL. Changing type
  clears attribute filters and an attribute sort.
- Asset form (page): tag on create only; name, description, purchase date; status
  optional on create, retired kinds not offered; type chosen on create, change on edit
  warns values are dropped. Attribute inputs by data type: text textarea, number with unit
  suffix (≤ 6 decimals), date (`YYYY-MM-DD`), boolean "— / Yes / No", select of active
  options (a removed current option shows "(removed)"). Field errors under inputs; 409
  `asset-changed` reloads. Edit sends back `location_id` and `holder_member_id` unchanged
  (PUT replaces everything).
- Asset detail: fields, attributes with units, edit, retire (reason), restore.
- Asset types: list (show archived), create (code, name, description); detail edits
  name/description (`version`), archive/restore, attributes table with add/edit/remove,
  options of select attributes (add, rename, position, remove).
- Statuses: table (show archived), create (name, kind, position), edit (name, position,
  make default), archive.

## Session, client, errors

- Access token in memory (Pinia), never in storage. Startup: `POST /auth/refresh` then
  `GET /me`; failure means signed out.
- One refresh at a time: one shared promise per tab, `navigator.locks` across tabs.
  Middleware adds the bearer token; on 401 refreshes once and retries once; a failed
  refresh clears the session and goes to `/login?redirect=`. Successful refresh reloads
  `/me`.
- Sign out: `POST /auth/logout`, clear store and query cache, `BroadcastChannel` tells
  other tabs.
- `npm run gen:api` generates `src/lib/api/{identity,inventory}.d.ts` (committed). Two
  `openapi-fetch` clients with base `/api/v1` share the middleware. Features wrap calls in
  Vue Query composables; mutations invalidate related queries.
- Every non-2xx becomes `ApiError` (problem fields). `useFormErrors` maps `errors[].field`
  to inputs; other errors show above the form. Default mutation error → Toast (403 "no
  permission", 429 with wait). Dates stay `YYYY-MM-DD` strings, never `toISOString()`.

## Checks

`npm run check`: `vue-tsc --noEmit`, Vitest, build. Unit tests: single-flight refresh and
retry, problem → field errors, list URL ↔ API query mapping, attribute value conversion.
No e2e; pages checked by hand against the running backend.

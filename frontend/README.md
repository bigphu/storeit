# storeit frontend

Vue prototype covering the identity and inventory modules. Design:
`../docs/superpowers/specs/2026-10-02-frontend-prototype-design.md`. Styling is PrimeVue's
Aura theme plus minimal layout CSS (`src/app/base.css`); restyle freely.

## Run

The usual way is the whole stack from the repo root: `make up` starts the `web` container
(Vite with hot reload, http://localhost:3000) next to the backend. Its `node_modules` lives
in a Docker volume and is reinstalled when `package-lock.json` changes
(`docker/dev.sh`). Run `npm install` on the host as well for your editor and `npm run
check`.

Vite on the host instead (stop the `web` container first, both use port 3000):

```sh
npm install
npm run dev        # http://localhost:3000, /api proxied to http://localhost:8080
```

Port 3000 matters: emailed invitation and reset links point to `IDENTITY_APP_URL`
(default `http://localhost:${WEB_PORT}`), and serving the app from the same origin as
`/api` is what lets the `SameSite=Strict` refresh cookie work without CORS. Set `API_URL`
to proxy elsewhere.

## Docker

`Dockerfile` targets: `dev` (Vite, used by `compose.yml`) and `prod` (built SPA on
unprivileged nginx, port 8080, `docker/nginx.conf`). In production nginx serves the SPA
and proxies `/api/` and `/healthz` to the `app` container, so the browser sees one origin;
TLS is the job of the reverse proxy in front. The build puts bundles under `/static/`
(`build.assetsDir`), because `/assets/...` are SPA routes that must fall back to
`index.html`.

## Scripts

| Script | Does |
|---|---|
| `npm run gen:api` | Regenerate `src/lib/api/{identity,inventory}.d.ts` from the backend OpenAPI specs. Run after changing a spec; commit the output. |
| `npm run check` | Type-check (`vue-tsc`), unit tests (Vitest), production build |
| `npm test` | Unit tests only |
| `docker build --target prod .` | Production image (the root `make images` builds it with the backend image) |

## Layout

| Path | Holds |
|---|---|
| `src/app` | Router (routes, auth guard, permission meta), layouts, fallback pages, base CSS |
| `src/lib/api` | Generated types, typed `openapi-fetch` clients (`identityApi`, `inventoryApi`), schema aliases |
| `src/lib/auth` | Token handling (`tokens.ts`: bearer, one shared refresh, retry once), session store (`session.ts`: `/me`, sign in/out, `can(perm)`), permission codes |
| `src/lib` | `errors.ts` (`ApiError`, `unwrap`, `describeError`), `forms.ts` (field errors), `query.ts` (Vue Query client, default error toasts), `urlState.ts`, `dates.ts`, `notify.ts` |
| `src/features/<feature>` | `api.ts` (Vue Query hooks), `pages/`, `components/` |

## Rules worth knowing

- The access token lives only in memory; the refresh token is an HttpOnly cookie. On a 401
  the client refreshes once (one refresh at a time across tabs via `navigator.locks`) and
  retries the request once. Sign-out is broadcast to other tabs.
- Call the API through `unwrap(identityApi.GET(...))` so failures become `ApiError`.
  Mutations show a toast on error unless created with `meta: { toast: false }` (forms that
  show field errors themselves via `useFormErrors`).
- Dates without time travel as `YYYY-MM-DD` strings (`lib/dates.ts`); never `toISOString()`.
- `PUT /assets/{id}` replaces everything: the asset form sends back `location_id` and
  `holder_member_id` even though it doesn't show them.
- Asset list state (filters, attribute filters, sort, page) lives in the URL using the API
  parameter names (`features/assets/listQuery.ts`).
- Navigation: the sidebar (`app/layouts/AppSidebar.vue`) has a workspace level (All assets
  expanding to every type with counts from `GET /asset-types?with_counts=true`,
  Configuration, Administration) and a type level for routes under `/types/:typeId`
  and an asset's own pages (Assets, Settings). `Ctrl K` or the switcher opens
  `TypeSwitcher.vue`; choosing a type keeps the section (`app/useTypeNav.ts`).
  Old `/asset-types…` and `/assets?type_id=` links redirect.
- Account settings live at `/account/(profile|password|preferences)`; preferences (theme,
  density, default rows per page, tables with their own size, recent types) are per
  account on the device (`lib/preferences.ts`). The theme toggles `app-dark` on `<html>`,
  PrimeVue's `darkModeSelector`; compact density overrides the DataTable padding tokens.
- Single-key shortcuts of a page go through `usePageKeys` (`lib/pageKeys.ts`): it listens
  only while the page is shown, so cached pages in background tabs don't react. `?` shows
  every shortcut.
- Build UI from PrimeVue v4 components (Breadcrumb, DataTable paginator, ContextMenu,
  Dialog, ConfirmDialog…) before writing custom markup.
- Tabs inside the app (`app/tabs/`): every place opened with Ctrl/⌘-click or middle-click
  (any internal link, or `openLocation(…, e)` from code) becomes a background tab;
  opening a page that already has a tab focuses it (lists may open more than once).
  The URL is always the active tab's location (`useTabs().sync`). Pages are kept alive
  per tab (`KeepAlive` keyed by tab and route), so:
  - read the URL query through `useTabQuery()` (or `useUrlState`), never `useRoute().query`
    directly in a page, or a page in a background tab follows another tab's URL;
  - name the tab with `useTabTitle(() => …)`, mark unsaved forms with `useTabDirty`;
  - go back within a tab with `useTabs().goBack()`, not `router.back()` (browser history
    mixes all tabs).
  Open and pinned tabs are saved per account on the device; the "Reopen my tabs"
  preference decides whether unpinned tabs come back after signing in. `Alt 1–9` jumps.
- Each type's asset list remembers its attribute filters, sort and page (`switchType`,
  kept per browser tab in `features/assets/listContext.ts`); search, status and
  “include retired” carry across types. The same store remembers the last list viewed,
  so the asset page can return to it and step through it (`J`/`K`).
- Rows per page: every paged table passes `rowsPerPageOptions` and keeps its own size
  through `usePageSize(tableKey)` (`lib/preferences.ts`, per account in localStorage);
  tables without a size use the default.
- **Stay on PrimeVue 4.x, `@primeuix/themes` 2.x and `primeicons` 7.x (MIT).** From
  PrimeVue 5 / primeicons 8 (July 2026) PrimeTek ships them under a commercial "PrimeUI"
  license that needs a license key (a free Community key exists for eligible users);
  without one the app shows "Invalid PrimeUI license". The `^` ranges in `package.json`
  keep `npm update` on the MIT majors.
- `openapi-typescript` declares a TypeScript 5 peer; `package.json` overrides it to the
  project's TypeScript 6 (it only generates types).

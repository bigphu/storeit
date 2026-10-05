# storeit frontend

Vue prototype covering the identity and inventory modules. Design:
`../docs/superpowers/specs/2026-10-02-frontend-prototype-design.md`. Styling is PrimeVue's
Aura theme plus minimal layout CSS (`src/app/base.css`); restyle freely.

## Run

```sh
npm install
npm run dev        # http://localhost:3000, /api proxied to http://localhost:8080
```

The backend runs from `../backend` (`docker compose up`). Port 3000 matters: emailed
invitation and reset links point to `IDENTITY_APP_URL` (default `http://localhost:3000`),
and serving the app from the same origin as `/api` is what lets the `SameSite=Strict`
refresh cookie work without CORS. Set `API_URL` to proxy elsewhere.

## Scripts

| Script | Does |
|---|---|
| `npm run gen:api` | Regenerate `src/lib/api/{identity,inventory}.d.ts` from the backend OpenAPI specs. Run after changing a spec; commit the output. |
| `npm run check` | Type-check (`vue-tsc`), unit tests (Vitest), production build |
| `npm test` | Unit tests only |

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
- **Stay on PrimeVue 4.x, `@primeuix/themes` 2.x and `primeicons` 7.x (MIT).** From
  PrimeVue 5 / primeicons 8 (July 2026) PrimeTek ships them under a commercial "PrimeUI"
  license that needs a license key (a free Community key exists for eligible users);
  without one the app shows "Invalid PrimeUI license". The `^` ranges in `package.json`
  keep `npm update` on the MIT majors.
- `openapi-typescript` declares a TypeScript 5 peer; `package.json` overrides it to the
  project's TypeScript 6 (it only generates types).

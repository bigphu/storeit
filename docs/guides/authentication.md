---
title: Sign-in and sessions
description: Access and refresh tokens, rotation, invitations and password reset.
---

## Tokens

- **Access token**: a JWT valid for 15 minutes that carries the account's permissions.
- **Refresh token**: 32 random bytes in the `storeit_refresh` cookie (`HttpOnly`, `SameSite=Strict`,
  `Path=/api/v1/auth`). Only its SHA-256 is stored.

A sign-in opens a **family** of refresh tokens. Each refresh marks the presented token as used and issues its
child. The decision is made in one transaction:

| Situation | Result |
|---|---|
| Family revoked, token unknown or expired | 401, nothing written |
| Account disabled | Family revoked, 401 |
| Past the family's absolute expiry | Family revoked, 401 |
| Used token presented again within the 30 s grace period | Treated as a retry (two tabs); rotates the family's current token |
| Used token presented after the grace period | **Reuse detected**: the whole family is revoked, 401 |
| Otherwise | Rotate; the new token expires at the earlier of now + sliding TTL and the absolute expiry |

Every refresh failure is the same 401 and clears the cookie. Refresh reloads permissions, so role changes reach
users within one access-token lifetime. A disabled account's access token works until it expires (at most
15 minutes) but it can no longer refresh.

Sessions end when an administrator disables the account, signs it out, or when a password reset completes.
Changing your own password keeps the current session and revokes the others.

> The frontend must run at most one refresh at a time, shared across tabs. Out-of-order responses can leave a
> tab holding an already-used token, and a later refresh would then be treated as reuse.

## Invitations and password reset

Accounts are created without a password and start as `invited`. Signing in to one looks the same as an unknown email.

- A link is a one-time token (32 random bytes, SHA-256 stored), one per account and purpose: `invite` (72 h) or
  `reset` (1 h). Issuing a new one replaces the old one.
- The request that issues the token also queues an email job in the same transaction. The email links to
  `{IDENTITY_APP_URL}/accept-invite#token=…` or `/reset-password#token=…`. The token is in the URL fragment so it
  never reaches a server log or a `Referer` header.
- The page posts the token to `POST /auth/password/set`. Exactly one concurrent submission wins.
- `POST /auth/password/forgot` always answers 202, so it does not reveal whether an email has an account.
  A 60-second per-account cooldown stops repeated sends.

## Rate limits

Per client IP, in memory (429 with `Retry-After`): login 10 requests then 1 per 6 s, forgot-password 5 then 1 per
minute, set-password 10 then 1 per 6 s. With several API replicas each counts separately.

## Settings

| Variable | Default |
|---|---|
| `IDENTITY_REFRESH_SLIDING_TTL` | 336h |
| `IDENTITY_REFRESH_ABSOLUTE_TTL` | 720h (must be at least the sliding TTL) |
| `IDENTITY_REFRESH_GRACE_PERIOD` | 30s |
| `IDENTITY_REFRESH_RETENTION` | 720h |
| `IDENTITY_COOKIE_SECURE` | true (development sets false) |
| `IDENTITY_APP_URL` | http://localhost:3000 |
| `IDENTITY_INVITE_TTL` / `IDENTITY_RESET_TTL` | 72h / 1h |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | Both or neither. Creates the first Administrator while no accounts exist |

The worker also needs `MAIL_*` settings. Development sends to Mailpit; production uses Resend's SMTP, and the
worker refuses to start while `deploy/app/secrets/smtp_password.txt` is empty.

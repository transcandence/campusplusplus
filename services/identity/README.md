# Identity Service

Authentication and session lifecycle management for the Campus++ platform.

## Architecture

- **Auth Model**: Dual-token system.
  - **Access Token (JWT)**: RS256-signed, 10-minute lifetime. Downstream services verify statelessly with the RSA public key without database round-trips.
  - **Refresh Token**: Opaque 32-byte entropy, 30-day lifetime. Rotated on every refresh. Stored as raw binary SHA-256 (`sessions.token_hash BYTEA`) with family-based reuse detection.
- **Error Standard**: RFC 9457 Problem Details (`application/problem+json`).
- **Contract**: Defined in `contracts/openapi.yaml`. Types & Gin server generated via `oapi-codegen`. Detailed specification lives in `docs/identity-auth-spec.md`.
- **Registration change (2026-11-25)**: email is now required and new accounts are active immediately. See [`docs/identity-registration-active.md`](../../docs/identity-registration-active.md).
- **Logout hardening (2026-11-25)**: request body required, storage errors propagated, `204` only on real/idempotent success. See [`docs/identity-logout-fix.md`](../../docs/identity-logout-fix.md).
- **Error mappings, UpdateMe and migrations (2026-11-25)**: see [`docs/identity-errors-updateme-migrations.md`](../../docs/identity-errors-updateme-migrations.md).
- **Session lifecycle (2026-11-25)**: see [`docs/identity-session-lifecycle.md`](../../docs/identity-session-lifecycle.md).

## Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/auth/register` | Public | Creates an email/password account, active immediately (no email verification yet). |
| `POST` | `/auth/login` | Public | Authenticates via email/password; returns JWT + Refresh Token. |
| `POST` | `/auth/refresh` | Public | Rotates refresh token and issues a new access token. |
| `POST` | `/auth/logout` | Public | Revokes the current session (idempotent, 204). |
| `POST` | `/auth/logout-all` | Bearer | Revokes all active sessions for the user (204). |
| `GET` | `/me` | Bearer | Returns the authenticated user profile and campus affiliation. |
| `GET` | `/sessions` | Bearer | Lists active sessions; marks the current one. |
| `DELETE` | `/sessions/{id}` | Bearer | Revokes one session owned by the current user. |
| `GET` | `/healthz` | Public | Liveness probe (200 OK). |

## Environment Configuration

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | *(required)* | PostgreSQL connection string (`postgres://...`). |
| `JWT_PRIVATE_KEY_PATH` | `private.pem` | RSA private key path for signing JWTs. |
| `JWT_PUBLIC_KEY_PATH` | `public.pem` | RSA public key path for verifying JWTs. |
| `HTTP_ADDR` | `:8080` | Bind address. |
| `ENV` | `dev` | Environment (`dev` or `prod`). |
| `MAX_ACTIVE_SESSIONS` | `10` | Active sessions per user; oldest sessions are revoked first. |
| `SESSION_RETENTION` | `720h` | Retention of expired/revoked sessions before GC. |
| `SESSION_CLEANUP_INTERVAL` | `1h` | Worker cleanup interval. |

## Quickstart

```bash
# 1. Apply database migrations
make migrate-up-identity

# 2. Seed development users (sara.benali@campus.ma)
make seed-identity

# 3. Run the service
cd services/identity/api && go run ./cmd/server

# 4. Verify login
curl -i -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"sara.benali@campus.ma","password":"Sara@benali1234"}'
```

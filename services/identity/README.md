# Identity Service

Authentication and session lifecycle management for the Campus++ platform.

## Architecture

- **Auth Model**: Dual-token system.
  - **Access Token (JWT)**: RS256-signed, 10-minute lifetime. Downstream services verify statelessly with the RSA public key without database round-trips.
  - **Refresh Token**: Opaque 32-byte entropy, 30-day lifetime. Rotated on every refresh. Stored as raw binary SHA-256 (`sessions.token_hash BYTEA`) with family-based reuse detection.
- **Error Standard**: RFC 9457 Problem Details (`application/problem+json`).
- **Contract**: Defined in `contracts/openapi.yaml`. Types & Gin server generated via `oapi-codegen`. Detailed specification lives in `docs/identity-auth-spec.md`.

## Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/auth/login` | Public | Authenticates via email/password; returns JWT + Refresh Token. |
| `POST` | `/auth/refresh` | Public | Rotates refresh token and issues a new access token. |
| `POST` | `/auth/logout` | Public | Revokes the current session (idempotent, 204). |
| `POST` | `/auth/logout-all` | Bearer | Revokes all active sessions for the user (204). |
| `GET` | `/me` | Bearer | Returns the authenticated user profile and campus affiliation. |
| `GET` | `/healthz` | Public | Liveness probe (200 OK). |

## Environment Configuration

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | *(required)* | PostgreSQL connection string (`postgres://...`). |
| `JWT_PRIVATE_KEY_PATH` | `private.pem` | RSA private key path for signing JWTs. |
| `JWT_PUBLIC_KEY_PATH` | `public.pem` | RSA public key path for verifying JWTs. |
| `HTTP_ADDR` | `:8080` | Bind address. |
| `ENV` | `dev` | Environment (`dev` or `prod`). |

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

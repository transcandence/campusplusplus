# Identity Service — Deep Architecture Specification

The Identity Service provides authentication, session lifecycle management, and current-user identity context for the Campus++ platform (housing, transport, restaurant, gate, events, reports).

---

## 1. Architecture Overview

Authentication follows a **dual-token model**:
1. **Access Token (JWT)**:
   - Short-lived (**10 minutes / 600s**).
   - Signed using **RS256** with an RSA private key. Downstream services verify tokens statelessly using the public key without querying the identity database.
   - Carried as `Authorization: Bearer <token>`.
2. **Refresh Token**:
   - Long-lived (**30 days**).
   - Opaque, high-entropy random string returned in the response body.
   - **Never stored in plaintext**: only the **32-byte SHA-256 hash** (`Sum256`) is stored in the database (`sessions.token_hash BYTEA`).
   - Rotated on every `/auth/refresh` call with strict **family-based reuse detection**.

---

## 2. Database Schema (Reconciled with `0002_login_reconcile`)

### 2.1 Enums & Types
- `affiliation`: `'1337', 'ABS', 'FGSES', 'SHBM', 'SAS', 'CC', 'AEM', 'um6p'`
- `user_role`: `'student', 'staff', 'admin', 'guest', 'worker'`
- `gender`: `'male', 'female', 'other'`
- `user_status`:
  - `pending_verification`: Account created, email not yet verified (cannot log in).
  - `active`: Verified and in good standing (can log in).
  - `suspended`: Banned or locked account (cannot log in).

### 2.2 Tables

#### `campuses` (Tenancy Reference Table)
| Column | Type | Constraints / Description |
|---|---|---|
| `id` | `SERIAL PRIMARY KEY` | Internal ID |
| `code` | `VARCHAR(30) UNIQUE NOT NULL` | Lowercase code (`rabat`, `benguerir`, `laayoune`), checked with `^[a-z0-9_-]+$` |
| `name` | `VARCHAR(60) NOT NULL` | Display name (e.g. `Rabat`) |
| `created_at` | `TIMESTAMPTZ NOT NULL` | Default `now()` |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | Default `now()`, managed by trigger |

#### `users`
| Column | Type | Constraints / Description |
|---|---|---|
| `id` | `SERIAL PRIMARY KEY` | User ID (`sub` in JWT) |
| `first_name` | `VARCHAR(30) NOT NULL` | First name |
| `last_name` | `VARCHAR(30) NOT NULL` | Last name |
| `email` | `CITEXT UNIQUE NOT NULL` | Case-insensitive email (`Sara@x` == `sara@x`) |
| `phone_number` | `VARCHAR(20) UNIQUE NOT NULL`| User phone |
| `birthday` | `DATE` | Optional date of birth |
| `affiliation` | `affiliation NOT NULL` | Native enum |
| `role` | `user_role NOT NULL` | Default `'student'::user_role` |
| `gender` | `gender` | Optional enum |
| `password_hash` | `TEXT NOT NULL` | Slow hash (Argon2id or bcrypt) |
| `nationality` | `VARCHAR(60)` | Optional |
| `status` | `user_status NOT NULL` | Default `'pending_verification'` |
| `campus_id` | `INTEGER NOT NULL` | FK $\rightarrow$ `campuses(id) ON DELETE RESTRICT` |
| `created_at` | `TIMESTAMPTZ NOT NULL` | UTC timestamp |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | UTC timestamp, updated via trigger |

#### `sessions` (Lifecycle & Refresh Tokens)
| Column | Type | Constraints / Description |
|---|---|---|
| `id` | `SERIAL PRIMARY KEY` | Session ID (`sid` in JWT) |
| `user_id` | `INTEGER NOT NULL` | FK $\rightarrow$ `users(id) ON DELETE CASCADE`. Multiple sessions per user allowed |
| `token_hash` | `BYTEA UNIQUE NOT NULL` | 32-byte raw binary SHA-256 of the opaque refresh token |
| `ip_address` | `INET NOT NULL` | Client IP from request |
| `user_agent` | `TEXT` | Optional user-agent string |
| `expires_at` | `TIMESTAMPTZ NOT NULL` | Session expiration instant (`expires_at > created_at`) |
| `revoked_at` | `TIMESTAMPTZ NULL` | `NULL` = active. Timestamp set on logout, rotation, or suspension |
| `last_used_at`| `TIMESTAMPTZ NULL` | Updated on `/auth/refresh` (~every 10 min) |
| `family_id` | `UUID NOT NULL` | Groups rotated tokens originating from the same login session |
| `rotated_at` | `TIMESTAMPTZ NULL` | Timestamp when this session was rotated |
| `replaced_by` | `INTEGER NULL` | FK $\rightarrow$ `sessions(id)` pointing to the replacement session |
| `created_at` | `TIMESTAMPTZ NOT NULL` | Session start |
| `updated_at` | `TIMESTAMPTZ NOT NULL` | Managed by trigger |

*Indexes & Triggers:*
- Partial index `idx_sessions_user_id_active` ON `sessions(user_id) WHERE revoked_at IS NULL`
- Partial index `idx_sessions_expires_at_active` ON `sessions(expires_at) WHERE revoked_at IS NULL`
- Index `idx_sessions_family_id` ON `sessions(family_id)`
- Trigger `trg_sessions_updated_at` calls `set_updated_at()` before update.

---

## 3. Token Specifications

### 3.1 JWT Access Token
- **Signing Algorithm**: `RS256` (Asymmetric RSA private key for signing, public key for verification).
- **TTL**: 10 minutes (600 seconds).
- **Header**:
  ```json
  {
    "alg": "RS256",
    "typ": "JWT"
  }
  ```
- **Payload Claims**:
  ```json
  {
    "sub": 42,
    "sid": 105,
    "campus_id": "rabat",
    "role": "student",
    "iss": "campus-identity",
    "aud": ["campus-api"],
    "jti": "550e8400-e29b-41d4-a716-446655440000",
    "iat": 1714820000,
    "exp": 1714820600
  }
  ```
  - `sub` (`int64`): User ID from `users.id`.
  - `sid` (`int64`): Session ID from `sessions.id`.
  - `campus_id` (`string`): Lowercase campus code from `campuses.code` (e.g. `"rabat"`).
  - `role` (`string`): User role (`student`, `staff`, `admin`, `guest`, `worker`).
  - `iss` (`string`): `"campus-identity"`.
  - `aud` (`string[]`): `["campus-api"]`.
  - `jti` (`string`): Unique UUID per token.
  - `iat` / `exp`: Issued-at and expiry unix timestamps (10-minute window).

### 3.2 Opaque Refresh Token
- **Format**: 32 cryptographically secure random bytes (Base64URL encoded).
- **TTL**: 30 days.
- **Database Storage**:
  ```go
  hash := sha256.Sum256([]byte(rawRefreshToken))
  // Store hash[:] directly into sessions.token_hash (BYTEA)
  ```

---

## 4. Deep Dive: Login, Refresh, Logout, Revocation & Families

### 4.1 The Security Problem
- **Access Tokens (JWT)** are stateless and expire quickly (**10 minutes**). If intercepted, the exposure window is brief.
- **Refresh Tokens** live longer (**30 days**). If an attacker steals a refresh token, they could impersonate the user indefinitely.
- To prevent this, the service implements **Refresh Token Rotation with Family-based Reuse Detection**.

### 4.2 Core Concepts
- **`token_hash`**: `SHA-256(raw_refresh_token)` stored as 32 raw bytes (`BYTEA`). Fast $O(1)$ lookup with no risk of token leaks if the database is exposed.
- **`family_id`**: A unique UUID created during initial login. **All tokens created through rotation on that same device share the same `family_id`.**
- **`rotated_at`**: A timestamp indicating that the token was already exchanged for a successor.
- **`revoked_at`**: `NULL` means the session is currently active. Any non-NULL timestamp means the session has been terminated (via logout, rotation, or security revocation).
- **`replaced_by`**: Self-referencing FK (`sessions.id`) tracking token ancestry for debugging and security audits.

---

### 4.3 Flow 1: Login (`POST /auth/login`)

```
User (Device)                      Identity Service                     Database (Postgres)
     |                                    |                                      |
     |--- 1. POST /auth/login ----------->|                                      |
     |    (email, password)               |--- 2. SELECT user by email --------->|
     |                                    |<-- (status, password_hash, etc.) ----|
     |                                    |                                      |
     |                                    | 3. Checks:                           |
     |                                    |    - Password match (Argon2/bcrypt)  |
     |                                    |    - status == 'active'              |
     |                                    |                                      |
     |                                    | 4. Generate Tokens:                  |
     |                                    |    - Family: family_id = UUID()      |
     |                                    |    - RefreshToken: RT_1 (random 32B) |
     |                                    |    - Hash: H(RT_1) = sha256(RT_1)   |
     |                                    |                                      |
     |                                    |--- 5. INSERT into sessions --------->|
     |                                    |       (family_id, token_hash=H(RT_1),|
     |                                    |        revoked_at=NULL)              |
     |                                    |                                      |
     |                                    | 6. Sign Access Token JWT (10 min)    |
     |<-- 200 OK (JWT, RT_1) -------------|                                      |
```

---

### 4.4 Flow 2: Normal Token Refresh (`POST /auth/refresh`)

When the 10-minute JWT expires, the client sends `RT_1` to obtain a new token pair without re-entering credentials.

```
Client                             Identity Service                     Database (Postgres)
 |                                        |                                      |
 |-- 1. POST /auth/refresh (RT_1) ------->|                                      |
 |                                        |-- 2. Hash RT_1 -> H(RT_1)            |
 |                                        |-- 3. SELECT * FROM sessions -------->|
 |                                        |      WHERE token_hash = H(RT_1)      |
 |                                        |<-- Row 1 found (revoked_at is NULL) -|
 |                                        |                                      |
 |                                        | 4. Rotate:                           |
 |                                        |    - Generate RT_2 (new random 32B)  |
 |                                        |    - Retain family_id (FAM_A)        |
 |                                        |                                      |
 |                                        |-- 5. INSERT new session Row 2 ------>|
 |                                        |      (family_id=FAM_A, H(RT_2),      |
 |                                        |       revoked_at=NULL)               |
 |                                        |                                      |
 |                                        |-- 6. UPDATE Row 1 (retire old one) ->|
 |                                        |      revoked_at = now(),             |
 |                                        |      rotated_at = now(),             |
 |                                        |      replaced_by = 2                 |
 |                                        |                                      |
 |                                        | 7. Sign fresh JWT (10 min)           |
 |<-- 200 OK (New JWT, RT_2) -------------|                                      |
```

---

### 4.5 Flow 3: Reuse Attack Detection (Token Theft Defense)

Suppose an attacker intercepts and copies `RT_1` before the legitimate user refreshes.
1. The legitimate user refreshes at `12:10`. The server invalidates `RT_1` and issues `RT_2`.
2. The attacker subsequently attempts to use the stolen `RT_1` at `12:12`.

```
Attacker                           Identity Service                     Database (Postgres)
   |                                      |                                      |
   |-- 1. POST /auth/refresh (RT_1) ----->|                                      |
   |                                      |-- 2. Hash RT_1 -> H(RT_1)            |
   |                                      |-- 3. SELECT * FROM sessions -------->|
   |                                      |      WHERE token_hash = H(RT_1)      |
   |                                      |<-- Row 1: rotated_at IS NOT NULL! ---|
   |                                      |                                      |
   |                                      | 🚨 ALERT: TOKEN REUSE DETECTED!      |
   |                                      |    (An already-rotated token was used)|
   |                                      |                                      |
   |                                      |-- 4. REVOKE ENTIRE FAMILY: --------->|
   |                                      |      UPDATE sessions                 |
   |                                      |         SET revoked_at = now()       |
   |                                      |       WHERE family_id = 'FAM_A'      |
   |                                      |                                      |
   |<-- 401 Unauthorized -----------------|                                      |
        (code: "invalid_refresh")
```

---

### 4.6 Flow 4: Logout Strategies

#### Single-Device Logout (`POST /auth/logout`)
- Client sends `refreshToken` (`RT_2`).
- The service hashes the token and marks `revoked_at = now()`.
- **Only the calling device is logged out.** Other active families remain unaffected.
- **Idempotent**: Returns `204 No Content` even if already revoked.

#### All-Devices Logout (`POST /auth/logout-all`)
- Client sends their valid access token (`Authorization: Bearer <JWT>`).
- The service extracts the user ID (`sub`) from JWT claims and revokes every active session:
  ```sql
  UPDATE sessions
     SET revoked_at = now()
   WHERE user_id = :sub
     AND revoked_at IS NULL;
  ```
- All families across all devices are immediately invalidated.

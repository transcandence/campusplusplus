# Repository Structure & Interaction Guide

> Companion document to `structure.txt`. Explains what every folder/file **should contain**
> and how the parts **interact**, mapped to the domain model (Users, Housing, Transportation).

---

## 1. Root layout (big picture)

```javascript
.
├── services/          # Business microservices (Go APIs + web clients)
│   ├── identity/      # Users, accounts, profiles, roles, JWT issuing
│   ├── housing/       # Buildings, dorm-units, stays, cleaning, maintenance
│   └── transport/     # Buses, golfettes, bookings, rides
├── api-gateway/       # Single entry point: routing, authn, rate-limiting
├── frontend/          # Web app (student / worker / manager UIs)
├── platform/          # Shared libraries — go/ (backend) and ts/ (frontend)
├── infra/             # Docker compose, DB init, observability stacks
├── docs/              # Domain model, architecture decisions (ADRs)
├── tools/             # Dev scripts, generators, local tooling
├── .github/           # CI workflows, PR template, code owners
└── root config files  # Makefile, go.work, pnpm-workspace.yaml, ...
```

**Interaction rule of thumb:** `frontend → api-gateway → services → postgres`;
services talk to each other **only** through `contracts/` (OpenAPI for sync REST, `events/` for async messaging via `platform/go/events` + `messaging`).

---

## 2. Root files

| File | Purpose |
| --- | --- |
| `README.md` | Project intro, quickstart (`make up`), architecture diagram, links to `docs/`. |
| `Makefile` | Developer commands: `make up` (compose), `make migrate`, `make test`, `make lint`, `make gen` (codegen from openapi). |
| `go.work` | Go workspace uniting `services/*/api`, `api-gateway`, `platform/go` so local cross-module edits work without publishing. |
| `pnpm-workspace.yaml` | pnpm workspace uniting `frontend`, `services/*/web`, `platform/ts/*` for a single lockfile and shared TS config. |
| `package.json` (root) | Workspace scripts: lint/format (prettier, eslint), codegen (`orval`/`openapi-typescript` against `contracts/openapi.yaml`), commit hooks. |
| `.golangci.yml` | Shared Go linter config used by every Go module in CI. |
| `.editorconfig` / `.gitattributes` / `.gitignore` / `.dockerignore` | Editor consistency, line endings, ignore rules (`.env`, `node_modules`, `bin/`, coverage). |
| `script.sh` | One-off bootstrap/dev helper (e.g., `go install` tool deps, `migrate` wrapper). Should stay thin; real commands live in the Makefile. |
| `structure.txt` | Machine-generated tree (refresh with `make structure`). |
| `docs/domain-model.md` | The full domain model (Users, Housing, Transportation) — the vocabulary every service shares. |
| `docs/architecture.md` | C4-style diagrams: system context, containers, request flow, event flow. |
| `docs/adr/` | Architecture Decision Records (`.gitkeep` → add `0001-monorepo.md`, `0002-sync-rest-async-events.md`, …). |

---

## 3. `services/` — the three business services

Each service (`identity`, `housing`, `transport`) follows the **identical layout** so engineers learn one pattern:

```javascript
services/<name>/
├── api/                  # Go backend (the service itself)
│   ├── cmd/
│   │   ├── server/main.go    # Entrypoint: config → logging → db → events → http server
│   │   └── migrate/          # Standalone binary: runs SQL migrations up/down
│   ├── internal/
│   │   ├── handler/          # HTTP layer: thin — parse/validate → call service → write response
│   │   ├── service/          # Business logic: rules from domain-model.md (e.g. "at most one active Stay")
│   │   ├── repository/       # Postgres access: SQL queries, row mapping, transactions
│   │   ├── model/            # Domain entities (structs) + DB mapping tags
│   │   ├── config/           # env parsing (with defaults), validated at boot
│   │   └── events/           # Producers/consumers: publish & handle domain events
│   ├── migrations/
│   │   ├── 0001_init.up.sql
│   │   └── 0001_init.down.sql
│   ├── Dockerfile            # Multi-stage: build static binary → distroless/scratch
│   ├── .env.example          # Every env var the service reads, with dummy values
│   ├── go.mod
│   └── README.md             # Service-specific docs: endpoints, events produced/consumed, local run
├── contracts/
│   ├── openapi.yaml          # Source of truth for this service's HTTP API
│   └── events/               # Event schemas (asyncapi / json-schema / go structs)
├── web/                      # (Optional) service-scoped frontend app / micro-frontend
│   ├── src/                  # React components & pages specific to this service
│   └── package.json          # Depends on platform/ts/ui + platform/ts/api-client
└── README.md                 # What this service owns in the domain
```

### What goes in each layer (and what must NOT)

- **handler** — no business rules, no SQL. Only: auth context extraction, request validation (from openapi-generated types), calling `service`, mapping errors to HTTP codes.
- **service** — all invariants, e.g. Housing: *"a room has at most one active Stay"*, *"a visitor never has a Stay"*, *"only operator-level housekeepers execute a cleaning-shift"*. Orchestrates repositories + publishes events **after** the DB transaction commits.
- **repository** — only place that knows SQL. One repo per aggregate (`stay_repository.go`, `dorm_unit_repository.go`).
- **events** — domain events such as `user.created`, `stay.activated`, `maintenance.resolved`, `ride.completed`. Producer writes to outbox table in the same tx as the state change (via `platform/go/events`); consumer handlers call `service` methods.

### Service-by-service contents (mapped to the domain model)

**`services/identity/`** — *User Management*

- Entities: `Account` (email/phone ≥1, password hash, status active/suspended/archived), `Profile` (name, gender), one-of `UserStudent` / `UserWorker` / `UserGuest` (single-table inheritance or joined sub-tables, enforced in service layer — *"a user cannot have two profile types"*).
- Worker roles: `Manager`, `Housekeeper` (level: supervisor/operator), `Maintenance`.
- Guests: stay period (arrival→departure) + host FK to the inviting student/worker; subtype guest-student vs visitor.
- Owns: sign-in/sign-up, JWT issuing, role/level claims. Other services never read the `accounts`/`profiles` tables — they call identity's API (`GET /internal/users/{id}`) or consume `user.*` events.

**`services/housing/`** — *Housing, Cleaning, Maintenance*

- Entities: `Building → Floor → DormUnit` (types: room(cap 1), apart(cap>1), corridor, kitchen, laundry), `Stay` (start, end, status planned/active/ended — housing history), `CleaningShift` (date, time slot, status, exactly one operator housekeeper, planned by supervisor), `CleaningTask` (one per dorm-unit, own status + done-at), `MaintenanceRequest` (description, priority, status open→assigned→in progress→resolved, timestamps, raised-by, handled-by).
- Enforces: room = 1 active stay / apart = ≤ capacity active stays; only housing units get stays; corridor/kitchen/laundry cleaning only from supervisor scheduling; room/apart cleaning requestable by the stay's student.
- Publishes: `stay.activated`, `stay.ended`, `cleaning.shift.done`, `maintenance.resolved`.

**`services/transport/`** — *Transportation*

- Entities: `Bus` (fixed departure/destination, schedule, capacity), `SeatBooking`, `Golfette` (seat-capacity, trunk-capacity, current-location), `Ride` (pick-up, drop-off, vehicle, passengers; window 07:00–22:00 enforced in service layer).
- Validates bookings against capacity (bus seats, golfette seats/trunk) and permitted-user check via identity roles.
- Publishes: `ride.requested`, `ride.completed`, `seat.booked`.

---

## 4. `api-gateway/`

```javascript
api-gateway/
├── cmd/server/main.go          # Boot: config → middleware chain → route table → reverse proxy
├── internal/
│   ├── middleware/             # JWT verification (uses platform/go/auth), request-id, rate-limit, CORS
│   ├── proxy/                  # Reverse proxy to services; path→service routing table, error translation
│   └── config/                 # Routes, upstream URLs, JWT public key, timeouts
├── Dockerfile
├── .env.example
└── go.mod
```

- **Contains**: nothing domain-specific. No dorm-unit, no ride logic — ever.
- **Flow**: `frontend → gateway` at `/api/housing/...`, `/api/transport/...`, `/api/identity/...`; gateway strips/verifies auth, attaches `X-User-Id` + `X-Roles` headers, proxies. Services trust these headers **only** because traffic between gateway and services is internal (mTLS or network policy).
- 404/401/403 are produced here; 4xx/5xx from upstream are passed through.

---

## 5. `frontend/`

```javascript
frontend/
├── src/app/        # App shell: router, auth provider, layout (nav, role-based menus)
├── Dockerfile      # Multi-stage node build → nginx serving static SPA
├── .env.example    # VITE_API_URL (points to gateway), feature flags
└── package.json    # Depends on platform/ts/ui and platform/ts/api-client
```

- Contains **no generated API types by hand** — they come from `platform/ts/api-client`.
- Role-based views driven by identity claims: student (stay, cleaning requests, bookings), housekeeper (shifts), maintenance (requests), manager (users, housing, reports).

---

## 6. `platform/` — shared code

```javascript
platform/
├── go/
│   ├── go.mod                  # module e.g. `github.com/org/platform`
│   ├── auth/                   # JWT verify/sign helpers, role/level types (Operator, Supervisor, Manager…)
│   ├── tenancy/                # Tenant/org context if multi-tenant later
│   ├── events/                 # Outbox publisher, consumer registry, envelope {id, type, occurred_at, payload}
│   ├── messaging/              # Broker client (NATS/Kafka/Rabbit) abstraction used by events/
│   ├── httpx/                  # HTTP client with retries/timeouts + server helpers (decode/encode/errors)
│   ├── config/                 # env-loading helper shared by all services
│   ├── logging/                # Structured logger (slog wrapper), request-id correlation
│   └── observability/          # Metrics (prometheus), tracing (otel) setup
└── ts/
    ├── ui/                     # Design-system components (buttons, tables, forms, date pickers)
    └── api-client/             # Generated TS clients from all three openapi.yaml files + typed event DTOs
```

- **Rule:** services import `platform/go/*`; frontend + service webs import `platform/ts/*`. No cross-imports between `services/*` (no `import "…/housing/api/internal/…"` — internals are unexported anyway).
- `auth` defines the shared vocabulary for roles so `housing` can check *operator level* the same way `transport` checks *permitted user*.

---

## 7. `infra/`

```javascript
infra/
├── compose/
│   ├── docker-compose.yml      # postgres, (nats|kafka|rabbit), redis?, prometheus+grafana, jaeger/otel-collector
│   ├── .env.example            # DB passwords, ports, image tags
│   └── initdb/01-databases.sql # CREATE DATABASE identity / housing / transport (one DB per service)
└── observability/              # grafana dashboards, alert rules, otel-collector config (.gitkeep → real files)
```

- **One database per service** — no shared tables; cross-service reads go through contracts.
- Compose profiles: `make up` runs infra + gateway + all services; `make up svc=housing` runs just what housing needs.

---

## 8. `tools/` and `.github/`

- `tools/.gitkeep` → put codegen scripts, `migrate` version pinners, seed-data generators, `structure.txt` generator.
- `.github/workflows/` → CI per stack: `go-ci.yml` (lint+test+build all modules), `ts-ci.yml`, `openapi-check.yml` (breaking-change detection on `contracts/openapi.yaml`), `docker-publish.yml`.
- `pull_request_template.md` → checklist: contracts updated? migration included? ADR needed?
- `CODEOWNERS` → each `services/<name>/` owned by its team; `platform/` changes require platform-team review.

---

## 9. How everything interacts (end-to-end)

### 9.1 Sync request (student books a bus seat)

```javascript
browser → frontend (React)
  → platform/ts/api-client (generated from transport/openapi.yaml)
  → api-gateway /api/transport/seats        (JWT verified; X-User-Id set)
  → transport/handler  → transport/service  (capacity rule)
  → transport/repository (tx: insert booking)
  → transport/events (outbox: seat.booked)  → commit
  → 201 back up the chain
```

### 9.2 Async event (identity → housing)

1. `identity/service` creates a User-student → writes `users` row + outbox `user.created` → commit.
2. Outbox relay (`platform/go/events`) publishes to broker.
3. `housing/events` consumer receives it → creates read-only `resident` projection used for *"student has an active stay here?"* checks — avoiding sync calls to identity at request time.

Same pattern: `stay.activated` → transport may grant ride permissions; `maintenance.resolved` → manager dashboards.

### 9.3 Cross-service read without events

If housing needs the student's name **now**, it calls identity's internal endpoint through a client generated from `identity/contracts/openapi.yaml` (cached, short timeout, circuit breaker via `platform/go/httpx`). Never touches identity's DB.

### 9.4 Dependency direction (must always hold)

```javascript
frontend ─▶ platform/ts
services/*/web ─▶ platform/ts
services/*/api ─▶ platform/go, services/*/contracts (as clients, generated)
api-gateway ─▶ platform/go
infra ─▶ (nothing depends on it; it depends on all images)
```

Forbidden: service→service DB access, handler→repository, frontend→service direct calls (bypassing gateway), business rules in gateway or platform.

---

## 10. Where each domain rule lives (quick index)

| Rule | Enforced in |
| --- | --- |
| ≥1 of email/phone on account | identity/service + DB constraint |
| Exactly one profile type | identity/service + DB unique constraint on user_id |
| At most one active Stay per student/guest-student | housing/service (partial unique index) |
| Room ≤1 active Stay; apart ≤ capacity | housing/service + repository query + exclusion constraint |
| Only operator executes a cleaning-shift | housing/service (role check via platform/go/auth) |
| Corridor/kitchen/laundry cleaning supervisor-only | housing/service |
| Visitor never has a Stay | housing/service (guest subtype check) |
| Ride only 07:00–22:00, permitted users | transport/service |
| Bus seat capacity / golfette seat+trunk capacity | transport/service |
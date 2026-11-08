# Stage 1: Build identity and shared platform/go
FROM golang:1.25-alpine AS builder

WORKDIR /src

# 1. Copy workspace manifests and target module definition files
COPY go.work go.work.sum* ./
COPY platform/go/go.mod platform/go/go.sum* ./platform/go/
COPY services/identity/api/go.mod services/identity/api/go.sum* ./services/identity/api/

# 2. Download dependencies per-module with GOWORK=off to bypass missing sibling modules
RUN cd platform/go && GOWORK=off go mod download
RUN cd services/identity/api && GOWORK=off go mod download

# 3. Copy source trees needed for the identity service
COPY platform/go/ ./platform/go/
COPY services/identity/api/ ./services/identity/api/

# 4. Compile with GOWORK=off
WORKDIR /src/services/identity/api
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off \
    go build -trimpath -ldflags="-s -w" \
    -o /identity_service ./cmd/server/main.go

# Stage 2: Minimal runtime image
FROM alpine:3.22 AS runtime

WORKDIR /app

RUN adduser -D -g '' app

COPY --from=builder --chown=app:app /identity_service ./identity_service

USER app

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["./identity_service"]
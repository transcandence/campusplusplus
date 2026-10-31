# Stage 1: Build the Go Identity Service
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
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
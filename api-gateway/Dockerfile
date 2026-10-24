# Stage 1: Build the Go API Gateway
FROM golang:1.25.4-alpine AS builder

WORKDIR /app

COPY go.mod ./
#COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO=ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" \
    -o /api_gateway ./cmd/server/main.go

# Stage 2: Minimal runtime image
FROM alpine:3.22 AS runtime

WORKDIR /app

RUN adduser -D -g '' app

COPY --from=builder /api-api_gateway ./api_gateway

USER app

EXPOSE 8080

ENV GATEWAY_ADDR=:8080
ENV IDENTITY_URL=http://identity:8080

CMD ["./api_gateway"]
FROM golang:1.25.11-alpine AS builder
ENV GOWORK=off GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux
WORKDIR /src

COPY platform/go/go.mod platform/go/go.sum* ./platform/go/
COPY api-gateway/go.mod api-gateway/go.sum* ./api-gateway/

RUN cd platform/go && go mod download
RUN cd api-gateway/ && go mod download

COPY platform/go/ ./platform/go/
COPY api-gateway/ ./api-gateway/

WORKDIR /src/api-gateway
RUN go build -trimpath -ldflags="-s -w" -o /out/api_gateway ./cmd/server

FROM alpine:3.22 AS runtime
RUN adduser -D -g '' app
WORKDIR /app
COPY --from=builder --chown=app:app /out/ ./
USER app
EXPOSE 8080
# HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
#     CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null|| exit 1
CMD ["./api_gateway"]
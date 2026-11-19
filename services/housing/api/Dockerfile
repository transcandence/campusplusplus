
FROM golang:1.25.11-alpine AS builder

ENV GOWORK=off \
    GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /src

COPY services/housing/api/go.* ./services/housing/api/
COPY platform/go.* ./platform/go/

RUN cd platform/go && go mod download
RUN cd services/housing/api && go mod download

COPY platform/go ./platform/go
COPY services/housing/api/ ./services/housing/api/

WORKDIR /src/services/housing/api

RUN go build -trimpath -ldflags="-s -w" \
    -o /out/server ./cmd/server/


FROM alpine:3.22 AS runtime

RUN adduser -D -g '' app

WORKDIR /app

COPY --from=builder --chown=app:app /out/ ./

# COPY --from=builder --chown=app:app \
#     /src/services/housing/api/migrations ./migrations

USER app

EXPOSE 8080

CMD ["./server"]


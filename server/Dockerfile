# =============================================================================
# ai-platform server — multi-stage Docker build
# =============================================================================

# ---- Stage 1: Build ----
FROM golang:1.26-alpine AS builder

# No gcc/musl-dev needed — CGO_ENABLED=0
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o /app/server ./main.go

# (Migrations use built-in runner in internal/boot/migrate.go — no goose needed)

# ---- Stage 2: Run ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl gettext

COPY --from=builder /app/server                 /app/server
COPY --from=builder /src/manifest               /app/manifest
COPY --from=builder /src/migrations             /app/migrations
COPY entrypoint.sh                              /app/entrypoint.sh

RUN chmod +x /app/entrypoint.sh /app/server

WORKDIR /app
EXPOSE 8080 8081 8082

HEALTHCHECK --interval=30s --timeout=10s --retries=5 \
  CMD curl -sf http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]

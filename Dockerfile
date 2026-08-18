# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache build-base
WORKDIR /workspace/src

COPY src/go.mod src/go.sum ./
RUN go mod download

COPY src/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/taawun ./cmd

FROM alpine:3.22

RUN apk add --no-cache ca-certificates su-exec tzdata \
    && addgroup -S -g 10001 taawun \
    && adduser -S -D -H -u 10001 -G taawun taawun \
    && install -d -o taawun -g taawun /app /data /data/artifacts

WORKDIR /app
COPY --from=builder --chown=taawun:taawun /out/taawun /app/taawun
COPY --chmod=755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN test "$(head -c 9 /usr/local/bin/docker-entrypoint.sh)" = '#!/bin/sh' \
    && ! grep -q "$(printf '\r')" /usr/local/bin/docker-entrypoint.sh

ENV PORT=8080 \
    APP_DB_PATH=/data/taawun.db \
    TAWUN_ARTIFACT_ROOT=/data/artifacts
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/api/health" >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

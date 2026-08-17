# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app

# Copy workspace and module manifests first for better layer caching
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY cmd/server/go.mod cmd/server/go.sum ./cmd/server/

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application binary
RUN go build -ldflags="-s -w" -o /app/taawun ./cmd/server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite

WORKDIR /app

# Copy binary and web assets from builder
COPY --from=builder /app/taawun .
COPY --from=builder /app/internal/web ./internal/web

# Prepare data directory and declare volume
RUN mkdir -p /app/data
VOLUME /app/data

EXPOSE 8080

# Default environment variables
ENV PORT=8080
ENV DB_PATH=/app/data/taawun.db
ENV COOKIE_SECURE=false

CMD ["./taawun"]

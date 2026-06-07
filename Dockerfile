# Stage 1: Build the Go binary
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy dependency manifests first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree
COPY . .

# Build a statically-linked binary
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /feedreader ./cmd/srv

# Stage 2: Minimal runtime image
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -S feedreader && adduser -S feedreader -G feedreader

# Create data directory for SQLite DB and config
RUN mkdir -p /data && chown feedreader:feedreader /data

# Copy the binary
COPY --from=builder /feedreader /usr/local/bin/feedreader

# Copy static assets and templates.
# runtime.Caller(0) in srv/server.go resolves to the compile-time path
# /app/srv/server.go, so the app looks for templates and static files
# relative to that path. We must place them at the same location.
COPY --from=builder /app/srv/templates/ /app/srv/templates/
COPY --from=builder /app/srv/static/ /app/srv/static/

# DB migrations are embedded in the binary via go:embed — no copy needed.

VOLUME /data
WORKDIR /data

USER feedreader

# Default config file location. The app also checks for config.toml in the
# working directory (/data), so mounting a config there works without this env.
ENV CONFIG_FILE=/data/config.toml

EXPOSE 8000

ENTRYPOINT ["feedreader"]
CMD ["--db", "/data/db.sqlite3"]

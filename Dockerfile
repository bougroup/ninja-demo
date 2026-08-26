# syntax=docker/dockerfile:1

# ---- builder -----------------------------------------------------------
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

# ham compiles the static landing page (web/src -> web/public).
RUN go install github.com/fobilow/ham/cmd/ham@latest

COPY . .

# CGO_ENABLED=0: modernc.org/sqlite is pure Go, so the binary is fully static.
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

RUN sh web/build.sh

# ---- runtime -------------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/server ./server
COPY --from=builder /src/templates ./templates
COPY --from=builder /src/web/public ./web/public

# SQLite file lives on a volume so it survives container recreation.
RUN mkdir -p /data && chown -R app:app /data /app
VOLUME ["/data"]

ENV DATABASE_URL=/data/data.db \
    ADDR=:4000

USER app
EXPOSE 4000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
    CMD wget -q -O- http://localhost:4000/ >/dev/null || exit 1

ENTRYPOINT ["./server"]

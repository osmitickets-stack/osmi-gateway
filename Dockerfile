# ==========================
# Builder
# ==========================
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o gateway \
    ./cmd/main.go

# ==========================
# Runtime
# ==========================
FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    netcat-openbsd

RUN addgroup -S osmi && adduser -S osmi -G osmi

WORKDIR /app

COPY --from=builder /app/gateway .

USER osmi

EXPOSE 8083

HEALTHCHECK \
    --interval=30s \
    --timeout=5s \
    --start-period=10s \
    --retries=3 \
    CMD nc -z localhost 8083 || exit 1

CMD ["./gateway"]
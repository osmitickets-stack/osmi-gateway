FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY osmi-gateway/go.mod osmi-gateway/go.sum ./osmi-gateway/
COPY osmi-protobuf/go.mod ./osmi-protobuf/
COPY osmi-protobuf/go.sum ./osmi-protobuf/
COPY osmi-protobuf/gen ./osmi-protobuf/gen
COPY osmi-protobuf/proto ./osmi-protobuf/proto

WORKDIR /app/osmi-gateway

RUN go mod download

COPY osmi-gateway ./

RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/main.go


FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/osmi-gateway/gateway .
COPY --from=builder /app/osmi-gateway/.env.production ./.env

EXPOSE 8080

CMD ["./gateway"]
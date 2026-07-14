# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.3

########################
# builder
########################
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y \
    libaom-dev git build-essential pkg-config \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./

RUN --mount=type=secret,id=netrc,target=/root/.netrc,required=false \
    go mod download

COPY . .

# 1) HTTP service
RUN go build -trimpath -o /app/delivery ./cmd/http

# 2) Console / Cobra
RUN go build -trimpath -o /app/cobra ./cmd/console


########################
# runtime local
########################
FROM golang:${GO_VERSION}-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y \
    curl \
    libaom-dev \
    openssl \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/delivery /app/delivery
COPY --from=builder /app/cobra /app/cobra

COPY --from=builder /app/config /app/config

RUN mkdir -p /app/logs /app/entrypoints /app/secret

EXPOSE 7540

ENTRYPOINT ["/app/entrypoints/app-entrypoint.sh"]
CMD ["/app/delivery"]

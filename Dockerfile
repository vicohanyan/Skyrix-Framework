# syntax=docker/dockerfile:1.7

########################
# builder
########################
FROM golang:1.25.1-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y \
    libaom-dev git build-essential pkg-config \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./

RUN --mount=type=secret,id=netrc,target=/root/.netrc \
    go mod download

COPY . .

# 1) HTTP service
RUN go build -trimpath -o /app/delivery ./cmd/http

# 2) Console (cobra)
RUN go build -trimpath -o /app/cobra ./cmd/console


########################
# runtime (local)
########################
FROM golang:1.25.1-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y \
    curl \
    libaom-dev \
    openssl \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/delivery /app/delivery
COPY --from=builder /app/cobra    /app/cobra

RUN mkdir -p /app/logs /app/entrypoints /app/secret

EXPOSE 6060

ENTRYPOINT ["/app/entrypoints/app-entrypoint.sh"]
CMD ["/app/delivery"]

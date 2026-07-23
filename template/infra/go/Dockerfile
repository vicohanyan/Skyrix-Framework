# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26.3

########################
# builder
########################
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /workspace/template

RUN apt-get update && apt-get install -y \
    libaom-dev git build-essential pkg-config \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum /workspace/
COPY template/go.mod template/go.sum /workspace/template/

RUN --mount=type=secret,id=netrc,target=/root/.netrc,required=false \
    go mod download

COPY . /workspace/

# 1) HTTP service
RUN go build -trimpath -o /out/delivery ./cmd/http

# 2) Console / Cobra
RUN go build -trimpath -o /out/cobra ./cmd/cobra


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

COPY --from=builder /out/delivery /app/delivery
COPY --from=builder /out/cobra /app/cobra

COPY --from=builder /workspace/template/config /app/config

RUN mkdir -p /app/logs /app/entrypoints /app/secret

EXPOSE 7540

ENTRYPOINT ["/app/entrypoints/app-entrypoint.sh"]
CMD ["/app/delivery"]

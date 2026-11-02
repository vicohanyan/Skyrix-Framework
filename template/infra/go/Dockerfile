# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.27.3

########################
# builder
########################
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /workspace/template

RUN apt-get update && apt-get install -y \
    git build-essential pkg-config \
 && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum /workspace/
COPY template/go.mod template/go.sum /workspace/template/

RUN --mount=type=secret,id=netrc,target=/root/.netrc,required=false \
    go mod download

COPY . /workspace/

# 1) HTTP binary
RUN go build -trimpath -o /out/http ./cmd/http

# 2) Console / Cobra
RUN go build -trimpath -o /out/cobra ./cmd/cobra

# 3) Worker (background subscribers and jobs, no HTTP)
RUN go build -trimpath -o /out/worker ./cmd/worker


########################
# runtime local
########################
FROM golang:${GO_VERSION}-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y \
    curl \
    openssl \
 && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/http /app/http
COPY --from=builder /out/cobra /app/cobra
COPY --from=builder /out/worker /app/worker

COPY --from=builder /workspace/template/config /app/config

RUN mkdir -p /app/logs /app/entrypoints /app/secret

EXPOSE 8080

ENTRYPOINT ["/app/entrypoints/app-entrypoint.sh"]
CMD ["/app/http"]

# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.26.2-alpine AS builder

RUN apk add --no-cache git ca-certificates

# eBPF build tools — required for bpf2go and cilium/ebpf CGO bindings
RUN apk add --no-cache clang llvm linux-headers libbpf-dev musl-dev \
    && go install github.com/cilium/ebpf/cmd/bpf2go@v0.21.0

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w -extldflags '-static'" -o /api ./cmd/api

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM alpine:3.23

ARG DOTENVX_VERSION=1.61.0
RUN apk add --no-cache ca-certificates tzdata curl \
    && curl -sfS "https://dotenvx.sh/install.sh?version=v${DOTENVX_VERSION}" | sh \
    && addgroup -g 1000 raven \
    && adduser -u 1000 -G raven -D raven

COPY --from=builder /api /app/api

WORKDIR /app

USER raven

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["dotenvx", "run", "--", "/app/api"]

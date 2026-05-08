# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.26.3-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS builder

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
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

# hadolint DL4006: the curl | sh pipe below needs an explicit pipefail-aware
# shell. Alpine symlinks /bin/sh to busybox; ash supports `-o pipefail`.
SHELL ["/bin/ash", "-o", "pipefail", "-c"]

RUN apk add --no-cache ca-certificates tzdata curl \
    && curl -sfS "https://dotenvx.sh?version=1.59.1" | sh \
    && addgroup -g 1000 raven \
    && adduser -u 1000 -G raven -D raven

COPY --from=builder /api /app/api

WORKDIR /app

USER raven

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["dotenvx", "run", "--", "/app/api"]

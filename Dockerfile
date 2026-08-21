# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS builder

RUN apk add --no-cache git ca-certificates

# eBPF build tools — required for bpf2go and cilium/ebpf CGO bindings
RUN apk add --no-cache clang llvm linux-headers libbpf-dev musl-dev \
    && go install github.com/cilium/ebpf/cmd/bpf2go@v0.21.0

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w -extldflags '-static'" -o /api ./cmd/api \
 && CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w -extldflags '-static'" -o /worker ./cmd/worker

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

# hadolint DL4006: the curl pipe below needs an explicit pipefail-aware
# shell. Alpine symlinks /bin/sh to busybox; ash supports `-o pipefail`.
SHELL ["/bin/ash", "-o", "pipefail", "-c"]

# dotenvx — pinned to a specific release so the install.sh URL is immutable
# and verified against the SHA-256 published in that release's checksums.txt.
# Bump in lockstep with go.yml / python.yml / ai-worker/Dockerfile.
ARG DOTENVX_VERSION=1.65.0
ARG DOTENVX_INSTALLER_SHA256=a1faad2613cd88286d02a5201c34e880f935558957eb9c09e3c11f27039a6296

RUN apk add --no-cache ca-certificates tzdata curl \
    && curl -fsSL "https://github.com/dotenvx/dotenvx/releases/download/v${DOTENVX_VERSION}/install.sh" -o /tmp/dotenvx-install.sh \
    && echo "${DOTENVX_INSTALLER_SHA256}  /tmp/dotenvx-install.sh" | sha256sum -c - \
    && sh /tmp/dotenvx-install.sh \
    && rm /tmp/dotenvx-install.sh \
    && addgroup -g 1000 raven \
    && adduser -u 1000 -G raven -D raven

COPY --from=builder /api /app/api
# The Asynq queue consumer. Same image, separate process — deploy as a
# second container that overrides CMD to /app/worker. Lives in the same
# image to keep the multi-arch build matrix from doubling.
COPY --from=builder /worker /app/worker

WORKDIR /app

USER raven

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["dotenvx", "run", "--", "/app/api"]

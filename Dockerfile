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

ARG TARGETARCH
ARG DOTENVX_VERSION=1.61.0
ARG DOTENVX_SHA256_AMD64=58fe4a5f84af835dce71e9e09bda47e7d54f0dc5b302bbf4f50224cc906646d3
ARG DOTENVX_SHA256_ARM64=082a25127493197dcd9f167ba32986e2934af999c822fb5350ceda1da228d9ba
RUN apk add --no-cache ca-certificates tzdata curl \
    && DOTENVX_ARCH="${TARGETARCH}" \
    && if [ "$DOTENVX_ARCH" = "amd64" ]; then DOTENVX_SHA256="$DOTENVX_SHA256_AMD64"; \
       elif [ "$DOTENVX_ARCH" = "arm64" ]; then DOTENVX_SHA256="$DOTENVX_SHA256_ARM64"; \
       else echo "Unsupported arch: $DOTENVX_ARCH" && exit 1; fi \
    && curl -sL -o /tmp/dotenvx.tar.gz "https://github.com/dotenvx/dotenvx/releases/download/v${DOTENVX_VERSION}/dotenvx-${DOTENVX_VERSION}-linux-${DOTENVX_ARCH}.tar.gz" \
    && echo "${DOTENVX_SHA256}  /tmp/dotenvx.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/dotenvx.tar.gz -C /usr/local/bin dotenvx \
    && chmod 755 /usr/local/bin/dotenvx \
    && rm /tmp/dotenvx.tar.gz \
    && apk del curl \
    && addgroup -g 1000 raven \
    && adduser -u 1000 -G raven -D raven

COPY --from=builder /api /app/api

WORKDIR /app

USER raven

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["dotenvx", "run", "--", "/app/api"]

# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.26.1-alpine AS builder

RUN apk add --no-cache git ca-certificates

# eBPF build tools — required for bpf2go and cilium/ebpf CGO bindings
RUN apk add --no-cache clang llvm linux-headers libbpf-dev musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w -extldflags '-static'" -o /api ./cmd/api

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 raven \
    && adduser -u 1000 -G raven -D raven

COPY --from=builder /api /api

USER raven

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["/api"]

FROM python:3.14-slim AS builder

WORKDIR /build

COPY requirements.txt .
RUN pip install --no-cache-dir --prefix=/install -r requirements.txt

COPY . .
RUN pip install --no-cache-dir --prefix=/install .

# ---------------------------------------------------------------------------
FROM python:3.14-slim

ARG TARGETARCH
ARG DOTENVX_VERSION=1.61.0
ARG DOTENVX_SHA256_AMD64=58fe4a5f84af835dce71e9e09bda47e7d54f0dc5b302bbf4f50224cc906646d3
ARG DOTENVX_SHA256_ARM64=082a25127493197dcd9f167ba32986e2934af999c822fb5350ceda1da228d9ba
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && DOTENVX_ARCH="${TARGETARCH}" \
    && if [ "$DOTENVX_ARCH" = "amd64" ]; then DOTENVX_SHA256="$DOTENVX_SHA256_AMD64"; \
       elif [ "$DOTENVX_ARCH" = "arm64" ]; then DOTENVX_SHA256="$DOTENVX_SHA256_ARM64"; \
       else echo "Unsupported arch: $DOTENVX_ARCH" && exit 1; fi \
    && curl -sL -o /tmp/dotenvx.tar.gz "https://github.com/dotenvx/dotenvx/releases/download/v${DOTENVX_VERSION}/dotenvx-${DOTENVX_VERSION}-linux-${DOTENVX_ARCH}.tar.gz" \
    && echo "${DOTENVX_SHA256}  /tmp/dotenvx.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/dotenvx.tar.gz -C /usr/local/bin dotenvx \
    && chmod 755 /usr/local/bin/dotenvx \
    && rm /tmp/dotenvx.tar.gz \
    && apt-get purge -y curl && apt-get autoremove -y && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 1000 raven \
    && useradd --uid 1000 --gid raven --create-home raven

COPY --from=builder /install /usr/local

WORKDIR /app
COPY raven_worker/ raven_worker/

USER raven

EXPOSE 50051

ENTRYPOINT ["dotenvx", "run", "--"]
CMD ["python", "-m", "raven_worker"]

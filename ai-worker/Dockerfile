FROM python:3.14-slim AS builder

WORKDIR /build

COPY requirements.txt .
RUN pip install --no-cache-dir --prefix=/install -r requirements.txt

COPY . .
RUN pip install --no-cache-dir --prefix=/install .

# ---------------------------------------------------------------------------
FROM python:3.14-slim

ARG DOTENVX_VERSION=1.61.0
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && curl -sfS "https://dotenvx.sh/install.sh?version=v${DOTENVX_VERSION}" | sh \
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

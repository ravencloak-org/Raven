FROM python:3.14-slim@sha256:5b3879b6f3cb77e712644d50262d05a7c146b7312d784a18eff7ff5462e77033 AS builder

WORKDIR /build

COPY requirements.txt .
RUN pip install --no-cache-dir --prefix=/install -r requirements.txt

COPY . .
RUN pip install --no-cache-dir --prefix=/install .

# ---------------------------------------------------------------------------
FROM python:3.14-slim@sha256:5b3879b6f3cb77e712644d50262d05a7c146b7312d784a18eff7ff5462e77033

# hadolint DL4006: the curl | sh pipe below needs an explicit pipefail-aware
# shell; python:3.14-slim ships bash so use it instead of /bin/sh.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && curl -sfS https://dotenvx.sh | sh \
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

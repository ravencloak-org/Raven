FROM python:3.14-slim@sha256:7a500125bc50693f2214e842a621440a1b1b9cbb2188f74ab045d29ed2ea5856 AS builder

WORKDIR /build

# Third-party deps come from a fully-resolved version-pinned lockfile.
# The lockfile currently does NOT carry sha256 hashes (the
# deps-bundle export in 471318c26 used `uv export --no-hashes`), so
# `pip install --require-hashes` would refuse every line. Re-enable
# hash-pinning by regenerating with hashes — see follow-up issue —
# and add `--require-hashes` back here once the file has them.
# Regenerate with:
#   uv export --no-emit-project --no-dev -o requirements.txt
COPY requirements.txt .
# Compensating integrity gate: every non-blank, non-comment dependency
# line must be exact-pinned with `==` so range specifiers cannot slip
# into image builds while --require-hashes is disabled.
# hadolint ignore=DL4006
RUN if awk 'NF && $0 !~ /^[[:space:]]*(#|--)/ && $0 !~ /==/' requirements.txt | grep -q .; then \
        echo "ERROR: requirements.txt contains non-exact-pinned entries:"; \
        awk 'NF && $0 !~ /^[[:space:]]*(#|--)/ && $0 !~ /==/' requirements.txt; \
        exit 1; \
    fi
RUN pip install --no-cache-dir --prefix=/install -r requirements.txt

# The local package is built from in-tree source (no network fetch), so
# `--no-deps` keeps the install offline-pure: every dep was already pinned
# above and no late-resolution happens here.
COPY . .
RUN pip install --no-cache-dir --no-deps --prefix=/install .

# ---------------------------------------------------------------------------
FROM python:3.14-slim@sha256:7a500125bc50693f2214e842a621440a1b1b9cbb2188f74ab045d29ed2ea5856

# hadolint DL4006: the curl pipe below needs an explicit pipefail-aware
# shell; python:3.14-slim ships bash so use it instead of /bin/sh.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# dotenvx — pinned to a specific release so the install.sh URL is immutable
# and verified against the SHA-256 published in that release's checksums.txt.
# Bump in lockstep with go.yml / python.yml / root Dockerfile so all five
# install sites stay on the same dotenvx version. To bump:
#   1. pick the new tag (e.g. v1.66.0)
#   2. fetch
#      https://github.com/dotenvx/dotenvx/releases/download/<TAG>/checksums.txt
#      and copy the install.sh hash from it
#   3. update DOTENVX_VERSION and DOTENVX_INSTALLER_SHA256 here and in the
#      other four files
ARG DOTENVX_VERSION=1.65.0
ARG DOTENVX_INSTALLER_SHA256=a1faad2613cd88286d02a5201c34e880f935558957eb9c09e3c11f27039a6296

RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
    && curl -fsSL "https://github.com/dotenvx/dotenvx/releases/download/v${DOTENVX_VERSION}/install.sh" -o /tmp/dotenvx-install.sh \
    && echo "${DOTENVX_INSTALLER_SHA256}  /tmp/dotenvx-install.sh" | sha256sum -c - \
    && sh /tmp/dotenvx-install.sh \
    && rm /tmp/dotenvx-install.sh \
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

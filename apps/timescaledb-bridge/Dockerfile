# syntax=docker/dockerfile:1
#
# One-time bridge image for in-place TimescaleDB major-version upgrades in CNPG.
#
# Problem: when CNPG switches to a new extension image that does not contain the
# OLD versioned loader .so (e.g. timescaledb-2.23.1.so), PostgreSQL cannot load
# the currently-installed extension and the pod crash-loops before ALTER EXTENSION
# can be run.
#
# Solution: this image bundles:
#   - ALL extension files for VERSION  (the upgrade target)
#   - The versioned loader .so from OLD_VERSION alone  (so PG can load the
#     currently-installed extension, then ALTER EXTENSION upgrades it)
#
# Migration procedure
# -------------------
#   1. Set CNPG extensionImageRef to this bridge image; wait for cluster healthy.
#   2. On the primary (NOT via pgbouncer):
#        ALTER EXTENSION timescaledb UPDATE;
#   3. Set CNPG extensionImageRef to the final new-version image.
#   4. Retire / delete this app from the repo.

ARG VERSION
ARG OLD_VERSION
ARG PG_MAJOR=18
ARG RUNTIME_IMAGE=ghcr.io/cloudnative-pg/postgresql:18.3-standard-trixie@sha256:81bad466fe006454482678c6c67a24c1e30454b519011e0196cba2e4c83e1e1a
ARG OLD_IMAGE=ghcr.io/shusaan/timescaledb-testing:2.23.1-18-trixie@sha256:4709935bed4cce5977bc4b1858a94e2e80f22f0303217163a9e97b8898e33ee2

# ---------------------------------------------------------------------------
# Stage 1: build NEW extension files against the exact CNPG runtime image.
# ---------------------------------------------------------------------------
FROM ${RUNTIME_IMAGE} AS build-new
ARG VERSION
ARG PG_MAJOR

RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cmake \
    curl \
    libicu-dev \
    libssl-dev \
    pkg-config \
    "postgresql-server-dev-${PG_MAJOR}" \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp
RUN mkdir -p /tmp/src /tmp/built /tmp/out/lib /tmp/out/share/extension \
    && curl -fsSL "https://github.com/timescale/timescaledb/archive/refs/tags/${VERSION}.tar.gz" \
    | tar xz -C /tmp/src --strip-components=1 \
    && cd /tmp/src \
    && ./bootstrap \
    && make -C build install DESTDIR=/tmp/built \
    && find /tmp/built -type f -name "timescaledb*.so" -exec cp {} /tmp/out/lib/ \; \
    && find /tmp/built -type f -path "*/extension/timescaledb*" -exec cp {} /tmp/out/share/extension/ \; \
    && test -f /tmp/out/lib/timescaledb.so \
    && test -f /tmp/out/share/extension/timescaledb.control

# ---------------------------------------------------------------------------
# Stage 2: pull the OLD versioned loader .so from the previous release image.
#          Only the file named timescaledb-<OLD_VERSION>.so is needed; the
#          other old files (timescaledb-2.so, etc.) must NOT overwrite the new
#          ones, so we extract with a helper stage.
# ---------------------------------------------------------------------------
FROM ${OLD_IMAGE} AS old-release

FROM docker.io/library/debian:trixie-slim AS assemble
ARG OLD_VERSION
COPY --from=build-new /tmp/out/lib/ /new/lib/
COPY --from=build-new /tmp/out/share/extension/ /new/share/extension/
COPY --from=old-release /lib/ /old/lib/
RUN mkdir -p /out/lib /out/share/extension \
    && cp -a /new/lib/. /out/lib/ \
    && cp -a /new/share/extension/. /out/share/extension/ \
    && find /old/lib -maxdepth 1 -type f -name "*-${OLD_VERSION}.so" -exec cp {} /out/lib/ \; \
    && test -f "/out/lib/timescaledb-${OLD_VERSION}.so" \
    && test -f "/out/lib/timescaledb.so" \
    && test -f "/out/share/extension/timescaledb.control"

# ---------------------------------------------------------------------------
# Final: scratch image — new version files + single old loader .so
# ---------------------------------------------------------------------------
FROM scratch
# New-version extension files at standard CNPG scratch layout paths.
COPY --from=assemble /out/ /
# Old versioned loader required for ALTER EXTENSION to succeed.

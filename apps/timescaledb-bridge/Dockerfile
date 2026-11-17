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
# OLD_VERSION MUST match the exact extversion in pg_extension, not necessarily
# the raw git tag string:
#   SELECT extversion FROM pg_extension WHERE extname = 'timescaledb';
# The old loader is rebuilt from source here (not copied from a prior image)
# so its compiled-in version string always matches OLD_VERSION exactly. This
# matters because TimescaleDB FATALs on connect if "shared library version"
# (baked into the .so at compile time) differs from "SQL version" (extversion
# in pg_extension) — a mismatch here breaks EVERY connection to the database,
# not just calls to timescaledb functions, since the check runs on backend
# startup via shared_preload_libraries.
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
# Pin the server-dev headers to the EXACT same minor version as the CNPG runtime
# postgres binary to avoid symbol mismatches at extension load time.
ARG PG_DEV_PKG_VERSION=18.6-1.pgdg13+2
ARG RUNTIME_IMAGE=ghcr.io/cloudnative-pg/postgresql:18.6-standard-trixie@sha256:de572898499fab9406f0da51929da562f96f3be40ca79fafea8ec1a8d5b42065

# ---------------------------------------------------------------------------
# Stage 1: build NEW extension files against the exact CNPG runtime image.
# ---------------------------------------------------------------------------
FROM ${RUNTIME_IMAGE} AS build-new
ARG VERSION
ARG PG_MAJOR
ARG PG_DEV_PKG_VERSION

# CNPG image runs as uid 26 (postgres); switch to root for package install + build.
USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cmake \
    curl \
    libicu-dev \
    libssl-dev \
    pkg-config \
    "postgresql-server-dev-${PG_MAJOR}=${PG_DEV_PKG_VERSION}" \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp
RUN mkdir -p /tmp/src /tmp/built /tmp/out/lib /tmp/out/share/extension \
    && curl -fsSL "https://github.com/timescale/timescaledb/archive/refs/tags/${VERSION}.tar.gz" \
    | tar xz -C /tmp/src --strip-components=1 \
    && cd /tmp/src \
    # upstream's version.config can embed a "-dev" suffix even on tagged releases
    # (e.g. tag 2.27.0 -> "2.27.0-dev"), which breaks ALTER EXTENSION UPDATE
    # since no upgrade script targets that string; force it to the clean tag.
    && sed -i "s/^version[[:space:]]*=.*/version = ${VERSION}/" version.config \
    && ./bootstrap \
    && make -C build install DESTDIR=/tmp/built \
    && find /tmp/built -type f -name "timescaledb*.so" -exec cp {} /tmp/out/lib/ \; \
    && find /tmp/built -type f -path "*/extension/timescaledb*" -exec cp {} /tmp/out/share/extension/ \; \
    && test -f /tmp/out/lib/timescaledb.so \
    && test -f /tmp/out/share/extension/timescaledb.control

# ---------------------------------------------------------------------------
# Stage 2: build the OLD versioned loader .so from source, same as build-new.
#          Only its timescaledb-<OLD_VERSION>.so is used; rebuilding (rather
#          than reusing pre-built bytes from a prior image) guarantees the
#          compiled-in version string matches OLD_VERSION exactly.
# ---------------------------------------------------------------------------
FROM ${RUNTIME_IMAGE} AS build-old
ARG OLD_VERSION
ARG PG_MAJOR
ARG PG_DEV_PKG_VERSION

USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cmake \
    curl \
    libicu-dev \
    libssl-dev \
    pkg-config \
    "postgresql-server-dev-${PG_MAJOR}=${PG_DEV_PKG_VERSION}" \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp
RUN mkdir -p /tmp/src /tmp/built /tmp/out/lib \
    && curl -fsSL "https://github.com/timescale/timescaledb/archive/refs/tags/${OLD_VERSION}.tar.gz" \
    | tar xz -C /tmp/src --strip-components=1 \
    && cd /tmp/src \
    && sed -i "s/^version[[:space:]]*=.*/version = ${OLD_VERSION}/" version.config \
    && ./bootstrap \
    && make -C build install DESTDIR=/tmp/built \
    # timescaledb-tsl-<version>.so (community-licensed functions) is a
    # separate module from timescaledb-<version>.so; grab both.
    && find /tmp/built -type f -name "timescaledb*${OLD_VERSION}.so" -exec cp {} /tmp/out/lib/ \; \
    && test -f "/tmp/out/lib/timescaledb-${OLD_VERSION}.so"

FROM docker.io/library/debian:trixie-slim AS assemble
ARG OLD_VERSION
COPY --from=build-new /tmp/out/lib/ /new/lib/
COPY --from=build-new /tmp/out/share/extension/ /new/share/extension/
COPY --from=build-old /tmp/out/lib/ /old/lib/
RUN mkdir -p /out/lib /out/share/extension \
    && cp -a /new/lib/. /out/lib/ \
    && cp -a /new/share/extension/. /out/share/extension/ \
    && cp -a /old/lib/. /out/lib/ \
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

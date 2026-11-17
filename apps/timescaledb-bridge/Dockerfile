# syntax=docker/dockerfile:1
#
# TimescaleDB extension image for CNPG, with an upgrade bridge built in.
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
# When OLD_VERSION == VERSION there is nothing to bridge and the build-old
# stage is skipped.
#
# Upgrade procedure
# -----------------
#   1. Bump VERSION (OLD_VERSION = currently installed extversion); merge.
#   2. Point the CNPG cluster at the new image; wait for cluster healthy.
#   3. On the primary (NOT via pgbouncer), as the first command of a fresh
#      session (psql -X):
#        ALTER EXTENSION timescaledb UPDATE;
#   4. Set OLD_VERSION = VERSION so the next bump bridges from the new version.

ARG RUNTIME_IMAGE

# ---------------------------------------------------------------------------
# Build toolchain on top of the exact CNPG runtime image, so headers, pg_config
# and the postgres binary that will load the extension all agree.
# ---------------------------------------------------------------------------
FROM ${RUNTIME_IMAGE} AS build-base
ARG PG_MAJOR

# CNPG image runs as uid 26 (postgres); switch to root for package install + build.
USER root
# postgresql-server-dev is pinned to the exact version of the postgresql
# package already installed in the runtime image. apt.postgresql.org only
# keeps the latest few versions, so the pgdg archive (same signing key) is
# added to keep older pins installable.
RUN CODENAME="$(. /etc/os-release && echo "${VERSION_CODENAME}")" \
    && printf '%s\n' \
    "Types: deb" \
    "URIs: https://apt-archive.postgresql.org/pub/repos/apt" \
    "Suites: ${CODENAME}-pgdg-archive" \
    "Components: main" \
    "Signed-By: /usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg" \
    > /etc/apt/sources.list.d/pgdg-archive.sources \
    && PG_PKG_VERSION="$(dpkg-query -W -f='${Version}' "postgresql-${PG_MAJOR}")" \
    && echo "Runtime postgresql-${PG_MAJOR} ${PG_PKG_VERSION}" \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cmake \
    curl \
    libicu-dev \
    libssl-dev \
    pkg-config \
    "postgresql-server-dev-${PG_MAJOR}=${PG_PKG_VERSION}" \
    && rm -rf /var/lib/apt/lists/*

# ---------------------------------------------------------------------------
# Stage 1: build NEW extension files.
# ---------------------------------------------------------------------------
FROM build-base AS build-new
ARG VERSION

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
FROM build-base AS build-old
ARG VERSION
ARG OLD_VERSION

WORKDIR /tmp
RUN mkdir -p /tmp/src /tmp/built /tmp/out/lib \
    && if [ "${OLD_VERSION}" = "${VERSION}" ]; then echo "OLD_VERSION == VERSION, nothing to bridge"; exit 0; fi \
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

FROM build-base AS assemble
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
# Final: scratch image — new version files + single old loader .so, at the
# standard CNPG extension image layout (/lib, /share/extension).
# ---------------------------------------------------------------------------
FROM scratch
COPY --from=assemble /out/ /

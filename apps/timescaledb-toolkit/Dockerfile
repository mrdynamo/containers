# syntax=docker/dockerfile:1
#
# timescaledb_toolkit extension image for CNPG.
#
# Like the TimescaleDB image, it also carries the shared library of the version
# production runs (OLD_VERSION): installed toolkit functions reference
# $libdir/timescaledb_toolkit-<installed version>, so without it every toolkit
# function errors from rollout until ALTER EXTENSION timescaledb_toolkit UPDATE.
# Both are built from source the same way (build-toolkit.sh) against the exact
# CNPG runtime. When OLD_VERSION == VERSION the build-old stage is skipped.

ARG RUNTIME_IMAGE

# ---------------------------------------------------------------------------
# Toolchain on top of the exact CNPG runtime image, so pg_config, the headers
# pgrx compiles against, and the postgres binary that loads the extension all
# agree.
# ---------------------------------------------------------------------------
FROM ${RUNTIME_IMAGE} AS build-base
ARG PG_MAJOR

# CNPG image runs as uid 26 (postgres); switch to root for package install + build.
USER root
# postgresql-server-dev is pinned to the exact version of the postgresql
# package already installed in the runtime image. apt.postgresql.org only
# keeps the latest few versions, so the pgdg archive (same signing key) is
# added to keep older pins installable. The archive also holds newer majors'
# pre-releases (e.g. libpq5 19~rc2), so it gets a low pin priority: it only
# supplies a package when that exact version is requested.
RUN CODENAME="$(. /etc/os-release && echo "${VERSION_CODENAME}")" \
    && printf '%s\n' \
    "Types: deb" \
    "URIs: https://apt-archive.postgresql.org/pub/repos/apt" \
    "Suites: ${CODENAME}-pgdg-archive" \
    "Components: main" \
    "Signed-By: /usr/share/postgresql-common/pgdg/apt.postgresql.org.gpg" \
    > /etc/apt/sources.list.d/pgdg-archive.sources \
    && printf '%s\n' \
    "Package: *" \
    "Pin: origin apt-archive.postgresql.org" \
    "Pin-Priority: 100" \
    > /etc/apt/preferences.d/pgdg-archive.pref \
    && PG_PKG_VERSION="$(dpkg-query -W -f='${Version}' "postgresql-${PG_MAJOR}")" \
    && LIBPQ_VERSION="$(dpkg-query -W -f='${Version}' libpq5)" \
    && echo "Runtime postgresql-${PG_MAJOR} ${PG_PKG_VERSION}, libpq5 ${LIBPQ_VERSION}" \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    clang \
    curl \
    libssl-dev \
    pkg-config \
    "postgresql-server-dev-${PG_MAJOR}=${PG_PKG_VERSION}" \
    "libpq-dev=${LIBPQ_VERSION}" \
    "libpq5=${LIBPQ_VERSION}" \
    && rm -rf /var/lib/apt/lists/*

# Rust toolchain (cargo-pgrx requires llvm-tools-preview)
ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH \
    PG_MAJOR=${PG_MAJOR}
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
    | sh -s -- -y --no-modify-path --profile minimal --default-toolchain stable \
    && rustup component add llvm-tools-preview

COPY --chmod=0755 build-toolkit.sh /usr/local/bin/build-toolkit

# ---------------------------------------------------------------------------
# Stage 1: the new version (VERSION is the upstream tag, e.g. 1.27.0+p2).
# ---------------------------------------------------------------------------
FROM build-base AS build-new
ARG VERSION
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    build-toolkit "${VERSION}" /out full

# ---------------------------------------------------------------------------
# Stage 2: only the library of the version production runs.
# ---------------------------------------------------------------------------
FROM build-base AS build-old
ARG VERSION
ARG OLD_VERSION
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    mkdir -p /out/lib \
    && if [ "${OLD_VERSION}" = "${VERSION%%+*}" ]; then echo "OLD_VERSION == VERSION, nothing to bundle"; exit 0; fi \
    && build-toolkit "${OLD_VERSION}" /out lib

FROM build-base AS assemble
ARG VERSION
ARG OLD_VERSION
COPY --from=build-new /out/ /assembled/
COPY --from=build-old /out/lib/ /assembled/lib/
RUN test -f "/assembled/lib/timescaledb_toolkit-${VERSION%%+*}.so" \
    && test -f "/assembled/lib/timescaledb_toolkit-${OLD_VERSION}.so" \
    && test -f /assembled/share/extension/timescaledb_toolkit.control \
    && ls -l /assembled/lib/

# scratch: zero OS footprint — only the extension .so and SQL files are present,
# at the standard CNPG extension image layout (/lib, /share/extension).
FROM scratch
COPY --from=assemble /assembled/ /

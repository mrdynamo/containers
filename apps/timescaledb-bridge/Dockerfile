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
ARG OLD_IMAGE

# ---------------------------------------------------------------------------
# Stage 1: build the NEW TimescaleDB version (mirrors m00nwtchr pattern)
# ---------------------------------------------------------------------------
FROM docker.io/library/postgres:${PG_MAJOR}-trixie AS build-new
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
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp
RUN curl -fsSL "https://github.com/timescale/timescaledb/archive/refs/tags/${VERSION}.tar.gz" \
    | tar xz \
    && ./timescaledb-*/bootstrap \
    && DESTDIR=../built make -C build install \
    && mkdir -p out/share/extension out/lib \
    && find built/usr/share -type f -print0 | xargs -0 mv -vt out/share/extension \
    && find built/usr/lib -type f -print0 | xargs -0 mv -vt out/lib

# ---------------------------------------------------------------------------
# Stage 2: pull the OLD versioned loader .so from the previous release image.
#          Only the file named timescaledb-<OLD_VERSION>.so is needed; the
#          other old files (timescaledb-2.so, etc.) must NOT overwrite the new
#          ones, so we extract with a helper stage.
# ---------------------------------------------------------------------------
FROM ${OLD_IMAGE} AS old-release

FROM docker.io/library/debian:trixie-slim AS extract-old-loader
ARG OLD_VERSION
COPY --from=old-release /lib/ /old-lib/
RUN mkdir -p /old-out \
    && cp "/old-lib/timescaledb-${OLD_VERSION}.so" /old-out/

# ---------------------------------------------------------------------------
# Final: scratch image — new version files + single old loader .so
# ---------------------------------------------------------------------------
FROM scratch
# New-version extension files at standard CNPG scratch layout paths.
COPY --from=build-new /tmp/out /
# Old versioned loader required for ALTER EXTENSION to succeed.
COPY --from=extract-old-loader /old-out/ /lib/

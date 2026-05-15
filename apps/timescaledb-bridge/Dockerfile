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
ARG NEW_IMAGE=ghcr.io/m00nwtchr/timescaledb:2.27.0
ARG OLD_IMAGE=ghcr.io/shusaan/timescaledb-testing:2.23.1-18-trixie

# ---------------------------------------------------------------------------
# Stage 1: take all NEW extension files from the final upstream image.
# ---------------------------------------------------------------------------
FROM ${NEW_IMAGE} AS new-release

# ---------------------------------------------------------------------------
# Stage 2: pull the OLD versioned loader .so from the previous release image.
#          Only the file named timescaledb-<OLD_VERSION>.so is needed; the
#          other old files (timescaledb-2.so, etc.) must NOT overwrite the new
#          ones, so we extract with a helper stage.
# ---------------------------------------------------------------------------
FROM ${OLD_IMAGE} AS old-release

FROM docker.io/library/debian:trixie-slim AS assemble
ARG OLD_VERSION
COPY --from=new-release /lib/ /new/lib/
COPY --from=new-release /share/extension/ /new/share/extension/
COPY --from=old-release /lib/ /old/lib/
RUN mkdir -p /out/lib /out/share/extension \
    && cp -a /new/lib/. /out/lib/ \
    && cp -a /new/share/extension/. /out/share/extension/ \
    && cp "/old/lib/timescaledb-${OLD_VERSION}.so" /out/lib/ \
    && if [ -f "/old/lib/timescaledb-tsl-${OLD_VERSION}.so" ]; then cp "/old/lib/timescaledb-tsl-${OLD_VERSION}.so" /out/lib/; fi \
    && test -f "/out/lib/timescaledb-${OLD_VERSION}.so" \
    && test -f "/out/lib/timescaledb.so" \
    && test -f "/out/lib/timescaledb-2.so" \
    && test -f "/out/share/extension/timescaledb.control"

# ---------------------------------------------------------------------------
# Final: scratch image — new version files + single old loader .so
# ---------------------------------------------------------------------------
FROM scratch
# New-version extension files at standard CNPG scratch layout paths.
COPY --from=assemble /out/ /
# Old versioned loader required for ALTER EXTENSION to succeed.

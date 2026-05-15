# syntax=docker/dockerfile:1

ARG VERSION
ARG PG_MAJOR=18

FROM docker.io/library/postgres:${PG_MAJOR}-bookworm AS builder
ARG VERSION
ARG PG_MAJOR

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    gnupg \
    && curl -fsSL https://packagecloud.io/timescale/timescaledb/gpgkey \
    | gpg --dearmor -o /usr/share/keyrings/timescaledb.gpg \
    && printf "deb [signed-by=/usr/share/keyrings/timescaledb.gpg] https://packagecloud.io/timescale/timescaledb/debian/ bookworm main\n" \
    > /etc/apt/sources.list.d/timescaledb.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
    "timescaledb-toolkit-postgresql-${PG_MAJOR}=${VERSION}*" \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /out/lib /out/share/extension \
    && find "/usr/lib/postgresql/${PG_MAJOR}/lib" -name "timescaledb_toolkit*.so" -exec cp {} /out/lib/ \; \
    && find "/usr/share/postgresql/${PG_MAJOR}/extension" -name "timescaledb_toolkit*" -exec cp {} /out/share/extension/ \;

FROM docker.io/library/debian:bookworm-slim
COPY --from=builder /out /out

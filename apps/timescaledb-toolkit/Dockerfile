# syntax=docker/dockerfile:1

ARG VERSION
ARG PG_MAJOR=18

# Use the official postgres image: PGDG apt repo is pre-configured, giving
# postgresql-server-dev-* at the exact version shipped by the image and
# guaranteeing ABI compatibility with production CNPG postgres containers.
FROM docker.io/library/postgres:${PG_MAJOR}-trixie AS builder
ARG VERSION
ARG PG_MAJOR

# Upgrade all packages first (security patches), then add build deps.
# postgresql-server-dev-${PG_MAJOR} resolves from the pre-configured PGDG repo.
RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    clang \
    curl \
    libssl-dev \
    pkg-config \
    "postgresql-server-dev-${PG_MAJOR}" \
    && rm -rf /var/lib/apt/lists/*

# Install Rust toolchain (cargo-pgrx requires llvm-tools-preview)
ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
    | sh -s -- -y --no-modify-path --profile minimal --default-toolchain stable \
    && rustup component add llvm-tools-preview

# Download toolkit source at the pinned release tag
RUN mkdir -p /build \
    && curl -fsSL "https://github.com/timescale/timescaledb-toolkit/archive/refs/tags/${VERSION}.tar.gz" \
    | tar xzf - -C /build --strip-components=1

# Install cargo-pgrx at the exact version pinned in the upstream extension/Cargo.toml
# (the comment in that file says: "Keep synchronized with cargo install --version N.N.N cargo-pgrx")
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    PGRX_VER=$(grep '^pgrx = "=' /build/extension/Cargo.toml | head -1 | sed 's/pgrx = "=//;s/"//') \
    && cargo install --version "=${PGRX_VER}" --force cargo-pgrx

# Point pgrx at the already-installed system PostgreSQL; no download needed
RUN cargo pgrx init "--pg${PG_MAJOR}" "/usr/lib/postgresql/${PG_MAJOR}/bin/pg_config"

WORKDIR /build/extension

# Build and install the .so + SQL files into the PostgreSQL installation
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/build/target \
    cargo pgrx install --release \
    --pg-config "/usr/lib/postgresql/${PG_MAJOR}/bin/pg_config"

# Run the upstream post-install tool to finalise SQL upgrade scripts
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/build/target \
    cargo run \
    --manifest-path ../tools/post-install/Cargo.toml -- \
    "/usr/lib/postgresql/${PG_MAJOR}/bin/pg_config"

# Stage only the extension artifacts
RUN mkdir -p /out/lib /out/share/extension \
    && find "/usr/lib/postgresql/${PG_MAJOR}/lib" -name "timescaledb_toolkit*.so" -exec cp {} /out/lib/ \; \
    && find "/usr/share/postgresql/${PG_MAJOR}/extension" -name "timescaledb_toolkit*" -exec cp {} /out/share/extension/ \;

# scratch: zero OS footprint — only the extension .so and SQL files are present.
FROM scratch
COPY --from=builder /out /

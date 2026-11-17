# syntax=docker/dockerfile:1

ARG RUNTIME_IMAGE

# Build on the exact CNPG runtime image so pg_config, the headers pgrx compiles
# against, and the postgres binary that loads the extension all agree.
FROM ${RUNTIME_IMAGE} AS builder
ARG VERSION
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
    && find "/usr/share/postgresql/${PG_MAJOR}/extension" -name "timescaledb_toolkit*" -exec cp {} /out/share/extension/ \; \
    && test -f /out/share/extension/timescaledb_toolkit.control

# scratch: zero OS footprint — only the extension .so and SQL files are present.
FROM scratch
COPY --from=builder /out /

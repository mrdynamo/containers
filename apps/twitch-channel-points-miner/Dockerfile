# syntax=docker/dockerfile:1

FROM docker.io/library/python:3.13-alpine3.23
ARG VERSION

ENV \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_ROOT_USER_ACTION=ignore \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_BREAK_SYSTEM_PACKAGES=1 \
    CRYPTOGRAPHY_DONT_BUILD_RUST=1 \
    UV_NO_CACHE=true \
    UV_SYSTEM_PYTHON=true

USER root
WORKDIR /app

# Install uv installer script early so we can use `uv sync` to install the
# project without building wheels (use copy-link mode to avoid setuptools
# building wheels for flat-layout projects).
ADD https://astral.sh/uv/install.sh /uv-installer.sh
RUN sh /uv-installer.sh && rm /uv-installer.sh
ENV PATH="/root/.local/bin/:$PATH"
ENV UV_LINK_MODE=copy
ENV UV_NO_DEV=1
ENV UV_PYTHON_DOWNLOADS=0

RUN apk add --no-cache \
    bash \
    ca-certificates \
    catatonit \
    coreutils \
    curl \
    jq \
    nano \
    tzdata \
    && apk add --no-cache --virtual=.build-deps \
    build-base \
    libffi-dev \
    openssl-dev \
    musl-dev \
    python3-dev \
    && \
    # Download upstream release tarball and extract into /app
    curl -fsSL "https://github.com/mpforce1/Twitch-Channel-Points-Miner/archive/refs/tags/${VERSION}.tar.gz" \
    | tar xzf - -C /app --strip-components=1 \
    && \
    # Use uv to sync dependencies and install the project in copy-link mode
    # (avoids building wheel for flat-layout repo).
    -- "" \
    && uv sync --locked --no-install-project || true \
    && uv sync --locked \
    && \
    # Clean up build dependencies and caches
    chown -R root:root /app && chmod -R 755 /app \
    && apk del --purge .build-deps \
    && rm -rf /root/.cache /root/.cargo /tmp/*

COPY entrypoint.sh /entrypoint.sh
RUN chmod 0555 /entrypoint.sh || true

USER nobody:nogroup
WORKDIR /config
VOLUME ["/config"]

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]


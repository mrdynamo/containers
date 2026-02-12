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
    # Install transient helper `uv` via the interpreter to avoid PATH issues,
    # use it to install the upstream package with the same semantics as the
    # project's Dockerfile, then remove it.
    python3 -m pip install --no-cache-dir uv \
    && python3 -m uv pip install --system /app \
    && python3 -m pip uninstall -y uv \
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


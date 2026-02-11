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
    && \
    apk add --no-cache --virtual=.build-deps \
    build-base \
    git \
    libffi-dev \
    openssl-dev \
    musl-dev \
    && \
    git clone --single-branch --branch "${VERSION}" https://github.com/mpforce1/Twitch-Channel-Points-Miner.git . \
    && \
    pip install --upgrade uv && \
    uv pip install --system . \
    && chown -R root:root /app && chmod -R 755 /app \
    && apk del --purge .build-deps \
    && rm -rf /root/.cache /root/.cargo /tmp/*

COPY . /
RUN chmod +x /entrypoint.sh || true

USER nobody:nogroup
WORKDIR /config
VOLUME ["/config"]

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]


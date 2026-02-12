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
    libffi-dev \
    openssl-dev \
    musl-dev \
    && curl -fsSL -o /tmp/app.zip "https://github.com/mpforce1/Twitch-Channel-Points-Miner/archive/refs/tags/${VERSION}.tar.gz" \
    | tar xzf - -C /app --strip-components=1 \
    && pip install uv \
    && uv pip install \
    && chown -R root:root /app && chmod -R 755 /app \
    && pip uninstall --yes uv && apk del --purge .build-deps \
    && rm -rf /root/.cache /root/.cargo /tmp/* /app/bin/bin

COPY . /
RUN chmod 0555 /entrypoint.sh || true

USER nobody:nogroup
WORKDIR /config
VOLUME ["/config"]

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]


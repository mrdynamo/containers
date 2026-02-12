# syntax=docker/dockerfile:1

ARG VERSION

FROM docker.io/library/python:3.13-alpine3.23 AS builder
ARG VERSION

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_ROOT_USER_ACTION=ignore \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_BREAK_SYSTEM_PACKAGES=1 \
    CRYPTOGRAPHY_DONT_BUILD_RUST=1 \
    UV_NO_CACHE=true \
    UV_SYSTEM_PYTHON=true \
    UV_LINK_MODE=copy \
    UV_NO_DEV=1 \
    UV_PYTHON_DOWNLOADS=0

WORKDIR /app

# installer for uv
ADD https://astral.sh/uv/install.sh /uv-installer.sh
RUN sh /uv-installer.sh && rm /uv-installer.sh
ENV PATH="/root/.local/bin/:$PATH"

# build deps required by some dependencies (pandas, numpy, etc.)
RUN apk add --no-cache --virtual=.build-deps \
    build-base \
    libffi-dev \
    openssl-dev \
    musl-dev \
    python3-dev \
    && apk add --no-cache curl ca-certificates jq tzdata coreutils bash nano catatonit

# fetch the upstream release and extract into /app
RUN curl -fsSL "https://github.com/mpforce1/Twitch-Channel-Points-Miner/archive/refs/tags/${VERSION}.tar.gz" \
    | tar xzf - -C /app --strip-components=1

# Use uv to sync dependencies and install the project into a local .venv
RUN --mount=type=cache,target=/root/.cache/uv \
    --mount=type=cache,target=/root/.cache/pip \
    uv sync --locked --no-install-project || true \
    && uv sync --locked

# Ensure files are readable by final user, leave .venv in /app
RUN chown -R 1000:1000 /app && chmod -R 755 /app \
    && apk del --purge .build-deps \
    && rm -rf /tmp/*

FROM docker.io/library/python:3.13-alpine3.23 AS runtime
WORKDIR /usr/src/app

# runtime deps
RUN apk add --no-cache ca-certificates catatonit coreutils

# copy application and virtualenv from builder
COPY --from=builder /app /usr/src/app

# expose venv on PATH
ENV PATH="/usr/src/app/.venv/bin:$PATH"

# create non-root user
RUN addgroup -S nonroot && adduser -S -G nonroot nonroot
RUN chown -R nonroot:nonroot /usr/src/app

COPY entrypoint.sh /entrypoint.sh
RUN chmod 0555 /entrypoint.sh || true

USER nonroot
WORKDIR /config
VOLUME ["/config"]

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]


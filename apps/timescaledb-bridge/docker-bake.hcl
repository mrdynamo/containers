target "docker-metadata-action" {}

variable "APP" {
  default = "timescaledb-bridge"
}

# VERSION     = the TimescaleDB release this image ships (upgrade TO).
# OLD_VERSION = the exact extversion currently installed in the database
#               (upgrade FROM). Verify with:
#   SELECT extversion FROM pg_extension WHERE extname = 'timescaledb';
# The old loader is rebuilt from source (see Dockerfile build-old stage) with
# version.config patched to this exact string, so OLD_VERSION must match the
# catalog extversion, not necessarily the raw upstream git tag.
#
# Renovate only proposes VERSION bumps after approval on the Dependency
# Dashboard and never automerges them: OLD_VERSION must be correct for the
# database first, and ALTER EXTENSION has to be run after rollout. After the
# ALTER, set OLD_VERSION = VERSION.

variable "VERSION" {
  // renovate: datasource=github-releases depName=timescale/timescaledb
  default = "2.32.0"
}

variable "OLD_VERSION" {
  default = "2.29.2"
}

# Build against the exact CNPG runtime image used by the cluster, to avoid
# backend symbol/export mismatches. The matching postgresql-server-dev
# version is derived from this image at build time.
variable "RUNTIME_IMAGE" {
  // renovate: datasource=docker depName=ghcr.io/cloudnative-pg/postgresql
  default = "ghcr.io/cloudnative-pg/postgresql:18.6-standard-trixie@sha256:771f9eab5225587af259d84680cc69a5b6e374ac32bb24effafde1f3368a1241"
}

variable "PG_MAJOR" {
  default = "18"
}

variable "SOURCE" {
  default = "https://github.com/timescale/timescaledb"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  inherits = ["docker-metadata-action"]
  args = {
    VERSION       = "${VERSION}"
    OLD_VERSION   = "${OLD_VERSION}"
    PG_MAJOR      = "${PG_MAJOR}"
    RUNTIME_IMAGE = "${RUNTIME_IMAGE}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
  }
}

target "image-local" {
  inherits = ["image"]
  output   = ["type=docker"]
  tags     = ["${APP}:${VERSION}-from-${OLD_VERSION}"]
}

target "image-all" {
  inherits = ["image"]
  # Bridge is currently constrained to amd64 for this migration path.
  # Expand to arm64 after verifying runtime + old-image compatibility for that architecture.
  platforms = [
    "linux/amd64"
  ]
}

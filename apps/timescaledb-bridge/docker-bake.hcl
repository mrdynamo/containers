target "docker-metadata-action" {}

variable "APP" {
  default = "timescaledb-bridge"
}

# VERSION  = the TimescaleDB release you are upgrading TO.
# OLD_VERSION = the exact extversion currently installed in the database (upgrade FROM).
# Neither variable carries a Renovate annotation: this is a one-time migration
# image and should be removed from the repo once the upgrade is complete.
# Verify with: SELECT extversion FROM pg_extension WHERE extname = 'timescaledb';
# NOTE: upstream's version.config for a tag doesn't always match the tag name
# itself (e.g. tag 2.27.0 embeds version "2.27.0-dev"), so OLD_VERSION must be
# the catalog extversion, not the git tag.

variable "VERSION" {
  default = "2.29.2"
}

variable "OLD_VERSION" {
  default = "2.27.0-dev"
}

# Build target extension files against the exact CNPG runtime image used by
# the cluster to avoid backend symbol/export mismatches.
variable "RUNTIME_IMAGE" {
  default = "ghcr.io/cloudnative-pg/postgresql:18.6-standard-trixie@sha256:771f9eab5225587af259d84680cc69a5b6e374ac32bb24effafde1f3368a1241"
}

variable "PG_MAJOR" {
  default = "18"
}

# Pin the server-dev headers to the exact minor version of the CNPG runtime
# postgres binary to avoid symbol mismatches at extension load time.
variable "PG_DEV_PKG_VERSION" {
  default = "18.6-1.pgdg13+2"
}

# OLD_IMAGE is pinned to the exact old extension image currently in use.
variable "OLD_IMAGE" {
  default = "ghcr.io/mrdynamo/timescaledb-bridge:2.27.0@sha256:918528675538a8b3a73d5c2dd4e4486796f6e1859fcc60946ccd3cca369cf93e"
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
    VERSION            = "${VERSION}"
    OLD_VERSION        = "${OLD_VERSION}"
    PG_MAJOR           = "${PG_MAJOR}"
    PG_DEV_PKG_VERSION = "${PG_DEV_PKG_VERSION}"
    RUNTIME_IMAGE      = "${RUNTIME_IMAGE}"
    OLD_IMAGE          = "${OLD_IMAGE}"
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

target "docker-metadata-action" {}

variable "APP" {
  default = "timescaledb-bridge"
}

# VERSION  = the TimescaleDB release you are upgrading TO.
# OLD_VERSION = the release currently installed in the database (upgrade FROM).
# Neither variable carries a Renovate annotation: this is a one-time migration
# image and should be removed from the repo once the upgrade is complete.

variable "VERSION" {
  default = "2.27.0"
}

variable "OLD_VERSION" {
  default = "2.23.1"
}

# Build target extension files against the exact CNPG runtime image used by
# the cluster to avoid backend symbol/export mismatches.
variable "RUNTIME_IMAGE" {
  default = "ghcr.io/cloudnative-pg/postgresql:18.3-standard-trixie@sha256:81bad466fe006454482678c6c67a24c1e30454b519011e0196cba2e4c83e1e1a"
}

variable "PG_MAJOR" {
  default = "18"
}

# OLD_IMAGE is pinned to the exact old extension image currently in use.
variable "OLD_IMAGE" {
  default = "ghcr.io/shusaan/timescaledb-testing:2.23.1-18-trixie@sha256:4709935bed4cce5977bc4b1858a94e2e80f22f0303217163a9e97b8898e33ee2"
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
    VERSION     = "${VERSION}"
    OLD_VERSION = "${OLD_VERSION}"
    PG_MAJOR    = "${PG_MAJOR}"
    RUNTIME_IMAGE = "${RUNTIME_IMAGE}"
    OLD_IMAGE   = "${OLD_IMAGE}"
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

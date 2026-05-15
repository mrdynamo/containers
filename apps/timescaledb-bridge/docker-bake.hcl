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

# NEW_IMAGE is pinned to the exact target extension image for cutover.
variable "NEW_IMAGE" {
  default = "ghcr.io/m00nwtchr/timescaledb:2.27.0@sha256:fde617cd1b90993118728ef4561e4ebf98d5e6ef6111cb9f80e6ebbf4b5f0ca1"
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
    NEW_IMAGE   = "${NEW_IMAGE}"
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
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}

target "docker-metadata-action" {}

variable "APP" {
  default = "timescaledb-toolkit"
}

variable "VERSION" {
  // renovate: datasource=github-releases depName=timescale/timescaledb-toolkit
  default = "1.22.0"
}

variable "PG_MAJOR" {
  default = "18"
}

# Pin the builder base image and dev-headers package to the exact PG minor version
# used by the CNPG runtime to avoid symbol mismatches at extension load time.
variable "PG_IMAGE" {
  default = "docker.io/library/postgres:18.4-trixie@sha256:f7ce845ee6873dd84be93c9828fe0d1fab0f9707dc9ac569694657398b290bce"
}

variable "PG_DEV_PKG_VERSION" {
  default = "18.4-1.pgdg13+1"
}

variable "SOURCE" {
  default = "https://github.com/timescale/timescaledb-toolkit"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  inherits = ["docker-metadata-action"]
  args = {
    VERSION            = "${VERSION}"
    PG_MAJOR           = "${PG_MAJOR}"
    PG_IMAGE           = "${PG_IMAGE}"
    PG_DEV_PKG_VERSION = "${PG_DEV_PKG_VERSION}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${VERSION}"]
}

target "image-all" {
  inherits = ["image"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}

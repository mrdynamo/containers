target "docker-metadata-action" {}

variable "APP" {
  default = "timescaledb-toolkit"
}

variable "VERSION" {
  // renovate: datasource=github-releases depName=timescale/timescaledb-toolkit
  default = "1.25.0"
}

variable "PG_MAJOR" {
  default = "18"
}

# Pin the builder base image and dev-headers package to the exact PG minor version
# used by the CNPG runtime to avoid symbol mismatches at extension load time.
variable "PG_IMAGE" {
  default = "docker.io/library/postgres:18.6-trixie@sha256:06cad38a5d9f5d24b4d83d86def30795d5e4b757fedbf5281172b576dedcd941"
}

variable "PG_DEV_PKG_VERSION" {
  default = "18.6-1.pgdg13+2"
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

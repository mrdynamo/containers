target "docker-metadata-action" {}

variable "APP" {
  default = "timescaledb-toolkit"
}

variable "VERSION" {
  // renovate: datasource=github-releases depName=timescale/timescaledb-toolkit
  default = "1.22.0"
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
    VERSION = "${VERSION}"
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

target "docker-metadata-action" {}

variable "APP" {
  default = "twitch-channel-points-miner"
}

variable "VERSION" {
  // renovate: datasource=github-releases depName=mpforce1/Twitch-Channel-Points-Miner versioning=loose
  default = "1.0.0"
}

variable "SOURCE" {
  default = "https://github.com/mpforce1/Twitch-Channel-Points-Miner"
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


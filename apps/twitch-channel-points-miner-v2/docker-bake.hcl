target "docker-metadata-action" {}

variable "APP" {
  default = "twitch-channel-points-miner-v2"
}

variable "VERSION" {
  default = "main"
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
  tags = ["${APP}:rolling"]
}

target "image-all" {
  inherits = ["image"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}


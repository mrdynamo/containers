target "docker-metadata-action" {}

variable "APP" {
  default = "timescaledb-toolkit"
}

# Upstream re-tags fixed releases as X.Y.Z+pN (e.g. 1.27.0 shipped with a
# "-dev" crate version; 1.27.0+p2 is the corrected tag). The image itself is
# tagged with the plain X.Y.Z.
variable "VERSION" {
  // renovate: datasource=github-releases depName=timescale/timescaledb-toolkit
  default = "1.25.0"
}

# OLD_VERSION = the toolkit version production runs. Its shared library is
# bundled: installed toolkit functions reference
# $libdir/timescaledb_toolkit-<installed version>, so without it they error
# from rollout until ALTER EXTENSION timescaledb_toolkit UPDATE. The
# sync-production-versions workflow keeps it in sync with home-ops main (PR);
# merge that PR only after the ALTER.
# TestUpgradeFromProduction fails if it is not production's.
variable "OLD_VERSION" {
  default = "1.25.0"
}

variable "PG_MAJOR" {
  default = "18"
}

# Build against the exact CNPG runtime image used by the cluster, to avoid
# symbol mismatches at extension load time. The matching postgresql-server-dev
# version is derived from this image at build time.
variable "RUNTIME_IMAGE" {
  // renovate: datasource=docker depName=ghcr.io/cloudnative-pg/postgresql
  default = "ghcr.io/cloudnative-pg/postgresql:18.6-standard-trixie@sha256:771f9eab5225587af259d84680cc69a5b6e374ac32bb24effafde1f3368a1241"
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
    VERSION       = "${VERSION}"
    OLD_VERSION   = "${OLD_VERSION}"
    PG_MAJOR      = "${PG_MAJOR}"
    RUNTIME_IMAGE = "${RUNTIME_IMAGE}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
    # Read by the upgrade-rehearsal tests: an image is rehearsed on the
    # runtime it was built for (the runtime production will run with it).
    "io.github.mrdynamo.runtime-image" = "${RUNTIME_IMAGE}"
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

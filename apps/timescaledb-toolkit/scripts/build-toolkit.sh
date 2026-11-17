#!/usr/bin/env bash
# Build one timescaledb_toolkit release against the PostgreSQL installed in the
# image (the CNPG runtime) and stage its files.
#
#   build-toolkit.sh <source-tag> <out-dir> [full|lib]
#
#   full  the whole extension: shared library, control file, install and
#         update SQL scripts
#   lib   only timescaledb_toolkit-<version>.so (the library of the version
#         production runs, bundled next to the new version)
#
# Requires PG_MAJOR in the environment.
set -euo pipefail

tag=$1
out=$2
mode=${3:-full}

# Upstream re-tags fixed releases as X.Y.Z+pN; the extension version is X.Y.Z.
version=${tag%%+*}
pg_config="/usr/lib/postgresql/${PG_MAJOR}/bin/pg_config"
src="/build/${version}"

mkdir -p "${src}" "${out}/lib" "${out}/share/extension"
curl -fsSL "https://github.com/timescale/timescaledb-toolkit/archive/refs/tags/${tag}.tar.gz" \
    | tar xzf - -C "${src}" --strip-components=1

# Some upstream tags still carry a "-dev" crate version (1.27.0 did; 1.27.0+p2
# fixed it). The library name and the extension version come from it, so force
# the clean version: the same fix the TimescaleDB image applies to
# version.config.
sed -i "0,/^version = \".*\"/s//version = \"${version}\"/" "${src}/extension/Cargo.toml"
grep -q "^version = \"${version}\"" "${src}/extension/Cargo.toml"

# cargo-pgrx at the exact version pinned in extension/Cargo.toml (upstream:
# "Keep synchronized with cargo install --version N.N.N cargo-pgrx"), in its
# own root so two toolkit versions with different pins never clash.
pgrx_version=$(grep '^pgrx = "=' "${src}/extension/Cargo.toml" | head -1 | sed 's/pgrx = "=//;s/"//')
pgrx_root="/opt/cargo-pgrx-${pgrx_version}"
if [[ ! -x "${pgrx_root}/bin/cargo-pgrx" ]]; then
    cargo install --root "${pgrx_root}" --version "=${pgrx_version}" --force cargo-pgrx
fi
export PATH="${pgrx_root}/bin:${PATH}"

cd "${src}/extension"
# Point pgrx at the installed PostgreSQL; no download needed.
cargo pgrx init "--pg${PG_MAJOR}" "${pg_config}"
cargo pgrx install --release --pg-config "${pg_config}"
# Upstream post-install: renames timescaledb_toolkit.so to
# timescaledb_toolkit-<version>.so and finalises the SQL scripts.
cargo run --manifest-path ../tools/post-install/Cargo.toml -- "${pg_config}"

libdir=$("${pg_config}" --pkglibdir)
sharedir="$("${pg_config}" --sharedir)/extension"
case "${mode}" in
    full)
        find "${libdir}" -name "timescaledb_toolkit*.so" -exec cp {} "${out}/lib/" \;
        find "${sharedir}" -name "timescaledb_toolkit*" -exec cp {} "${out}/share/extension/" \;
        test -f "${out}/share/extension/timescaledb_toolkit.control"
        ;;
    lib)
        cp "${libdir}/timescaledb_toolkit-${version}.so" "${out}/lib/"
        ;;
    *)
        echo "unknown mode ${mode}" >&2
        exit 1
        ;;
esac
test -f "${out}/lib/timescaledb_toolkit-${version}.so"
echo "Built timescaledb_toolkit ${version} (${mode}) from tag ${tag}"

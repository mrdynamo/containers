package main

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/home-operations/containers/testhelpers"
	"github.com/stretchr/testify/require"
)

const extName = "timescaledb_toolkit"

// cleanVersion: upstream tags fixed releases as X.Y.Z+pN; the extension
// version is X.Y.Z.
func cleanVersion(t *testing.T) string {
	v, _, _ := strings.Cut(testhelpers.BakeVariable(t, "VERSION"), "+")
	return v
}

// TestFreshInstall: the image under test installs and works on the runtime
// it was built for, next to production's TimescaleDB (preloaded), as in
// production.
func TestFreshInstall(t *testing.T) {
	ctx := context.Background()
	version := cleanVersion(t)
	prod := testhelpers.Production(t)

	pg := testhelpers.NewPostgres(t, ctx, testhelpers.BakeVariable(t, "PG_MAJOR"), prod.Settings...)
	pg.Start(testhelpers.Phase{
		RuntimeImage: testhelpers.BakeVariable(t, "RUNTIME_IMAGE"),
		Extensions:   prod.With(extName, testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-toolkit:rolling")),
	})

	for name := range prod.Extensions {
		pg.Psql("CREATE EXTENSION " + name)
	}
	// Catches upstream tags whose crate version still says "-dev", which
	// leaves no ALTER EXTENSION ... UPDATE path to or from this release.
	require.Equal(t, version, extVersion(pg))
	pg.RequireFile("/extensions/timescaledb_toolkit/lib/timescaledb_toolkit-" + version + ".so")
	verifyFunctions(t, pg)

	// The bundled library of the version production runs, and the update
	// script from it.
	oldVersion := testhelpers.BakeVariable(t, "OLD_VERSION")
	pg.RequireFile("/extensions/timescaledb_toolkit/lib/timescaledb_toolkit-" + oldVersion + ".so")
	if oldVersion != version {
		pg.RequireFile("/extensions/timescaledb_toolkit/share/extension/timescaledb_toolkit--" + oldVersion + "--" + version + ".sql")
	}
}

// TestUpgradeFromProduction rehearses deploying the image under test:
// production exactly (from mrdynamo/home-ops main) -> CNPG restart with the
// image under test on its runtime, same data: toolkit functions must keep
// working on the old version (its library is bundled) -> ALTER EXTENSION
// timescaledb_toolkit UPDATE in a new session -> functions work on the new
// version.
func TestUpgradeFromProduction(t *testing.T) {
	ctx := context.Background()
	version := cleanVersion(t)
	image := testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-toolkit:rolling")
	prod := testhelpers.Production(t)
	require.Contains(t, prod.Extensions, extName, "production has no %s extension", extName)

	pg := testhelpers.NewPostgres(t, ctx, testhelpers.BakeVariable(t, "PG_MAJOR"), prod.Settings...)
	target := pg.TargetRuntime(image, testhelpers.BakeVariable(t, "RUNTIME_IMAGE"))
	t.Logf("from (production): %s", prod)
	t.Logf("to: %s=%s on %s", extName, image, target)

	pg.Start(testhelpers.Phase{RuntimeImage: prod.RuntimeImage, Extensions: prod.Extensions})
	for name := range prod.Extensions {
		pg.Psql("CREATE EXTENSION " + name)
	}
	installed := extVersion(pg)
	oldVersion := testhelpers.BakeVariable(t, "OLD_VERSION")
	require.Equal(t, oldVersion, installed,
		"production runs timescaledb_toolkit %s but OLD_VERSION is %s; OLD_VERSION must be the version running in production", installed, oldVersion)
	verifyFunctions(t, pg)

	pg.Start(testhelpers.Phase{RuntimeImage: target, Extensions: prod.With(extName, image)})
	// Before ALTER: the database is still on the old toolkit version, and its
	// functions (which reference timescaledb_toolkit-<old>.so) must keep
	// working: that library is bundled in the new image.
	require.Equal(t, installed, extVersion(pg))
	verifyFunctions(t, pg)

	pg.Psql("ALTER EXTENSION timescaledb_toolkit UPDATE")

	require.Equal(t, version, extVersion(pg))
	verifyFunctions(t, pg)
}

func extVersion(pg *testhelpers.Postgres) string {
	return pg.Psql("SELECT extversion FROM pg_extension WHERE extname = 'timescaledb_toolkit'")
}

// verifyFunctions exercises the shared library.
func verifyFunctions(t *testing.T, pg *testhelpers.Postgres) {
	t.Helper()

	require.Equal(t, "t", pg.Psql(`
		SELECT approx_percentile(0.5, percentile_agg(x::double precision)) BETWEEN 400 AND 600
		FROM generate_series(1, 1000) x`))
	require.Equal(t, "500.5", pg.Psql(`
		SELECT average(stats_agg(x::double precision)) FROM generate_series(1, 1000) x`))
	// HyperLogLog is an estimate: 4096 buckets keeps the standard error around
	// 1.6% (1.04/sqrt(buckets)), well inside the +/-10% window. (64 buckets is
	// ~13% and misses it.)
	hll := pg.Psql(`SELECT distinct_count(hyperloglog(4096, x)) FROM generate_series(1, 1000) x`)
	n, err := strconv.Atoi(hll)
	require.NoError(t, err, "distinct_count returned %q", hll)
	require.InDelta(t, 1000, n, 100, "hyperloglog estimate for 1000 distinct values was %d", n)
}

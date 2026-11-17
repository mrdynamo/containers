package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/home-operations/containers/testhelpers"
	"github.com/stretchr/testify/require"
)

const extName = "timescaledb_toolkit"

// Production runs the toolkit next to TimescaleDB (preloaded); so do the tests.
var settings = []string{
	"shared_preload_libraries=timescaledb",
	"timescaledb.telemetry_level=off",
}

func timescaledbImage() string {
	if image := os.Getenv("TIMESCALEDB_IMAGE"); image != "" {
		return image
	}
	return "ghcr.io/mrdynamo/timescaledb-bridge:rolling"
}

// cleanVersion: upstream tags fixed releases as X.Y.Z+pN; the extension
// version is X.Y.Z.
func cleanVersion(t *testing.T) string {
	v, _, _ := strings.Cut(testhelpers.BakeVariable(t, "VERSION"), "+")
	return v
}

// TestFreshInstall: the image under test installs and works on the runtime
// it was built for.
func TestFreshInstall(t *testing.T) {
	ctx := context.Background()
	version := cleanVersion(t)

	pg := testhelpers.NewPostgres(t, ctx, testhelpers.BakeVariable(t, "PG_MAJOR"), settings...)
	pg.Start(testhelpers.Phase{
		RuntimeImage: testhelpers.BakeVariable(t, "RUNTIME_IMAGE"),
		Extensions: map[string]string{
			"timescaledb": timescaledbImage(),
			extName:       testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-toolkit:rolling"),
		},
	})

	pg.Psql("CREATE EXTENSION timescaledb")
	pg.Psql("CREATE EXTENSION timescaledb_toolkit")
	// Catches upstream tags whose crate version still says "-dev", which
	// leaves no ALTER EXTENSION ... UPDATE path to or from this release.
	require.Equal(t, version, extVersion(pg))
	pg.RequireFile("/extensions/timescaledb_toolkit/lib/timescaledb_toolkit-" + version + ".so")
	verifyFunctions(t, pg)
}

// TestUpgradeFromReleased rehearses the production rollout: released image on
// its runtime -> CNPG restart with the new runtime + image under test on the
// same data -> ALTER EXTENSION timescaledb_toolkit UPDATE in a new session.
func TestUpgradeFromReleased(t *testing.T) {
	ctx := context.Background()
	version := cleanVersion(t)
	runtime := testhelpers.BakeVariable(t, "RUNTIME_IMAGE")
	previous := testhelpers.PreviousImage("ghcr.io/mrdynamo/timescaledb-toolkit:rolling")
	image := testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-toolkit:rolling")

	pg := testhelpers.NewPostgres(t, ctx, testhelpers.BakeVariable(t, "PG_MAJOR"), settings...)
	previousRuntime := pg.ImageRuntime(previous, runtime)
	t.Logf("from: %s on %s", previous, previousRuntime)
	t.Logf("to:   %s on %s", image, runtime)

	pg.Start(testhelpers.Phase{
		RuntimeImage: previousRuntime,
		Extensions:   map[string]string{"timescaledb": timescaledbImage(), extName: previous},
	})
	pg.Psql("CREATE EXTENSION timescaledb")
	pg.Psql("CREATE EXTENSION timescaledb_toolkit")
	installed := extVersion(pg)
	verifyFunctions(t, pg)

	pg.Start(testhelpers.Phase{
		RuntimeImage: runtime,
		Extensions:   map[string]string{"timescaledb": timescaledbImage(), extName: image},
	})
	// Before ALTER: the database is still on the old toolkit version and
	// sessions must work (toolkit functions themselves point at the old .so
	// until the update runs).
	require.Equal(t, installed, extVersion(pg))
	require.Equal(t, "1", pg.Psql("SELECT 1"))

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

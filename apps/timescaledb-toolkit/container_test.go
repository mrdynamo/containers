package main

import (
	"context"
	"strings"
	"testing"

	"github.com/home-operations/containers/testhelpers"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	ctx := context.Background()

	// Upstream tags fixed releases as X.Y.Z+pN; the extension version is X.Y.Z.
	version, _, _ := strings.Cut(testhelpers.BakeVariable(t, "VERSION"), "+")
	image := testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-toolkit:rolling")

	c := testhelpers.StartPostgresWithExtension(t, ctx, testhelpers.PostgresExtensionConfig{
		ExtensionImage: image,
		RuntimeImage:   testhelpers.BakeVariable(t, "RUNTIME_IMAGE"),
		PGMajor:        testhelpers.BakeVariable(t, "PG_MAJOR"),
		Name:           "timescaledb_toolkit",
	})

	testhelpers.Psql(t, ctx, c, "CREATE EXTENSION timescaledb_toolkit")

	// Catches upstream tags whose crate version still says "-dev", which
	// leaves no ALTER EXTENSION ... UPDATE path to or from this release.
	require.Equal(t, version,
		testhelpers.Psql(t, ctx, c, "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb_toolkit'"))
	testhelpers.RequireFile(t, ctx, c, "/extensions/timescaledb_toolkit/lib/timescaledb_toolkit-"+version+".so")

	// Exercise the shared library: approximate percentile and stats aggregates.
	require.Equal(t, "t", testhelpers.Psql(t, ctx, c, `
		SELECT approx_percentile(0.5, percentile_agg(x::double precision)) BETWEEN 400 AND 600
		FROM generate_series(1, 1000) x`))
	require.Equal(t, "500.5", testhelpers.Psql(t, ctx, c, `
		SELECT average(stats_agg(x::double precision)) FROM generate_series(1, 1000) x`))
}

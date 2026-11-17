package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/home-operations/containers/testhelpers"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	ctx := context.Background()

	version := testhelpers.BakeVariable(t, "VERSION")
	oldVersion := testhelpers.BakeVariable(t, "OLD_VERSION")
	image := testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-bridge:rolling")

	c := testhelpers.StartPostgresWithExtension(t, ctx, testhelpers.PostgresExtensionConfig{
		ExtensionImage: image,
		RuntimeImage:   testhelpers.BakeVariable(t, "RUNTIME_IMAGE"),
		PGMajor:        testhelpers.BakeVariable(t, "PG_MAJOR"),
		Name:           "timescaledb",
		Settings: []string{
			"shared_preload_libraries=timescaledb",
			"timescaledb.telemetry_level=off",
		},
	})

	// The bridge part: the installed (old) versioned loader must be present,
	// plus an update script from it when it differs from VERSION.
	testhelpers.RequireFile(t, ctx, c, fmt.Sprintf("/extensions/timescaledb/lib/timescaledb-%s.so", oldVersion))
	if oldVersion != version {
		testhelpers.RequireFile(t, ctx, c,
			fmt.Sprintf("/extensions/timescaledb/share/extension/timescaledb--%s--%s.sql", oldVersion, version))
	}

	testhelpers.Psql(t, ctx, c, "CREATE EXTENSION timescaledb")
	require.Equal(t, version,
		testhelpers.Psql(t, ctx, c, "SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'"))

	testhelpers.Psql(t, ctx, c, `
		CREATE TABLE metrics (time timestamptz NOT NULL, value double precision);
		SELECT create_hypertable('metrics', by_range('time', INTERVAL '1 day'));
		INSERT INTO metrics
			SELECT ts, random() FROM generate_series(now() - INTERVAL '3 days', now(), INTERVAL '1 minute') ts;
		ALTER TABLE metrics SET (timescaledb.compress);
	`)

	// Run a policy through the background scheduler, not run_job(): that is
	// the path that breaks when the extension and server minor disagree
	// (e.g. TimescaleDB < 2.32.1 on PostgreSQL 18.7: "cannot drop portal").
	jobID := testhelpers.Psql(t, ctx, c, `
		SELECT add_compression_policy('metrics', compress_after => INTERVAL '1 hour',
			schedule_interval => INTERVAL '5 seconds', initial_start => now())`)

	deadline := time.Now().Add(90 * time.Second)
	for {
		stats := testhelpers.Psql(t, ctx, c, fmt.Sprintf(`
			SELECT coalesce(total_successes, 0) || '|' || coalesce(total_failures, 0)
			FROM timescaledb_information.job_stats WHERE job_id = %s`, jobID))
		if stats != "" {
			successes, failures, _ := strings.Cut(stats, "|")
			if failures != "0" {
				errs := testhelpers.Psql(t, ctx, c, fmt.Sprintf(
					"SELECT string_agg(err_message, E'\\n') FROM timescaledb_information.job_errors WHERE job_id = %s", jobID))
				t.Fatalf("background job %s failed: %s", jobID, errs)
			}
			if successes != "0" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("background job %s did not run within 90s (stats: %q)", jobID, stats)
		}
		time.Sleep(2 * time.Second)
	}

	compressed := testhelpers.Psql(t, ctx, c,
		"SELECT count(*) FROM chunk_compression_stats('metrics') WHERE compression_status = 'Compressed'")
	require.NotEqual(t, "0", compressed, "expected the policy to compress at least one chunk")
	require.Equal(t, "4321", testhelpers.Psql(t, ctx, c, "SELECT count(*) FROM metrics"))
}

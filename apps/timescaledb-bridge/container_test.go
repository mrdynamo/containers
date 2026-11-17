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

const extName = "timescaledb"

var settings = []string{
	"shared_preload_libraries=timescaledb",
	"timescaledb.telemetry_level=off",
}

// TestFreshInstall: the image under test, on the runtime it was built for,
// installs and runs TimescaleDB, including jobs run by the background
// scheduler.
func TestFreshInstall(t *testing.T) {
	ctx := context.Background()
	version := testhelpers.BakeVariable(t, "VERSION")
	oldVersion := testhelpers.BakeVariable(t, "OLD_VERSION")

	pg := testhelpers.NewPostgres(t, ctx, testhelpers.BakeVariable(t, "PG_MAJOR"), settings...)
	pg.Start(testhelpers.Phase{
		RuntimeImage: testhelpers.BakeVariable(t, "RUNTIME_IMAGE"),
		Extensions:   map[string]string{extName: testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-bridge:rolling")},
	})

	// The bridge: the installed (old) versioned libraries must be present,
	// plus a direct update script from OLD_VERSION when it differs.
	pg.RequireFile(fmt.Sprintf("/extensions/timescaledb/lib/timescaledb-%s.so", oldVersion))
	pg.RequireFile(fmt.Sprintf("/extensions/timescaledb/lib/timescaledb-tsl-%s.so", oldVersion))
	if oldVersion != version {
		pg.RequireFile(fmt.Sprintf("/extensions/timescaledb/share/extension/timescaledb--%s--%s.sql", oldVersion, version))
	}

	pg.Psql("CREATE EXTENSION timescaledb")
	require.Equal(t, version, extVersion(pg))

	jobs := createWorkload(t, pg)
	waitForJobs(t, pg, jobs, dbNow(pg))
	verifyData(t, pg)
}

// TestUpgradeFromProduction rehearses deploying the image under test:
//  1. production exactly, as deployed from mrdynamo/home-ops main (runtime
//     image, every extension image, shared_preload_libraries and parameters),
//     with the extensions installed and jobs running;
//  2. CNPG restarts the instance with the image under test (and the runtime
//     it was built for) on the same data directory — before ALTER EXTENSION,
//     connections and jobs must keep working on the old extension version
//     (this is where a missing versioned loader makes every connection FATAL);
//  3. ALTER EXTENSION timescaledb UPDATE as the first command of a new
//     session (psql -X), exactly as in the runbook;
//  4. extension at VERSION, all jobs succeed via the scheduler, data intact.
func TestUpgradeFromProduction(t *testing.T) {
	ctx := context.Background()
	version := testhelpers.BakeVariable(t, "VERSION")
	oldVersion := testhelpers.BakeVariable(t, "OLD_VERSION")
	image := testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-bridge:rolling")
	prod := testhelpers.Production(t)
	require.Contains(t, prod.Extensions, extName, "production has no %s extension", extName)

	pg := testhelpers.NewPostgres(t, ctx, testhelpers.BakeVariable(t, "PG_MAJOR"), prod.Settings...)
	target := pg.TargetRuntime(image, testhelpers.BakeVariable(t, "RUNTIME_IMAGE"))
	t.Logf("from (production): %s", prod)
	t.Logf("to: %s=%s on %s", extName, image, target)

	// 1. Production state.
	pg.Start(testhelpers.Phase{RuntimeImage: prod.RuntimeImage, Extensions: prod.Extensions})
	for name := range prod.Extensions {
		pg.Psql("CREATE EXTENSION " + name)
	}
	installed := extVersion(pg)
	require.Equal(t, oldVersion, installed,
		"production runs timescaledb %s but OLD_VERSION is %s; OLD_VERSION must be the version running in production", installed, oldVersion)
	jobs := createWorkload(t, pg)
	waitForJobs(t, pg, jobs, dbNow(pg))
	pauseJobs(t, pg, jobs)

	// 2. Rolling update: the image under test on its runtime, same data.
	pg.Start(testhelpers.Phase{RuntimeImage: target, Extensions: prod.With(extName, image)})
	require.Equal(t, oldVersion, extVersion(pg), "extension must still be at the old version before ALTER")
	verifyData(t, pg)
	since := dbNow(pg)
	resumeJobs(pg, jobs)
	waitForJobs(t, pg, jobs, since)
	pauseJobs(t, pg, jobs)

	// 3. The runbook step.
	pg.Psql("ALTER EXTENSION timescaledb UPDATE")

	// 4. Upgraded state.
	require.Equal(t, version, extVersion(pg))
	verifyData(t, pg)
	since = dbNow(pg)
	resumeJobs(pg, jobs)
	waitForJobs(t, pg, jobs, since)
}

func extVersion(pg *testhelpers.Postgres) string {
	return pg.Psql("SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'")
}

func dbNow(pg *testhelpers.Postgres) string {
	return pg.Psql("SELECT now()")
}

// createWorkload mirrors what Tracearr runs in production: a compressed
// hypertable with a compression policy, a continuous aggregate with a
// refresh policy, and a retention policy. Returns the job ids.
func createWorkload(t *testing.T, pg *testhelpers.Postgres) []string {
	t.Helper()

	pg.Psql(`
		CREATE TABLE metrics (time timestamptz NOT NULL, value double precision);
		SELECT create_hypertable('metrics', by_range('time', INTERVAL '1 day'));
		INSERT INTO metrics
			SELECT ts, random() FROM generate_series(now() - INTERVAL '3 days', now(), INTERVAL '1 minute') ts;
		ALTER TABLE metrics SET (timescaledb.compress);
		CREATE MATERIALIZED VIEW metrics_hourly WITH (timescaledb.continuous) AS
			SELECT time_bucket(INTERVAL '1 hour', time) AS bucket, count(*) AS n, avg(value) AS avg_value
			FROM metrics GROUP BY 1 WITH NO DATA;
	`)
	jobs := strings.Fields(pg.Psql(`
		SELECT add_compression_policy('metrics', compress_after => INTERVAL '1 hour',
			schedule_interval => INTERVAL '5 seconds', initial_start => now());
		SELECT add_continuous_aggregate_policy('metrics_hourly', start_offset => INTERVAL '7 days',
			end_offset => INTERVAL '1 hour', schedule_interval => INTERVAL '5 seconds', initial_start => now());
		SELECT add_retention_policy('metrics', drop_after => INTERVAL '30 days',
			schedule_interval => INTERVAL '5 seconds', initial_start => now());
	`))
	require.Len(t, jobs, 3)
	return jobs
}

// waitForJobs waits until every job has finished successfully after `since`
// (read from the bgw_job_stat catalog: the job_stats view has no crash count),
// run by the background scheduler (not run_job(): the scheduler path is what
// breaks when the extension and server minor disagree, e.g. TimescaleDB
// < 2.32.1 on PostgreSQL 18.7: "cannot drop portal"). Any failure or crash
// fails the test.
func waitForJobs(t *testing.T, pg *testhelpers.Postgres, jobs []string, since string) {
	t.Helper()

	ids := strings.Join(jobs, ",")
	deadline := time.Now().Add(2 * time.Minute)
	for {
		status := pg.Psql(fmt.Sprintf(`
			SELECT count(*) FILTER (WHERE last_successful_finish > '%s')
				|| '|' || coalesce(sum(total_failures), 0) || '|' || coalesce(sum(total_crashes), 0)
			FROM _timescaledb_internal.bgw_job_stat WHERE job_id IN (%s)`, since, ids))
		parts := strings.Split(status, "|")
		require.Len(t, parts, 3, "unexpected job_stats output %q", status)
		if parts[1] != "0" || parts[2] != "0" {
			errs := pg.Psql(fmt.Sprintf(`
				SELECT string_agg(job_id || ': ' || coalesce(err_message, '?'), E'\n')
				FROM timescaledb_information.job_errors WHERE job_id IN (%s)`, ids))
			t.Fatalf("jobs failed (failures=%s crashes=%s):\n%s\n--- postgres.log ---\n%s", parts[1], parts[2], errs, pg.LogTail())
		}
		if parts[0] == fmt.Sprint(len(jobs)) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %s/%d jobs succeeded since %s within 2m\n--- postgres.log ---\n%s", parts[0], len(jobs), since, pg.LogTail())
		}
		time.Sleep(2 * time.Second)
	}
}

// pauseJobs unschedules the jobs and waits until none is running, so the
// test's own fast shutdown never interrupts a job (an interrupted job counts
// as a crash and is held back for 5 minutes).
func pauseJobs(t *testing.T, pg *testhelpers.Postgres, jobs []string) {
	t.Helper()

	for _, id := range jobs {
		pg.Psql(fmt.Sprintf("SELECT alter_job(%s, scheduled => false)", id))
	}
	deadline := time.Now().Add(time.Minute)
	for pg.Psql(fmt.Sprintf(
		"SELECT count(*) FROM timescaledb_information.job_stats WHERE job_status = 'Running' AND job_id IN (%s)",
		strings.Join(jobs, ","))) != "0" {
		if time.Now().After(deadline) {
			t.Fatalf("jobs still running 1m after being unscheduled\n--- postgres.log ---\n%s", pg.LogTail())
		}
		time.Sleep(time.Second)
	}
}

func resumeJobs(pg *testhelpers.Postgres, jobs []string) {
	for _, id := range jobs {
		pg.Psql(fmt.Sprintf("SELECT alter_job(%s, scheduled => true, next_start => now())", id))
	}
}

func verifyData(t *testing.T, pg *testhelpers.Postgres) {
	t.Helper()

	// 3 days of 1-minute points, inclusive of both ends.
	require.Equal(t, "4321", pg.Psql("SELECT count(*) FROM metrics"))
	require.Equal(t, "t", pg.Psql("SELECT count(*) > 0 FROM metrics_hourly"))
}

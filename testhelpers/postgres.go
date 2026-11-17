package testhelpers

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

// BakeVariable returns the default value of a variable declared in the
// docker-bake.hcl of the app under test (go test runs in the app directory).
func BakeVariable(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile("docker-bake.hcl")
	require.NoError(t, err)

	re := regexp.MustCompile(`(?s)variable\s+"` + regexp.QuoteMeta(name) + `"\s*\{.*?default\s*=\s*"([^"]*)"`)
	m := re.FindSubmatch(data)
	require.NotNil(t, m, "variable %q not found in docker-bake.hcl", name)
	return string(m[1])
}

// PostgresExtensionConfig describes how to start a CNPG runtime image with a
// CNPG-style extension image mounted the same way the operator does it.
type PostgresExtensionConfig struct {
	ExtensionImage string   // the FROM scratch extension image under test
	RuntimeImage   string   // the CNPG postgresql image the extension targets
	PGMajor        string   // PostgreSQL major version, e.g. "18"
	Name           string   // CNPG extension name, mounted at /extensions/<Name>
	Settings       []string // extra postgres settings as "key=value"
}

// StartPostgresWithExtension builds an image that layers the extension image
// onto the runtime image at /extensions/<name> (CNPG's layout), initialises a
// throwaway cluster and starts postgres with extension_control_path and
// dynamic_library_path set the way CNPG sets them.
func StartPostgresWithExtension(t *testing.T, ctx context.Context, cfg PostgresExtensionConfig) testcontainers.Container {
	t.Helper()

	dir := t.TempDir()
	dockerfile := fmt.Sprintf("FROM %s AS ext\nFROM %s\nCOPY --from=ext / /extensions/%s/\n",
		cfg.ExtensionImage, cfg.RuntimeImage, cfg.Name)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0o644))

	bin := "/usr/lib/postgresql/" + cfg.PGMajor + "/bin"
	script := bin + "/initdb -D /tmp/pgdata -U postgres --auth=trust >/dev/null && exec " + bin + `/postgres "$@"`

	args := []string{
		"-D", "/tmp/pgdata",
		"-c", "listen_addresses=",
		"-c", "unix_socket_directories=/tmp",
		"-c", "extension_control_path=$system:/extensions/" + cfg.Name + "/share",
		"-c", "dynamic_library_path=$libdir:/extensions/" + cfg.Name + "/lib",
	}
	for _, s := range cfg.Settings {
		args = append(args, "-c", s)
	}

	c, err := testcontainers.Run(ctx, "",
		testcontainers.WithDockerfile(testcontainers.FromDockerfile{Context: dir}),
		testcontainers.WithEntrypoint("sh", "-c", script, "sh"),
		testcontainers.WithCmd(args...),
		testcontainers.WithWaitStrategyAndDeadline(2*time.Minute,
			wait.ForLog("database system is ready to accept connections"),
		),
	)
	testcontainers.CleanupContainer(t, c)
	require.NoError(t, err)
	return c
}

// Exec runs a command in the container and returns its exit code and combined output.
func Exec(t *testing.T, ctx context.Context, c testcontainers.Container, cmd ...string) (int, string) {
	t.Helper()

	code, reader, err := c.Exec(ctx, cmd, tcexec.Multiplexed())
	require.NoError(t, err)
	out, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, strings.TrimSpace(string(out))
}

// Psql runs SQL (stopping at the first error) and returns the unaligned,
// tuples-only output. The test fails if psql exits non-zero.
func Psql(t *testing.T, ctx context.Context, c testcontainers.Container, sql string) string {
	t.Helper()

	code, out := Exec(t, ctx, c, "psql", "-XAtq", "-v", "ON_ERROR_STOP=1",
		"-h", "/tmp", "-U", "postgres", "-d", "postgres", "-c", sql)
	require.Equal(t, 0, code, "psql failed for:\n%s\n%s", sql, out)
	return out
}

// RequireFile fails the test if path is not a regular file in the container.
func RequireFile(t *testing.T, ctx context.Context, c testcontainers.Container, path string) {
	t.Helper()

	code, out := Exec(t, ctx, c, "test", "-f", path)
	require.Equal(t, 0, code, "expected file %s to exist: %s", path, out)
}

package testhelpers

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
)

// RuntimeImageLabel is set on every extension image (see docker-bake.hcl) to
// record the CNPG runtime image it was built against.
const RuntimeImageLabel = "io.github.mrdynamo.runtime-image"

const (
	pgDataRoot = "/pgdata"
	pgDataDir  = pgDataRoot + "/data"
	pgLogFile  = pgDataRoot + "/postgres.log"
	pgConfFile = pgDataDir + "/cnpg-extensions.conf"
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

// PreviousImage returns the released image an upgrade is rehearsed from:
// PREVIOUS_IMAGE if set, otherwise the given default (normally :rolling).
func PreviousImage(defaultImage string) string {
	if image := os.Getenv("PREVIOUS_IMAGE"); image != "" {
		return image
	}
	return defaultImage
}

// Phase is one "pod spec" of the cluster: a CNPG runtime image plus the
// extension images mounted into it, keyed by CNPG extension name.
type Phase struct {
	RuntimeImage string
	Extensions   map[string]string
}

// Postgres reproduces how CNPG runs an instance: the runtime image, each
// extension image mounted read-only at /extensions/<name> (Docker image
// mounts, the same mechanism as Kubernetes image volumes), and
// extension_control_path / dynamic_library_path set the way CNPG sets them.
// The data directory lives on a named volume, so Start can be called again
// with a different Phase to rehearse a rolling update exactly as CNPG does
// it: stop the instance, swap the images, start it on the same data.
type Postgres struct {
	t        *testing.T
	ctx      context.Context
	provider *testcontainers.DockerProvider
	volume   string
	pgMajor  string
	settings []string
	c        testcontainers.Container
	phase    Phase
}

// NewPostgres prepares a Postgres harness. settings are postgres parameters
// ("key=value") applied in every phase, e.g. shared_preload_libraries.
func NewPostgres(t *testing.T, ctx context.Context, pgMajor string, settings ...string) *Postgres {
	t.Helper()

	provider, err := testcontainers.NewDockerProvider()
	require.NoError(t, err)

	// Docker image mounts need Engine API 1.48+ (Docker 28+).
	sv, err := provider.Client().ServerVersion(ctx, client.ServerVersionOptions{})
	require.NoError(t, err)
	require.True(t, apiVersionAtLeast(sv.APIVersion, 1, 48),
		"Docker API %s does not support image mounts (need 1.48+, Docker 28+)", sv.APIVersion)

	p := &Postgres{
		t:        t,
		ctx:      ctx,
		provider: provider,
		volume:   fmt.Sprintf("pg-test-%d", time.Now().UnixNano()),
		pgMajor:  pgMajor,
		settings: settings,
	}
	_, err = provider.Client().VolumeCreate(ctx, client.VolumeCreateOptions{Name: p.volume})
	require.NoError(t, err)

	// Registered first so it runs last, after every container is gone.
	t.Cleanup(func() {
		if p.c != nil {
			_ = p.c.Terminate(context.Background())
		}
		_, _ = provider.Client().VolumeRemove(context.Background(), p.volume, client.VolumeRemoveOptions{Force: true})
		_ = provider.Close()
	})
	return p
}

// Start runs the instance with the given phase, stopping the previous phase's
// instance first. The data directory is initialised on first start.
func (p *Postgres) Start(phase Phase) {
	t := p.t
	t.Helper()

	if p.c != nil {
		p.Stop()
	}

	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithEntrypoint("sleep", "infinity"),
		testcontainers.WithMounts(testcontainers.VolumeMount(p.volume, pgDataRoot)),
	}
	names := make([]string, 0, len(phase.Extensions))
	for name := range phase.Extensions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		image := phase.Extensions[name]
		p.ensureImage(image)
		opts = append(opts, testcontainers.WithImageMount(image, ".", testcontainers.ContainerMountTarget("/extensions/"+name)))
	}

	c, err := testcontainers.Run(p.ctx, phase.RuntimeImage, opts...)
	require.NoError(t, err)
	p.c = c
	p.phase = phase

	p.mustExec([]string{"chown", "26:26", pgDataRoot}, tcexec.WithUser("root"))
	if code, _ := p.Exec("test", "-f", pgDataDir+"/PG_VERSION"); code != 0 {
		p.mustExec([]string{p.bin("initdb"), "-D", pgDataDir, "-U", "postgres", "--auth=trust"})
		p.mustExec([]string{"sh", "-c", `echo "include_if_exists = 'cnpg-extensions.conf'" >> "$1"`, "sh", pgDataDir + "/postgresql.conf"})
	}

	// Same path layout CNPG generates for its extensions stanza.
	control := []string{"$system"}
	libs := []string{"$libdir"}
	for _, name := range names {
		control = append(control, "/extensions/"+name+"/share")
		libs = append(libs, "/extensions/"+name+"/lib")
	}
	conf := []string{
		"listen_addresses = ''",
		"unix_socket_directories = '/tmp'",
		fmt.Sprintf("extension_control_path = '%s'", strings.Join(control, ":")),
		fmt.Sprintf("dynamic_library_path = '%s'", strings.Join(libs, ":")),
	}
	for _, s := range p.settings {
		k, v, ok := strings.Cut(s, "=")
		require.True(t, ok, "setting %q must be key=value", s)
		conf = append(conf, fmt.Sprintf("%s = '%s'", strings.TrimSpace(k), strings.TrimSpace(v)))
	}
	p.mustExec(append([]string{"sh", "-c", `f=$1; shift; printf '%s\n' "$@" > "$f"`, "sh", pgConfFile}, conf...))

	if code, out := p.Exec(p.bin("pg_ctl"), "-D", pgDataDir, "-l", pgLogFile, "-w", "-t", "120", "start"); code != 0 {
		t.Fatalf("postgres failed to start (%s):\n%s\n--- postgres.log ---\n%s", phase.RuntimeImage, out, p.LogTail())
	}
}

// Stop shuts the instance down (fast shutdown, as on a pod restart) and
// removes the container. The data volume is kept.
func (p *Postgres) Stop() {
	t := p.t
	t.Helper()

	if code, out := p.Exec(p.bin("pg_ctl"), "-D", pgDataDir, "-w", "-t", "120", "-m", "fast", "stop"); code != 0 {
		t.Fatalf("postgres failed to stop:\n%s\n--- postgres.log ---\n%s", out, p.LogTail())
	}
	require.NoError(t, p.c.Terminate(p.ctx))
	p.c = nil
}

// Psql runs SQL in a NEW session (psql -X, so it is the first statement of
// the session) and returns tuples-only, unaligned output. Fails the test on
// any error.
func (p *Postgres) Psql(sql string) string {
	p.t.Helper()

	code, out := p.PsqlResult(sql)
	if code != 0 {
		p.t.Fatalf("psql failed for:\n%s\n%s\n--- postgres.log ---\n%s", sql, out, p.LogTail())
	}
	return out
}

// PsqlResult is Psql without failing the test; it returns psql's exit code.
func (p *Postgres) PsqlResult(sql string) (int, string) {
	p.t.Helper()

	return p.Exec(p.bin("psql"), "-XAtq", "-v", "ON_ERROR_STOP=1",
		"-h", "/tmp", "-U", "postgres", "-d", "postgres", "-c", sql)
}

// RequireFile fails the test if path is not a regular file in the instance.
func (p *Postgres) RequireFile(path string) {
	p.t.Helper()

	code, out := p.Exec("test", "-f", path)
	require.Equal(p.t, 0, code, "expected file %s to exist: %s", path, out)
}

// LogTail returns the last lines of the postgres server log.
func (p *Postgres) LogTail() string {
	if p.c == nil {
		return "(no running instance)"
	}
	_, out := p.Exec("tail", "-n", "80", pgLogFile)
	return out
}

// Exec runs a command in the instance container and returns its exit code
// and combined output.
func (p *Postgres) Exec(cmd ...string) (int, string) {
	return p.exec(cmd)
}

func (p *Postgres) exec(cmd []string, opts ...tcexec.ProcessOption) (int, string) {
	p.t.Helper()

	code, reader, err := p.c.Exec(p.ctx, cmd, append(opts, tcexec.Multiplexed())...)
	require.NoError(p.t, err)
	out, err := io.ReadAll(reader)
	require.NoError(p.t, err)
	return code, strings.TrimSpace(string(out))
}

func (p *Postgres) mustExec(cmd []string, opts ...tcexec.ProcessOption) {
	p.t.Helper()

	if code, out := p.exec(cmd, opts...); code != 0 {
		p.t.Fatalf("%v failed (%d):\n%s", cmd, code, out)
	}
}

func (p *Postgres) bin(name string) string {
	return "/usr/lib/postgresql/" + p.pgMajor + "/bin/" + name
}

// ensureImage makes sure an image used as a mount source exists locally
// (Docker does not pull image-mount sources on its own).
func (p *Postgres) ensureImage(image string) {
	p.t.Helper()

	if _, err := p.provider.Client().ImageInspect(p.ctx, image); err == nil {
		return
	}
	require.NoError(p.t, p.provider.PullImage(p.ctx, image), "pull %s", image)
}

// ImageRuntime returns the CNPG runtime image an extension image was built
// against, from its RuntimeImageLabel, or fallback for images built before
// the label existed.
func (p *Postgres) ImageRuntime(image, fallback string) string {
	p.t.Helper()

	p.ensureImage(image)
	info, err := p.provider.Client().ImageInspect(p.ctx, image)
	require.NoError(p.t, err)
	if info.Config != nil {
		if v := info.Config.Labels[RuntimeImageLabel]; v != "" {
			return v
		}
	}
	p.t.Logf("%s has no %s label; using %s", image, RuntimeImageLabel, fallback)
	return fallback
}

func apiVersionAtLeast(v string, major, minor int) bool {
	maj, min, ok := strings.Cut(v, ".")
	if !ok {
		return false
	}
	a, err1 := strconv.Atoi(maj)
	b, err2 := strconv.Atoi(min)
	if err1 != nil || err2 != nil {
		return false
	}
	return a > major || (a == major && b >= minor)
}

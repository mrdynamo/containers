package testhelpers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// DefaultProductionClusterURL is the CNPG Cluster that Flux deploys from
// mrdynamo/home-ops main: the definition of "production" for the
// upgrade-rehearsal tests.
const DefaultProductionClusterURL = "https://raw.githubusercontent.com/mrdynamo/home-ops/main/kubernetes/apps/database/cloudnative-pg/cluster/cluster.yaml"

// ProductionCluster is what production runs: the runtime image, the
// extension images (CNPG extension name -> image reference) and the
// postgres settings that matter for extensions.
type ProductionCluster struct {
	Source       string
	RuntimeImage string
	Extensions   map[string]string
	Settings     []string // "key=value", incl. shared_preload_libraries
}

type cnpgCluster struct {
	Spec struct {
		ImageName  string `yaml:"imageName"`
		PostgreSQL struct {
			SharedPreloadLibraries []string          `yaml:"shared_preload_libraries"`
			Parameters             map[string]string `yaml:"parameters"`
			Extensions             []struct {
				Name  string `yaml:"name"`
				Image struct {
					Reference string `yaml:"reference"`
				} `yaml:"image"`
			} `yaml:"extensions"`
		} `yaml:"postgresql"`
	} `yaml:"spec"`
}

// Production reads the production CNPG Cluster from PRODUCTION_CLUSTER_YAML
// (a URL or a local file path), defaulting to home-ops main on GitHub.
func Production(t *testing.T) ProductionCluster {
	t.Helper()

	source := os.Getenv("PRODUCTION_CLUSTER_YAML")
	if source == "" {
		source = DefaultProductionClusterURL
	}

	var data []byte
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(source)
		require.NoError(t, err, "fetch %s", source)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "fetch %s", source)
		data, err = io.ReadAll(resp.Body)
		require.NoError(t, err)
	} else {
		var err error
		data, err = os.ReadFile(source)
		require.NoError(t, err)
	}

	var c cnpgCluster
	require.NoError(t, yaml.Unmarshal(data, &c), "parse %s", source)
	pg := c.Spec.PostgreSQL
	require.NotEmpty(t, c.Spec.ImageName, "%s has no spec.imageName", source)

	prod := ProductionCluster{
		Source:       source,
		RuntimeImage: c.Spec.ImageName,
		Extensions:   map[string]string{},
	}
	for _, e := range pg.Extensions {
		require.NotEmpty(t, e.Image.Reference, "extension %q has no image.reference", e.Name)
		prod.Extensions[e.Name] = e.Image.Reference
	}
	if len(pg.SharedPreloadLibraries) > 0 {
		prod.Settings = append(prod.Settings, "shared_preload_libraries="+strings.Join(pg.SharedPreloadLibraries, ","))
	}
	keys := make([]string, 0, len(pg.Parameters))
	for k := range pg.Parameters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		prod.Settings = append(prod.Settings, k+"="+pg.Parameters[k])
	}
	return prod
}

// With returns the production extensions with one of them replaced, i.e.
// what production runs after deploying image for extension name.
func (p ProductionCluster) With(name, image string) map[string]string {
	out := make(map[string]string, len(p.Extensions))
	for k, v := range p.Extensions {
		out[k] = v
	}
	out[name] = image
	return out
}

func (p ProductionCluster) String() string {
	names := make([]string, 0, len(p.Extensions))
	for n := range p.Extensions {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "%s on %s", p.Source, p.RuntimeImage)
	for _, n := range names {
		fmt.Fprintf(&b, "\n    %s: %s", n, p.Extensions[n])
	}
	return b.String()
}

package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func resolvedCompose(t *testing.T) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "compose", "--env-file", "/dev/null", "-f", "../../loadtest/compose.yaml", "--profile", "build", "--profile", "prepare", "--profile", "measure", "config", "--format", "json")
	cmd.Env = append(os.Environ(), "LOADTEST_DB_PASSWORD=fixture-only", "LOADTEST_JWT_SECRET=fixture-only")
	output, err := cmd.Output()
	require.NoError(t, err)
	var config map[string]any
	require.NoError(t, json.Unmarshal(output, &config))
	return config
}

func TestCompose_SeparatesServicesAndAppliesEveryRuntimeLimit(t *testing.T) {
	config := resolvedCompose(t)
	require.Equal(t, "threadly-loadtest", config["name"])
	services := config["services"].(map[string]any)
	expected := map[string]struct {
		cpu    float64
		memory int
	}{
		"api": {1, 1024}, "loadtest-db": {1, 1024}, "k6": {.75, 768}, "otel-collector": {.25, 256}, "tempo": {.25, 512}, "grafana": {.25, 256}, "storage-init": {.25, 64}, "tool": {.25, 256}, "builder": {1, 1536},
	}
	require.Len(t, services, len(expected))
	for name, limit := range expected {
		t.Run(name, func(t *testing.T) {
			service := services[name].(map[string]any)
			require.Equal(t, limit.cpu, service["cpus"])
			require.Equal(t, float64(limit.memory*1024*1024), composeNumber(t, service["mem_limit"]))
			require.Equal(t, composeNumber(t, service["mem_limit"]), composeNumber(t, service["memswap_limit"]))
			require.Equal(t, "no", service["restart"])
			require.Equal(t, "threadly-loadtest", service["labels"].(map[string]any)["io.threadly.loadtest.project"])
		})
	}
	require.NotContains(t, services, "db")
	require.NotContains(t, services, "test-db")
	require.NotContains(t, services, "loki")
	require.NotContains(t, services, "prometheus")
}

func TestCompose_ProtectsVolumesNetworkAndPreparationProfiles(t *testing.T) {
	config := resolvedCompose(t)
	services := config["services"].(map[string]any)
	for _, name := range []string{"builder", "tool", "k6"} {
		service := services[name].(map[string]any)
		require.NotEmpty(t, service["profiles"])
		require.NotContains(t, service, "build")
	}
	builder := services["builder"].(map[string]any)
	require.Equal(t, []any{"build"}, builder["profiles"])
	require.Equal(t, "0", builder["environment"].(map[string]any)["CGO_ENABLED"])
	require.Contains(t, builder["command"], "go build -o /artifacts/threadly-api ./cmd/api && go build -o /artifacts/loadtest ./cmd/loadtest")
	volumes := config["volumes"].(map[string]any)
	require.Contains(t, volumes, "loadtest_db_data")
	require.Contains(t, volumes, "loadtest_trace_data")
	for _, value := range volumes {
		volume := value.(map[string]any)
		require.Contains(t, volume["name"], "threadly-loadtest_")
		require.NotEqual(t, true, volume["external"])
	}
	db := services["loadtest-db"].(map[string]any)
	require.NotContains(t, db, "ports")
	require.Equal(t, "threadly_loadtest", db["environment"].(map[string]any)["MYSQL_DATABASE"])
	api := services["api"].(map[string]any)
	require.Equal(t, "threadly_loadtest", api["environment"].(map[string]any)["DB"])
	require.Equal(t, "loadtest-db", api["environment"].(map[string]any)["DB_HOST"])
	for _, name := range []string{"api", "grafana", "tempo"} {
		service := services[name].(map[string]any)
		for _, port := range service["ports"].([]any) {
			require.Equal(t, "127.0.0.1", port.(map[string]any)["host_ip"])
		}
	}
	for _, name := range []string{"api", "k6"} {
		service := services[name].(map[string]any)
		for _, value := range service["volumes"].([]any) {
			require.NotContains(t, value.(map[string]any)["source"], "docker.sock")
		}
	}
}

func composeNumber(t *testing.T, value any) float64 {
	t.Helper()
	number, err := strconv.ParseFloat(fmt.Sprint(value), 64)
	require.NoError(t, err)
	return number
}

func TestCompose_UsesOnlyTempoForDedicatedGrafana(t *testing.T) {
	config := resolvedCompose(t)
	services := config["services"].(map[string]any)
	grafana := services["grafana"].(map[string]any)
	mount := grafana["volumes"].([]any)[0].(map[string]any)
	require.Equal(t, true, mount["read_only"])
	data, err := os.ReadFile(filepath.Join(mount["source"].(string), "datasources/datasources.yaml"))
	require.NoError(t, err)
	var provision struct {
		APIVersion  int `yaml:"apiVersion"`
		Datasources []struct {
			Type string `yaml:"type"`
			URL  string `yaml:"url"`
		} `yaml:"datasources"`
	}
	require.NoError(t, yaml.Unmarshal(data, &provision))
	require.Equal(t, 1, provision.APIVersion)
	require.Len(t, provision.Datasources, 1)
	require.Equal(t, "tempo", provision.Datasources[0].Type)
	require.Equal(t, "http://tempo:3200", provision.Datasources[0].URL)
}

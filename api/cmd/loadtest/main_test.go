package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	binary        string
	fakeDockerDir string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "threadly-loadtest-cli-")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "loadtest")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		panic(string(output))
	}
	// Docker VMを使えない自動テストでは、引数を記録する代替dockerだけをPATHへ置く。
	fakeDockerDir = filepath.Join(dir, "fakedocker")
	buildFake := exec.Command("go", "build", "-o", filepath.Join(fakeDockerDir, "docker"), "./testdata/fakedocker")
	if output, err := buildFake.CombinedOutput(); err != nil {
		panic(string(output))
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func command(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	require.NoError(t, ctx.Err())
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit)
	return stdout.String(), stderr.String(), exit.ExitCode()
}

func TestPlan_ProducesFixedMixedPlanWithoutDocker(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	stdout, stderr, status := command(t, "plan", "--scenario", "mixed", "--dataset", "medium")
	require.Equal(t, 0, status, stderr)
	var plan map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	require.Equal(t, float64(1), plan["schema_version"])
	require.Equal(t, "mixed", plan["scenario"])
	require.Equal(t, "medium", plan["dataset"])
	require.Equal(t, []any{float64(1), float64(2), float64(5), float64(10), float64(20), float64(40)}, plan["rates"])
	require.Equal(t, float64(60), plan["warmup_seconds"])
	require.Equal(t, float64(120), plan["stage_seconds"])
	require.Equal(t, float64(32), plan["max_vus"])
	require.Equal(t, float64(900), plan["max_run_seconds"])
	require.Equal(t, float64(890), plan["stop_request_seconds"])
	require.Equal(t, float64(5), plan["http_timeout_seconds"])
	require.Equal(t, float64(20), plan["operations_per_cycle"])
	require.Equal(t, float64(16), plan["reads_per_cycle"])
	require.Equal(t, float64(22), plan["http_per_cycle"])
	require.Equal(t, float64(3.5), plan["steady_cpu"])
	require.Equal(t, float64(3840), plan["steady_memory_mib"])
}

func TestPlan_RejectsInvalidInputsWithoutEchoingValues(t *testing.T) {
	for _, args := range [][]string{
		{}, {"unknown"}, {"plan"},
		{"plan", "--scenario", "secret-value", "--dataset", "medium"},
		{"plan", "--scenario", "mixed", "--dataset", "large"},
		{"plan", "--scenario", "smoke", "--dataset", "medium"},
		{"plan", "--scenario", "browse", "--dataset", "secret-value"},
		{"plan", "--scenario", "browse", "--dataset", "small", "--url", "secret-value"},
		{"plan", "--scenario", "browse", "--dataset", "small", "secret-value"},
	} {
		stdout, stderr, status := command(t, args...)
		require.Equal(t, 5, status)
		require.Empty(t, stdout)
		require.Equal(t, "loadtest invalid_input\n", stderr)
	}
}

func TestPlan_ProducesScenarioAndDatasetSpecificConditions(t *testing.T) {
	cases := []struct {
		scenario, dataset            string
		vus, warmup, stage, duration int
		rates                        []int
	}{
		{"browse", "small", 32, 60, 120, 900, []int{1, 2, 5, 10, 20, 40}},
		{"browse", "medium", 32, 60, 120, 900, []int{1, 2, 5, 10, 20, 40}},
		{"browse", "large", 32, 60, 120, 900, []int{1, 2, 5, 10, 20, 40}},
		{"login", "medium", 8, 60, 120, 900, []int{1, 2, 3, 4, 5, 6}},
		{"smoke", "small", 1, 0, 0, 30, []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.scenario+"/"+tc.dataset, func(t *testing.T) {
			stdout, stderr, status := command(t, "plan", "--scenario", tc.scenario, "--dataset", tc.dataset)
			require.Equal(t, 0, status, stderr)
			var plan struct {
				Scenario string `json:"scenario"`
				Dataset  string `json:"dataset"`
				MaxVUs   int    `json:"max_vus"`
				Warmup   int    `json:"warmup_seconds"`
				Stage    int    `json:"stage_seconds"`
				Duration int    `json:"max_run_seconds"`
				Rates    []int  `json:"rates"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
			require.Equal(t, tc.scenario, plan.Scenario)
			require.Equal(t, tc.dataset, plan.Dataset)
			require.Equal(t, tc.vus, plan.MaxVUs)
			require.Equal(t, tc.warmup, plan.Warmup)
			require.Equal(t, tc.stage, plan.Stage)
			require.Equal(t, tc.duration, plan.Duration)
			require.Equal(t, tc.rates, plan.Rates)
		})
	}
}

func TestPlan_ListsResourceFixtureAndEvaluationConditions(t *testing.T) {
	stdout, stderr, status := command(t, "plan", "--scenario", "browse", "--dataset", "large")
	require.Equal(t, 0, status, stderr)
	var plan map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	require.Equal(t, map[string]any{"users": float64(128), "posts": float64(10000), "comments": float64(100000), "post_likes": float64(20000), "comment_likes": float64(200000)}, plan["fixture_counts"])
	require.Equal(t, map[string]any{"cpu": float64(4), "memory_mib": float64(6144)}, plan["vm_max"])
	require.Equal(t, float64(10), plan["drain_seconds"])
	require.Equal(t, float64(30), plan["window_seconds"])
	require.Equal(t, float64(20), plan["window_min_http"])
	require.Equal(t, float64(100), plan["stage_min_http"])
	require.Equal(t, float64(500), plan["p95_limit_ms"])
	require.Equal(t, float64(1000), plan["p99_limit_ms"])
	require.Equal(t, float64(0.01), plan["error_rate_limit"])
	resources := plan["resources"].(map[string]any)
	require.Len(t, resources, 8)
	for _, value := range resources {
		resource := value.(map[string]any)
		require.Equal(t, resource["memory_mib"], resource["memswap_mib"])
		require.Equal(t, "no", resource["restart"])
	}
	require.Equal(t, float64(1), plan["operations_per_cycle"])
	require.Equal(t, float64(1), plan["http_per_cycle"])
	require.Equal(t, float64(1), plan["reads_per_cycle"])
	require.NotContains(t, stdout, "secret-value")
}

func TestPlan_SeparatesBuilderLimitsAndSmokeSuiteFromBusinessCycles(t *testing.T) {
	stdout, stderr, status := command(t, "plan", "--scenario", "smoke", "--dataset", "small")
	require.Equal(t, 0, status, stderr)
	var plan map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &plan))
	require.Equal(t, map[string]any{"cpu": float64(1), "memory_mib": float64(1536), "memswap_mib": float64(1536), "restart": "no"}, plan["builder_resource"])
	// smokeはhealth、未認証、3閲覧、2作成、4Like/Unlikeの確認を一度行う。
	require.Equal(t, float64(11), plan["http_per_cycle"])
	require.Equal(t, float64(3), plan["reads_per_cycle"])
}

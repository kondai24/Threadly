package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	smallChecksum   = "sha256:e38737c5799a81cd429cea5f2f23e1c68c2e82103ee0f6854a2cec1e022a479e"
	secretValue     = "dummy-secret-value"
	projectLabelsOK = `{"com.docker.compose.project":"threadly-loadtest","com.docker.compose.volume":"loadtest_db_data"}`
)

type dockerRule struct {
	Contains []string `json:"contains"`
	Stdout   string   `json:"stdout"`
	Exit     int      `json:"exit"`
}

// loadtestEnv は、api/ と同じ相対配置の一時ディレクトリと代替dockerの記録先を持つ。
type loadtestEnv struct {
	root  string
	log   string
	rules string
}

func newLoadtestEnv(t *testing.T, rules ...dockerRule) loadtestEnv {
	t.Helper()
	// CLIはos.Getwdの実パスを使うため、macOSの/private付きパスへ揃える。
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "loadtest", "secrets"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "loadtest", "compose.yaml"), []byte("name: threadly-loadtest\n"), 0o600))
	secrets := "LOADTEST_DB_PASSWORD=" + secretValue + "\nLOADTEST_JWT_SECRET=" + secretValue + "\nLOADTEST_USER_PASSWORD=" + secretValue + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "loadtest", "secrets", "loadtest.env"), []byte(secrets), 0o600))
	env := loadtestEnv{root: root, log: filepath.Join(root, "docker.log"), rules: filepath.Join(root, "rules.json")}
	env.setRules(t, rules...)
	return env
}

// dedicatedComposeConfigは、専用projectとDB名・volumeが一致する解決済みComposeの最小形である。
const dedicatedComposeConfig = `{"name":"threadly-loadtest","services":{"loadtest-db":{"environment":{"MYSQL_DATABASE":"threadly_loadtest"}}},"volumes":{"loadtest_db_data":{"name":"threadly-loadtest_loadtest_db_data"},"loadtest_trace_data":{"name":"threadly-loadtest_loadtest_trace_data"}}}`

func (e loadtestEnv) setRules(t *testing.T, rules ...dockerRule) {
	t.Helper()
	// テストごとの応答を優先し、未指定なら専用Compose設定を返す。
	rules = append(rules, dockerRule{Contains: []string{"config --format json"}, Stdout: dedicatedComposeConfig})
	data, err := json.Marshal(rules)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(e.rules, data, 0o600))
}

func (e loadtestEnv) run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = e.root
	cmd.Env = []string{"PATH=" + fakeDockerDir, "HOME=" + e.root, "FAKE_DOCKER_LOG=" + e.log, "FAKE_DOCKER_RULES=" + e.rules}
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

// dockerCalls は、代替dockerが受け取った引数を呼出順に返す。
func (e loadtestEnv) dockerCalls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(e.log)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	var calls []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var args []string
		require.NoError(t, json.Unmarshal([]byte(line), &args))
		calls = append(calls, strings.Join(args, " "))
	}
	return calls
}

func indexOfCall(calls []string, part string) int {
	for i, call := range calls {
		if strings.Contains(call, part) {
			return i
		}
	}
	return -1
}

func seedSummary(checksum string) string {
	return `{"version":"v1","dataset":"small","counts":{"users":32,"posts":100,"comments":1000,"post_likes":200,"comment_likes":2000},"checksum":"` + checksum + `"}` + "\n"
}

func TestPrepare_SeedsDedicatedDatabaseAndRecordsUnusedFixture(t *testing.T) {
	env := newLoadtestEnv(t, dockerRule{Contains: []string{"run --rm", "tool seed --dataset small"}, Stdout: seedSummary(smallChecksum)})

	stdout, stderr, status := env.run(t, "prepare", "--dataset", "small")

	require.Equal(t, 0, status, stderr)
	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	require.Equal(t, smallChecksum, summary["checksum"])
	state, err := os.ReadFile(filepath.Join(env.root, "loadtest", "state", "fixture.json"))
	require.NoError(t, err)
	var recorded map[string]any
	require.NoError(t, json.Unmarshal(state, &recorded))
	require.Equal(t, "small", recorded["dataset"])
	require.Equal(t, smallChecksum, recorded["checksum"])
	require.Equal(t, false, recorded["used"])

	calls := env.dockerCalls(t)
	build := indexOfCall(calls, "--profile build run --rm --name threadly-loadtest-prepare-build builder")
	database := indexOfCall(calls, "up -d --wait loadtest-db")
	migrate := indexOfCall(calls, "--profile prepare run --rm --name threadly-loadtest-prepare-migrate migrate")
	seed := indexOfCall(calls, "--profile prepare run --rm --name threadly-loadtest-prepare-seed tool seed --dataset small")
	// 測定サービス停止中にbuildし、DB起動・Atlas適用・fixture投入の順で準備する。
	require.True(t, build >= 0 && build < database && database < migrate && migrate < seed, calls)
	for _, call := range calls {
		if strings.HasPrefix(call, "compose ") {
			require.Contains(t, call, "-p threadly-loadtest -f "+filepath.Join(env.root, "loadtest", "compose.yaml"))
		}
		require.NotContains(t, call, secretValue)
		require.NotContains(t, call, "down -v")
	}
	require.NotContains(t, stdout+stderr, secretValue)
}

func TestPrepare_RefusesUnsafeOrInconsistentPreparation(t *testing.T) {
	cases := []struct {
		name       string
		rules      []dockerRule
		setup      func(t *testing.T, env loadtestEnv)
		wantStatus int
		wantReason string
		// noDocker は、拒否時にdockerを一度も呼ばないことを要求する。
		noDocker bool
	}{
		{
			name:       "準備済み状態があれば再初期化しない",
			setup:      func(t *testing.T, env loadtestEnv) { writeState(t, env, false) },
			wantStatus: 5, wantReason: "already_prepared", noDocker: true,
		},
		{
			name: "専用の秘密設定がなければ開発用.envへフォールバックしない",
			setup: func(t *testing.T, env loadtestEnv) {
				require.NoError(t, os.Remove(filepath.Join(env.root, "loadtest", "secrets", "loadtest.env")))
			},
			wantStatus: 5, wantReason: "secrets_missing", noDocker: true,
		},
		{
			name: "別projectのDB volumeなら何も起動しない",
			rules: []dockerRule{
				{Contains: []string{"volume ls"}, Stdout: "threadly-loadtest_loadtest_db_data\n"},
				{Contains: []string{"volume inspect"}, Stdout: `{"com.docker.compose.project":"api"}` + "\n"},
			},
			wantStatus: 5, wantReason: "target_mismatch",
		},
		{
			name:       "seedのchecksum不一致は準備完了にしない",
			rules:      []dockerRule{{Contains: []string{"run --rm", "tool seed"}, Stdout: seedSummary("sha256:" + strings.Repeat("0", 64))}},
			wantStatus: 5, wantReason: "fixture_mismatch",
		},
		{
			name:       "seedが初期化済みDBを拒否したら準備不整合",
			rules:      []dockerRule{{Contains: []string{"run --rm", "tool seed"}, Exit: 5}},
			wantStatus: 5, wantReason: "seed_rejected",
		},
		{
			name:       "解決済みComposeのDB名が専用DBでなければ何も起動しない",
			rules:      []dockerRule{{Contains: []string{"config --format json"}, Stdout: strings.Replace(dedicatedComposeConfig, "threadly_loadtest\"}", "threadly_test\"}", 1)}},
			wantStatus: 5, wantReason: "target_mismatch",
		},
		{
			name: "既存DB containerが別projectなら何も起動しない",
			rules: []dockerRule{
				{Contains: []string{"ps -a -q"}, Stdout: "abc123\n"},
				{Contains: []string{"inspect --format {{json .Config.Labels}}"}, Stdout: `{"com.docker.compose.project":"api","com.docker.compose.service":"loadtest-db"}` + "\n"},
			},
			wantStatus: 5, wantReason: "target_mismatch",
		},
		{
			name:       "DB起動失敗は環境障害",
			rules:      []dockerRule{{Contains: []string{"up -d --wait loadtest-db"}, Exit: 1}},
			wantStatus: 4, wantReason: "prepare_failed",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			env := newLoadtestEnv(t, tt.rules...)
			if tt.setup != nil {
				tt.setup(t, env)
			}
			stateBefore, _ := os.ReadFile(filepath.Join(env.root, "loadtest", "state", "fixture.json"))

			stdout, stderr, status := env.run(t, "prepare", "--dataset", "small")

			require.Equal(t, tt.wantStatus, status)
			require.Empty(t, stdout)
			require.Equal(t, "loadtest "+tt.wantReason+"\n", stderr)
			stateAfter, _ := os.ReadFile(filepath.Join(env.root, "loadtest", "state", "fixture.json"))
			require.Equal(t, stateBefore, stateAfter)
			calls := env.dockerCalls(t)
			if tt.noDocker {
				require.Empty(t, calls)
			}
			if tt.wantReason == "target_mismatch" {
				require.Equal(t, -1, indexOfCall(calls, " up "))
				require.Equal(t, -1, indexOfCall(calls, " run "))
			}
		})
	}
}

func writeState(t *testing.T, env loadtestEnv, used bool) {
	t.Helper()
	dir := filepath.Join(env.root, "loadtest", "state")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	state := map[string]any{
		"version": "v1", "dataset": "small", "checksum": smallChecksum, "used": used, "prepared_at": "2026-10-25T00:00:00Z",
		"counts": map[string]int{"users": 32, "posts": 100, "comments": 1000, "post_likes": 200, "comment_likes": 2000},
	}
	data, err := json.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fixture.json"), data, 0o600))
}

func TestSeed_RejectsMissingSecretsAndUnknownDatasetWithoutConnecting(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		env        []string
		wantReason string
	}{
		{name: "未知datasetは拒否する", args: []string{"seed", "--dataset", secretValue}, env: []string{"LOADTEST_DB_PASSWORD=" + secretValue, "LOADTEST_USER_PASSWORD=" + secretValue}, wantReason: "invalid_input"},
		{name: "DB passwordがなければ接続しない", args: []string{"seed", "--dataset", "small"}, env: []string{"LOADTEST_USER_PASSWORD=" + secretValue}, wantReason: "secrets_missing"},
		{name: "User passwordがなければ接続しない", args: []string{"seed", "--dataset", "small"}, env: []string{"LOADTEST_DB_PASSWORD=" + secretValue}, wantReason: "secrets_missing"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, tt.args...)
			cmd.Env = tt.env
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			var exit *exec.ExitError
			require.ErrorAs(t, err, &exit)
			require.Equal(t, 5, exit.ExitCode())
			require.Empty(t, stdout.String())
			require.Equal(t, "loadtest "+tt.wantReason+"\n", stderr.String())
		})
	}
}

func TestPrepare_RemovesNamedOneOffContainerWhenStepFails(t *testing.T) {
	env := newLoadtestEnv(t, dockerRule{Contains: []string{"run --rm", "migrate"}, Exit: 1})

	_, stderr, status := env.run(t, "prepare", "--dataset", "small")

	require.Equal(t, 4, status)
	require.Equal(t, "loadtest prepare_failed\n", stderr)
	calls := env.dockerCalls(t)
	migrate := indexOfCall(calls, "run --rm --name threadly-loadtest-prepare-migrate migrate")
	cleanup := indexOfCall(calls, "rm -f threadly-loadtest-prepare-migrate")
	// clientだけが終了してもcontainerが専用DBへ書き続けないよう、排他を解放する前に名前指定で削除する。
	require.True(t, migrate >= 0 && migrate < cleanup, calls)
	require.Equal(t, -1, indexOfCall(calls, "tool seed"))
}

package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	vmWithinLimits = `{"NCPU":4,"MemTotal":6442450944,"SwapLimit":true}`
	runningState   = `{"Status":"running","OOMKilled":false,"RestartCount":0}`
)

// apiFixtureは、Docker内APIの代わりにloopbackで応答するHTTP fixture serverである。
type apiFixture struct {
	server *httptest.Server
	mu     sync.Mutex
	logins []string
	// healthFailsAfterは、指定回数以降の/healthを503にする。0なら常に200。
	healthFailsAfter int
	healthCalls      int
}

func newAPIFixture(t *testing.T) *apiFixture {
	t.Helper()
	api := &apiFixture{}
	api.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.mu.Lock()
		defer api.mu.Unlock()
		switch r.URL.Path {
		case "/health":
			api.healthCalls++
			if api.healthFailsAfter > 0 && api.healthCalls >= api.healthFailsAfter {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "/ready", "/api/health":
			// 同じfixtureでTempoとGrafanaの準備完了応答も代替する。
			w.WriteHeader(http.StatusOK)
		case "/api/me":
			if cookie, err := r.Cookie("threadly-session"); err != nil || !strings.HasPrefix(cookie.Value, "session-lt_user_") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "/api/auth/login":
			var body struct{ Username, Password string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			index, err := strconv.Atoi(strings.TrimPrefix(body.Username, "lt_user_"))
			if err != nil || body.Password != secretValue {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			api.logins = append(api.logins, body.Username)
			http.SetCookie(w, &http.Cookie{Name: "threadly-session", Value: "session-" + body.Username, Path: "/", HttpOnly: true})
			w.Header().Set("Content-Type", "application/json")
			// fixture v1のUser IDは種別1と0始まり連番から作る固定値である。
			_, _ = fmt.Fprintf(w, `{"user":{"id":"00000001-0000-4000-8000-%012x","username":%q}}`, index-1, body.Username)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(api.server.Close)
	return api
}

func (a *apiFixture) loginCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.logins)
}

// statsLineは、docker stats --no-stream --format {{json .}} の1行である。CPUPercは100%=1CPU。
func statsLine(name, cpu, memPerc string) string {
	return fmt.Sprintf(`{"Name":%q,"CPUPerc":%q,"MemUsage":"100MiB / 1GiB","MemPerc":%q}`, name, cpu, memPerc) + "\n"
}

// appliedLimitsは、`docker inspect` で読む実際のcontainer上限の応答fixtureである。
const appliedLimits = `{"name":"/threadly-loadtest-api-1","nano_cpus":1000000000,"memory":1073741824,"memory_swap":1073741824}` + "\n"

func runDefaults() []dockerRule {
	stats := residentStats()
	// 段階名ごとのk6 containerを、どの段階でも採取できるようにする。
	for _, stage := range []string{"smoke", "warmup", "1", "2", "3", "4", "5", "6"} {
		stats += statsLine("threadly-loadtest-k6-"+stage, "10.00%", "10.00%")
	}
	return []dockerRule{
		{Contains: []string{"inspect --format", "NanoCpus"}, Stdout: appliedLimits},
		{Contains: []string{"info --format"}, Stdout: vmWithinLimits},
		{Contains: []string{"inspect --format", ".State.OOMKilled"}, Stdout: strings.Repeat(runningState+"\n", 5)},
		{Contains: []string{"run --rm", "tool verify --dataset small"}, Stdout: seedSummary(smallChecksum)},
		{Contains: []string{"stats --no-stream"}, Stdout: stats},
		{Contains: []string{"system df -v"}, Stdout: `[{"Name":"threadly-loadtest_loadtest_trace_data","Size":"10MB"},{"Name":"other","Size":"9GB"}]`},
		{Contains: []string{"version --format"}, Stdout: `{"Client":{"Version":"28.5.1"},"Server":{"Version":"28.5.1","Arch":"arm64"}}`},
	}
}

// newRunEnvは、未使用fixtureを準備済みの専用環境と、run用の既定Docker応答を用意する。
func newRunEnv(t *testing.T, api *apiFixture, rules ...dockerRule) (loadtestEnv, []string) {
	t.Helper()
	env := newLoadtestEnv(t, append(rules, runDefaults()...)...)
	writeState(t, env, false)
	pids := filepath.Join(env.root, "pids")
	require.NoError(t, os.MkdirAll(pids, 0o700))
	return env, []string{"LOADTEST_API_URL=" + api.server.URL, "LOADTEST_TEMPO_URL=" + api.server.URL, "LOADTEST_GRAFANA_URL=" + api.server.URL, "FAKE_DOCKER_PIDDIR=" + pids}
}

func (e loadtestEnv) start(t *testing.T, extra []string, args ...string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = e.root
	cmd.Env = append([]string{"PATH=" + fakeDockerDir, "HOME=" + e.root, "FAKE_DOCKER_LOG=" + e.log, "FAKE_DOCKER_RULES=" + e.rules}, extra...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoError(t, cmd.Start())
	return cmd, &stdout, &stderr
}

func (e loadtestEnv) runWith(t *testing.T, extra []string, args ...string) (string, string, int) {
	t.Helper()
	cmd, stdout, stderr := e.start(t, extra, args...)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return stdout.String(), stderr.String(), 0
		}
		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		return stdout.String(), stderr.String(), exit.ExitCode()
	case <-time.After(60 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("run did not finish within 60 seconds")
		return "", "", 0
	}
}

// k6Pointは、代替dockerが書き出すk6 JSON Pointの1行である。時刻とrun IDは書出し時に埋める。
func k6Point(metric string, value float64, phase, stage, scenario string, extra map[string]string) string {
	tags := map[string]string{"run_id": "{{RUN_ID}}", "phase": phase, "stage": stage, "scenario": scenario}
	for key, value := range extra {
		tags[key] = value
	}
	encodedTags, _ := json.Marshal(tags)
	return fmt.Sprintf(`{"type":"Point","metric":%q,"data":{"time":"{{NOW}}","value":%v,"tags":%s}}`, metric, value, encodedTags)
}

func smokeWrites() []map[string]any {
	endpoints := []string{"GET /health", "GET /api/posts", "GET /api/posts", "GET /api/posts/:id", "GET /api/posts/:id/comments", "POST /api/posts", "POST /api/posts/:id/comments", "PUT /api/posts/:id/like", "DELETE /api/posts/:id/like", "PUT /api/comments/:id/like", "DELETE /api/comments/:id/like"}
	lines := []string{k6Point("lt_operations_started", 1, "smoke", "smoke", "smoke", nil)}
	for _, endpoint := range endpoints {
		lines = append(lines,
			k6Point("lt_http_duration", 12, "smoke", "smoke", "smoke", map[string]string{"endpoint": endpoint, "outcome": "success"}),
			k6Point("http_reqs", 1, "smoke", "smoke", "smoke", nil))
	}
	lines = append(lines, k6Point("lt_operations_completed", 1, "smoke", "smoke", "smoke", map[string]string{"outcome": "success"}), k6Point("iterations", 1, "smoke", "smoke", "smoke", nil))
	writes := make([]map[string]any, 0, len(lines))
	for i, line := range lines {
		delay := 0
		if i == len(lines)-1 {
			delay = 50
		}
		writes = append(writes, map[string]any{"delay_ms": delay, "line": line})
	}
	return writes
}

func resultDir(t *testing.T, env loadtestEnv) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(env.root, "loadtest", "results"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	return filepath.Join(env.root, "loadtest", "results", entries[0].Name())
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var value map[string]any
	require.NoError(t, json.Unmarshal(data, &value))
	return value
}

func TestRun_RefusesVMOutsideLimitsBeforeStartingServices(t *testing.T) {
	api := newAPIFixture(t)
	env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"info --format"}, Stdout: `{"NCPU":10,"MemTotal":8217059328,"SwapLimit":true}`})

	stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "smoke", "--dataset", "small")

	require.Equal(t, 4, status)
	require.Empty(t, stdout)
	require.Equal(t, "loadtest vm_limits_exceeded\n", stderr)
	require.Equal(t, -1, indexOfCall(env.dockerCalls(t), " up "))
	require.Zero(t, api.loginCount())
}

func TestRun_SmokeSucceedsAndSavesResultsWithoutSecrets(t *testing.T) {
	api := newAPIFixture(t)
	env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"run --rm", "k6 run"}, Writes: smokeWrites()})

	stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "smoke", "--dataset", "small")

	require.Equal(t, 0, status, stderr+stdout)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	require.Equal(t, "smoke_passed", report["status"])
	require.Equal(t, 1, api.loginCount())
	dir := resultDir(t, env)
	manifest := readJSON(t, filepath.Join(dir, "manifest.json"))
	require.Equal(t, true, manifest["finalized"])
	require.Equal(t, filepath.Base(dir), manifest["run_id"])
	require.Equal(t, report, readJSON(t, filepath.Join(dir, "report.json")))
	// 事前認証のCookieは実行時ディレクトリだけに置き、終了時に削除して結果へ残さない。
	runtimeEntries, _ := os.ReadDir(filepath.Join(env.root, "loadtest", "runtime"))
	require.Empty(t, runtimeEntries)
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)
		if !info.IsDir() {
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			require.NotContains(t, string(data), "session-lt_user")
			require.NotContains(t, string(data), secretValue)
		}
		return nil
	})
	state := readJSON(t, filepath.Join(env.root, "loadtest", "state", "fixture.json"))
	require.Equal(t, true, state["used"])
	calls := env.dockerCalls(t)
	up := indexOfCall(calls, "up -d --wait api otel-collector tempo grafana")
	k6 := indexOfCall(calls, "--profile measure run --rm --no-deps --name threadly-loadtest-k6-smoke")
	require.True(t, up >= 0 && up < k6, calls)
	require.Contains(t, calls[k6], "k6 run --quiet --out json=/results/"+filepath.Base(dir)+"/points-smoke.jsonl /scripts/scenario.js")
}

var _ = context.Background

func TestRun_RefusesUnsafeEnvironmentBeforeSending(t *testing.T) {
	cases := []struct {
		name       string
		rules      []dockerRule
		used       bool
		wantStatus int
		wantStderr string
	}{
		{name: "swap制限を確認できなければ送信しない", rules: []dockerRule{{Contains: []string{"info --format"}, Stdout: `{"NCPU":4,"MemTotal":6442450944,"SwapLimit":false}`}}, wantStatus: 4, wantStderr: "loadtest swap_limit_unsupported\n"},
		{name: "上限合計に余裕がない小さなVMでは送信しない", rules: []dockerRule{{Contains: []string{"info --format"}, Stdout: `{"NCPU":3,"MemTotal":6442450944,"SwapLimit":true}`}}, wantStatus: 4, wantStderr: "loadtest vm_limits_exceeded\n"},
		{
			name:       "無関係なcontainerがあれば一覧を示して送信しない",
			rules:      []dockerRule{{Contains: []string{"ps --format"}, Stdout: `{"Names":"api-db-1","Labels":"com.docker.compose.project=api"}` + "\n" + `{"Names":"threadly-loadtest-tempo-1","Labels":"com.docker.compose.project=threadly-loadtest"}` + "\n"}},
			wantStatus: 4, wantStderr: "loadtest unrelated_containers api-db-1\n",
		},
		{name: "使用済みfixtureではresetまで送信しない", used: true, wantStatus: 5, wantStderr: "loadtest fixture_not_ready\n"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			api := newAPIFixture(t)
			env, extra := newRunEnv(t, api, tt.rules...)
			if tt.used {
				writeState(t, env, true)
			}

			stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "smoke", "--dataset", "small")

			require.Equal(t, tt.wantStatus, status)
			require.Empty(t, stdout)
			require.Equal(t, tt.wantStderr, stderr)
			require.Equal(t, -1, indexOfCall(env.dockerCalls(t), " up "))
			require.Equal(t, -1, indexOfCall(env.dockerCalls(t), "k6 run"))
			require.Zero(t, api.loginCount())
		})
	}
}

// browseWritesは、browse段階の成功または失敗HTTPを持つ業務操作をcount件書き出す。
func browseWrites(phase, stage, outcome string, count int) []map[string]any {
	var writes []map[string]any
	for i := 0; i < count; i++ {
		for _, line := range []string{
			k6Point("lt_operations_started", 1, phase, stage, "browse", nil),
			k6Point("lt_http_duration", 15, phase, stage, "browse", map[string]string{"endpoint": "GET /api/posts", "outcome": outcome}),
			k6Point("http_reqs", 1, phase, stage, "browse", nil),
			k6Point("lt_operations_completed", 1, phase, stage, "browse", map[string]string{"outcome": outcome}),
			k6Point("iterations", 1, phase, stage, "browse", nil),
		} {
			writes = append(writes, map[string]any{"line": line})
		}
	}
	return writes
}

func TestRun_DoesNotStartNextStageAfterWarmupGenerationShortfall(t *testing.T) {
	api := newAPIFixture(t)
	env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"k6 run", "points-warmup"}, Writes: browseWrites("warmup", "warmup", "success", 25)})

	stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "browse", "--dataset", "small")

	require.Equal(t, 3, status, stderr)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	require.Contains(t, report["reasons"], "generation_shortfall")
	require.Equal(t, 32, api.loginCount())
	calls := env.dockerCalls(t)
	require.GreaterOrEqual(t, indexOfCall(calls, "--name threadly-loadtest-k6-warmup"), 0)
	require.Equal(t, -1, indexOfCall(calls, "--name threadly-loadtest-k6-1 "))
	manifest := readJSON(t, filepath.Join(resultDir(t, env), "manifest.json"))
	require.Len(t, manifest["stages"], 1)
}

func TestRun_StopsStageAndSavesReasonWhenMonitorDetectsFailure(t *testing.T) {
	cases := []struct {
		name       string
		rules      []dockerRule
		healthFail int
		wantStatus int
		wantReason string
	}{
		{
			name:       "transport errorは直ちに停止する",
			rules:      []dockerRule{{Contains: []string{"k6 run"}, Writes: append(browseWrites("warmup", "warmup", "success", 1), map[string]any{"line": k6Point("lt_transport_errors", 1, "warmup", "warmup", "browse", nil)}), SleepMS: 30000}},
			wantStatus: 4, wantReason: "connectivity",
		},
		{
			name: "常駐サービスのOOMで停止する",
			rules: []dockerRule{
				{Contains: []string{"inspect --format", ".State.OOMKilled"}, AfterCalls: 1, Stdout: `{"Status":"exited","OOMKilled":true,"RestartCount":0}` + "\n" + strings.Repeat(runningState+"\n", 4)},
				{Contains: []string{"k6 run"}, Writes: browseWrites("warmup", "warmup", "success", 1), SleepMS: 30000},
			},
			wantStatus: 4, wantReason: "oom",
		},
		{
			name:       "APIのhealthが応答しなければ停止する",
			rules:      []dockerRule{{Contains: []string{"k6 run"}, Writes: browseWrites("warmup", "warmup", "success", 1), SleepMS: 30000}},
			healthFail: 2, wantStatus: 4, wantReason: "connectivity",
		},
		{
			name:       "k6の想定外終了で次段階を開始しない",
			rules:      []dockerRule{{Contains: []string{"k6 run"}, Writes: browseWrites("warmup", "warmup", "success", 1), Exit: 1}},
			wantStatus: 4, wantReason: "container_exit",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			api := newAPIFixture(t)
			api.healthFailsAfter = tt.healthFail
			env, extra := newRunEnv(t, api, tt.rules...)
			started := time.Now()

			stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "browse", "--dataset", "small")

			require.Equal(t, tt.wantStatus, status, stderr)
			// 検出遅延は監視間隔5秒とdrain上限10秒の範囲に収め、送信を続けない。
			require.Less(t, time.Since(started), 25*time.Second)
			var report map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &report))
			require.Contains(t, report["reasons"], tt.wantReason)
			manifest := readJSON(t, filepath.Join(resultDir(t, env), "manifest.json"))
			require.Equal(t, true, manifest["finalized"])
			require.Contains(t, manifest["reasons"], tt.wantReason)
			calls := env.dockerCalls(t)
			require.Equal(t, -1, indexOfCall(calls, "--name threadly-loadtest-k6-1 "))
			runtimeEntries, _ := os.ReadDir(filepath.Join(env.root, "loadtest", "runtime"))
			require.Empty(t, runtimeEntries)
		})
	}
}

func TestRun_InterruptStopsK6AndSavesResultBeforeExiting130(t *testing.T) {
	api := newAPIFixture(t)
	env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"k6 run"}, Writes: browseWrites("warmup", "warmup", "success", 1), SleepMS: 60000})
	cmd, stdout, _ := env.start(t, extra, "run", "--scenario", "browse", "--dataset", "small")
	pidFile := filepath.Join(env.root, "pids", "threadly-loadtest-k6-warmup")
	require.Eventually(t, func() bool { _, err := os.Stat(pidFile); return err == nil }, 10*time.Second, 50*time.Millisecond)

	require.NoError(t, cmd.Process.Signal(os.Interrupt))
	err := cmd.Wait()

	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit)
	require.Equal(t, 130, exit.ExitCode())
	var report map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	require.Equal(t, "interrupted", report["status"])
	require.GreaterOrEqual(t, indexOfCall(env.dockerCalls(t), "kill -s SIGINT threadly-loadtest-k6-warmup"), 0)
	manifest := readJSON(t, filepath.Join(resultDir(t, env), "manifest.json"))
	require.Contains(t, manifest["reasons"], "interrupted")
}

func TestRun_StopsWhenHTTPCompletionStallsForTenSeconds(t *testing.T) {
	api := newAPIFixture(t)
	env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"k6 run"}, Writes: browseWrites("warmup", "warmup", "success", 1), SleepMS: 60000})

	stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "browse", "--dataset", "small")

	require.Equal(t, 4, status, stderr)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	require.Contains(t, report["reasons"], "connectivity")
}

func TestRun_StopsAtFirstThirtySecondWindowThatFailsQuality(t *testing.T) {
	if testing.Short() {
		t.Skip("30秒窓の確定を実時間で待つため、-shortでは実行しない")
	}
	api := newAPIFixture(t)
	// 最初の窓に20件以上の失敗HTTPを置き、その後もHTTP完了を続けて停滞とは区別する。
	writes := browseWrites("warmup", "warmup", "failure", 25)
	for i := 0; i < 8; i++ {
		next := browseWrites("warmup", "warmup", "success", 1)
		next[0]["delay_ms"] = 5000
		writes = append(writes, next...)
	}
	env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"k6 run"}, Writes: writes, SleepMS: 60000})
	started := time.Now()

	stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "browse", "--dataset", "small")

	require.Equal(t, 2, status, stderr+stdout)
	require.Less(t, time.Since(started), 55*time.Second)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	require.Equal(t, "quality_failed", report["status"])
	manifest := readJSON(t, filepath.Join(resultDir(t, env), "manifest.json"))
	stage := manifest["stages"].([]any)[0].(map[string]any)
	require.Equal(t, true, stage["stopped"])
	require.Equal(t, -1, indexOfCall(env.dockerCalls(t), "--name threadly-loadtest-k6-1 "))
}

func TestRun_UsesEarliestPointTimeAsStageStartEvenWhenWrittenLater(t *testing.T) {
	api := newAPIFixture(t)
	writes := smokeWrites()
	// k6のJSON出力は複数VUの間で時刻順とは限らない。後から書かれた古い時刻のPointも段階に含める。
	late := strings.Replace(k6Point("http_reqs", 0, "smoke", "smoke", "smoke", nil), "{{NOW}}", "{{NOW-800ms}}", 1)
	writes = append(writes[:3], append([]map[string]any{{"line": late}}, writes[3:]...)...)
	env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"run --rm", "k6 run"}, Writes: writes})

	stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "smoke", "--dataset", "small")

	require.Equal(t, 0, status, stderr+stdout)
}

func TestRun_SavesManifestWhenPreparationFailsOrIsInterrupted(t *testing.T) {
	t.Run("サービス起動の失敗は理由を保存して環境障害にする", func(t *testing.T) {
		api := newAPIFixture(t)
		env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"up -d --wait api"}, Exit: 1})

		stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "smoke", "--dataset", "small")

		require.Equal(t, 4, status, stderr)
		var report map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &report))
		require.Contains(t, report["reasons"], "environment")
		manifest := readJSON(t, filepath.Join(resultDir(t, env), "manifest.json"))
		require.Contains(t, manifest["reasons"], "environment")
		require.Equal(t, -1, indexOfCall(env.dockerCalls(t), "k6 run"))
	})
	t.Run("準備中の中断は理由を保存して130で終了する", func(t *testing.T) {
		api := newAPIFixture(t)
		api.healthFailsAfter = 1
		env, extra := newRunEnv(t, api)
		cmd, stdout, _ := env.start(t, extra, "run", "--scenario", "smoke", "--dataset", "small")
		require.Eventually(t, func() bool { return indexOfCall(env.dockerCalls(t), "up -d --wait api") >= 0 }, 10*time.Second, 50*time.Millisecond)
		time.Sleep(500 * time.Millisecond)

		require.NoError(t, cmd.Process.Signal(os.Interrupt))
		err := cmd.Wait()

		var exit *exec.ExitError
		require.ErrorAs(t, err, &exit)
		require.Equal(t, 130, exit.ExitCode())
		var report map[string]any
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
		require.Equal(t, "interrupted", report["status"])
		manifest := readJSON(t, filepath.Join(resultDir(t, env), "manifest.json"))
		require.Contains(t, manifest["reasons"], "interrupted")
	})
}

func TestRun_RefusesToSendWhenDatabaseNoLongerMatchesPreparedFixture(t *testing.T) {
	api := newAPIFixture(t)
	env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"run --rm", "tool verify --dataset small"}, Stdout: seedSummary("sha256:changed")})

	stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "smoke", "--dataset", "small")

	require.Equal(t, 5, status, stderr)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	require.Contains(t, report["reasons"], "result_incomplete")
	require.Equal(t, -1, indexOfCall(env.dockerCalls(t), "k6 run"))
	require.Equal(t, 0, api.loginCount())
}

func TestRun_SavesStageCSVResourceSamplesAndConditionMetadata(t *testing.T) {
	api := newAPIFixture(t)
	env, extra := newRunEnv(t, api, dockerRule{Contains: []string{"run --rm", "k6 run"}, Writes: smokeWrites(), SleepMS: 1500})

	_, stderr, status := env.runWith(t, extra, "run", "--scenario", "smoke", "--dataset", "small")

	require.Equal(t, 0, status, stderr)
	dir := resultDir(t, env)
	csv, err := os.ReadFile(filepath.Join(dir, "report.csv"))
	require.NoError(t, err)
	// smokeは容量段階を持たないため見出しだけになる。値の対応はWriteReportCSVの単体テストで確認する。
	require.True(t, strings.HasPrefix(string(csv), "stage,requested_rate,planned_operations,"), string(csv))
	require.Equal(t, 1, strings.Count(string(csv), "\n"))
	resources, err := os.ReadFile(filepath.Join(dir, "resources.jsonl"))
	require.NoError(t, err)
	var sample map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.SplitN(string(resources), "\n", 2)[0]), &sample))
	require.Equal(t, "api", sample["service"])
	// Dockerの値は100%=1CPUで、上限1CPUのAPIではquota比も同じ0.1になる。
	require.Equal(t, 10.0, sample["cpu_percent"])
	require.Equal(t, 0.1, sample["cpu_quota_ratio"])
	require.Equal(t, 0.1, sample["memory_ratio"])
	require.NotEmpty(t, sample["time"])

	manifest := readJSON(t, filepath.Join(dir, "manifest.json"))
	plan := manifest["plan"].(map[string]any)
	require.Equal(t, "smoke", plan["scenario"])
	fixture := manifest["fixture"].(map[string]any)
	require.Equal(t, smallChecksum, fixture["checksum"])
	trace := manifest["trace"].(map[string]any)
	require.Equal(t, "threadly-api-loadtest", trace["service_name"])
	require.Equal(t, "parentbased_always_on", trace["sampler"])
	environment := manifest["environment"].(map[string]any)
	require.Equal(t, "28.5.1", environment["docker_server_version"])
	require.Equal(t, "arm64", environment["architecture"])
	require.Equal(t, float64(4), environment["vm_cpu"])
	require.Contains(t, manifest, "started_at")
	require.Contains(t, manifest, "finished_at")
	require.Contains(t, manifest["monitor"], "cpu_seconds")
	require.Equal(t, float64(10*1000*1000), manifest["trace_volume_bytes"])
	// 前回runから残るTrace量も比較条件にする。
	require.Equal(t, float64(10*1000*1000), manifest["trace_volume_bytes_start"])
	require.Contains(t, manifest["monitor"], "children_cpu_seconds")
	// 上限は起動後に実containerから読む。起動前に読むと未作成のcontainerが欠ける。
	limits := environment["container_limits"].([]any)
	require.Equal(t, float64(1000000000), limits[0].(map[string]any)["nano_cpus"])
	calls := env.dockerCalls(t)
	require.Greater(t, indexOfCall(calls, "NanoCpus"), indexOfCall(calls, "up -d --wait api"))
	k6Limits := readJSON(t, filepath.Join(dir, "k6-limits-smoke.json"))
	require.Equal(t, float64(1000000000), k6Limits["nano_cpus"])
}

func TestRun_IgnoresK6StatsFailureWhileK6ContainerIsBeingRemovedAtNormalExit(t *testing.T) {
	api := newAPIFixture(t)
	// compose run --rmはk6 containerを削除してからCLIが終わるため、その間のstatsは失敗する。
	env, extra := newRunEnv(t, api,
		dockerRule{Contains: []string{"stats --no-stream", "threadly-loadtest-k6-smoke"}, SleepMS: 200, Exit: 1},
		dockerRule{Contains: []string{"run --rm", "k6 run"}, Writes: smokeWrites(), SleepMS: 5600},
	)

	stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "smoke", "--dataset", "small")

	require.Equal(t, 0, status, stderr+stdout)
}

func residentStats() string {
	var stats string
	for _, service := range []string{"api", "loadtest-db", "otel-collector", "tempo", "grafana"} {
		stats += statsLine("threadly-loadtest-"+service+"-1", "10.00%", "10.00%")
	}
	return stats
}

// steadyBrowseWritesは、停滞判定にかからないよう、warmup中に成功HTTPを一定間隔で書き出す。
func steadyBrowseWrites(count, intervalMS int) []map[string]any {
	var writes []map[string]any
	for i := 0; i < count; i++ {
		next := browseWrites("warmup", "warmup", "success", 1)
		next[0]["delay_ms"] = intervalMS
		writes = append(writes, next...)
	}
	return writes
}

func TestRun_StopsOnGeneratorPressureAndStorageLimits(t *testing.T) {
	cases := []struct {
		name       string
		rules      []dockerRule
		wantStatus int
		wantReason string
	}{
		{
			name:       "k6のメモリ上限比90%以上は生成側の圧迫として停止する",
			rules:      []dockerRule{{Contains: []string{"stats --no-stream"}, Stdout: residentStats() + statsLine("threadly-loadtest-k6-warmup", "10.00%", "90.00%")}},
			wantStatus: 3, wantReason: "generator_pressure",
		},
		{
			name:       "Trace volumeが1GiBに達したら停止する",
			rules:      []dockerRule{{Contains: []string{"system df -v"}, AfterCalls: 1, Stdout: `[{"Name":"threadly-loadtest_loadtest_trace_data","Size":"1.1GB"}]`}},
			wantStatus: 4, wantReason: "disk_limit",
		},
		{
			name:       "Trace volumeが見つからなければ0扱いにせず停止する",
			rules:      []dockerRule{{Contains: []string{"system df -v"}, AfterCalls: 1, Stdout: `[{"Name":"other","Size":"9GB"}]`}},
			wantStatus: 4, wantReason: "monitoring_failed",
		},
		{
			name:       "起動済みk6の資源値が欠けたら圧迫を見逃さないよう停止する",
			rules:      []dockerRule{{Contains: []string{"stats --no-stream", "threadly-loadtest-k6-warmup"}, Stdout: residentStats()}},
			wantStatus: 4, wantReason: "monitoring_failed",
		},
		{
			name:       "送信前にTrace量を計測できなければ送信しない",
			rules:      []dockerRule{{Contains: []string{"system df -v"}, Exit: 1}},
			wantStatus: 4, wantReason: "monitoring_failed",
		},
		{
			name:       "保存量を計測できなければ合格扱いにせず停止する",
			rules:      []dockerRule{{Contains: []string{"system df -v"}, AfterCalls: 1, Exit: 1}},
			wantStatus: 4, wantReason: "monitoring_failed",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			api := newAPIFixture(t)
			rules := append(tt.rules, dockerRule{Contains: []string{"k6 run"}, Writes: steadyBrowseWrites(10, 1000), SleepMS: 30000})
			env, extra := newRunEnv(t, api, rules...)

			stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "browse", "--dataset", "small")

			require.Equal(t, tt.wantStatus, status, stderr+stdout)
			var report map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &report))
			require.Contains(t, report["reasons"], tt.wantReason)
			if strings.HasPrefix(tt.name, "送信前") {
				require.Equal(t, -1, indexOfCall(env.dockerCalls(t), "k6 run"))
				return
			}
			require.GreaterOrEqual(t, indexOfCall(env.dockerCalls(t), "kill -s SIGINT threadly-loadtest-k6-warmup"), 0)
		})
	}
}

func TestRun_StopsWhenK6CPUQuotaStaysAtNinetyPercentForThirtySeconds(t *testing.T) {
	if testing.Short() {
		t.Skip("30秒の継続を実時間で待つため、-shortでは実行しない")
	}
	api := newAPIFixture(t)
	// k6の上限0.75CPUに対し68%は0.91倍で、quota比90%以上が続く状態を表す。
	env, extra := newRunEnv(t, api,
		dockerRule{Contains: []string{"stats --no-stream"}, Stdout: residentStats() + statsLine("threadly-loadtest-k6-warmup", "68.00%", "10.00%")},
		dockerRule{Contains: []string{"k6 run"}, Writes: steadyBrowseWrites(45, 1000), SleepMS: 30000},
	)
	started := time.Now()

	stdout, stderr, status := env.runWith(t, extra, "run", "--scenario", "browse", "--dataset", "small")

	require.Equal(t, 3, status, stderr+stdout)
	require.GreaterOrEqual(t, time.Since(started), 30*time.Second)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	require.Contains(t, report["reasons"], "generator_pressure")
}

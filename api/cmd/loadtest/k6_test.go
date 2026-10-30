package main_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const fixturePost = "11111111-1111-4111-8111-111111111111"
const fixtureComment = "22222222-2222-4222-8222-222222222222"
const fixtureReply = "33333333-3333-4333-8333-333333333333"
const fixtureUser = "44444444-4444-4444-8444-444444444444"
const fixtureCookie = "private-fixture-cookie"

type receivedRequest struct {
	Method, Path, Cookie, Body string
}

type k6Fixture struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []receivedRequest
	fault    func(http.ResponseWriter, *http.Request) bool
}

func newK6Fixture(t *testing.T) *k6Fixture {
	t.Helper()
	f := &k6Fixture{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, receivedRequest{r.Method, r.URL.Path, r.Header.Get("Cookie"), string(body)})
		f.mu.Unlock()
		if f.fault != nil && f.fault(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		author := map[string]any{"id": fixtureUser, "username": "fixture-user"}
		post := map[string]any{"id": fixturePost, "title": "fixture title", "content": "fixture content", "author": author, "likeCount": 0, "likedByMe": false}
		reply := map[string]any{"id": fixtureReply, "content": "reply", "author": author, "likeCount": 0, "likedByMe": false, "replies": []any{}}
		comment := map[string]any{"id": fixtureComment, "content": "comment", "author": author, "likeCount": 0, "likedByMe": false, "replies": []any{reply}}
		var response any
		switch {
		case r.URL.Path == "/health":
			response = map[string]any{"status": "ok"}
		case r.URL.Path == "/api/posts" && r.Header.Get("Cookie") == "":
			w.WriteHeader(http.StatusUnauthorized)
			return
		case r.URL.Path == "/api/auth/login":
			http.SetCookie(w, &http.Cookie{Name: "threadly-session", Value: "private-login-cookie", Path: "/"})
			response = map[string]any{"user": author}
		case strings.HasSuffix(r.URL.Path, "/like"):
			target := fixturePost
			if strings.HasPrefix(r.URL.Path, "/api/comments/") {
				target = fixtureComment
			}
			response = map[string]any{"targetId": target, "likeCount": 0, "likedByMe": r.Method == http.MethodPut}
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			return
		case strings.HasSuffix(r.URL.Path, "/comments"):
			response = []any{comment}
		case r.URL.Path == "/api/posts":
			response = []any{post}
		default:
			response = post
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(f.server.Close)
	return f
}

type k6RunOptions struct {
	VUs      int
	Accounts []map[string]any
	Fixture  map[string]any
}

func runK6Operations(t *testing.T, f *k6Fixture, scenario string, iterations int, variants ...k6RunOptions) ([]metricPoint, string) {
	t.Helper()
	k6 := os.Getenv("K6_BINARY")
	if k6 == "" {
		t.Skip("K6_BINARY is required for real k6 functional tests")
	}
	dir := t.TempDir()
	module, err := filepath.Abs("../../loadtest/scripts/operations.js")
	require.NoError(t, err)
	config := map[string]any{"base_url": f.server.URL, "run_id": "fixture-run", "scenario": scenario, "phase": "measure", "stage": "1", "fixture": map[string]any{"post_ids": []string{fixturePost}, "parent_comment_ids": []string{fixtureComment}, "reply_ids": []string{fixtureReply}}}
	if scenario == "login" {
		delete(config, "fixture")
	}
	if scenario == "smoke" {
		config["phase"], config["stage"] = "smoke", "smoke"
	}
	accounts := `[ {"id":"` + fixtureUser + `","username":"fixture-user","password":"private-fixture-password","cookie":"` + fixtureCookie + `"} ]`
	vus := 1
	if len(variants) > 0 {
		vus = variants[0].VUs
		config["fixture"] = variants[0].Fixture
		data, err := json.Marshal(variants[0].Accounts)
		require.NoError(t, err)
		accounts = string(data)
	}
	encodedConfig, err := json.Marshal(config)
	require.NoError(t, err)
	authPath := filepath.Join(dir, "accounts.json")
	require.NoError(t, os.WriteFile(authPath, []byte(accounts), 0600))
	moduleJSON, _ := json.Marshal(module)
	authJSON, _ := json.Marshal(authPath)
	script := `import { executeOperation } from ` + string(moduleJSON) + `;
const accounts = JSON.parse(open(` + string(authJSON) + `));
const config = ` + string(encodedConfig) + `;
export const options = { vus: ` + jsonNumber(vus) + `, iterations: ` + jsonNumber(iterations) + `, maxDuration: '15s', systemTags: ['status','method','name'], tags: {run_id:config.run_id,scenario:config.scenario,phase:config.phase,stage:config.stage} };
export default function() { executeOperation(config, accounts); }
`
	scriptPath := filepath.Join(dir, "fixture.js")
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0600))
	pointsPath := filepath.Join(dir, "points.jsonl")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, k6, "run", "--quiet", "--no-usage-report", "--out", "json="+pointsPath, scriptPath)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "K6_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	output, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err())
	require.NoError(t, err, string(output))
	data, err := os.ReadFile(pointsPath)
	require.NoError(t, err)
	var points []metricPoint
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		var point metricPoint
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &point))
		if point.Type == "Point" {
			points = append(points, point)
		}
	}
	require.NoError(t, scanner.Err())
	return points, string(output) + string(data)
}

func jsonNumber(value int) string { encoded, _ := json.Marshal(value); return string(encoded) }

func TestK6_MixedCycleSendsTwentyOperationsAndTwentyTwoHTTP(t *testing.T) {
	f := newK6Fixture(t)
	points, output := runK6Operations(t, f, "mixed", 20)
	f.mu.Lock()
	requests := append([]receivedRequest(nil), f.requests...)
	f.mu.Unlock()
	require.Len(t, requests, 22)
	for i, r := range requests {
		require.Equal(t, "threadly-session="+fixtureCookie, r.Cookie)
		if i < 16 {
			require.Equal(t, http.MethodGet, r.Method)
		}
	}
	require.Equal(t, []string{"POST", "POST", "PUT", "DELETE", "PUT", "DELETE"}, []string{requests[16].Method, requests[17].Method, requests[18].Method, requests[19].Method, requests[20].Method, requests[21].Method})
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(requests[16].Body), &body))
	require.Equal(t, map[string]any{"title": "loadtest post", "content": "loadtest content"}, body)
	require.Equal(t, `{"content":"loadtest comment"}`, requests[17].Body)
	counts := map[string]int{}
	for _, p := range points {
		if strings.HasPrefix(p.Metric, "lt_") {
			counts[p.Metric]++
			require.Equal(t, "fixture-run", p.Data.Tags["run_id"])
			if p.Metric == "lt_http_duration" || p.Metric == "lt_operations_completed" {
				require.Equal(t, "success", p.Data.Tags["outcome"])
			}
		}
	}
	require.Equal(t, 22, counts["lt_http_duration"])
	require.Equal(t, 20, counts["lt_operations_started"])
	require.Equal(t, 20, counts["lt_operations_completed"])
	require.NotContains(t, output, fixtureCookie)
	require.NotContains(t, output, "private-fixture-password")
}

func TestK6_SemanticFailureStopsLikePairAndMarksHTTPAndOperation(t *testing.T) {
	f := newK6Fixture(t)
	f.fault = func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPut {
			_, _ = io.WriteString(w, `{"targetId":"wrong-target","likeCount":0,"likedByMe":true}`)
			return true
		}
		return false
	}
	points, output := runK6Operations(t, f, "mixed", 20)
	f.mu.Lock()
	requests := append([]receivedRequest(nil), f.requests...)
	f.mu.Unlock()
	require.Len(t, requests, 20)
	for _, r := range requests {
		require.NotEqual(t, http.MethodDelete, r.Method)
	}
	failures := map[string]int{}
	for _, p := range points {
		if p.Data.Tags["outcome"] == "failure" {
			failures[p.Metric]++
		}
	}
	require.Equal(t, 2, failures["lt_http_duration"])
	require.Equal(t, 2, failures["lt_operations_completed"])
	require.NotContains(t, output, "wrong-target")
}

func TestK6_BrowseCyclesThreeReadsWithoutWrites(t *testing.T) {
	f := newK6Fixture(t)
	points, _ := runK6Operations(t, f, "browse", 21)
	f.mu.Lock()
	requests := append([]receivedRequest(nil), f.requests...)
	f.mu.Unlock()
	require.Len(t, requests, 21)
	paths := []string{"/api/posts", "/api/posts/" + fixturePost, "/api/posts/" + fixturePost + "/comments"}
	for i, r := range requests {
		require.Equal(t, "GET", r.Method)
		require.Equal(t, paths[i%3], r.Path)
	}
	for _, p := range points {
		if p.Metric == "lt_http_duration" {
			require.Equal(t, "success", p.Data.Tags["outcome"])
			require.NotContains(t, p.Data.Tags["endpoint"], fixturePost)
			require.NotContains(t, p.Data.Tags, "url")
		}
	}
}

func TestK6_ReadsRejectInvalidJSONAndMissingFixtureContent(t *testing.T) {
	for _, body := range []string{`not-json-private-body`, `[]`, `{}`, `{"id":"wrong","author":{"id":"user"},"title":"title","content":"content","likeCount":-1,"likedByMe":false}`} {
		t.Run(body, func(t *testing.T) {
			f := newK6Fixture(t)
			f.fault = func(w http.ResponseWriter, _ *http.Request) bool { _, _ = io.WriteString(w, body); return true }
			points, output := runK6Operations(t, f, "browse", 3)
			failures := map[string]int{}
			for _, p := range points {
				if p.Data.Tags["outcome"] == "failure" {
					failures[p.Metric]++
				}
			}
			require.Equal(t, 3, failures["lt_http_duration"])
			require.Equal(t, 3, failures["lt_operations_completed"])
			require.NotContains(t, output, "not-json-private-body")
		})
	}
}

func TestK6_LoginSendsOnlyCredentialsAndDoesNotReuseCookie(t *testing.T) {
	f := newK6Fixture(t)
	points, output := runK6Operations(t, f, "login", 3)
	f.mu.Lock()
	requests := append([]receivedRequest(nil), f.requests...)
	f.mu.Unlock()
	require.Len(t, requests, 3)
	for _, r := range requests {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/api/auth/login", r.Path)
		require.Empty(t, r.Cookie)
		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(r.Body), &body))
		require.Equal(t, map[string]any{"username": "fixture-user", "password": "private-fixture-password"}, body)
	}
	for _, p := range points {
		if p.Metric == "lt_http_duration" {
			require.Equal(t, "success", p.Data.Tags["outcome"])
		}
	}
	require.NotContains(t, output, "private-login-cookie")
	require.NotContains(t, output, "private-fixture-password")
}

func TestK6_SmokeRunsElevenHTTPIncludingExpectedUnauthorized(t *testing.T) {
	f := newK6Fixture(t)
	points, output := runK6Operations(t, f, "smoke", 1)
	f.mu.Lock()
	requests := append([]receivedRequest(nil), f.requests...)
	f.mu.Unlock()
	require.Len(t, requests, 11)
	require.Equal(t, "/health", requests[0].Path)
	require.Empty(t, requests[0].Cookie)
	require.Equal(t, "/api/posts", requests[1].Path)
	require.Empty(t, requests[1].Cookie)
	for _, r := range requests[2:] {
		require.Equal(t, "threadly-session="+fixtureCookie, r.Cookie)
	}
	count := 0
	postLists := 0
	for _, p := range points {
		if p.Metric == "lt_http_duration" {
			count++
			require.Equal(t, "success", p.Data.Tags["outcome"])
			if p.Data.Tags["endpoint"] == "GET /api/posts" {
				postLists++
			}
		}
		if p.Metric == "http_req_failed" {
			require.Zero(t, p.Data.Value)
		}
	}
	require.Equal(t, 11, count)
	require.Equal(t, 2, postLists)
	require.NotContains(t, output, fixtureCookie)
}

func TestK6_TransportFailureIsSeparateFromHTTPStatusFailure(t *testing.T) {
	for _, transport := range []bool{true, false} {
		t.Run(jsonNumber(map[bool]int{true: 1, false: 0}[transport]), func(t *testing.T) {
			f := newK6Fixture(t)
			f.fault = func(w http.ResponseWriter, _ *http.Request) bool {
				if transport {
					connection, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = connection.Close()
					}
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = io.WriteString(w, "private-error-body")
				}
				return true
			}
			points, output := runK6Operations(t, f, "browse", 1)
			count := 0
			for _, p := range points {
				if p.Metric == "lt_transport_errors" {
					count++
				}
				if p.Metric == "lt_http_duration" {
					require.Equal(t, "failure", p.Data.Tags["outcome"])
				}
			}
			if transport {
				require.Equal(t, 1, count)
			} else {
				require.Zero(t, count)
			}
			require.NotContains(t, output, "private-error-body")
		})
	}
}

func TestK6_TimeoutStopsOneRequestWithinFiveSeconds(t *testing.T) {
	f := newK6Fixture(t)
	f.fault = func(_ http.ResponseWriter, r *http.Request) bool { <-r.Context().Done(); return true }
	start := time.Now()
	points, _ := runK6Operations(t, f, "browse", 1)
	require.Less(t, time.Since(start), 10*time.Second)
	count := 0
	for _, p := range points {
		if p.Metric == "lt_transport_errors" {
			count++
			require.Equal(t, "timeout", p.Data.Tags["kind"])
		}
	}
	require.Equal(t, 1, count)
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.requests, 1)
}

func TestK6_RedirectIsNotFollowedOrRetried(t *testing.T) {
	f := newK6Fixture(t)
	f.fault = func(w http.ResponseWriter, _ *http.Request) bool {
		w.Header().Set("Location", f.server.URL+"/must-not-follow")
		w.WriteHeader(http.StatusFound)
		return true
	}
	points, _ := runK6Operations(t, f, "browse", 1)
	for _, p := range points {
		if p.Metric == "lt_http_duration" {
			require.Equal(t, "failure", p.Data.Tags["outcome"])
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.requests, 1)
}

func TestK6_LoginRejectsWrongIdentityAndMissingCookie(t *testing.T) {
	for _, body := range []string{`{"user":{"id":"wrong","username":"fixture-user"}}`, `{"user":{"id":"` + fixtureUser + `","username":"fixture-user"}}`} {
		t.Run(body, func(t *testing.T) {
			f := newK6Fixture(t)
			f.fault = func(w http.ResponseWriter, _ *http.Request) bool { _, _ = io.WriteString(w, body); return true }
			points, _ := runK6Operations(t, f, "login", 1)
			for _, p := range points {
				if p.Metric == "lt_http_duration" || p.Metric == "lt_operations_completed" {
					require.Equal(t, "failure", p.Data.Tags["outcome"])
				}
			}
		})
	}
}

func TestK6_ResponseCookiesCannotReplaceAssignedCookie(t *testing.T) {
	f := newK6Fixture(t)
	f.fault = func(w http.ResponseWriter, _ *http.Request) bool {
		http.SetCookie(w, &http.Cookie{Name: "threadly-session", Value: "private-poison-cookie", Path: "/"})
		return false
	}
	_, output := runK6Operations(t, f, "browse", 3)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.requests {
		require.Equal(t, "threadly-session="+fixtureCookie, r.Cookie)
	}
	require.NotContains(t, output, "private-poison-cookie")
}

func TestK6_ScenarioOptionsFixArrivalRateVUsAndBoundedDuration(t *testing.T) {
	for _, tc := range []struct {
		scenario, phase, stage, executor, duration string
		rate, vus                                  int
	}{
		{"mixed", "measure", "6", "constant-arrival-rate", "120s", 40, 32},
		{"browse", "warmup", "warmup", "constant-arrival-rate", "60s", 1, 32},
		{"login", "measure", "6", "constant-arrival-rate", "120s", 6, 8},
		{"smoke", "smoke", "smoke", "shared-iterations", "30s", 0, 1},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			config := scenarioConfig(tc.scenario, tc.phase, tc.stage)
			accounts := scenarioAccounts(tc.vus)
			output, err := inspectK6Scenario(t, config, accounts)
			require.NoError(t, err, string(output))
			var options struct {
				Scenarios map[string]struct {
					Executor, Duration, TimeUnit, GracefulStop, MaxDuration string
					Rate, PreAllocatedVUs, MaxVUs, VUs, Iterations          int
				}
				SystemTags []string
			}
			require.NoError(t, json.Unmarshal(output, &options))
			require.Len(t, options.Scenarios, 1)
			stage := options.Scenarios["stage"]
			require.Equal(t, tc.executor, stage.Executor)
			if tc.scenario == "smoke" {
				require.Equal(t, tc.duration, stage.MaxDuration)
				require.Equal(t, 1, stage.VUs)
				require.Equal(t, 1, stage.Iterations)
				require.Equal(t, "0s", stage.GracefulStop)
			} else {
				expectedDuration, err := time.ParseDuration(tc.duration)
				require.NoError(t, err)
				actualDuration, err := time.ParseDuration(stage.Duration)
				require.NoError(t, err)
				require.Equal(t, expectedDuration, actualDuration)
				require.Equal(t, "1s", stage.TimeUnit)
				require.Equal(t, tc.rate, stage.Rate)
				require.Equal(t, tc.vus, stage.PreAllocatedVUs)
				require.Equal(t, tc.vus, stage.MaxVUs)
				require.Equal(t, "10s", stage.GracefulStop)
			}
			require.NotContains(t, options.SystemTags, "url")
			require.NotContains(t, options.SystemTags, "scenario")
			require.NotContains(t, string(output), fixtureCookie)
			require.NotContains(t, string(output), "private-fixture-password")
		})
	}
}

func scenarioConfig(scenario, phase, stage string) map[string]any {
	dataset := "medium"
	if scenario == "browse" || scenario == "smoke" {
		dataset = "small"
	}
	return map[string]any{"schema_version": 1, "base_url": "http://api:8080", "run_id": "fixture-run", "scenario": scenario, "dataset": dataset, "phase": phase, "stage": stage, "fixture": map[string]any{"post_ids": []string{fixturePost}, "parent_comment_ids": []string{fixtureComment}, "reply_ids": []string{fixtureReply}}}
}

func scenarioAccounts(count int) []map[string]any {
	accounts := make([]map[string]any, count)
	for i := range accounts {
		accounts[i] = map[string]any{"id": fmt.Sprintf("44444444-4444-4444-8444-%012d", i), "username": fmt.Sprintf("fixture-user-%d", i), "password": "private-fixture-password", "cookie": fmt.Sprintf("%s-%d", fixtureCookie, i)}
	}
	return accounts
}

func inspectK6Scenario(t *testing.T, config map[string]any, accounts []map[string]any) ([]byte, error) {
	t.Helper()
	k6 := os.Getenv("K6_BINARY")
	if k6 == "" {
		t.Skip("K6_BINARY is required for real k6 functional tests")
	}
	dir := t.TempDir()
	data, err := json.Marshal(config)
	require.NoError(t, err)
	configPath := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(configPath, data, 0600))
	data, err = json.Marshal(accounts)
	require.NoError(t, err)
	authPath := filepath.Join(dir, "accounts.json")
	require.NoError(t, os.WriteFile(authPath, data, 0600))
	script, err := filepath.Abs("../../loadtest/scripts/scenario.js")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, k6, "inspect", "-e", "LOADTEST_CONFIG="+configPath, "-e", "LOADTEST_ACCOUNTS="+authPath, script)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "K6_") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	output, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err())
	return output, err
}

func TestK6_ScenarioRejectsInvalidInputsWithoutEchoingSecrets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any, []map[string]any)
	}{
		{"external-target", func(c map[string]any, _ []map[string]any) { c["base_url"] = "http://private-input-value.invalid" }},
		{"invalid-stage", func(c map[string]any, _ []map[string]any) { c["stage"] = "private-input-value" }},
		{"invalid-dataset", func(c map[string]any, _ []map[string]any) { c["dataset"] = "large" }},
		{"invalid-schema", func(c map[string]any, _ []map[string]any) { c["schema_version"] = 2 }},
		{"fixture-path-injection", func(c map[string]any, _ []map[string]any) {
			c["fixture"] = map[string]any{"post_ids": []string{"../private-input-value"}}
		}},
		{"missing-cookie", func(_ map[string]any, a []map[string]any) { delete(a[0], "cookie") }},
		{"duplicate-user", func(_ map[string]any, a []map[string]any) { a[1]["id"] = a[0]["id"] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := scenarioConfig("mixed", "measure", "1")
			accounts := scenarioAccounts(32)
			tc.mutate(config, accounts)
			output, err := inspectK6Scenario(t, config, accounts)
			require.Error(t, err)
			require.Contains(t, string(output), "loadtest invalid_input")
			require.NotContains(t, string(output), "private-input-value")
			require.NotContains(t, string(output), fixtureCookie)
			require.NotContains(t, string(output), "private-fixture-password")
		})
	}
}

func TestK6_GlobalMixedCycleKeepsEachVUsCookieAndLikeTargets(t *testing.T) {
	const secondPost = "55555555-5555-4555-8555-555555555555"
	const secondComment = "66666666-6666-4666-8666-666666666666"
	f := newK6Fixture(t)
	f.fault = func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasSuffix(r.URL.Path, "/like") {
			return false
		}
		parts := strings.Split(r.URL.Path, "/")
		_ = json.NewEncoder(w).Encode(map[string]any{"targetId": parts[3], "likeCount": 0, "likedByMe": r.Method == "PUT"})
		return true
	}
	fixture := map[string]any{"post_ids": []string{fixturePost, secondPost}, "parent_comment_ids": []string{fixtureComment, secondComment}, "reply_ids": []string{fixtureReply, fixtureReply}}
	points, _ := runK6Operations(t, f, "mixed", 40, k6RunOptions{VUs: 2, Accounts: scenarioAccounts(2), Fixture: fixture})
	f.mu.Lock()
	requests := append([]receivedRequest(nil), f.requests...)
	f.mu.Unlock()
	require.Len(t, requests, 44)
	cookies := map[string]bool{}
	for _, r := range requests {
		cookies[r.Cookie] = true
		if strings.HasSuffix(r.Path, "/like") {
			post, comment := fixturePost, fixtureComment
			if r.Cookie == "threadly-session="+fixtureCookie+"-1" {
				post, comment = secondPost, secondComment
			}
			if strings.HasPrefix(r.Path, "/api/posts/") {
				require.Equal(t, "/api/posts/"+post+"/like", r.Path)
			} else {
				require.Equal(t, "/api/comments/"+comment+"/like", r.Path)
			}
		}
	}
	require.Equal(t, map[string]bool{"threadly-session=" + fixtureCookie + "-0": true, "threadly-session=" + fixtureCookie + "-1": true}, cookies)
	completed := 0
	for _, p := range points {
		if p.Metric == "lt_operations_completed" {
			completed++
		}
	}
	require.Equal(t, 40, completed)
}

func TestK6_SmokePointsCanBeEvaluatedByPublicReport(t *testing.T) {
	f := newK6Fixture(t)
	start := time.Now().UTC()
	points, _ := runK6Operations(t, f, "smoke", 1)
	end := time.Now().UTC()
	manifest := reportManifest{SchemaVersion: 1, RunID: "fixture-run", Scenario: "smoke", Dataset: "small", Finalized: true, Stages: []reportStage{{Phase: "smoke", Stage: "smoke", Start: start, SendEnd: end, End: end, Complete: true}}}
	manifestPath, pointsPath := reportFiles(t, manifest, points)
	stdout, stderr, status := command(t, "report", "--manifest", manifestPath, "--points", pointsPath)
	require.Equal(t, 0, status, stderr+stdout)
	require.Empty(t, stderr)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	require.Equal(t, "smoke_passed", report["status"])
	require.Equal(t, float64(0), report["max_qualified_rate"])
}

package loadtest

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultAPIURL     = "http://127.0.0.1:18080"
	defaultTempoURL   = "http://127.0.0.1:13200"
	defaultGrafanaURL = "http://127.0.0.1:13000"
	// k6からはCompose network内のAPIだけへ送る。ホスト側の確認・事前認証は127.0.0.1の公開portを使う。
	k6BaseURL = "http://api:8080"

	maxVMCPU         = 4
	maxVMMemoryBytes = 6 * 1024 * 1024 * 1024
	vmCPUHeadroom    = 0.5
	vmMemoryHeadroom = 1024 * 1024 * 1024

	monitorInterval    = 5 * time.Second
	healthTimeout      = 2 * time.Second
	drainLimit         = 10 * time.Second
	stopRequestAt      = 890 * time.Second
	stageStartMargin   = 10 * time.Second
	startupGrace       = 30 * time.Second
	httpStallLimit     = 10 * time.Second
	smokeLimit         = 30 * time.Second
	cookieMinRemaining = 30 * time.Minute
	sessionLifetime    = time.Hour
)

// monitoredServicesは、測定中に停止・OOM・再起動を監視する常駐サービスである。
var monitoredServices = []string{"api", "loadtest-db", "otel-collector", "tempo", "grafana"}

var loopbackURL = regexp.MustCompile(`^http://127\.0\.0\.1:[0-9]{1,5}$`)

// RunOutcomeは、保存した結果の場所と再生成したreportを表す。
type RunOutcome struct {
	RunID  string
	Dir    string
	Report Report
}

type account struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Cookie   string `json:"cookie,omitempty"`
	Password string `json:"password,omitempty"`
}

type stageSpec struct {
	phase, stage string
	duration     time.Duration
}

// StageFitsは、900秒上限の停止要求時刻までに、段階の送信・drain・停止余裕を確保できるかを返す。
// 確保できない段階は短縮せずに開始しない。
func StageFits(elapsed, sendDuration time.Duration) bool {
	return elapsed+sendDuration+drainLimit+stageStartMargin <= stopRequestAt
}

// Runは、準備済みfixtureと実環境を照合し、疎通または1つのシナリオを段階ごとに実行する。
// 新規送信を止めた後も結果を保存し、reportと同じ規則で終了コードを決める。
func (e Environment) Run(ctx context.Context, scenario, dataset string) (RunOutcome, error) {
	plan, err := NewPlan(scenario, dataset)
	if err != nil {
		return RunOutcome{}, invalidInput("invalid_input")
	}
	secrets, err := e.readSecrets()
	if err != nil {
		return RunOutcome{}, err
	}
	apiURL, err := e.apiURL()
	if err != nil {
		return RunOutcome{}, err
	}
	unlock, err := e.lock()
	if err != nil {
		return RunOutcome{}, err
	}
	defer unlock()
	state, prepared, err := e.readState()
	if err != nil {
		return RunOutcome{}, err
	}
	// 書き込みを含む試験の後はfixtureが変わるため、比較用runは常にreset/prepare済みの状態から始める。
	if !prepared || state.Used || state.Dataset != dataset || state.Version != FixtureVersion {
		return RunOutcome{}, invalidInput("fixture_not_ready")
	}
	fixture, err := GenerateFixture(dataset)
	if err != nil || fixture.Checksum() != state.Checksum {
		return RunOutcome{}, invalidInput("fixture_not_ready")
	}

	prepareCtx, cancelPrepare := context.WithTimeout(ctx, prepareTimeout)
	defer cancelPrepare()
	if err := e.preflight(prepareCtx, plan); err != nil {
		return RunOutcome{}, err
	}
	if err := e.verifyTarget(prepareCtx); err != nil {
		return RunOutcome{}, err
	}
	runID, dir, err := e.createRunDir()
	if err != nil {
		return RunOutcome{}, err
	}
	manifest := Manifest{SchemaVersion: 1, RunID: runID, Scenario: scenario, Dataset: dataset, Reasons: []string{}}
	// run IDの作成後は、準備中の失敗や中断でも理由を保存してから終了する。
	prepareFailed := func(reason string) (RunOutcome, error) {
		if ctx.Err() != nil {
			reason = "interrupted"
		}
		manifest.Reasons = appendReason(manifest.Reasons, reason)
		return e.finalize(dir, manifest, nil)
	}
	if _, err := e.compose(prepareCtx, "up", "-d", "--wait", "api", "otel-collector", "tempo", "grafana"); err != nil {
		return prepareFailed("environment")
	}
	if reason := e.waitReady(prepareCtx, apiURL, dataset, state.FixtureSummary); reason != "" {
		return prepareFailed(reason)
	}
	accounts, loggedInAt, err := authenticate(prepareCtx, apiURL, scenario, plan, fixture, secrets["LOADTEST_USER_PASSWORD"])
	if err != nil {
		return prepareFailed("environment")
	}
	if err := checkAuthenticatedRead(prepareCtx, apiURL, scenario, accounts); err != nil {
		return prepareFailed("environment")
	}
	runtimeDir := filepath.Join(e.Root, "loadtest", "runtime", runID)
	defer func() { _ = os.RemoveAll(runtimeDir) }()
	if err := writePrivateJSON(runtimeDir, "accounts.json", accounts); err != nil {
		return prepareFailed("result_incomplete")
	}
	// 送信開始前に使用済みとし、途中停止したrunのデータを次の比較へ流用しない。
	state.Used = true
	if err := e.writeState(state); err != nil {
		return prepareFailed("result_incomplete")
	}

	var points bytes.Buffer
	firstSend := time.Now()
	clock := stageClock{firstSend: firstSend}
	baseline := &serviceBaseline{restarts: map[string]int{}}
	measureCtx, cancelMeasure := context.WithDeadline(ctx, firstSend.Add(stopRequestAt))
	defer cancelMeasure()
	for _, spec := range plannedStages(plan) {
		// 中断・watchdogの停止要求後は、次段階のk6を起動しない。
		if measureCtx.Err() != nil {
			reason := "result_incomplete"
			if ctx.Err() != nil {
				reason = "interrupted"
			}
			manifest.Reasons = appendReason(manifest.Reasons, reason)
			break
		}
		if !StageFits(time.Since(firstSend), spec.duration) {
			manifest.Reasons = appendReason(manifest.Reasons, "result_incomplete")
			break
		}
		if time.Until(loggedInAt.Add(sessionLifetime)) < cookieMinRemaining {
			// 事前Cookieの残り期限が測定中に切れないよう、段階開始前に取り直す。
			accounts, loggedInAt, err = authenticate(measureCtx, apiURL, scenario, plan, fixture, secrets["LOADTEST_USER_PASSWORD"])
			if err != nil {
				reason := "connectivity"
				if ctx.Err() != nil {
					reason = "interrupted"
				}
				manifest.Reasons = appendReason(manifest.Reasons, reason)
				break
			}
			if err := writePrivateJSON(runtimeDir, "accounts.json", accounts); err != nil {
				manifest.Reasons = appendReason(manifest.Reasons, "result_incomplete")
				break
			}
		}
		if err := writePrivateJSON(dir, "k6-config-"+spec.stage+".json", k6Config(runID, scenario, dataset, spec, fixture)); err != nil {
			manifest.Reasons = appendReason(manifest.Reasons, "result_incomplete")
			break
		}
		record, stagePoints, reasons := e.runStage(measureCtx, ctx, clock, baseline, apiURL, runID, dir, spec, plan)
		manifest.Stages = append(manifest.Stages, record)
		points.Write(stagePoints)
		for _, reason := range reasons {
			manifest.Reasons = appendReason(manifest.Reasons, reason)
		}
		if len(reasons) > 0 {
			break
		}
		progress := manifest
		progress.Finalized = true
		evaluated, err := EvaluateProgress(progress, bytes.NewReader(points.Bytes()))
		if err != nil {
			manifest.Reasons = appendReason(manifest.Reasons, "result_incomplete")
			break
		}
		// 品質・生成不足・不完全のいずれかがあれば、次段階を開始しない。
		if len(evaluated.Reasons) > 0 {
			for _, reason := range evaluated.Reasons {
				manifest.Reasons = appendReason(manifest.Reasons, reason)
			}
			break
		}
	}
	return e.finalize(dir, manifest, points.Bytes())
}

// finalizeは、manifestと生Pointを保存し、保存済みファイルからreportを再生成する。
func (e Environment) finalize(dir string, manifest Manifest, points []byte) (RunOutcome, error) {
	manifest.Finalized = true
	if len(manifest.Stages) == 0 {
		manifest.Reasons = appendReason(manifest.Reasons, "result_incomplete")
	}
	outcome := RunOutcome{RunID: manifest.RunID, Dir: dir}
	if err := os.WriteFile(filepath.Join(dir, "points.jsonl"), points, 0o600); err != nil {
		return outcome, invalidInput("result_incomplete")
	}
	if err := writePrivateJSON(dir, "manifest.json", manifest); err != nil {
		return outcome, err
	}
	manifestFile, err := os.Open(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return outcome, invalidInput("result_incomplete")
	}
	defer func() { _ = manifestFile.Close() }()
	pointsFile, err := os.Open(filepath.Join(dir, "points.jsonl"))
	if err != nil {
		return outcome, invalidInput("result_incomplete")
	}
	defer func() { _ = pointsFile.Close() }()
	report, err := GenerateReport(manifestFile, pointsFile)
	if err != nil {
		return outcome, invalidInput("result_incomplete")
	}
	outcome.Report = report
	if err := writePrivateJSON(dir, "report.json", report); err != nil {
		return outcome, err
	}
	return outcome, nil
}

func plannedStages(plan Plan) []stageSpec {
	if plan.Scenario == "smoke" {
		return []stageSpec{{phase: "smoke", stage: "smoke", duration: smokeLimit}}
	}
	stages := []stageSpec{{phase: "warmup", stage: "warmup", duration: time.Duration(plan.WarmupSeconds) * time.Second}}
	for i := range plan.Rates {
		stages = append(stages, stageSpec{phase: "measure", stage: fmt.Sprint(i + 1), duration: time.Duration(plan.StageSeconds) * time.Second})
	}
	return stages
}

func k6Config(runID, scenario, dataset string, spec stageSpec, fixture Fixture) map[string]any {
	config := map[string]any{
		"schema_version": 1, "run_id": runID, "scenario": scenario, "dataset": dataset,
		"phase": spec.phase, "stage": spec.stage, "base_url": k6BaseURL,
	}
	if scenario == "login" {
		return config
	}
	postIDs := make([]string, 0, len(fixture.Posts))
	parents := make([]string, 0, len(fixture.Posts))
	replies := make([]string, 0, len(fixture.Posts))
	for i, post := range fixture.Posts {
		// 各Postの先頭の親Commentとその返信を、同じindexでk6へ渡す。
		postIDs = append(postIDs, string(post.ID))
		parents = append(parents, string(fixture.Comments[i*commentsPerPost].ID))
		replies = append(replies, string(fixture.Comments[i*commentsPerPost+1].ID))
	}
	config["fixture"] = map[string]any{"post_ids": postIDs, "parent_comment_ids": parents, "reply_ids": replies}
	return config
}

// preflightは、Docker VMの上限・swap制限・無関係なcontainerを確認し、満たさなければ送信を始めない。
// VM設定やcontainerの停止は利用者の判断であり、CLIから変更しない。
func (e Environment) preflight(ctx context.Context, plan Plan) error {
	output, err := e.docker(ctx, "info", "--format", "{{json .}}")
	if err != nil {
		return environmentFailure("docker_unavailable")
	}
	var info struct {
		NCPU      int   `json:"NCPU"`
		MemTotal  int64 `json:"MemTotal"`
		SwapLimit bool  `json:"SwapLimit"`
	}
	if err := json.Unmarshal(output, &info); err != nil {
		return environmentFailure("docker_unavailable")
	}
	steadyMemory := int64(plan.SteadyMemoryMiB) * 1024 * 1024
	if info.NCPU > maxVMCPU || info.MemTotal > maxVMMemoryBytes ||
		float64(info.NCPU) < plan.SteadyCPU+vmCPUHeadroom || info.MemTotal < steadyMemory+vmMemoryHeadroom {
		return environmentFailure("vm_limits_exceeded")
	}
	if !info.SwapLimit {
		return environmentFailure("swap_limit_unsupported")
	}
	output, err = e.docker(ctx, "ps", "--format", "{{json .}}")
	if err != nil {
		return environmentFailure("docker_unavailable")
	}
	var unrelated []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var container struct{ Names, Labels string }
		if json.Unmarshal(scanner.Bytes(), &container) != nil {
			return environmentFailure("docker_unavailable")
		}
		if !strings.Contains(","+container.Labels+",", ",com.docker.compose.project="+Project+",") {
			unrelated = append(unrelated, container.Names)
		}
	}
	if len(unrelated) > 0 {
		return &Failure{Code: 4, Reason: "unrelated_containers " + strings.Join(unrelated, ",")}
	}
	return nil
}

func (e Environment) readSecrets() (map[string]string, error) {
	data, err := os.ReadFile(e.secretsFile())
	if err != nil {
		return nil, invalidInput("secrets_missing")
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && !strings.HasPrefix(key, "#") {
			values[key] = value
		}
	}
	if values["LOADTEST_USER_PASSWORD"] == "" {
		return nil, invalidInput("secrets_missing")
	}
	return values, nil
}

// apiURLは、公開portへの事前確認先を返す。上書きは127.0.0.1のportだけを許可し、外部URLへ送らない。
func (e Environment) apiURL() (string, error) {
	value := os.Getenv("LOADTEST_API_URL")
	if value == "" {
		return defaultAPIURL, nil
	}
	if !loopbackURL.MatchString(value) {
		return "", invalidInput("invalid_input")
	}
	return value, nil
}

func (e Environment) createRunDir() (string, string, error) {
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return "", "", environmentFailure("result_save_failed")
	}
	runID := time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix)
	results := filepath.Join(e.Root, "loadtest", "results")
	if err := os.MkdirAll(results, 0o700); err != nil {
		return "", "", environmentFailure("result_save_failed")
	}
	dir := filepath.Join(results, runID)
	// 既存結果へ上書きしないよう、run IDのディレクトリは新規作成だけを許可する。
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", "", environmentFailure("result_save_failed")
	}
	return runID, dir, nil
}

// waitReadyは、測定前にAPI・Tempo・Grafanaの応答と、専用DB上のmigration・fixtureを確認する。
// Collectorは公開portを持たないため、常駐サービスの稼働監視で代替する。
func (e Environment) waitReady(ctx context.Context, apiURL, dataset string, expected FixtureSummary) string {
	for _, url := range []string{apiURL + "/health", e.serviceURL("LOADTEST_TEMPO_URL", defaultTempoURL) + "/ready", e.serviceURL("LOADTEST_GRAFANA_URL", defaultGrafanaURL) + "/api/health"} {
		if url == "" {
			return "result_incomplete"
		}
		for !ok(ctx, url) {
			select {
			case <-ctx.Done():
				return "environment"
			case <-time.After(time.Second):
			}
		}
	}
	// tool containerがSQLで専用DBを読み戻し、件数とchecksumを準備時の記録と照合する。
	output, err := e.composeOutput(ctx, "verify", "--profile", "prepare", "run", "--rm", "--name", oneOffName("verify"), "tool", "verify", "--dataset", dataset)
	if err != nil {
		return "environment"
	}
	var actual FixtureSummary
	if json.Unmarshal(output, &actual) != nil || actual != expected {
		return "result_incomplete"
	}
	return ""
}

func (e Environment) serviceURL(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	if !loopbackURL.MatchString(value) {
		return ""
	}
	return value
}

func ok(ctx context.Context, url string) bool {
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	response, err := noRedirectClient().Do(request)
	if err != nil {
		return false
	}
	_ = response.Body.Close()
	return response.StatusCode == http.StatusOK
}

// checkAuthenticatedReadは、事前ログインしたCookieで認証付きAPIが使えることを測定前に確認する。
func checkAuthenticatedRead(ctx context.Context, apiURL, scenario string, accounts []account) error {
	if scenario == "login" || len(accounts) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/api/me", nil)
	if err != nil {
		return err
	}
	request.AddCookie(&http.Cookie{Name: "threadly-session", Value: accounts[0].Cookie})
	response, err := noRedirectClient().Do(request)
	if err != nil {
		return err
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return environmentFailure("authentication_failed")
	}
	return nil
}

func healthy(ctx context.Context, apiURL string) bool {
	return ok(ctx, apiURL+"/health")
}

func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// authenticateは、VUへ1対1で割り当てるUserを直列に事前ログインする。loginシナリオでは
// 測定対象のloginを事前に行わず、passwordだけを渡す。Cookie値は結果やlogへ出さない。
func authenticate(ctx context.Context, apiURL, scenario string, plan Plan, fixture Fixture, password string) ([]account, time.Time, error) {
	accounts := make([]account, 0, plan.MaxVUs)
	loggedInAt := time.Now()
	for i := 0; i < plan.MaxVUs; i++ {
		user := fixture.Users[i]
		if scenario == "login" {
			accounts = append(accounts, account{ID: string(user.ID), Username: user.Username, Password: password})
			continue
		}
		cookie, err := login(ctx, apiURL, string(user.ID), user.Username, password)
		if err != nil {
			return nil, time.Time{}, err
		}
		accounts = append(accounts, account{ID: string(user.ID), Username: user.Username, Cookie: cookie})
	}
	return accounts, loggedInAt, nil
}

func login(ctx context.Context, apiURL, id, username, password string) (string, error) {
	body, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return "", environmentFailure("authentication_failed")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/api/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", environmentFailure("authentication_failed")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := noRedirectClient().Do(request)
	if err != nil {
		return "", environmentFailure("authentication_failed")
	}
	defer func() { _ = response.Body.Close() }()
	var decoded struct {
		User struct{ ID, Username string } `json:"user"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(&decoded) != nil ||
		decoded.User.ID != id || decoded.User.Username != username {
		return "", environmentFailure("authentication_failed")
	}
	for _, cookie := range response.Cookies() {
		if cookie.Name == "threadly-session" && cookie.Value != "" {
			return cookie.Value, nil
		}
	}
	return "", environmentFailure("authentication_failed")
}

// writePrivateJSONは、結果・認証ファイルを0600で保存する。保存できない結果は不完全（終了5）として扱う。
func writePrivateJSON(dir, name string, value any) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return invalidInput("result_incomplete")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return invalidInput("result_incomplete")
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		return invalidInput("result_incomplete")
	}
	return nil
}

func appendReason(reasons []string, reason string) []string {
	if hasReason(reasons, reason) {
		return reasons
	}
	return append(reasons, reason)
}

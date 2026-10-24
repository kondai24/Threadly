package loadtest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const serviceStateFormat = `{"Status":{{json .State.Status}},"OOMKilled":{{json .State.OOMKilled}},"RestartCount":{{.RestartCount}}}`

// forceStopAtは、最初の送信からk6の強制停止を始める時刻。docker操作のtimeout5秒を含めて900秒までに終える。
const forceStopAt = 895 * time.Second

// stageClockは、run全体の900秒上限を段階へ渡す。
type stageClock struct {
	firstSend time.Time
}

// stopperは、監視ループの処理時間に依存せず、停止要求・drain・強制停止を進める。
type stopper struct {
	env    Environment
	name   string
	cmd    *exec.Cmd
	exited chan struct{}

	mu          sync.Mutex
	reason      string
	requestedAt time.Time
	forced      bool
	forceFailed bool
}

// requestは、最初の停止理由だけを記録し、k6へ新規iterationの停止を要求する。
// drain上限を過ぎても終了しなければ強制停止する。
func (s *stopper) request(reason string) {
	s.mu.Lock()
	if s.reason != "" {
		s.mu.Unlock()
		return
	}
	s.reason = reason
	s.requestedAt = time.Now()
	s.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
		_, _ = s.env.docker(ctx, "kill", "-s", "SIGINT", s.name)
		cancel()
		select {
		case <-s.exited:
		case <-time.After(drainLimit):
			s.force()
		}
	}()
}

// forceは、k6 containerを強制停止し、containerが消えたことを確認する。確認できなければ
// 送信が続いている可能性があるため、結果を監視障害として残す。
func (s *stopper) force() {
	s.mu.Lock()
	if s.forced {
		s.mu.Unlock()
		return
	}
	s.forced = true
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()
	_, _ = s.env.docker(ctx, "kill", s.name)
	_ = s.cmd.Process.Kill()
	output, err := s.env.docker(ctx, "ps", "-a", "-q", "--filter", "name=^"+s.name+"$")
	if err == nil && len(bytes.TrimSpace(output)) > 0 {
		_, err = s.env.docker(ctx, "rm", "-f", s.name)
	}
	if err != nil {
		s.mu.Lock()
		s.forceFailed = true
		s.mu.Unlock()
	}
}

func (s *stopper) state() (string, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason, s.forced, s.forceFailed
}

// runStageは、段階ごとに1つのk6 processを起動し、生Pointと常駐サービスを監視する。
// 停止理由を検出したら新規送信を止め、最大10秒のdrain後に強制停止する。
func (e Environment) runStage(measureCtx, parentCtx context.Context, clock stageClock, baseline *serviceBaseline, apiURL, runID, dir string, spec stageSpec, plan Plan) (Stage, []byte, []string) {
	name := Project + "-k6-" + spec.stage
	pointsPath := filepath.Join(dir, "points-"+spec.stage+".jsonl")
	args := []string{
		"compose", "-p", Project, "-f", e.composeFile(), "--env-file", e.secretsFile(),
		"--profile", "measure", "run", "--rm", "--no-deps", "--name", name,
		"-e", "LOADTEST_CONFIG=/results/" + runID + "/k6-config-" + spec.stage + ".json",
		"-e", "LOADTEST_ACCOUNTS=/runtime/" + runID + "/accounts.json",
		"k6", "run", "--quiet", "--out", "json=/results/" + runID + "/points-" + spec.stage + ".jsonl", "/scripts/scenario.js",
	}
	launch := time.Now().UTC()
	live := newLiveEvaluator(spec, plan, launch)
	cmd := exec.Command("docker", args...)
	cmd.Dir = e.Root
	// k6のconsole出力も結果として残す。scriptはCookieやresponse bodyを出力しない。
	if logFile, err := os.OpenFile(filepath.Join(dir, "k6-"+spec.stage+".log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600); err == nil {
		defer func() { _ = logFile.Close() }()
		cmd.Stdout, cmd.Stderr = logFile, logFile
	}
	if err := cmd.Start(); err != nil {
		return live.record(time.Now().UTC(), false, "", false), nil, []string{"container_exit"}
	}
	stop := &stopper{env: e, name: name, cmd: cmd, exited: make(chan struct{})}
	exited := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		close(stop.exited)
		exited <- err
	}()
	// 監視ループとは独立に、900秒上限までに強制停止を始める。
	watchdog := time.AfterFunc(time.Until(clock.firstSend.Add(forceStopAt)), func() {
		stop.request("result_incomplete")
		stop.force()
	})
	defer watchdog.Stop()

	reasons := make(chan string, 8)
	monitorDone := make(chan struct{})
	defer close(monitorDone)
	var k6Started atomic.Bool
	target := monitorTarget{dir: dir, k6Name: name, k6Started: &k6Started, plan: plan}
	go e.monitorStage(monitorDone, reasons, baseline, apiURL, target)

	tail := pointTail{path: pointsPath}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	done := measureCtx.Done()
	var exitErr error
	for waiting := true; waiting; {
		select {
		case exitErr = <-exited:
			waiting = false
		case <-done:
			done = nil
			reason := "result_incomplete"
			if parentCtx.Err() != nil {
				reason = "interrupted"
			}
			stop.request(reason)
		case reason := <-reasons:
			stop.request(reason)
		case now := <-ticker.C:
			live.add(tail.read(), now)
			// 最初のPointを観測するまではk6 containerが存在しないことがあるため、資源採取の対象に含めない。
			if !live.anchor.IsZero() {
				k6Started.Store(true)
			}
			if reason := live.check(now); reason != "" {
				stop.request(reason)
			}
		}
	}
	exitedAt := time.Now().UTC()
	live.add(tail.read(), exitedAt)
	points, _ := os.ReadFile(pointsPath)
	reason, forced, forceFailed := stop.state()
	stageReasons := []string{}
	switch {
	case reason != "":
		stageReasons = append(stageReasons, reason)
	case exitErr != nil:
		stageReasons = append(stageReasons, "container_exit")
	}
	if forceFailed {
		stageReasons = append(stageReasons, "monitoring_failed")
	}
	return live.record(exitedAt, exitErr == nil, reason, forced), points, stageReasons
}

// serviceBaselineは、run全体で常駐サービスの再起動回数の基準を保持する。
type serviceBaseline struct {
	restarts map[string]int
}

// monitorStageは、5秒ごとに常駐サービスとAPI healthを確認し、停止理由を送る。
// docker操作やhealth確認の待ちが、Point判定と停止制御を遅らせないよう別goroutineで行う。
// monitorTargetは、段階ごとの資源採取と保存量確認の対象を表す。
type monitorTarget struct {
	dir       string
	k6Name    string
	k6Started *atomic.Bool
	plan      Plan
}

func (e Environment) monitorStage(done <-chan struct{}, reasons chan<- string, baseline *serviceBaseline, apiURL string, target monitorTarget) {
	send := func(reason string) {
		select {
		case reasons <- reason:
		case <-done:
		}
	}
	var pressure generatorPressure
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()
	// 段階開始直後にも1回採取し、短い段階でも資源と保存量の記録を残す。
	for {
		if reason := e.checkServices(baseline, apiURL); reason != "" {
			send(reason)
		}
		k6Name := ""
		if target.k6Started.Load() {
			k6Name = target.k6Name
		}
		samples, err := e.sampleResources(target.dir, k6Name, target.plan)
		if err != nil {
			send("monitoring_failed")
		} else if pressure.observe(samples) {
			send("generator_pressure")
		}
		if reason := e.checkStorage(target.dir); reason != "" {
			send(reason)
		}
		select {
		case <-done:
			return
		case <-ticker.C:
		}
	}
}

// checkServicesは、常駐サービスのOOM・停止・再起動とAPIのhealthを確認する。
func (e Environment) checkServices(baseline *serviceBaseline, apiURL string) string {
	names := make([]string, 0, len(monitoredServices))
	for _, service := range monitoredServices {
		names = append(names, Project+"-"+service+"-1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()
	output, err := e.docker(ctx, append([]string{"inspect", "--format", serviceStateFormat}, names...)...)
	if err != nil {
		return "monitoring_failed"
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	count := 0
	for scanner.Scan() {
		var state struct {
			Status       string
			OOMKilled    bool
			RestartCount int
		}
		if json.Unmarshal(scanner.Bytes(), &state) != nil || count >= len(names) {
			return "monitoring_failed"
		}
		name := names[count]
		count++
		switch {
		case state.OOMKilled:
			return "oom"
		case state.Status != "running":
			return "container_exit"
		}
		if previous, ok := baseline.restarts[name]; ok && state.RestartCount > previous {
			return "container_exit"
		}
		baseline.restarts[name] = state.RestartCount
	}
	if count != len(names) {
		return "monitoring_failed"
	}
	if !healthy(context.Background(), apiURL) {
		return "connectivity"
	}
	return ""
}

// pointTailは、k6が追記するJSON出力を、改行で完結した行だけ逐次読み込む。
type pointTail struct {
	path    string
	offset  int64
	partial []byte
}

func (t *pointTail) read() []point {
	file, err := os.Open(t.path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Seek(t.offset, io.SeekStart); err != nil {
		return nil
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil
	}
	t.offset += int64(len(data))
	data = append(t.partial, data...)
	lastNewline := bytes.LastIndexByte(data, '\n')
	if lastNewline < 0 {
		t.partial = data
		return nil
	}
	t.partial = append([]byte(nil), data[lastNewline+1:]...)
	var points []point
	for _, line := range bytes.Split(data[:lastNewline], []byte{'\n'}) {
		var p point
		if json.Unmarshal(line, &p) == nil && p.Type == "Point" && p.Data.Value != nil {
			points = append(points, p)
		}
	}
	return points
}

type httpSample struct {
	at       time.Time
	duration float64
	failure  bool
}

// liveEvaluatorは、reportと同じ30秒非重複窓・品質基準で、送信中の段階を早期判定する。
// 窓の起点は観測した最も早いPoint時刻とし、窓は境界から5秒の出力待ちの後に確定する。
type liveEvaluator struct {
	spec       stageSpec
	plan       Plan
	launch     time.Time
	anchor     time.Time
	latest     time.Time
	lastHTTP   time.Time
	transport  bool
	samples    []httpSample
	evaluated  map[int]bool
	incomplete map[int]bool
}

func newLiveEvaluator(spec stageSpec, plan Plan, launch time.Time) *liveEvaluator {
	return &liveEvaluator{spec: spec, plan: plan, launch: launch, evaluated: map[int]bool{}, incomplete: map[int]bool{}}
}

func (l *liveEvaluator) add(points []point, now time.Time) {
	for _, p := range points {
		if l.anchor.IsZero() {
			l.lastHTTP = now
		}
		if l.anchor.IsZero() || p.Data.Time.Before(l.anchor) {
			l.anchor = p.Data.Time
		}
		if p.Data.Time.After(l.latest) {
			l.latest = p.Data.Time
		}
		switch p.Metric {
		case "lt_transport_errors":
			if *p.Data.Value > 0 {
				l.transport = true
			}
		case "lt_http_duration":
			l.lastHTTP = now
			l.samples = append(l.samples, httpSample{at: p.Data.Time, duration: *p.Data.Value, failure: p.Data.Tags["outcome"] == "failure"})
			// 確定済みの窓へ後から届いたPointは捨てずに保持し、その窓を不完全として記録する。
			if index := l.window(p.Data.Time); l.evaluated[index] {
				l.incomplete[index] = true
			}
		}
	}
}

func (l *liveEvaluator) window(at time.Time) int {
	if at.Before(l.anchor) {
		return 0
	}
	return int(at.Sub(l.anchor) / (30 * time.Second))
}

func (l *liveEvaluator) check(now time.Time) string {
	if l.transport {
		return "connectivity"
	}
	if l.anchor.IsZero() {
		if now.Sub(l.launch) > startupGrace {
			return "monitoring_failed"
		}
		return ""
	}
	if l.spec.phase == "smoke" {
		if now.Sub(l.launch) > smokeLimit+drainLimit {
			return "result_incomplete"
		}
		return ""
	}
	sendEnd := l.anchor.Add(l.spec.duration)
	// 開始予定があるのにHTTP完了が10秒以上届かない状態を、品質判定を待たずに停止する。
	if now.Before(sendEnd) && now.Sub(l.lastHTTP) > httpStallLimit {
		return "connectivity"
	}
	windows := int(l.spec.duration / (30 * time.Second))
	for index := 0; index < windows; index++ {
		windowEnd := l.anchor.Add(time.Duration(index+1) * 30 * time.Second)
		if l.evaluated[index] || now.Before(windowEnd.Add(monitorInterval)) {
			continue
		}
		l.evaluated[index] = true
		var values []float64
		failures := 0
		for _, sample := range l.samples {
			if l.window(sample.at) == index {
				values = append(values, sample.duration)
				if sample.failure {
					failures++
				}
			}
		}
		if len(values) < l.plan.WindowMinHTTP {
			continue
		}
		sort.Float64s(values)
		if qualityFailed(percentile(values, .95), percentile(values, .99), float64(failures)/float64(len(values)), l.plan) {
			return "quality"
		}
	}
	return ""
}

// recordは、観測したPoint時刻から段階を記録する。startは最も早いPoint、終了は最後の観測時刻で、
// 予定送信区間を満たして正常終了した段階だけをcompleteにする。
func (l *liveEvaluator) record(exitedAt time.Time, exitOK bool, stopReason string, forced bool) Stage {
	start := l.anchor
	if start.IsZero() {
		start = l.launch
	}
	end := exitedAt
	if l.latest.After(end) {
		end = l.latest
	}
	start, end = start.UTC(), end.UTC()
	stage := Stage{Phase: l.spec.phase, Stage: l.spec.stage, Start: start}
	normal := exitOK && stopReason == ""
	if l.spec.phase == "smoke" {
		stage.SendEnd = minTime(end, start.Add(smokeLimit))
		if !stage.SendEnd.After(start) {
			stage.SendEnd = start.Add(time.Millisecond)
		}
		stage.End = stage.SendEnd
		stage.Complete = normal
		return stage
	}
	for index := range l.incomplete {
		stage.IncompleteWindows = append(stage.IncompleteWindows, index)
	}
	sort.Ints(stage.IncompleteWindows)
	stage.SendEnd = start.Add(l.spec.duration)
	if normal {
		stage.End = minTime(maxTime(end, stage.SendEnd), stage.SendEnd.Add(drainLimit))
		stage.Complete = true
		return stage
	}
	stage.End = minTime(end, stage.SendEnd.Add(drainLimit))
	// drain内に終了した停止だけを、開始済み操作の結果が揃った段階として扱う。
	stage.Stopped = stopReason != "" && !forced
	return stage
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

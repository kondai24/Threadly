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
	"time"
)

const serviceStateFormat = `{"Status":{{json .State.Status}},"OOMKilled":{{json .State.OOMKilled}},"RestartCount":{{.RestartCount}}}`

// stageRunは、1つのk6 processの実行中に観測した結果と停止要求を保持する。
type stageRun struct {
	spec   stageSpec
	plan   Plan
	launch time.Time
	live   liveEvaluator

	stopReason      string
	stopRequestedAt time.Time
	forced          bool
}

// runStageは、段階ごとに1つのk6 processを起動し、生Pointと常駐サービスを監視する。
// 停止理由を検出したら新規送信を止め、最大10秒のdrain後に強制停止する。
func (e Environment) runStage(measureCtx, parentCtx context.Context, apiURL, runID, dir string, spec stageSpec, plan Plan) (Stage, []byte, []string) {
	name := Project + "-k6-" + spec.stage
	pointsPath := filepath.Join(dir, "points-"+spec.stage+".jsonl")
	args := []string{
		"compose", "-p", Project, "-f", e.composeFile(), "--env-file", e.secretsFile(),
		"--profile", "measure", "run", "--rm", "--no-deps", "--name", name,
		"-e", "LOADTEST_CONFIG=/results/" + runID + "/k6-config-" + spec.stage + ".json",
		"-e", "LOADTEST_ACCOUNTS=/runtime/" + runID + "/accounts.json",
		"k6", "run", "--quiet", "--out", "json=/results/" + runID + "/points-" + spec.stage + ".jsonl", "/scripts/scenario.js",
	}
	run := &stageRun{spec: spec, plan: plan, launch: time.Now().UTC()}
	run.live = liveEvaluator{spec: spec, plan: plan, launch: run.launch, windows: map[int]*windowStats{}, evaluated: map[int]bool{}}
	cmd := exec.Command("docker", args...)
	cmd.Dir = e.Root
	if err := cmd.Start(); err != nil {
		return run.record(time.Now(), false), nil, []string{"container_exit"}
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	tail := pointTail{path: pointsPath}
	baseline := map[string]int{}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastMonitor time.Time
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
			e.requestStop(run, name, reason)
		case now := <-ticker.C:
			run.live.add(tail.read(), now)
			if reason := run.live.check(now); reason != "" {
				e.requestStop(run, name, reason)
			}
			if now.Sub(lastMonitor) >= monitorInterval {
				lastMonitor = now
				if reason := e.checkServices(baseline, apiURL); reason != "" {
					e.requestStop(run, name, reason)
				}
			}
			if run.stopReason != "" && !run.forced && now.Sub(run.stopRequestedAt) > drainLimit {
				// drain上限を超えたら強制停止する。失われた操作は成功に数えない。
				run.forced = true
				ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
				_, _ = e.docker(ctx, "kill", name)
				cancel()
				_ = cmd.Process.Kill()
			}
		}
	}
	exitedAt := time.Now().UTC()
	run.live.add(tail.read(), exitedAt)
	points, _ := os.ReadFile(pointsPath)
	reasons := []string{}
	if run.stopReason != "" {
		reasons = append(reasons, run.stopReason)
	} else if exitErr != nil {
		reasons = append(reasons, "container_exit")
	}
	return run.record(exitedAt, exitErr == nil), points, reasons
}

// requestStopは、最初に検出した停止理由だけを記録し、k6へ新規iterationの停止を要求する。
func (e Environment) requestStop(run *stageRun, name, reason string) {
	if run.stopReason != "" {
		return
	}
	run.stopReason = reason
	run.stopRequestedAt = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()
	_, _ = e.docker(ctx, "kill", "-s", "SIGINT", name)
}

// recordは、観測したPoint時刻から段階を記録する。startは最初のPoint、終了は最後の観測時刻で、
// 予定送信区間を満たして正常終了した段階だけをcompleteにする。
func (r *stageRun) record(exitedAt time.Time, exitOK bool) Stage {
	start := r.live.anchor
	if start.IsZero() {
		start = r.launch
	}
	end := exitedAt
	if r.live.latest.After(end) {
		end = r.live.latest
	}
	start, end = start.UTC(), end.UTC()
	stage := Stage{Phase: r.spec.phase, Stage: r.spec.stage, Start: start}
	normal := exitOK && r.stopReason == ""
	if r.spec.phase == "smoke" {
		stage.SendEnd = minTime(end, start.Add(smokeLimit))
		if !stage.SendEnd.After(start) {
			stage.SendEnd = start.Add(time.Millisecond)
		}
		stage.End = stage.SendEnd
		stage.Complete = normal
		return stage
	}
	stage.SendEnd = start.Add(r.spec.duration)
	if normal {
		stage.End = minTime(maxTime(end, stage.SendEnd), stage.SendEnd.Add(drainLimit))
		stage.Complete = true
		return stage
	}
	stage.End = minTime(end, stage.SendEnd.Add(drainLimit))
	// drain内に終了した停止だけを、開始済み操作の結果が揃った段階として扱う。
	stage.Stopped = r.stopReason != "" && !r.forced
	return stage
}

// checkServicesは、常駐サービスのOOM・停止・再起動とAPIのhealthを確認する。
func (e Environment) checkServices(baseline map[string]int, apiURL string) string {
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
		if previous, ok := baseline[name]; ok && state.RestartCount > previous {
			return "container_exit"
		}
		baseline[name] = state.RestartCount
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

type windowStats struct {
	durations []float64
	failures  int
}

// liveEvaluatorは、reportと同じ30秒非重複窓・品質基準で、送信中の段階を早期判定する。
type liveEvaluator struct {
	spec      stageSpec
	plan      Plan
	launch    time.Time
	anchor    time.Time
	latest    time.Time
	lastHTTP  time.Time
	transport bool
	windows   map[int]*windowStats
	evaluated map[int]bool
}

func (l *liveEvaluator) add(points []point, now time.Time) {
	for _, p := range points {
		if l.anchor.IsZero() {
			l.anchor = p.Data.Time
			l.lastHTTP = now
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
			index := int(p.Data.Time.Sub(l.anchor) / (30 * time.Second))
			if l.windows[index] == nil {
				l.windows[index] = &windowStats{}
			}
			l.windows[index].durations = append(l.windows[index].durations, *p.Data.Value)
			if p.Data.Tags["outcome"] == "failure" {
				l.windows[index].failures++
			}
		}
	}
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
	for index, stats := range l.windows {
		windowEnd := l.anchor.Add(time.Duration(index+1) * 30 * time.Second)
		closed := !l.latest.Before(windowEnd) || !now.Before(windowEnd.Add(monitorInterval))
		if l.evaluated[index] || !closed || windowEnd.After(sendEnd) {
			continue
		}
		l.evaluated[index] = true
		if len(stats.durations) < l.plan.WindowMinHTTP {
			continue
		}
		values := append([]float64(nil), stats.durations...)
		sort.Float64s(values)
		errorRate := float64(stats.failures) / float64(len(values))
		if qualityFailed(percentile(values, .95), percentile(values, .99), errorRate, l.plan) {
			return "quality"
		}
	}
	return ""
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

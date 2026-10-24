package loadtest

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strconv"
	"time"
)

// Stageは、送信区間とdrainを含む段階の記録を表す。
type Stage struct {
	Stage    string    `json:"stage"`
	Phase    string    `json:"phase"`
	Start    time.Time `json:"start"`
	SendEnd  time.Time `json:"send_end"`
	End      time.Time `json:"end"`
	Complete bool      `json:"complete"`
	// Stoppedは、監視器が停止理由を検出して新規送信を止め、開始済み操作のdrainと保存を終えた終端段階を表す。
	Stopped           bool  `json:"stopped,omitempty"`
	IncompleteWindows []int `json:"incomplete_windows,omitempty"`
}

// Manifestは、reportが生Pointを解釈するための測定条件を表す。
type Manifest struct {
	SchemaVersion int      `json:"schema_version"`
	RunID         string   `json:"run_id"`
	Scenario      string   `json:"scenario"`
	Dataset       string   `json:"dataset"`
	Finalized     bool     `json:"finalized"`
	Stages        []Stage  `json:"stages"`
	Reasons       []string `json:"reasons"`

	// 以降は比較条件の記録で、段階評価には使わない。
	Plan             *Plan             `json:"plan,omitempty"`
	Fixture          *FixtureSummary   `json:"fixture,omitempty"`
	Trace            map[string]string `json:"trace,omitempty"`
	Environment      map[string]any    `json:"environment,omitempty"`
	Monitor          map[string]any    `json:"monitor,omitempty"`
	StartedAt        time.Time         `json:"started_at,omitzero"`
	MeasureStartedAt time.Time         `json:"measure_started_at,omitzero"`
	FinishedAt       time.Time         `json:"finished_at,omitzero"`
	TraceVolumeBytes int64             `json:"trace_volume_bytes,omitempty"`
}

type point struct {
	Type   string `json:"type"`
	Metric string `json:"metric"`
	Data   struct {
		Time  time.Time         `json:"time"`
		Value *float64          `json:"value"`
		Tags  map[string]string `json:"tags"`
	} `json:"data"`
}

// StageResultは、warmupを除いた段階別のHTTP品質と容量候補を表す。
type StageResult struct {
	Stage               string         `json:"stage"`
	Rate                int            `json:"requested_rate"`
	HTTPCount           int            `json:"http_count"`
	HTTPPerSecond       float64        `json:"http_per_second"`
	P95MS               float64        `json:"p95_ms"`
	P99MS               float64        `json:"p99_ms"`
	ErrorRate           float64        `json:"error_rate"`
	Qualified           bool           `json:"qualified"`
	Windows             []WindowResult `json:"windows"`
	DrainSeconds        float64        `json:"drain_seconds"`
	OperationsStarted   int            `json:"operations_started"`
	OperationsCompleted int            `json:"operations_completed"`
	OperationsFailed    int            `json:"operations_failed"`
	OperationsPerSecond float64        `json:"operations_per_second"`
}

// WindowResultは、非重複窓の暫定/品質判定を表す。
type WindowResult struct {
	Index     int     `json:"index"`
	HTTPCount int     `json:"http_count"`
	P95MS     float64 `json:"p95_ms"`
	P99MS     float64 `json:"p99_ms"`
	ErrorRate float64 `json:"error_rate"`
	Status    string  `json:"status"`
}

// Reportは、再生成した段階評価と試験範囲内の最大確認rateを表す。
type Report struct {
	SchemaVersion    int           `json:"schema_version"`
	Status           string        `json:"status"`
	ExitCode         int           `json:"exit_code"`
	Reasons          []string      `json:"reasons"`
	MaxQualifiedRate int           `json:"max_qualified_rate"`
	Stages           []StageResult `json:"stages"`
}

// GenerateReportは、保存済みJSONだけを読み、DockerやAPIへ接続せず評価する。
func GenerateReport(manifestInput, pointsInput io.Reader) (Report, error) {
	var manifest Manifest
	manifestDecoder := json.NewDecoder(manifestInput)
	if err := manifestDecoder.Decode(&manifest); err != nil {
		return Report{}, err
	}
	var trailing any
	if err := manifestDecoder.Decode(&trailing); err != io.EOF {
		return Report{}, errors.New("result_incomplete")
	}
	return evaluate(manifest, pointsInput, true)
}

// EvaluateProgressは、runが次段階を開始してよいかを、終了済み段階だけについてreportと同じ規則で判定する。
// 予定段階がまだ残っていることは理由にしない。
func EvaluateProgress(manifest Manifest, pointsInput io.Reader) (Report, error) {
	return evaluate(manifest, pointsInput, false)
}

func evaluate(manifest Manifest, pointsInput io.Reader, requireAllStages bool) (Report, error) {
	plan, err := NewPlan(manifest.Scenario, manifest.Dataset)
	if err != nil {
		return Report{}, err
	}
	stageByKey, err := validateManifest(manifest)
	if err != nil {
		return Report{}, err
	}
	durations := map[string][]float64{}
	failures := map[string]int{}
	samples := map[string][]point{}
	counters := map[string]map[string]float64{}
	operationFailures := map[string]int{}
	var latest time.Time
	lateWindows := map[string]map[int]bool{}
	// 生Pointの時刻は採取時刻であり受信時刻ではない。監視器が記録した窓の欠損も引き継ぐ。
	for key, stage := range stageByKey {
		if len(stage.IncompleteWindows) > 0 {
			lateWindows[key] = map[int]bool{}
			for _, window := range stage.IncompleteWindows {
				lateWindows[key][window] = true
			}
		}
	}
	decoder := json.NewDecoder(pointsInput)
	for {
		var p point
		if err := decoder.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return Report{}, err
		}
		if p.Type == "Metric" {
			continue
		}
		if p.Type != "Point" {
			return Report{}, errors.New("result_incomplete")
		}
		if !requiredMetric(p.Metric) {
			continue
		}
		stage, ok := stageByKey[p.Data.Tags["phase"]+"/"+p.Data.Tags["stage"]]
		if !ok || p.Data.Value == nil || p.Data.Tags["run_id"] != manifest.RunID || p.Data.Tags["scenario"] != manifest.Scenario || p.Data.Time.Before(stage.Start) || p.Data.Time.After(stage.End) || math.IsNaN((*p.Data.Value)) || math.IsInf((*p.Data.Value), 0) || (*p.Data.Value) < 0 {
			return Report{}, errors.New("result_incomplete")
		}
		if (p.Metric == "lt_http_duration" || p.Metric == "lt_operations_completed") && p.Data.Tags["outcome"] != "success" && p.Data.Tags["outcome"] != "failure" {
			return Report{}, errors.New("result_incomplete")
		}
		if p.Metric != "lt_http_duration" && math.Trunc(*p.Data.Value) != *p.Data.Value {
			return Report{}, errors.New("result_incomplete")
		}
		if p.Metric == "lt_http_duration" && !validEndpoint(p.Data.Tags["endpoint"]) {
			return Report{}, errors.New("result_incomplete")
		}
		if p.Type == "Point" && p.Metric == "lt_http_duration" {
			key := p.Data.Tags["phase"] + "/" + p.Data.Tags["stage"]
			for _, stage := range manifest.Stages {
				if key != stage.Phase+"/"+stage.Stage || p.Data.Time.Before(stage.Start) || !p.Data.Time.Before(stage.SendEnd) {
					continue
				}
				window := int(p.Data.Time.Sub(stage.Start) / (30 * time.Second))
				closeTime := stage.Start.Add(time.Duration(window+1)*30*time.Second + 5*time.Second)
				// 後の時刻のPointが既に記録されていれば、窓確定後の遅着が少なくとも発生している。
				// 値は捨てず、最終段階集計へ含める。
				if latest.After(closeTime) {
					if lateWindows[key] == nil {
						lateWindows[key] = map[int]bool{}
					}
					lateWindows[key][window] = true
				}
			}
			samples[key] = append(samples[key], p)
			durations[key] = append(durations[key], (*p.Data.Value))
			if p.Data.Tags["outcome"] == "failure" {
				failures[key]++
			}
		}
		if p.Type == "Point" {
			key := p.Data.Tags["phase"] + "/" + p.Data.Tags["stage"]
			if p.Data.Time.After(latest) {
				latest = p.Data.Time
			}
			if counters[key] == nil {
				counters[key] = map[string]float64{}
			}
			counters[key][p.Metric] += (*p.Data.Value)
			if p.Metric == "lt_operations_completed" && p.Data.Tags["outcome"] == "failure" {
				operationFailures[key] += int((*p.Data.Value))
			}
		}
	}
	report := Report{SchemaVersion: 1, Status: "limit_not_reached", Reasons: []string{}, Stages: []StageResult{}}
	for stageIndex, stage := range manifest.Stages {
		values := durations[stage.Phase+"/"+stage.Stage]
		sort.Float64s(values)
		result := StageResult{Stage: stage.Stage, HTTPCount: len(values), HTTPPerSecond: float64(len(values)) / 120, P95MS: percentile(values, .95), P99MS: percentile(values, .99), Qualified: len(values) >= 100}
		result.DrainSeconds = math.Max(0, stage.End.Sub(stage.SendEnd).Seconds())
		if len(values) > 0 {
			result.ErrorRate = float64(failures[stage.Phase+"/"+stage.Stage]) / float64(len(values))
		}
		counts := counters[stage.Phase+"/"+stage.Stage]
		result.OperationsStarted = int(counts["lt_operations_started"])
		result.OperationsCompleted = int(counts["lt_operations_completed"])
		result.OperationsFailed = operationFailures[stage.Phase+"/"+stage.Stage]
		result.OperationsPerSecond = counts["lt_operations_completed"] / 120
		if counts["lt_transport_errors"] > 0 {
			result.Qualified = false
			if !hasReason(report.Reasons, "connectivity") {
				report.Reasons = append(report.Reasons, "connectivity")
			}
		}
		// 停止理由で止めた段階は予定より短いが、開始済み操作の結果が揃っていれば不完全とはしない。
		determined := stage.Complete || stage.Stopped
		if len(values) == 0 || counts["http_reqs"] != float64(len(values)) || counts["lt_operations_completed"] != counts["lt_operations_started"] || counts["iterations"] != counts["lt_operations_completed"] || counts["http_reqs"] < counts["lt_operations_completed"] || !determined || !manifest.Finalized {
			result.Qualified = false
			if !hasReason(report.Reasons, "result_incomplete") {
				report.Reasons = append(report.Reasons, "result_incomplete")
			}
		}
		if len(lateWindows[stage.Phase+"/"+stage.Stage]) > 0 {
			result.Qualified = false
			if !hasReason(report.Reasons, "window_incomplete") {
				report.Reasons = append(report.Reasons, "window_incomplete")
			}
		}
		if stage.Phase == "smoke" {
			counts := counters[stage.Phase+"/"+stage.Stage]
			if failures[stage.Phase+"/"+stage.Stage] > 0 || operationFailures[stage.Phase+"/"+stage.Stage] > 0 {
				report.Reasons = append(report.Reasons, "quality")
			}
			if len(values) == 0 || counts["http_reqs"] != float64(len(values)) || counts["lt_operations_started"] != 1 || counts["lt_operations_completed"] != 1 || counts["iterations"] != 1 || !stage.Complete || !manifest.Finalized {
				report.Reasons = append(report.Reasons, "result_incomplete")
			}
			continue
		}
		windowFailed := false
		for start := stage.Start; start.Before(stage.SendEnd); start = start.Add(30 * time.Second) {
			end := start.Add(30 * time.Second)
			var windowValues []float64
			failed := 0
			for _, p := range samples[stage.Phase+"/"+stage.Stage] {
				if !p.Data.Time.Before(start) && p.Data.Time.Before(end) {
					windowValues = append(windowValues, (*p.Data.Value))
					if p.Data.Tags["outcome"] == "failure" {
						failed++
					}
				}
			}
			sort.Float64s(windowValues)
			window := WindowResult{Index: len(result.Windows), HTTPCount: len(windowValues), P95MS: percentile(windowValues, .95), P99MS: percentile(windowValues, .99), Status: "provisional"}
			if len(windowValues) > 0 {
				window.ErrorRate = float64(failed) / float64(len(windowValues))
			}
			if len(windowValues) >= 20 {
				window.Status = "passed"
				if qualityFailed(window.P95MS, window.P99MS, window.ErrorRate, plan) {
					window.Status = "quality_failed"
					windowFailed = true
				}
			}
			if lateWindows[stage.Phase+"/"+stage.Stage][window.Index] {
				window.Status = "incomplete"
			}
			result.Windows = append(result.Windows, window)
		}
		if qualityFailed(result.P95MS, result.P99MS, result.ErrorRate, plan) || windowFailed {
			result.Qualified = false
			if !hasReason(report.Reasons, "quality") {
				report.Reasons = append(report.Reasons, "quality")
			}
		}
		expectedOperations := 60
		if stage.Phase == "measure" {
			result.Rate = plan.Rates[len(report.Stages)]
			expectedOperations = result.Rate * 120
		}
		if stage.Stopped {
			// 停止した段階は容量候補にせず、予定開始数との差を生成不足として数えない。
			result.Qualified = false
		} else if counts["lt_operations_started"] != float64(expectedOperations) {
			result.Qualified = false
			if !hasReason(report.Reasons, "generation_shortfall") {
				report.Reasons = append(report.Reasons, "generation_shortfall")
			}
		}
		if counts["dropped_iterations"] > 0 {
			result.Qualified = false
			if !hasReason(report.Reasons, "dropped_iterations") {
				report.Reasons = append(report.Reasons, "dropped_iterations")
			}
		}
		if stage.Phase != "measure" {
			continue
		}
		// 資源圧迫による停止理由は終端段階を指す。HTTP品質が正常でも容量候補にはできない。
		if stageIndex == len(manifest.Stages)-1 && hasReason(manifest.Reasons, "generator_pressure") {
			result.Qualified = false
		}
		if result.Qualified {
			report.MaxQualifiedRate = result.Rate
		}
		report.Stages = append(report.Stages, result)
	}
	for _, reason := range manifest.Reasons {
		if !hasReason(report.Reasons, reason) {
			report.Reasons = append(report.Reasons, reason)
		}
	}
	if requireAllStages && len(report.Reasons) == 0 && len(report.Stages) != len(plan.Rates) {
		report.Reasons = append(report.Reasons, "result_incomplete")
	}
	// 同時検出した理由を消さず、測定経路の障害をAPI品質より優先する。
	resolveStatus(&report)
	if manifest.Scenario == "smoke" && report.ExitCode == 0 {
		report.Status = "smoke_passed"
	}
	return report, nil
}

func validEndpoint(endpoint string) bool {
	switch endpoint {
	case "GET /health", "POST /api/auth/login", "GET /api/posts", "GET /api/posts/:id", "GET /api/posts/:id/comments", "POST /api/posts", "POST /api/posts/:id/comments", "PUT /api/posts/:id/like", "DELETE /api/posts/:id/like", "PUT /api/comments/:id/like", "DELETE /api/comments/:id/like":
		return true
	}
	return false
}

func resolveStatus(report *Report) {
	report.ExitCode = 0
	report.Status = "limit_not_reached"
	currentPriority := 0
	for _, reason := range report.Reasons {
		code, status, priority := reasonStatus(reason)
		if priority > currentPriority {
			currentPriority = priority
			report.ExitCode = code
			report.Status = status
		}
	}
}

func reasonStatus(reason string) (int, string, int) {
	switch reason {
	case "quality":
		return 2, "quality_failed", 1
	case "generation_shortfall", "dropped_iterations", "generator_pressure":
		return 3, "generation_shortfall", 2
	case "result_incomplete", "window_incomplete":
		return 5, "result_incomplete", 3
	case "environment", "connectivity", "oom", "container_exit", "monitoring_failed", "disk_limit":
		return 4, "environment_failed", 4
	case "interrupted":
		return 130, "interrupted", 5
	}
	return 5, "result_incomplete", 3
}

func requiredMetric(metric string) bool {
	switch metric {
	case "lt_http_duration", "lt_operations_started", "lt_operations_completed", "http_reqs", "iterations", "dropped_iterations", "lt_transport_errors":
		return true
	}
	return false
}

func validateManifest(manifest Manifest) (map[string]Stage, error) {
	invalid := errors.New("result_incomplete")
	// 準備中に止まったrunは段階を持たないが、停止理由があれば結果として受け入れる。
	if manifest.SchemaVersion != 1 || manifest.RunID == "" || (len(manifest.Stages) == 0 && len(manifest.Reasons) == 0) || len(manifest.Stages) > 7 {
		return nil, invalid
	}
	for _, reason := range manifest.Reasons {
		switch reason {
		case "quality", "generation_shortfall", "dropped_iterations", "generator_pressure", "result_incomplete", "window_incomplete", "environment", "connectivity", "oom", "container_exit", "monitoring_failed", "disk_limit", "interrupted":
		default:
			return nil, invalid
		}
	}
	stages := map[string]Stage{}
	measured := 0
	var previousEnd time.Time
	for _, stage := range manifest.Stages {
		key := stage.Phase + "/" + stage.Stage
		if manifest.Scenario == "smoke" && stage.Phase != "smoke" {
			return nil, invalid
		}
		if _, exists := stages[key]; exists {
			return nil, invalid
		}
		duration := 120 * time.Second
		switch stage.Phase {
		case "smoke":
			if manifest.Scenario != "smoke" || stage.Stage != "smoke" || len(manifest.Stages) != 1 {
				return nil, invalid
			}
			duration = stage.SendEnd.Sub(stage.Start)
			if duration <= 0 || duration > 30*time.Second || stage.End.After(stage.Start.Add(30*time.Second)) {
				return nil, invalid
			}
		case "warmup":
			if stage.Stage != "warmup" || measured > 0 {
				return nil, invalid
			}
			duration = 60 * time.Second
		case "measure":
			measured++
			if stage.Stage != strconv.Itoa(measured) || measured > 6 {
				return nil, invalid
			}
		default:
			return nil, invalid
		}
		// stoppedは停止理由を伴う終端段階だけに許可し、理由のない短縮を結果として受け入れない。
		if stage.Stopped && (stage.Complete || len(manifest.Reasons) == 0 || stage.Phase == "smoke" || len(stages)+1 != len(manifest.Stages)) {
			return nil, invalid
		}
		if stage.Start.IsZero() || stage.SendEnd.Sub(stage.Start) != duration || stage.End.Before(stage.Start) || stage.End.After(stage.SendEnd.Add(10*time.Second)) || stage.Start.Before(previousEnd) || (stage.Complete && stage.End.Before(stage.SendEnd)) {
			return nil, invalid
		}
		for _, window := range stage.IncompleteWindows {
			if window < 0 || window >= int(duration/(30*time.Second)) {
				return nil, invalid
			}
		}
		stages[key] = stage
		previousEnd = stage.End
	}
	return stages, nil
}

func hasReason(reasons []string, reason string) bool {
	for _, r := range reasons {
		if r == reason {
			return true
		}
	}
	return false
}

func qualityFailed(p95, p99, errorRate float64, plan Plan) bool {
	return p95 >= float64(plan.P95LimitMS) || p99 >= float64(plan.P99LimitMS) || errorRate >= plan.ErrorRateLimit
}

func percentile(sorted []float64, fraction float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(math.Ceil(fraction*float64(len(sorted))))-1]
}

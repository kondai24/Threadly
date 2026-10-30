package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type reportStage struct {
	Stage             string    `json:"stage"`
	Phase             string    `json:"phase"`
	Start             time.Time `json:"start"`
	SendEnd           time.Time `json:"send_end"`
	End               time.Time `json:"end"`
	Complete          bool      `json:"complete"`
	IncompleteWindows []int     `json:"incomplete_windows,omitempty"`
}

type reportManifest struct {
	SchemaVersion int           `json:"schema_version"`
	RunID         string        `json:"run_id"`
	Scenario      string        `json:"scenario"`
	Dataset       string        `json:"dataset"`
	Finalized     bool          `json:"finalized"`
	Stages        []reportStage `json:"stages"`
	Reasons       []string      `json:"reasons"`
}

type metricPoint struct {
	Type   string `json:"type"`
	Metric string `json:"metric"`
	Data   struct {
		Time  time.Time         `json:"time"`
		Value float64           `json:"value"`
		Tags  map[string]string `json:"tags"`
	} `json:"data"`
}

func point(stage reportStage, metric string, second int, value float64, outcome string) metricPoint {
	p := metricPoint{Type: "Point", Metric: metric}
	p.Data.Time = stage.Start.Add(time.Duration(second) * time.Second)
	p.Data.Value = value
	p.Data.Tags = map[string]string{"run_id": "fixture-run", "phase": stage.Phase, "scenario": "browse", "stage": stage.Stage}
	if metric == "lt_http_duration" {
		p.Data.Tags["endpoint"] = "GET /api/posts"
		p.Data.Tags["outcome"] = outcome
	}
	if metric == "lt_operations_completed" {
		p.Data.Tags["outcome"] = outcome
	}
	return p
}

func completeReportFixture() (reportManifest, []metricPoint) {
	manifest := reportManifest{SchemaVersion: 1, RunID: "fixture-run", Scenario: "browse", Dataset: "small", Finalized: true, Reasons: []string{}}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var points []metricPoint
	for index, rate := range []int{1, 2, 5, 10, 20, 40} {
		stage := reportStage{Stage: fmt.Sprint(index + 1), Phase: "measure", Start: start, SendEnd: start.Add(120 * time.Second), End: start.Add(120 * time.Second), Complete: true}
		manifest.Stages = append(manifest.Stages, stage)
		for i := 0; i < 120*rate; i++ {
			second := i / rate
			points = append(points, point(stage, "lt_operations_started", second, 1, ""), point(stage, "lt_http_duration", second, float64(i%100+1), "success"), point(stage, "http_reqs", second, 1, ""), point(stage, "lt_operations_completed", second, 1, "success"), point(stage, "iterations", second, 1, ""))
		}
		start = start.Add(130 * time.Second)
	}
	return manifest, points
}

func reportFiles(t *testing.T, manifest reportManifest, points []metricPoint) (string, string) {
	t.Helper()
	dir := t.TempDir()
	encoded, err := json.Marshal(manifest)
	require.NoError(t, err)
	manifestPath := filepath.Join(dir, "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, encoded, 0600))
	pointsPath := filepath.Join(dir, "points.jsonl")
	f, err := os.Create(pointsPath)
	require.NoError(t, err)
	encoder := json.NewEncoder(f)
	for _, p := range points {
		require.NoError(t, encoder.Encode(p))
	}
	require.NoError(t, f.Close())
	return manifestPath, pointsPath
}

func runReport(t *testing.T, manifest reportManifest, points []metricPoint) (map[string]any, int) {
	t.Helper()
	m, p := reportFiles(t, manifest, points)
	stdout, stderr, status := command(t, "report", "--manifest", m, "--points", p)
	require.Empty(t, stderr)
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &report), stdout)
	return report, status
}

func TestReport_UsesNearestRankAndFindsMaximumCompleteStage(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	manifest, points := completeReportFixture()
	report, status := runReport(t, manifest, points)
	require.Equal(t, 0, status)
	require.Equal(t, "limit_not_reached", report["status"])
	require.Equal(t, float64(40), report["max_qualified_rate"])
	stages := report["stages"].([]any)
	require.Len(t, stages, 6)
	// 4800件は1..100の48反復なのでnearest rankのp95=95、p99=99。
	last := stages[5].(map[string]any)
	require.Equal(t, float64(95), last["p95_ms"])
	require.Equal(t, float64(99), last["p99_ms"])
	require.Equal(t, float64(4800), last["http_count"])
	require.Equal(t, float64(40), last["http_per_second"])
	require.Equal(t, true, last["qualified"])
}

func TestReport_RejectsExactLatencyAndErrorRateBoundaries(t *testing.T) {
	for _, boundary := range []string{"p95", "p99", "error_rate"} {
		t.Run(boundary, func(t *testing.T) {
			manifest, points := completeReportFixture()
			httpIndex := 0
			for i := range points {
				p := &points[i]
				if p.Metric != "lt_http_duration" || p.Data.Tags["stage"] != "6" {
					continue
				}
				httpIndex++
				switch boundary {
				case "p95":
					p.Data.Value = 500
				case "p99":
					if httpIndex > 4704 {
						p.Data.Value = 1000
					}
				case "error_rate":
					if httpIndex <= 48 {
						p.Data.Tags["outcome"] = "failure"
					}
				}
			}
			report, status := runReport(t, manifest, points)
			require.Equal(t, 2, status)
			require.Contains(t, report["reasons"], "quality")
			require.Equal(t, float64(20), report["max_qualified_rate"])
			require.Equal(t, false, report["stages"].([]any)[5].(map[string]any)["qualified"])
		})
	}
}

func TestReport_KeepsWarmupOutOfCapacityButEvaluatesItsQuality(t *testing.T) {
	manifest, _ := completeReportFixture()
	warmup := reportStage{Stage: "warmup", Phase: "warmup", Start: manifest.Stages[0].Start.Add(-70 * time.Second), SendEnd: manifest.Stages[0].Start.Add(-10 * time.Second), End: manifest.Stages[0].Start.Add(-10 * time.Second), Complete: true}
	manifest.Stages = []reportStage{warmup}
	var points []metricPoint
	for i := 0; i < 60; i++ {
		points = append(points, point(warmup, "lt_http_duration", i, 500, "success"), point(warmup, "http_reqs", i, 1, ""), point(warmup, "lt_operations_started", i, 1, ""), point(warmup, "lt_operations_completed", i, 1, "success"), point(warmup, "iterations", i, 1, ""))
	}
	report, status := runReport(t, manifest, points)
	require.Equal(t, 2, status)
	require.Contains(t, report["reasons"], "quality")
	require.Len(t, report["stages"], 0)
	require.Equal(t, float64(0), report["max_qualified_rate"])
}

func TestReport_EvaluatesThirtySecondWindowsIndependently(t *testing.T) {
	manifest, points := completeReportFixture()
	// 最終段階4800HTTPのうち最初の30秒だけ2%失敗。段階全体は0.5%で基準内。
	// 最初の40HTTPのうち24件を失敗に固定する。
	n := 0
	for i := range points {
		p := &points[i]
		if p.Metric == "lt_http_duration" && p.Data.Tags["stage"] == "6" {
			if n < 24 {
				p.Data.Tags["outcome"] = "failure"
			} else {
				p.Data.Tags["outcome"] = "success"
			}
			n++
		}
	}
	report, status := runReport(t, manifest, points)
	require.Equal(t, 2, status)
	last := report["stages"].([]any)[5].(map[string]any)
	require.InDelta(t, .005, last["error_rate"], .000001)
	require.Equal(t, false, last["qualified"])
	windows := last["windows"].([]any)
	require.Len(t, windows, 4)
	require.InDelta(t, .02, windows[0].(map[string]any)["error_rate"], .000001)
	require.Equal(t, "quality_failed", windows[0].(map[string]any)["status"])
	require.Equal(t, "passed", windows[1].(map[string]any)["status"])
}

func TestReport_ExcludesInsufficientStageAndKeepsSmallWindowsProvisional(t *testing.T) {
	manifest, points := completeReportFixture()
	kept := points[:0]
	count := 0
	for _, p := range points {
		if p.Data.Tags["stage"] == "6" {
			if count >= 95*5 {
				continue
			}
			count++
		}
		kept = append(kept, p)
	}
	report, status := runReport(t, manifest, kept)
	require.Equal(t, 3, status)
	require.Contains(t, report["reasons"], "generation_shortfall")
	require.Equal(t, float64(20), report["max_qualified_rate"])
	last := report["stages"].([]any)[5].(map[string]any)
	require.Equal(t, false, last["qualified"])
	require.Equal(t, "provisional", last["windows"].([]any)[1].(map[string]any)["status"])
}

func TestReport_ExcludesDroppedStageEvenWhenQualityPasses(t *testing.T) {
	manifest, points := completeReportFixture()
	points = append(points, point(manifest.Stages[5], "dropped_iterations", 119, 1, ""))
	report, status := runReport(t, manifest, points)
	require.Equal(t, 3, status)
	require.Contains(t, report["reasons"], "dropped_iterations")
	require.Equal(t, float64(20), report["max_qualified_rate"])
}

func TestReport_RejectsMissingHTTPOrOperationResults(t *testing.T) {
	for _, metric := range []string{"lt_http_duration", "http_reqs", "lt_operations_completed", "iterations"} {
		t.Run(metric, func(t *testing.T) {
			manifest, points := completeReportFixture()
			for i, p := range points {
				if p.Metric == metric && p.Data.Tags["stage"] == "6" {
					points = append(points[:i], points[i+1:]...)
					break
				}
			}
			report, status := runReport(t, manifest, points)
			require.Equal(t, 5, status)
			require.Contains(t, report["reasons"], "result_incomplete")
			require.Equal(t, float64(20), report["max_qualified_rate"])
		})
	}
}

func TestReport_RetainsLatePointsAndMarksWindowIncomplete(t *testing.T) {
	manifest, points := completeReportFixture()
	for i, p := range points {
		if p.Metric == "lt_http_duration" && p.Data.Tags["stage"] == "6" {
			p.Data.Value = 499
			points = append(append(points[:i], points[i+1:]...), p)
			break
		}
	}
	report, status := runReport(t, manifest, points)
	require.Equal(t, 5, status)
	require.Contains(t, report["reasons"], "window_incomplete")
	last := report["stages"].([]any)[5].(map[string]any)
	require.Equal(t, float64(4800), last["http_count"])
	require.Equal(t, false, last["qualified"])
	require.Equal(t, "incomplete", last["windows"].([]any)[0].(map[string]any)["status"])
}

func TestReport_RejectsInvalidManifestAndPointContractsWithoutLeakingInput(t *testing.T) {
	for _, kind := range []string{"empty", "schema", "stage", "duplicate", "unknown_point_stage", "wrong_run", "missing_outcome", "missing_endpoint", "negative", "outside_stage", "invalid_json"} {
		t.Run(kind, func(t *testing.T) {
			manifest, points := completeReportFixture()
			switch kind {
			case "empty":
				manifest.Stages = nil
			case "schema":
				manifest.SchemaVersion = 2
			case "stage":
				manifest.Stages[0].Stage = "secret-value"
			case "duplicate":
				manifest.Stages = append(manifest.Stages, manifest.Stages[5])
			case "unknown_point_stage":
				points[1].Data.Tags["stage"] = "secret-value"
			case "wrong_run":
				points[1].Data.Tags["run_id"] = "secret-value"
			case "missing_outcome":
				delete(points[1].Data.Tags, "outcome")
			case "missing_endpoint":
				delete(points[1].Data.Tags, "endpoint")
			case "negative":
				points[1].Data.Value = -1
			case "outside_stage":
				points[1].Data.Time = manifest.Stages[0].Start.Add(-time.Second)
			}
			m, p := reportFiles(t, manifest, points)
			if kind == "invalid_json" {
				require.NoError(t, os.WriteFile(p, []byte("secret-value\n"), 0600))
			}
			stdout, stderr, status := command(t, "report", "--manifest", m, "--points", p)
			require.Equal(t, 5, status)
			require.NotContains(t, stdout+stderr, "secret-value")
			require.Empty(t, stdout)
			require.Equal(t, "loadtest invalid_input\n", stderr)
		})
	}
}

func TestReport_PreservesAllReasonsWithEnvironmentTakingPriority(t *testing.T) {
	manifest, points := completeReportFixture()
	manifest.Reasons = []string{"quality", "generation_shortfall", "result_incomplete", "connectivity"}
	manifest.Stages[5].Complete = false
	points = append(points, point(manifest.Stages[5], "dropped_iterations", 119, 1, ""))
	report, status := runReport(t, manifest, points)
	require.Equal(t, 4, status)
	for _, reason := range []string{"quality", "generation_shortfall", "result_incomplete", "connectivity", "dropped_iterations"} {
		require.Contains(t, report["reasons"], reason)
	}
	require.Equal(t, "environment_failed", report["status"])
	require.Equal(t, float64(20), report["max_qualified_rate"])
}

func TestReport_RejectsUnfinishedOrMissingPlannedStages(t *testing.T) {
	for _, kind := range []string{"missing_stage", "not_finalized", "partial_stage"} {
		t.Run(kind, func(t *testing.T) {
			manifest, points := completeReportFixture()
			switch kind {
			case "missing_stage":
				manifest.Stages = manifest.Stages[:5]
				kept := points[:0]
				for _, p := range points {
					if p.Data.Tags["stage"] != "6" {
						kept = append(kept, p)
					}
				}
				points = kept
			case "not_finalized":
				manifest.Finalized = false
			case "partial_stage":
				manifest.Stages[5].Complete = false
			}
			report, status := runReport(t, manifest, points)
			require.Equal(t, 5, status)
			require.Contains(t, report["reasons"], "result_incomplete")
		})
	}
}

func TestReport_IncludesDrainInStageButNotSendWindows(t *testing.T) {
	manifest, points := completeReportFixture()
	last := &manifest.Stages[5]
	last.End = last.SendEnd.Add(5 * time.Second)
	// 最後の1HTTPだけdrainへ移動。段階の件数と120秒分母を維持する。
	for i := len(points) - 1; i >= 0; i-- {
		if points[i].Metric == "lt_http_duration" {
			points[i].Data.Time = last.End
			break
		}
	}
	report, status := runReport(t, manifest, points)
	require.Equal(t, 0, status)
	result := report["stages"].([]any)[5].(map[string]any)
	require.Equal(t, float64(4800), result["http_count"])
	require.Equal(t, float64(40), result["http_per_second"])
	require.Equal(t, float64(5), result["drain_seconds"])
	require.Equal(t, float64(1199), result["windows"].([]any)[3].(map[string]any)["http_count"])
}

func TestReport_RecordsOperationCountsAndDistinguishesTransportFailure(t *testing.T) {
	manifest, points := completeReportFixture()
	for i := range points {
		p := &points[i]
		if p.Data.Tags["stage"] == "6" && (p.Metric == "lt_http_duration" || p.Metric == "lt_operations_completed") {
			p.Data.Tags["outcome"] = "failure"
		}
	}
	points = append(points, point(manifest.Stages[5], "lt_transport_errors", 119, 1, ""))
	report, status := runReport(t, manifest, points)
	require.Equal(t, 4, status)
	require.Contains(t, report["reasons"], "connectivity")
	require.Contains(t, report["reasons"], "quality")
	result := report["stages"].([]any)[5].(map[string]any)
	require.Equal(t, float64(4800), result["operations_started"])
	require.Equal(t, float64(4800), result["operations_completed"])
	require.Equal(t, float64(4800), result["operations_failed"])
	require.Equal(t, float64(40), result["operations_per_second"])
	require.Equal(t, false, result["qualified"])
}

func TestReport_RequiresAtLeastOneHTTPPerCompletedBrowseOperation(t *testing.T) {
	manifest, points := completeReportFixture()
	kept := points[:0]
	seen := 0
	for _, p := range points {
		if p.Data.Tags["stage"] == "6" && (p.Metric == "lt_http_duration" || p.Metric == "http_reqs") {
			seen++
			if seen > 198 {
				continue
			}
		}
		kept = append(kept, p)
	}
	report, status := runReport(t, manifest, kept)
	require.Equal(t, 5, status)
	require.Equal(t, false, report["stages"].([]any)[5].(map[string]any)["qualified"])
}

func TestReport_RejectsUnrecognizedReasonsAndTrailingManifest(t *testing.T) {
	for _, kind := range []string{"reason", "trailing"} {
		t.Run(kind, func(t *testing.T) {
			manifest, points := completeReportFixture()
			if kind == "reason" {
				manifest.Reasons = []string{"secret-value"}
			}
			m, p := reportFiles(t, manifest, points)
			if kind == "trailing" {
				f, err := os.OpenFile(m, os.O_APPEND|os.O_WRONLY, 0600)
				require.NoError(t, err)
				_, err = f.WriteString(" {} ")
				require.NoError(t, err)
				require.NoError(t, f.Close())
			}
			stdout, stderr, status := command(t, "report", "--manifest", m, "--points", p)
			require.Equal(t, 5, status)
			require.NotContains(t, stdout+stderr, "secret-value")
			require.Empty(t, stdout)
			require.Equal(t, "loadtest invalid_input\n", stderr)
			require.Empty(t, stdout)
		})
	}
}

func TestReport_UsesLoginThresholds(t *testing.T) {
	for _, duration := range []float64{1999, 2000} {
		t.Run(fmt.Sprint(duration), func(t *testing.T) {
			manifest, points := completeReportFixture()
			manifest.Scenario = "login"
			manifest.Dataset = "medium"
			rates := []int{1, 2, 3, 4, 5, 6}
			counts := map[string]int{}
			kept := points[:0]
			for _, p := range points {
				var stage int
				_, err := fmt.Sscan(p.Data.Tags["stage"], &stage)
				require.NoError(t, err)
				if counts[p.Data.Tags["stage"]] >= rates[stage-1]*120*5 {
					continue
				}
				counts[p.Data.Tags["stage"]]++
				p.Data.Tags["scenario"] = "login"
				if p.Metric == "lt_http_duration" {
					p.Data.Value = duration
				}
				kept = append(kept, p)
			}
			report, status := runReport(t, manifest, kept)
			if duration == 1999 {
				require.Equal(t, 0, status)
				require.Equal(t, float64(6), report["max_qualified_rate"])
			} else {
				require.Equal(t, 2, status)
				require.Equal(t, float64(0), report["max_qualified_rate"])
			}
		})
	}
}

func TestReport_ReportsMixedHTTPAndOperationRatesSeparately(t *testing.T) {
	manifest, points := completeReportFixture()
	manifest.Scenario = "mixed"
	manifest.Dataset = "medium"
	counters := map[string]int{}
	for i := range points {
		points[i].Data.Tags["scenario"] = "mixed"
	}
	original := append([]metricPoint(nil), points...)
	for _, p := range original {
		if p.Metric != "lt_http_duration" {
			continue
		}
		key := p.Data.Tags["stage"]
		counters[key]++
		if counters[key]%20 == 0 {
			for j := 0; j < 2; j++ {
				extra := p
				points = append(points, extra)
				extra.Metric = "http_reqs"
				extra.Data.Value = 1
				points = append(points, extra)
			}
		}
	}
	// 時刻順に整列し、遅着のfixtureとは区別する。
	sort.SliceStable(points, func(i, j int) bool { return points[i].Data.Time.Before(points[j].Data.Time) })
	report, status := runReport(t, manifest, points)
	require.Equal(t, 0, status)
	last := report["stages"].([]any)[5].(map[string]any)
	require.Equal(t, float64(44), last["http_per_second"])
	require.Equal(t, float64(40), last["operations_per_second"])
}

func smokeReportFixture() (reportManifest, []metricPoint) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stage := reportStage{Stage: "smoke", Phase: "smoke", Start: start, SendEnd: start.Add(30 * time.Second), End: start.Add(30 * time.Second), Complete: true}
	manifest := reportManifest{SchemaVersion: 1, RunID: "fixture-run", Scenario: "smoke", Dataset: "small", Finalized: true, Stages: []reportStage{stage}, Reasons: []string{}}
	points := []metricPoint{point(stage, "lt_operations_started", 0, 1, ""), point(stage, "lt_operations_completed", 10, 1, "success"), point(stage, "iterations", 10, 1, "")}
	for i := 0; i < 11; i++ {
		points = append(points, point(stage, "lt_http_duration", i, 10, "success"), point(stage, "http_reqs", i, 1, ""))
	}
	for i := range points {
		points[i].Data.Tags["scenario"] = "smoke"
	}
	return manifest, points
}

func TestReport_ReportsSmokeWithoutClaimingCapacity(t *testing.T) {
	manifest, points := smokeReportFixture()
	report, status := runReport(t, manifest, points)
	require.Equal(t, 0, status)
	require.Equal(t, "smoke_passed", report["status"])
	require.Equal(t, float64(0), report["max_qualified_rate"])
	require.Empty(t, report["stages"])
}

func TestReport_RejectsSmokeManifestWithMeasurementPhase(t *testing.T) {
	manifest, points := completeReportFixture()
	manifest.Scenario = "smoke"
	for i := range points {
		points[i].Data.Tags["scenario"] = "smoke"
	}
	m, p := reportFiles(t, manifest, points)
	stdout, stderr, status := command(t, "report", "--manifest", m, "--points", p)
	require.Equal(t, 5, status)
	require.Empty(t, stdout)
	require.Equal(t, "loadtest invalid_input\n", stderr)
}

func TestReport_ChecksWarmupCompletenessAndConnectivity(t *testing.T) {
	for _, kind := range []string{"missing", "connectivity"} {
		t.Run(kind, func(t *testing.T) {
			manifest, points := completeReportFixture()
			start := manifest.Stages[0].Start.Add(-70 * time.Second)
			warmup := reportStage{Stage: "warmup", Phase: "warmup", Start: start, SendEnd: start.Add(60 * time.Second), End: start.Add(60 * time.Second), Complete: kind != "missing"}
			manifest.Stages = append([]reportStage{warmup}, manifest.Stages...)
			if kind == "connectivity" {
				for i := 0; i < 60; i++ {
					points = append(points, point(warmup, "lt_operations_started", i, 1, ""), point(warmup, "lt_http_duration", i, 10, "success"), point(warmup, "http_reqs", i, 1, ""), point(warmup, "lt_operations_completed", i, 1, "success"), point(warmup, "iterations", i, 1, ""))
				}
				points = append(points, point(warmup, "lt_transport_errors", 59, 1, ""))
			}
			report, status := runReport(t, manifest, points)
			if kind == "missing" {
				require.Equal(t, 5, status)
				require.Contains(t, report["reasons"], "result_incomplete")
			} else {
				require.Equal(t, 4, status)
				require.Contains(t, report["reasons"], "connectivity")
			}
		})
	}
}

func TestReport_RejectsSmokeOperationAndTransportFailures(t *testing.T) {
	for _, kind := range []string{"operation", "connectivity"} {
		t.Run(kind, func(t *testing.T) {
			manifest, points := smokeReportFixture()
			if kind == "operation" {
				points[1].Data.Tags["outcome"] = "failure"
			} else {
				extra := point(manifest.Stages[0], "lt_transport_errors", 10, 1, "")
				extra.Data.Tags["scenario"] = "smoke"
				points = append(points, extra)
			}
			report, status := runReport(t, manifest, points)
			if kind == "operation" {
				require.Equal(t, 2, status)
				require.Contains(t, report["reasons"], "quality")
			} else {
				require.Equal(t, 4, status)
				require.Contains(t, report["reasons"], "connectivity")
			}
		})
	}
}

func TestReport_InvalidatesTerminalStageUnderGeneratorPressure(t *testing.T) {
	manifest, points := completeReportFixture()
	manifest.Reasons = []string{"generator_pressure"}
	report, status := runReport(t, manifest, points)
	require.Equal(t, 3, status)
	require.Equal(t, float64(20), report["max_qualified_rate"])
	require.Equal(t, false, report["stages"].([]any)[5].(map[string]any)["qualified"])
}

func TestReport_DetectsPointDelayedUntilNextStage(t *testing.T) {
	manifest, points := completeReportFixture()
	for i, p := range points {
		if p.Metric == "lt_http_duration" && p.Data.Tags["stage"] == "1" && p.Data.Time.Equal(manifest.Stages[0].Start.Add(119*time.Second)) {
			points = append(append(points[:i], points[i+1:]...), p)
			break
		}
	}
	report, status := runReport(t, manifest, points)
	require.Equal(t, 5, status)
	require.Contains(t, report["reasons"], "window_incomplete")
	first := report["stages"].([]any)[0].(map[string]any)
	require.Equal(t, float64(120), first["http_count"])
	require.Equal(t, false, first["qualified"])
	require.Equal(t, "incomplete", first["windows"].([]any)[3].(map[string]any)["status"])
}

func TestReport_RejectsMissingValuesAndFractionalCounters(t *testing.T) {
	for _, kind := range []string{"missing_value", "fractional_counter"} {
		t.Run(kind, func(t *testing.T) {
			manifest, points := completeReportFixture()
			m, p := reportFiles(t, manifest, points)
			data, err := os.ReadFile(p)
			require.NoError(t, err)
			lines := strings.Split(string(data), "\n")
			index := 1
			if kind == "fractional_counter" {
				index = 0
			}
			var row map[string]any
			require.NoError(t, json.Unmarshal([]byte(lines[index]), &row))
			values := row["data"].(map[string]any)
			if kind == "missing_value" {
				delete(values, "value")
			} else {
				values["value"] = 0.5
			}
			encoded, err := json.Marshal(row)
			require.NoError(t, err)
			lines[index] = string(encoded)
			require.NoError(t, os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0600))
			stdout, stderr, status := command(t, "report", "--manifest", m, "--points", p)
			require.Equal(t, 5, status)
			require.Empty(t, stdout)
			require.Equal(t, "loadtest invalid_input\n", stderr)
		})
	}
}

func TestReport_PreservesRecordedWindowCompleteness(t *testing.T) {
	manifest, points := completeReportFixture()
	manifest.Stages[5].IncompleteWindows = []int{3}
	report, status := runReport(t, manifest, points)
	require.Equal(t, 5, status)
	require.Equal(t, false, report["stages"].([]any)[5].(map[string]any)["qualified"])
	require.Equal(t, "incomplete", report["stages"].([]any)[5].(map[string]any)["windows"].([]any)[3].(map[string]any)["status"])
}

func TestReport_StopsOnWarmupGenerationShortfall(t *testing.T) {
	manifest, points := completeReportFixture()
	start := manifest.Stages[0].Start.Add(-70 * time.Second)
	warmup := reportStage{Stage: "warmup", Phase: "warmup", Start: start, SendEnd: start.Add(60 * time.Second), End: start.Add(60 * time.Second), Complete: true}
	manifest.Stages = append([]reportStage{warmup}, manifest.Stages...)
	var warmupPoints []metricPoint
	for i := 0; i < 59; i++ {
		warmupPoints = append(warmupPoints, point(warmup, "lt_operations_started", i, 1, ""), point(warmup, "lt_http_duration", i, 10, "success"), point(warmup, "http_reqs", i, 1, ""), point(warmup, "lt_operations_completed", i, 1, "success"), point(warmup, "iterations", i, 1, ""))
	}
	warmupPoints = append(warmupPoints, point(warmup, "dropped_iterations", 59, 1, ""))
	points = append(warmupPoints, points...)
	report, status := runReport(t, manifest, points)
	require.Equal(t, 3, status)
	require.Contains(t, report["reasons"], "generation_shortfall")
	require.Contains(t, report["reasons"], "dropped_iterations")
}

func TestReport_AcceptsSmokeFinishingBeforeThirtySeconds(t *testing.T) {
	manifest, points := smokeReportFixture()
	manifest.Stages[0].SendEnd = manifest.Stages[0].Start.Add(10 * time.Second)
	manifest.Stages[0].End = manifest.Stages[0].SendEnd
	report, status := runReport(t, manifest, points)
	require.Equal(t, 0, status)
	require.Equal(t, "smoke_passed", report["status"])
}

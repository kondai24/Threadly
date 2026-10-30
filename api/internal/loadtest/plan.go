package loadtest

import "errors"

// Planは、Dockerを変更せず確認できる負荷試験の固定条件を表す。
type Plan struct {
	SchemaVersion      int                 `json:"schema_version"`
	Scenario           string              `json:"scenario"`
	Dataset            string              `json:"dataset"`
	Rates              []int               `json:"rates"`
	WarmupSeconds      int                 `json:"warmup_seconds"`
	StageSeconds       int                 `json:"stage_seconds"`
	MaxVUs             int                 `json:"max_vus"`
	MaxRunSeconds      int                 `json:"max_run_seconds"`
	StopRequestSeconds int                 `json:"stop_request_seconds"`
	HTTPTimeoutSeconds int                 `json:"http_timeout_seconds"`
	OperationsPerCycle int                 `json:"operations_per_cycle"`
	ReadsPerCycle      int                 `json:"reads_per_cycle"`
	HTTPPerCycle       int                 `json:"http_per_cycle"`
	SteadyCPU          float64             `json:"steady_cpu"`
	SteadyMemoryMiB    int                 `json:"steady_memory_mib"`
	Resources          map[string]Resource `json:"resources"`
	VMMax              VM                  `json:"vm_max"`
	FixtureCounts      Counts              `json:"fixture_counts"`
	DrainSeconds       int                 `json:"drain_seconds"`
	WindowSeconds      int                 `json:"window_seconds"`
	WindowMinHTTP      int                 `json:"window_min_http"`
	StageMinHTTP       int                 `json:"stage_min_http"`
	P95LimitMS         int                 `json:"p95_limit_ms"`
	P99LimitMS         int                 `json:"p99_limit_ms"`
	ErrorRateLimit     float64             `json:"error_rate_limit"`
	BuilderResource    Resource            `json:"builder_resource"`
}

// Resourceは、swap追加枠を含むサービス別の実行上限を表す。
type Resource struct {
	CPU        float64 `json:"cpu"`
	MemoryMiB  int     `json:"memory_mib"`
	MemswapMiB int     `json:"memswap_mib"`
	Restart    string  `json:"restart"`
}

// VMは、Docker VMに設定する上限を表す。
type VM struct {
	CPU       int `json:"cpu"`
	MemoryMiB int `json:"memory_mib"`
}

// Countsは、初期fixtureの件数を表す。
type Counts struct {
	Users        int `json:"users"`
	Posts        int `json:"posts"`
	Comments     int `json:"comments"`
	PostLikes    int `json:"post_likes"`
	CommentLikes int `json:"comment_likes"`
}

// NewPlanは、scenarioとdatasetの組合せを検証して固定計画を返す。
func NewPlan(scenario, dataset string) (Plan, error) {
	if dataset != "small" && dataset != "medium" && dataset != "large" {
		return Plan{}, errors.New("invalid_input")
	}
	switch scenario {
	case "browse":
	case "mixed", "login":
		if dataset != "medium" {
			return Plan{}, errors.New("invalid_input")
		}
	case "smoke":
		if dataset != "small" {
			return Plan{}, errors.New("invalid_input")
		}
	default:
		return Plan{}, errors.New("invalid_input")
	}
	plan := Plan{
		SchemaVersion: 1, Scenario: scenario, Dataset: dataset,
		Rates: []int{1, 2, 5, 10, 20, 40}, WarmupSeconds: 60, StageSeconds: 120,
		MaxVUs: 32, MaxRunSeconds: 900, StopRequestSeconds: 890, HTTPTimeoutSeconds: 5,
		// Like/Unlikeは1操作でHTTP2回のため、80:20の20操作は22HTTPになる。
		OperationsPerCycle: 20, ReadsPerCycle: 16, HTTPPerCycle: 22,
		SteadyCPU: 3.5, SteadyMemoryMiB: 3840,
	}
	// 一時toolとbuilderは測定へ重ねず、常駐+k6の合計上限とVMの間に余裕を残す。
	plan.Resources = map[string]Resource{
		"api": {1, 1024, 1024, "no"}, "loadtest-db": {1, 1024, 1024, "no"}, "k6": {0.75, 768, 768, "no"},
		"otel-collector": {0.25, 256, 256, "no"}, "tempo": {0.25, 512, 512, "no"}, "grafana": {0.25, 256, 256, "no"},
		"storage-init": {0.25, 64, 64, "no"}, "tool": {0.25, 256, 256, "no"},
	}
	plan.VMMax = VM{4, 6144}
	plan.BuilderResource = Resource{1, 1536, 1536, "no"}
	plan.DrainSeconds = 10
	plan.WindowSeconds = 30
	plan.WindowMinHTTP = 20
	plan.StageMinHTTP = 100
	plan.P95LimitMS = 500
	plan.P99LimitMS = 1000
	plan.ErrorRateLimit = 0.01
	switch dataset {
	case "small":
		plan.FixtureCounts = Counts{32, 100, 1000, 200, 2000}
	case "medium":
		plan.FixtureCounts = Counts{64, 1000, 10000, 2000, 20000}
	case "large":
		plan.FixtureCounts = Counts{128, 10000, 100000, 20000, 200000}
	}
	if scenario != "mixed" {
		plan.OperationsPerCycle = 1
		plan.HTTPPerCycle = 1
		plan.ReadsPerCycle = 1
	}
	if scenario == "login" {
		plan.Rates = []int{1, 2, 3, 4, 5, 6}
		plan.MaxVUs = 8
		plan.ReadsPerCycle = 0
		plan.P95LimitMS = 2000
		plan.P99LimitMS = 3000
	}
	if scenario == "smoke" {
		plan.ReadsPerCycle = 3
		plan.HTTPPerCycle = 11
		plan.Rates = []int{}
		plan.MaxVUs = 1
		plan.WarmupSeconds = 0
		plan.StageSeconds = 0
		plan.MaxRunSeconds = 30
		plan.StopRequestSeconds = 20
	}
	return plan, nil
}

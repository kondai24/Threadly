package loadtest

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStageFits_KeepsSendDrainAndMarginBeforeStopRequest(t *testing.T) {
	cases := []struct {
		name    string
		elapsed time.Duration
		send    time.Duration
		want    bool
	}{
		// 890秒の停止要求までに送信120秒・drain10秒・余裕10秒が必要になる。
		{name: "750秒経過なら最後の段階を開始できる", elapsed: 750 * time.Second, send: 120 * time.Second, want: true},
		{name: "751秒経過なら短縮せず開始しない", elapsed: 751 * time.Second, send: 120 * time.Second, want: false},
		{name: "warmupは開始時点で必ず収まる", elapsed: 0, send: 60 * time.Second, want: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, StageFits(tt.elapsed, tt.send))
		})
	}
}

func TestWriteReportCSV_WritesSameStageValuesAsReport(t *testing.T) {
	report := Report{Stages: []StageResult{{
		Stage: "1", Rate: 5, PlannedOperations: 600, HTTPCount: 600, HTTPPerSecond: 5, P95MS: 120.5, P99MS: 300, ErrorRate: 0.001, Qualified: true,
		DrainSeconds: 1.5, OperationsStarted: 600, OperationsCompleted: 598, OperationsFailed: 2, OperationsPerSecond: 4.98, DroppedIterations: 3,
		Windows: []WindowResult{{Status: "passed"}, {Status: "passed"}, {Status: "quality_failed"}, {Status: "incomplete"}},
	}}}
	var out bytes.Buffer

	require.NoError(t, WriteReportCSV(&out, report))

	require.Equal(t, "stage,requested_rate,planned_operations,operations_started,operations_completed,operations_failed,dropped_iterations,"+
		"http_count,http_per_second,operations_per_second,p95_ms,p99_ms,error_rate,windows_passed,windows_failed,windows_provisional,windows_incomplete,qualified,drain_seconds\n"+
		"1,5,600,600,598,2,3,600,5,4.98,120.5,300,0.001,2,1,0,1,true,1.5\n", out.String())
}

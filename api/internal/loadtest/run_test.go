package loadtest

import (
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

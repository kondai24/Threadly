package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func ownedVolumeRules() []dockerRule {
	return []dockerRule{
		{Contains: []string{"volume ls", "loadtest_db_data"}, Stdout: "threadly-loadtest_loadtest_db_data\n"},
		{Contains: []string{"volume ls", "loadtest_trace_data"}, Stdout: "threadly-loadtest_loadtest_trace_data\n"},
		{Contains: []string{"volume inspect", "threadly-loadtest_loadtest_db_data"}, Stdout: projectLabelsOK + "\n"},
		{Contains: []string{"volume inspect", "threadly-loadtest_loadtest_trace_data"}, Stdout: `{"com.docker.compose.project":"threadly-loadtest","com.docker.compose.volume":"loadtest_trace_data"}` + "\n"},
	}
}

func TestReset_RecreatesOnlyDedicatedDatabaseByDefault(t *testing.T) {
	env := newLoadtestEnv(t, ownedVolumeRules()...)
	writeState(t, env, true)

	_, stderr, status := env.run(t, "reset", "--confirm-project", "threadly-loadtest")

	require.Equal(t, 0, status, stderr)
	calls := env.dockerCalls(t)
	down := indexOfCall(calls, "-p threadly-loadtest -f "+filepath.Join(env.root, "loadtest", "compose.yaml"))
	removeDB := indexOfCall(calls, "volume rm threadly-loadtest_loadtest_db_data")
	// containerを止めてから専用DB volumeだけを削除し、Trace volumeは明示指定まで残す。
	require.True(t, down >= 0 && down < removeDB, calls)
	require.Contains(t, calls[down], " down")
	require.Equal(t, -1, indexOfCall(calls, "volume rm threadly-loadtest_loadtest_trace_data"))
	for _, call := range calls {
		require.NotContains(t, call, "down -v")
		require.NotContains(t, call, "--volumes")
	}
	_, err := os.Stat(filepath.Join(env.root, "loadtest", "state", "fixture.json"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestReset_RemovesTraceVolumeOnlyWhenRequested(t *testing.T) {
	env := newLoadtestEnv(t, ownedVolumeRules()...)

	_, stderr, status := env.run(t, "reset", "--reset-traces", "--confirm-project", "threadly-loadtest")

	require.Equal(t, 0, status, stderr)
	calls := env.dockerCalls(t)
	require.GreaterOrEqual(t, indexOfCall(calls, "volume rm threadly-loadtest_loadtest_db_data"), 0)
	require.GreaterOrEqual(t, indexOfCall(calls, "volume rm threadly-loadtest_loadtest_trace_data"), 0)
}

func TestReset_RefusesWithoutDeletingWhenTargetIsNotConfirmed(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		rules      []dockerRule
		lock       bool
		wantReason string
		noDocker   bool
	}{
		{name: "確認指定がなければ削除しない", args: []string{"reset"}, wantReason: "invalid_input", noDocker: true},
		{name: "確認値が専用projectと違えば削除しない", args: []string{"reset", "--confirm-project", "api"}, wantReason: "invalid_input", noDocker: true},
		{
			name: "volumeの所有projectが違えば停止も削除もしない", args: []string{"reset", "--confirm-project", "threadly-loadtest"},
			rules: []dockerRule{
				{Contains: []string{"volume ls"}, Stdout: "threadly-loadtest_loadtest_db_data\n"},
				{Contains: []string{"volume inspect"}, Stdout: `{"com.docker.compose.project":"api","com.docker.compose.volume":"loadtest_db_data"}` + "\n"},
			},
			wantReason: "target_mismatch",
		},
		{name: "実行中の操作があれば何もしない", args: []string{"reset", "--confirm-project", "threadly-loadtest"}, rules: ownedVolumeRules(), lock: true, wantReason: "locked", noDocker: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			env := newLoadtestEnv(t, tt.rules...)
			writeState(t, env, true)
			if tt.lock {
				// 別processが排他を保持している状態を、同じlock fileのflockで再現する。
				file, err := os.OpenFile(filepath.Join(env.root, "loadtest", "state", "lock"), os.O_CREATE|os.O_RDWR, 0o600)
				require.NoError(t, err)
				require.NoError(t, syscall.Flock(int(file.Fd()), syscall.LOCK_EX))
				t.Cleanup(func() { _ = file.Close() })
			}

			_, stderr, status := env.run(t, tt.args...)

			require.Equal(t, 5, status)
			require.Equal(t, "loadtest "+tt.wantReason+"\n", stderr)
			calls := env.dockerCalls(t)
			if tt.noDocker {
				require.Empty(t, calls)
			}
			for _, call := range calls {
				require.False(t, strings.HasPrefix(call, "volume rm"), call)
				require.NotContains(t, call, " down")
			}
			_, err := os.Stat(filepath.Join(env.root, "loadtest", "state", "fixture.json"))
			require.NoError(t, err)
		})
	}
}

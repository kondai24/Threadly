package loadtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	// Projectは、負荷試験専用のCompose project名。初期化対象の照合にも使う固定値である。
	Project     = "threadly-loadtest"
	dbVolume    = Project + "_loadtest_db_data"
	traceVolume = Project + "_loadtest_trace_data"

	prepareTimeout = 10 * time.Minute
)

// Failureは、終了コードと固定分類だけを持つerror。Dockerやfixtureのraw errorを利用者へ出さない。
type Failure struct {
	Code   int
	Reason string
}

func (f *Failure) Error() string { return f.Reason }

func invalidInput(reason string) error { return &Failure{Code: 5, Reason: reason} }
func environmentFailure(reason string) error {
	return &Failure{Code: 4, Reason: reason}
}

// FixtureStateは、準備済みfixtureと使用済みかどうかを記録する。runは未使用の状態だけを受け付ける。
type FixtureState struct {
	FixtureSummary
	PreparedAt time.Time `json:"prepared_at"`
	Used       bool      `json:"used"`
}

// Environmentは、api/ を基点に専用Composeと状態ファイルを扱う。
type Environment struct {
	Root string
}

func (e Environment) composeFile() string { return filepath.Join(e.Root, "loadtest", "compose.yaml") }
func (e Environment) secretsFile() string {
	return filepath.Join(e.Root, "loadtest", "secrets", "loadtest.env")
}
func (e Environment) stateDir() string  { return filepath.Join(e.Root, "loadtest", "state") }
func (e Environment) stateFile() string { return filepath.Join(e.stateDir(), "fixture.json") }

// docker は、引数を固定したdocker CLIを実行し、stdoutだけを返す。stderrは秘密値を含み得るため表示しない。
func (e Environment) docker(ctx context.Context, args ...string) ([]byte, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = e.Root
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// compose は、固定project・専用Composeファイル・専用秘密設定だけを使う。
// 開発用.envや任意のproject名へフォールバックしない。
func (e Environment) compose(ctx context.Context, args ...string) ([]byte, error) {
	base := []string{"compose", "-p", Project, "-f", e.composeFile(), "--env-file", e.secretsFile()}
	return e.docker(ctx, append(base, args...)...)
}

// lockは、prepare・reset・runを同時に1つだけ実行する。processの終了でOSが解放する。
func (e Environment) lock() (func(), error) {
	if err := os.MkdirAll(e.stateDir(), 0o700); err != nil {
		return nil, environmentFailure("state_unavailable")
	}
	file, err := os.OpenFile(filepath.Join(e.stateDir(), "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, environmentFailure("state_unavailable")
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, invalidInput("locked")
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func (e Environment) readState() (FixtureState, bool, error) {
	data, err := os.ReadFile(e.stateFile())
	if errors.Is(err, os.ErrNotExist) {
		return FixtureState{}, false, nil
	}
	if err != nil {
		return FixtureState{}, false, environmentFailure("state_unavailable")
	}
	var state FixtureState
	if err := json.Unmarshal(data, &state); err != nil {
		return FixtureState{}, false, invalidInput("state_invalid")
	}
	return state, true, nil
}

func (e Environment) writeState(state FixtureState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return environmentFailure("state_unavailable")
	}
	temp := e.stateFile() + ".tmp"
	if err := os.WriteFile(temp, data, 0o600); err != nil {
		return environmentFailure("state_unavailable")
	}
	if err := os.Rename(temp, e.stateFile()); err != nil {
		return environmentFailure("state_unavailable")
	}
	return nil
}

// Prepareは、空の専用DBへ既存Atlas migrationとfixtureを明示的に適用し、未使用状態を記録する。
func (e Environment) Prepare(ctx context.Context, dataset string) (FixtureSummary, error) {
	fixture, err := GenerateFixture(dataset)
	if err != nil {
		return FixtureSummary{}, invalidInput("invalid_input")
	}
	if _, err := os.Stat(e.secretsFile()); err != nil {
		return FixtureSummary{}, invalidInput("secrets_missing")
	}
	unlock, err := e.lock()
	if err != nil {
		return FixtureSummary{}, err
	}
	defer unlock()
	if _, prepared, err := e.readState(); err != nil {
		return FixtureSummary{}, err
	} else if prepared {
		return FixtureSummary{}, invalidInput("already_prepared")
	}
	ctx, cancel := context.WithTimeout(ctx, prepareTimeout)
	defer cancel()
	if _, err := e.verifyVolume(ctx, dbVolume, "loadtest_db_data"); err != nil {
		return FixtureSummary{}, err
	}
	// builderは常駐サービスより先に、制限付きの専用containerで通常buildする。
	steps := [][]string{
		{"--profile", "build", "run", "--rm", "builder"},
		{"up", "-d", "--wait", "loadtest-db"},
		{"--profile", "prepare", "run", "--rm", "migrate"},
	}
	for _, step := range steps {
		if _, err := e.compose(ctx, step...); err != nil {
			return FixtureSummary{}, environmentFailure("prepare_failed")
		}
	}
	output, err := e.compose(ctx, "--profile", "prepare", "run", "--rm", "tool", "seed", "--dataset", dataset)
	if err != nil {
		return FixtureSummary{}, environmentFailure("prepare_failed")
	}
	var summary FixtureSummary
	if err := json.Unmarshal(output, &summary); err != nil {
		return FixtureSummary{}, invalidInput("fixture_mismatch")
	}
	expected := FixtureSummary{Version: FixtureVersion, Dataset: dataset, Counts: fixture.Counts(), Checksum: fixture.Checksum()}
	if summary != expected {
		return FixtureSummary{}, invalidInput("fixture_mismatch")
	}
	if err := e.writeState(FixtureState{FixtureSummary: summary, PreparedAt: time.Now().UTC()}); err != nil {
		return FixtureSummary{}, err
	}
	return summary, nil
}

// verifyVolumeは、volumeが存在する場合に専用projectのComposeが作ったものかを照合する。
// 名前だけが一致する外部volumeや他projectのvolumeを初期化・削除対象にしない。
func (e Environment) verifyVolume(ctx context.Context, name, key string) (bool, error) {
	output, err := e.docker(ctx, "volume", "ls", "-q", "--filter", "name=^"+name+"$")
	if err != nil {
		return false, environmentFailure("docker_unavailable")
	}
	if len(bytes.TrimSpace(output)) == 0 {
		return false, nil
	}
	output, err = e.docker(ctx, "volume", "inspect", "--format", "{{json .Labels}}", name)
	if err != nil {
		return false, environmentFailure("docker_unavailable")
	}
	var labels map[string]string
	if err := json.Unmarshal(output, &labels); err != nil ||
		labels["com.docker.compose.project"] != Project || labels["com.docker.compose.volume"] != key {
		return false, invalidInput("target_mismatch")
	}
	return true, nil
}

// Resetは、専用projectのcontainerを止め、専用DB volume（明示時はTrace volumeも）だけを削除する。
// 汎用のdown -vは使わず、所有projectを照合できたvolumeを名前指定で削除する。
func (e Environment) Reset(ctx context.Context, confirmProject string, resetTraces bool) error {
	if confirmProject != Project {
		return invalidInput("invalid_input")
	}
	unlock, err := e.lock()
	if err != nil {
		return err
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(ctx, prepareTimeout)
	defer cancel()
	targets := []struct{ name, key string }{{dbVolume, "loadtest_db_data"}}
	if resetTraces {
		targets = append(targets, struct{ name, key string }{traceVolume, "loadtest_trace_data"})
	}
	// 削除前に全対象を照合し、1つでも不一致なら停止・削除を始めない。
	var existing []string
	for _, target := range targets {
		exists, err := e.verifyVolume(ctx, target.name, target.key)
		if err != nil {
			return err
		}
		if exists {
			existing = append(existing, target.name)
		}
	}
	if _, err := e.compose(ctx, "down", "--remove-orphans"); err != nil {
		return environmentFailure("reset_failed")
	}
	for _, name := range existing {
		if _, err := e.docker(ctx, "volume", "rm", name); err != nil {
			return environmentFailure("reset_failed")
		}
	}
	if err := os.Remove(e.stateFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return environmentFailure("state_unavailable")
	}
	return nil
}

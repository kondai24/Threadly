// fakedockerは、loadtest CLIのテストでdocker実行ファイルの代わりにPATHへ置く。
// 受け取った引数をFAKE_DOCKER_LOGへ1行ずつ記録し、FAKE_DOCKER_RULESの最初に一致した応答を返す。
// `--out json=/results/...` を含む実行ではk6の代わりにPointを書き出し、`kill` では
// `--name` で起動中の代替processへsignalを送る。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var pastTime = regexp.MustCompile(`\{\{NOW-[0-9]+(ms|s)\}\}`)

type write struct {
	DelayMS int    `json:"delay_ms"`
	Line    string `json:"line"`
}

type rule struct {
	Contains []string `json:"contains"`
	// AfterCallsは、同じ条件に一致する過去の呼出がこの回数以上ある場合だけ適用する。
	AfterCalls int     `json:"after_calls"`
	Stdout     string  `json:"stdout"`
	Exit       int     `json:"exit"`
	Writes     []write `json:"writes"`
	SleepMS    int     `json:"sleep_ms"`
}

func main() {
	args := os.Args[1:]
	joined := strings.Join(args, " ")
	previous := previousCalls()
	logCall(args)
	if len(args) > 0 && args[0] == "kill" {
		os.Exit(kill(args))
	}
	var rules []rule
	if path := os.Getenv("FAKE_DOCKER_RULES"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(data, &rules) != nil {
			os.Exit(98)
		}
	}
	for _, r := range rules {
		if !matches(joined, r.Contains) || countMatches(previous, r.Contains) < r.AfterCalls {
			continue
		}
		fmt.Print(r.Stdout)
		os.Exit(execute(args, r))
	}
}

func execute(args []string, r rule) int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	if name := argAfter(args, "--name"); name != "" && os.Getenv("FAKE_DOCKER_PIDDIR") != "" {
		pidFile := filepath.Join(os.Getenv("FAKE_DOCKER_PIDDIR"), name)
		_ = os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600)
		defer func() { _ = os.Remove(pidFile) }()
	}
	target, runID := outputTarget(args)
	for _, w := range r.Writes {
		select {
		case <-signals:
			return 130
		case <-time.After(time.Duration(w.DelayMS) * time.Millisecond):
		}
		if target == "" {
			continue
		}
		line := strings.ReplaceAll(w.Line, "{{RUN_ID}}", runID)
		// {{NOW-500ms}} のように、書出し時刻より前の採取時刻を持つPointも再現する。
		line = pastTime.ReplaceAllStringFunc(line, func(match string) string {
			offset, _ := time.ParseDuration(match[len("{{NOW-") : len(match)-2])
			return time.Now().UTC().Add(-offset).Format(time.RFC3339Nano)
		})
		line = strings.ReplaceAll(line, "{{NOW}}", time.Now().UTC().Format(time.RFC3339Nano))
		file, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return 97
		}
		_, _ = file.WriteString(line + "\n")
		_ = file.Close()
	}
	select {
	case <-signals:
		return 130
	case <-time.After(time.Duration(r.SleepMS) * time.Millisecond):
	}
	return r.Exit
}

// outputTargetは、container内の /results/<run>/... を、CLIの作業ディレクトリ配下の
// loadtest/results/<run>/... へ対応させる。
func outputTarget(args []string) (string, string) {
	for _, arg := range args {
		if rest, ok := strings.CutPrefix(arg, "json=/results/"); ok {
			runID, _, _ := strings.Cut(rest, "/")
			return filepath.Join("loadtest", "results", rest), runID
		}
	}
	return "", ""
}

func kill(args []string) int {
	sig := syscall.SIGKILL
	if value := argAfter(args, "-s"); value == "SIGINT" {
		sig = syscall.SIGINT
	}
	name := args[len(args)-1]
	data, err := os.ReadFile(filepath.Join(os.Getenv("FAKE_DOCKER_PIDDIR"), name))
	if err != nil {
		return 1
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil || syscall.Kill(pid, sig) != nil {
		return 1
	}
	return 0
}

func argAfter(args []string, flag string) string {
	for i, arg := range args[:len(args)-1] {
		if arg == flag {
			return args[i+1]
		}
	}
	return ""
}

func matches(joined string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(joined, part) {
			return false
		}
	}
	return true
}

func previousCalls() []string {
	data, err := os.ReadFile(os.Getenv("FAKE_DOCKER_LOG"))
	if err != nil {
		return nil
	}
	var calls []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var args []string
		if json.Unmarshal([]byte(line), &args) == nil {
			calls = append(calls, strings.Join(args, " "))
		}
	}
	return calls
}

func countMatches(calls []string, parts []string) int {
	count := 0
	for _, call := range calls {
		if matches(call, parts) {
			count++
		}
	}
	return count
}

func logCall(args []string) {
	path := os.Getenv("FAKE_DOCKER_LOG")
	if path == "" {
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(99)
	}
	line, _ := json.Marshal(args)
	_, _ = file.Write(append(line, '\n'))
	_ = file.Close()
}

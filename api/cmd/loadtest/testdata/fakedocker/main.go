// fakedockerは、loadtest CLIのテストでdocker実行ファイルの代わりにPATHへ置く。
// 受け取った引数をFAKE_DOCKER_LOGへ1行ずつ記録し、FAKE_DOCKER_RULESの最初に一致した応答を返す。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type rule struct {
	Contains []string `json:"contains"`
	Stdout   string   `json:"stdout"`
	Exit     int      `json:"exit"`
}

func main() {
	args := os.Args[1:]
	if path := os.Getenv("FAKE_DOCKER_LOG"); path != "" {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(99)
		}
		line, _ := json.Marshal(args)
		_, _ = file.Write(append(line, '\n'))
		_ = file.Close()
	}
	var rules []rule
	if path := os.Getenv("FAKE_DOCKER_RULES"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(data, &rules) != nil {
			os.Exit(98)
		}
	}
	joined := strings.Join(args, " ")
	for _, r := range rules {
		matched := true
		for _, part := range r.Contains {
			if !strings.Contains(joined, part) {
				matched = false
				break
			}
		}
		if matched {
			fmt.Print(r.Stdout)
			os.Exit(r.Exit)
		}
	}
}

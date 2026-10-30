package loadtest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	traceVolumeLimitBytes = 1024 * 1024 * 1024
	resultsLimitBytes     = 512 * 1024 * 1024
	pressureRatio         = 0.9
	pressureDuration      = 30 * time.Second
	// docker system df -vはVM上の全volume・imageを走査して重いため、Trace volumeは30秒ごとに確認する。
	traceCheckEvery = 6
)

// ResourceSampleは、5秒ごとのcontainer資源の実値と上限比である。Dockerの%値は100%=1CPUなので、
// サービスのCPU上限で割ったquota比を別に残す。
type ResourceSample struct {
	Time             time.Time `json:"time"`
	Service          string    `json:"service"`
	CPUPercent       float64   `json:"cpu_percent"`
	CPUQuotaRatio    float64   `json:"cpu_quota_ratio"`
	MemoryRatio      float64   `json:"memory_ratio"`
	MemoryUsageRaw   string    `json:"memory_usage_raw"`
	ContainerName    string    `json:"container"`
	MemoryPercentRaw string    `json:"memory_percent_raw"`
}

// sampleResourcesは、常駐サービスとk6の資源を1回採取して結果ディレクトリへ追記する。
// k6Nameが空なら、k6 containerがまだ起動していないとみなして常駐サービスだけを採取する。
func (e Environment) sampleResources(dir, k6Name string, plan Plan) ([]ResourceSample, error) {
	services := map[string]string{}
	names := []string{}
	for _, service := range monitoredServices {
		name := Project + "-" + service + "-1"
		services[name] = service
		names = append(names, name)
	}
	if k6Name != "" {
		services[k6Name] = "k6"
		names = append(names, k6Name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()
	output, err := e.docker(ctx, append([]string{"stats", "--no-stream", "--format", "{{json .}}"}, names...)...)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var samples []ResourceSample
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var raw struct{ Name, CPUPerc, MemUsage, MemPerc string }
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			return nil, err
		}
		service, ok := services[raw.Name]
		if !ok {
			continue
		}
		cpu, cpuErr := parsePercent(raw.CPUPerc)
		memory, memErr := parsePercent(raw.MemPerc)
		if cpuErr != nil || memErr != nil {
			return nil, fmt.Errorf("parse stats")
		}
		sample := ResourceSample{
			Time: now, Service: service, ContainerName: raw.Name, CPUPercent: cpu,
			MemoryRatio: memory / 100, MemoryUsageRaw: raw.MemUsage, MemoryPercentRaw: raw.MemPerc,
		}
		if limit := plan.Resources[service].CPU; limit > 0 {
			sample.CPUQuotaRatio = cpu / 100 / limit
		}
		samples = append(samples, sample)
		delete(services, raw.Name)
	}
	// 欠けたcontainerを合格扱いにすると、圧迫や停止を見逃す。
	if len(services) > 0 {
		return nil, fmt.Errorf("missing stats")
	}
	file, err := os.OpenFile(filepath.Join(dir, "resources.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	encoder := json.NewEncoder(file)
	for _, sample := range samples {
		if err := encoder.Encode(sample); err != nil {
			return nil, err
		}
	}
	return samples, nil
}

func parsePercent(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, "%")), 64)
}

// generatorPressureは、k6のCPU quota比90%以上が30秒続くか、メモリ上限比90%以上で生成側の圧迫とみなす。
// 制限値は増やさず、該当段階を容量判定から外すために停止する。
type generatorPressure struct {
	cpuSince time.Time
}

func (g *generatorPressure) observe(samples []ResourceSample) bool {
	for _, sample := range samples {
		if sample.Service != "k6" {
			continue
		}
		if sample.MemoryRatio >= pressureRatio {
			return true
		}
		if sample.CPUQuotaRatio < pressureRatio {
			g.cpuSince = time.Time{}
			continue
		}
		if g.cpuSince.IsZero() {
			g.cpuSince = sample.Time
		}
		if sample.Time.Sub(g.cpuSince) >= pressureDuration {
			return true
		}
	}
	return false
}

// traceVolumeBytesは、専用Trace volumeの使用量を返す。容量監視の計測不能は合格扱いにしない。
func (e Environment) traceVolumeBytes() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()
	output, err := e.docker(ctx, "system", "df", "-v", "--format", "{{json .Volumes}}")
	if err != nil {
		return 0, err
	}
	var volumes []struct{ Name, Size string }
	if err := json.Unmarshal(output, &volumes); err != nil {
		return 0, err
	}
	for _, volume := range volumes {
		if volume.Name == traceVolume {
			return parseDockerSize(volume.Size)
		}
	}
	return 0, fmt.Errorf("trace volume not found")
}

// parseDockerSizeは、Docker CLIの10進単位（kB/MB/GB/TB）の表示をbyteへ戻す。
func parseDockerSize(value string) (int64, error) {
	units := []struct {
		suffix string
		factor float64
	}{{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1}}
	for _, unit := range units {
		if number, ok := strings.CutSuffix(strings.TrimSpace(value), unit.suffix); ok {
			parsed, err := strconv.ParseFloat(number, 64)
			if err != nil {
				return 0, err
			}
			return int64(parsed * unit.factor), nil
		}
	}
	return 0, fmt.Errorf("unknown size")
}

func dirSize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// checkStorageは、結果ディレクトリと、includeTraceならTrace volumeの上限超過・計測不能を停止理由にする。
func (e Environment) checkStorage(dir string, includeTrace bool) string {
	if includeTrace {
		traceBytes, err := e.traceVolumeBytes()
		if err != nil {
			return "monitoring_failed"
		}
		if traceBytes >= traceVolumeLimitBytes {
			return "disk_limit"
		}
	}
	resultBytes, err := dirSize(dir)
	if err != nil {
		return "monitoring_failed"
	}
	if resultBytes >= resultsLimitBytes {
		return "disk_limit"
	}
	return ""
}

// conditionMetadataは、比較条件をmanifestへ残すための版・環境情報を集める。
// 取得できない値はunknownとし、取得失敗を測定結果の合格へ混ぜない。秘密値は含めない。
func (e Environment) conditionMetadata(vm dockerInfo) map[string]any {
	// 遅いdaemonで後半の項目だけ欠けないよう、コマンドごとに時間制限を与える。
	ctx := context.Background()
	environment := map[string]any{
		"vm_cpu": vm.NCPU, "vm_memory_bytes": vm.MemTotal, "swap_limit": vm.SwapLimit,
		"docker_client_version": "unknown", "docker_server_version": "unknown", "architecture": "unknown",
		"compose_version": "unknown", "commit_sha": "unknown", "dirty": "unknown",
	}
	if output, err := e.dockerWithin(ctx, "version", "--format", "{{json .}}"); err == nil {
		var version struct {
			Client struct{ Version string }
			Server struct{ Version, Arch string }
		}
		if json.Unmarshal(output, &version) == nil {
			environment["docker_client_version"] = version.Client.Version
			environment["docker_server_version"] = version.Server.Version
			environment["architecture"] = version.Server.Arch
		}
	}
	if output, err := e.dockerWithin(ctx, "compose", "version", "--short"); err == nil && len(bytes.TrimSpace(output)) > 0 {
		environment["compose_version"] = strings.TrimSpace(string(output))
	}
	if output, err := e.git(ctx, "rev-parse", "HEAD"); err == nil {
		environment["commit_sha"] = strings.TrimSpace(string(output))
	}
	if output, err := e.git(ctx, "status", "--porcelain"); err == nil {
		environment["dirty"] = len(bytes.TrimSpace(output)) > 0
	}
	environment["images"] = e.imageMetadata(ctx)
	names := make([]string, 0, len(monitoredServices))
	for _, service := range monitoredServices {
		names = append(names, Project+"-"+service+"-1")
	}
	environment["container_limits"] = e.containerLimits(ctx, names...)
	return environment
}

func (e Environment) dockerWithin(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, dockerCommandTimeout)
	defer cancel()
	return e.docker(ctx, args...)
}

func (e Environment) git(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, dockerCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = e.Root
	return cmd.Output()
}

// imageMetadataは、解決済みComposeのimage tagと、取得済みならdigestを返す。
func (e Environment) imageMetadata(ctx context.Context) map[string]any {
	images := map[string]any{}
	configCtx, cancel := context.WithTimeout(ctx, dockerCommandTimeout)
	output, err := e.compose(configCtx, "--profile", "build", "--profile", "prepare", "--profile", "measure", "config", "--format", "json")
	cancel()
	if err != nil {
		return images
	}
	var config struct {
		Services map[string]struct {
			Image string `json:"image"`
		} `json:"services"`
	}
	if json.Unmarshal(output, &config) != nil {
		return images
	}
	for service, value := range config.Services {
		entry := map[string]any{"image": value.Image, "digests": "unknown"}
		if digests, err := e.dockerWithin(ctx, "image", "inspect", "--format", "{{json .RepoDigests}}", value.Image); err == nil {
			var parsed []string
			if json.Unmarshal(digests, &parsed) == nil {
				entry["digests"] = parsed
			}
		}
		images[service] = entry
	}
	return images
}

// containerLimitsは、起動済みcontainerへ実際に適用されたCPU・メモリ・swap上限を返す。
func (e Environment) containerLimits(ctx context.Context, names ...string) []map[string]any {
	format := `{"name":{{json .Name}},"nano_cpus":{{.HostConfig.NanoCpus}},"memory":{{.HostConfig.Memory}},"memory_swap":{{.HostConfig.MemorySwap}}}`
	output, err := e.dockerWithin(ctx, append([]string{"inspect", "--format", format}, names...)...)
	if err != nil {
		return nil
	}
	var limits []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var limit map[string]any
		if json.Unmarshal(scanner.Bytes(), &limit) == nil {
			limits = append(limits, limit)
		}
	}
	return limits
}

// monitorUsageは、比較条件として残すホスト監視器自身のCPU時間と最大メモリである。
func monitorUsage() map[string]any {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return map[string]any{"cpu_seconds": "unknown"}
	}
	cpu := time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
	result := map[string]any{"cpu_seconds": cpu.Seconds(), "max_rss_raw": usage.Maxrss}
	// 監視の大半はdocker・git子processが行うため、終了済み子processの消費も別に残す。
	var children syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_CHILDREN, &children); err == nil {
		result["children_cpu_seconds"] = time.Duration(children.Utime.Nano() + children.Stime.Nano()).Seconds()
		result["children_max_rss_raw"] = children.Maxrss
	}
	return result
}

// WriteReportCSVは、runが保存したreportと同じ値を段階別CSVへ書く。
func WriteReportCSV(w io.Writer, report Report) error {
	writer := csv.NewWriter(w)
	header := []string{
		"stage", "requested_rate", "planned_operations", "operations_started", "operations_completed", "operations_failed", "dropped_iterations",
		"http_count", "http_per_second", "operations_per_second", "p95_ms", "p99_ms", "error_rate",
		"windows_passed", "windows_failed", "windows_provisional", "windows_incomplete", "qualified", "drain_seconds",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, stage := range report.Stages {
		windows := map[string]int{}
		for _, window := range stage.Windows {
			windows[window.Status]++
		}
		row := []string{
			stage.Stage, strconv.Itoa(stage.Rate), strconv.Itoa(stage.PlannedOperations),
			strconv.Itoa(stage.OperationsStarted), strconv.Itoa(stage.OperationsCompleted), strconv.Itoa(stage.OperationsFailed), strconv.Itoa(stage.DroppedIterations),
			strconv.Itoa(stage.HTTPCount), formatFloat(stage.HTTPPerSecond), formatFloat(stage.OperationsPerSecond),
			formatFloat(stage.P95MS), formatFloat(stage.P99MS), formatFloat(stage.ErrorRate),
			strconv.Itoa(windows["passed"]), strconv.Itoa(windows["quality_failed"]), strconv.Itoa(windows["provisional"]), strconv.Itoa(windows["incomplete"]),
			strconv.FormatBool(stage.Qualified), formatFloat(stage.DrainSeconds),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func formatFloat(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

package score

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gmssh-cli-plugin/internal/bench"
	"gmssh-cli-plugin/internal/guard"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/logging"
	"gmssh-cli-plugin/internal/middleware"
	"gmssh-cli-plugin/internal/platform"
)

const (
	CPUWeight  = 4000
	MemWeight  = 3000
	DiskWeight = 3000
	historyMax = 30
	fileName   = "scores.json"

	// probeHardTimeout 每条 probe 内部硬上限，先于宿主 30s 超时返回。
	probeHardTimeout = 12 * time.Second
)

type Part struct {
	Key     string  `json:"key"`
	Score   int     `json:"score"`
	Weight  int     `json:"weight"`
	Value   float64 `json:"value"`
	Unit    string  `json:"unit"`
	Percent int     `json:"percent"`
}

type Result struct {
	CPU   int    `json:"cpu"`
	Mem   int    `json:"mem"`
	Disk  int    `json:"disk"`
	Total int    `json:"total"`
	Parts []Part `json:"parts"`
}

type Point struct {
	At    string `json:"at"`
	Label string `json:"label"`
	Score int    `json:"score"`
	CPU   int    `json:"cpu"`
	Mem   int    `json:"mem"`
	Disk  int    `json:"disk"`
}

type Host struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	OS     string `json:"os"`
	Disk   string `json:"disk"`
	Cores  int    `json:"cores"`
	MemKB  int    `json:"memKB"`
}

type Overview struct {
	Host   Host    `json:"host"`
	Latest *Result `json:"latest"`
	Trend  []Point `json:"trend"`
}

// RunScore 串行测量三个维度并持久化。总分 = 三维度之和。
func RunScore(ctx *middleware.Context, _ input.Input) (any, error) {
	lock, err := guard.AcquireLock(guard.DefaultDiskDir())
	if err != nil {
		return Result{}, err
	}
	defer lock.Release()

	platform.SetLowPriority()

	parts := make([]Part, 0, 3)
	total := 0
	for _, key := range []string{"cpu", "mem", "disk"} {
		res := runProbeWithContext(context.Background(), key, "", false)
		p := Part{Key: key, Score: res.Score, Weight: res.Weight, Value: res.Value, Unit: res.Unit, Percent: res.Percent}
		parts = append(parts, p)
		total += res.Score
	}
	result := Result{CPU: parts[0].Score, Mem: parts[1].Score, Disk: parts[2].Score, Total: total, Parts: parts}
	if err := save(ctx, result); err != nil {
		logging.Business().Errorf("score save failed: %v", err)
		return nil, err
	}
	logging.Business().Infof("score total=%d", result.Total)
	return result, nil
}

// RunProbe 单维度测量。输出同时带前端兼容字段与新诊断字段。
func RunProbe(_ *middleware.Context, in input.Input) (any, error) {
	key := strings.ToLower(strings.TrimSpace(in.Str("key")))
	switch key {
	case "cpu", "mem", "disk":
	default:
		return nil, input.UsageFail("cli.invalid_enum", "key", key, "cpu, mem, disk")
	}
	lock, err := guard.AcquireLock(guard.DefaultDiskDir())
	if err != nil {
		return bench.Result{Key: key, Status: "busy", Max: weightOf(key), Weight: weightOf(key), Reasons: []string{"busy"}}, nil
	}
	defer lock.Release()
	platform.SetLowPriority()
	force := strings.EqualFold(in.Str("force"), "true")
	return runProbeWithContext(context.Background(), key, in.Str("dir"), force), nil
}

// RunCheck 仅预检与环境探测，不跑测试，1 秒内返回。
func RunCheck(_ *middleware.Context, _ input.Input) (any, error) {
	pfCPU := guard.PreflightCPU(false)
	pfMem, memOK := func() (guard.Preflight, bool) {
		budget, p := guard.PreflightMem()
		return p, budget > 0
	}()
	pfDisk := guard.PreflightDisk(guard.DefaultDiskDir(), 256*1024*1024)
	return map[string]any{
		"cpu":  map[string]any{"ok": pfCPU.OK, "reasons": pfCPU.Reasons, "detail": pfCPU.Detail},
		"mem":  map[string]any{"ok": pfMem.OK && memOK, "reasons": pfMem.Reasons, "detail": pfMem.Detail},
		"disk": map[string]any{"ok": pfDisk.OK, "reasons": pfDisk.Reasons, "detail": pfDisk.Detail},
	}, nil
}

// RunHistory 返回历史记录。
func RunHistory(ctx *middleware.Context, _ input.Input) (any, error) {
	return load(ctx), nil
}

// RunOverview 返回主机信息 + 最近成绩 + 趋势。
func RunOverview(ctx *middleware.Context, _ input.Input) (any, error) {
	points := load(ctx)
	out := Overview{Host: ReadHost(), Trend: last7(points)}
	if n := len(points); n > 0 {
		out.Latest = resultFrom(points[n-1])
	}
	return out, nil
}

// runProbeWithContext 用独立超时 context 执行单个维度测量。
func runProbeWithContext(ctx context.Context, key, dir string, force bool) bench.Result {
	cctx, cancel := context.WithTimeout(ctx, probeHardTimeout)
	defer cancel()
	switch key {
	case "cpu":
		return bench.CPUProbe(cctx, 0, guard.PreflightCPU(force))
	case "mem":
		budget, pf := guard.PreflightMem()
		return bench.MemProbe(cctx, budget, pf)
	case "disk":
		if dir == "" {
			dir = guard.DefaultDiskDir()
		}
		return bench.DiskProbe(cctx, dir, guard.PreflightDisk(dir, 256*1024*1024))
	default:
		return bench.Result{Key: key, Status: "unsupported", Reasons: []string{"unsupported"}}
	}
}

func weightOf(key string) int {
	switch key {
	case "cpu":
		return CPUWeight
	case "mem":
		return MemWeight
	case "disk":
		return DiskWeight
	default:
		return 0
	}
}

// save/load/last7/resultFrom/ReadHost 保持原有契约。
func save(ctx *middleware.Context, result Result) error {
	if ctx == nil || ctx.Store == nil {
		return nil
	}
	points := load(ctx)
	now := time.Now()
	points = append(points, Point{
		At: now.Format(time.RFC3339), Label: now.Format("01-02"),
		Score: result.Total, CPU: result.CPU, Mem: result.Mem, Disk: result.Disk,
	})
	if len(points) > historyMax {
		points = points[len(points)-historyMax:]
	}
	return ctx.Store.WriteJSON(fileName, points)
}

func load(ctx *middleware.Context) []Point {
	if ctx == nil || ctx.Store == nil {
		return nil
	}
	var points []Point
	if err := ctx.Store.ReadJSON(fileName, &points); err != nil {
		return nil
	}
	return points
}

func last7(points []Point) []Point {
	if len(points) <= 7 {
		return points
	}
	return points[len(points)-7:]
}

func resultFrom(p Point) *Result {
	cpu := Part{Key: "cpu", Score: p.CPU, Weight: CPUWeight, Unit: "ops/s", Percent: percent(p.CPU, CPUWeight)}
	mem := Part{Key: "mem", Score: p.Mem, Weight: MemWeight, Unit: "GB/s", Percent: percent(p.Mem, MemWeight)}
	disk := Part{Key: "disk", Score: p.Disk, Weight: DiskWeight, Unit: "MB/s", Percent: percent(p.Disk, DiskWeight)}
	return &Result{CPU: p.CPU, Mem: p.Mem, Disk: p.Disk, Total: p.Score, Parts: []Part{cpu, mem, disk}}
}

func ReadHost() Host {
	cores := runtime.NumCPU()
	memKB := memTotalKB()
	return Host{
		CPU: strconv.Itoa(cores) + " cores", Memory: formatGB(memKB),
		OS: readOS(), Disk: diskSize(), Cores: cores, MemKB: memKB,
	}
}

func readOS() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return runtime.GOOS
}

func diskSize() string {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(".", &stat); err != nil || stat.Bsize == 0 {
		return ""
	}
	gb := float64(stat.Blocks*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	if gb < 1 {
		return ""
	}
	return strconv.Itoa(int(gb+0.5)) + " GB"
}

func formatGB(kb int) string {
	if kb <= 0 {
		return ""
	}
	gb := float64(kb) / (1024 * 1024)
	if gb < 1 {
		return "1 GB"
	}
	return strconv.Itoa(int(kb/(1024*1024)+1)) + " GB"
}

func percent(score, weight int) int {
	if weight <= 0 {
		return 0
	}
	v := score * 100 / weight
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

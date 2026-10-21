package score

import (
	"context"
	"fmt"
	"sort"
	"time"

	"gmssh-cli-plugin/internal/bench"
	"gmssh-cli-plugin/internal/guard"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
)

// Calibrate 在一台可复测的机器上连跑 n 次各维度测量，
// 取子项中位数，输出新 profile 草稿 JSON（供开发期固化参考值）。
// n 默认 5，上限 20；单条 probe 内部已有 12s 硬上限。
func Calibrate(ctx *middleware.Context, in input.Input) (any, error) {
	n := in.Int("n")
	if n <= 0 {
		n = 5
	}
	if n > 20 {
		n = 20
	}

	medians := map[string][]float64{} // itemID -> 每轮中位数
	bg := context.Background()
	for round := 1; round <= n; round++ {
		for _, key := range []string{"cpu", "mem", "disk"} {
			var res bench.Result
			switch key {
			case "cpu":
				res = bench.CPUProbe(bg, 0, guard.PreflightCPU(true))
			case "mem":
				budget, pf := guard.PreflightMem()
				res = bench.MemProbe(bg, budget, pf)
			case "disk":
				dir := in.Str("dir")
				if dir == "" {
					dir = guard.DefaultDiskDir()
				}
				res = bench.DiskProbe(bg, dir, guard.PreflightDisk(dir, 256*1024*1024))
			}
			for _, it := range res.Items {
				medians[it.ID] = append(medians[it.ID], it.Median)
			}
		}
		// 轮间让设备排空，避免热效应拉低后面轮次
		time.Sleep(2 * time.Second)
	}

	draft := map[string]any{
		"version":   "1.0.0-draft",
		"preview":   false,
		"k":         0.357,
		"source":    fmt.Sprintf("calibrate n=%d", n),
		"generated": time.Now().Format(time.RFC3339),
		"refs":      map[string]any{},
	}
	refs := draft["refs"].(map[string]any)
	for id, vals := range medians {
		sorted := append([]float64(nil), vals...)
		sort.Float64s(sorted)
		mid := len(sorted) / 2
		median := sorted[mid]
		if len(sorted)%2 == 0 {
			median = (sorted[mid-1] + sorted[mid]) / 2
		}
		refs[id] = map[string]any{
			"median": median,
			"rounds": len(vals),
			"cv_pct": bench.CV(vals),
			"stable": bench.CV(vals) <= 8,
			"raw":    vals,
			"unit":   unitOf(id),
		}
	}
	return draft, nil
}

func unitOf(id string) string {
	switch id {
	case "single", "multi":
		return "ops/s"
	case "bandwidth":
		return "GB/s"
	case "latency":
		return "ns"
	case "fsync":
		return "ops/s"
	default:
		return "MB/s" // 磁盘吞吐类
	}
}

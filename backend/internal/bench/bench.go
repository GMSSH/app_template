// Package bench 定义统一测量接口与公共类型。
package bench

import (
	"context"
	"math"
	"time"
)

// ItemResult 是子项测量结果。
type ItemResult struct {
	ID      string  `json:"id"`
	Unit    string  `json:"unit"`
	Median  float64 `json:"median"`
	CV      float64 `json:"cv"`
	Samples int     `json:"samples"`
	Scored  bool    `json:"scored"`
}

// Result 是单个维度 probe 的结果（含前端兼容字段）。
type Result struct {
	Key        string       `json:"key"`
	Status     string       `json:"status"`
	Score      int          `json:"score"`
	Max        int          `json:"max"`
	Weight     int          `json:"weight"`
	Value      float64      `json:"value"`
	Unit       string       `json:"unit"`
	Percent    int          `json:"percent"`
	Stable     bool         `json:"stable"`
	Comparable bool         `json:"comparable"`
	Reasons    []string     `json:"reasons"`
	Items      []ItemResult `json:"items"`
	ElapsedMS  int64        `json:"elapsed_ms"`
	Profile    string       `json:"profile"`
}

// MeasureRate 在 dur 时长内反复调用 fn，返回每秒次数；ctx 取消返回 ok=false。
func MeasureRate(ctx context.Context, dur time.Duration, fn func()) (float64, bool) {
	deadline := time.Now().Add(dur)
	count := 0
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, false
		default:
		}
		fn()
		count++
	}
	elapsed := time.Since(deadline.Add(-dur)).Seconds()
	if elapsed <= 0 {
		elapsed = float64(dur) / float64(time.Second)
	}
	return float64(count) / elapsed, true
}

// Median 取中位数。
func Median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// CV 返回变异系数（百分比）。
func CV(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := Mean(vals)
	if m <= 0 {
		return 0
	}
	var s float64
	for _, v := range vals {
		d := v - m
		s += d * d
	}
	return math.Sqrt(s/float64(len(vals)-1)) / m * 100
}

// Mean 算术平均。
func Mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var s float64
	for _, v := range vals {
		s += v
	}
	return s / float64(len(vals))
}

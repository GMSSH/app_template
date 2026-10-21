package bench

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"gmssh-cli-plugin/internal/guard"
	"gmssh-cli-plugin/internal/probe"
)

// MemProbe 执行内存维度测量：带宽 3s + 延迟 2s，内部约 6 秒。
func MemProbe(ctx context.Context, budget int64, pf guard.Preflight) Result {
	r := Result{Key: "mem", Max: 3000, Weight: 3000, Unit: "GB/s", Profile: "0.9.0-preview", Status: "ok", Stable: true, Comparable: true}
	start := time.Now()

	if !pf.OK {
		r.Status = "refused"
		for _, x := range pf.Reasons {
			r.Reasons = append(r.Reasons, string(x))
		}
		return r
	}

	if budget <= 0 {
		r.Status = "refused"
		r.Reasons = append(r.Reasons, string(guard.ReasonLowMemory))
		return r
	}

	// 数组大小 = max(4×最大缓存, 64MiB)，受预算约束
	cacheKB := probe.MaxCacheKB()
	if cacheKB <= 0 {
		cacheKB = 32 * 1024
	} // 读不到按 32MiB 估计
	arrBytes := cacheKB * 1024 * 4
	if arrBytes < 64*1024*1024 {
		arrBytes = 64 * 1024 * 1024
	}
	if arrBytes > budget/2 {
		arrBytes = budget / 2
	} // 最多用一半预算
	if arrBytes < 8*1024*1024 {
		r.Status = "refused"
		r.Reasons = append(r.Reasons, string(guard.ReasonLowMemory))
		return r
	}

	// 缓存污染检查：数组 < 2×缓存则带宽不可信
	if arrBytes < cacheKB*1024*2 {
		r.Comparable = false
		r.Reasons = append(r.Reasons, "cache_polluted")
	}

	// ---- 带宽 ----
	bwRates := make([]float64, 0, 4)
	for round := 0; round < 4; round++ {
		rate, ok := runBandwidth(ctx, arrBytes, 700*time.Millisecond)
		if !ok {
			r.Status = "aborted"
			r.Reasons = append(r.Reasons, string(guard.ReasonTimeout))
			return r
		}
		if round > 0 {
			bwRates = append(bwRates, rate)
		}
	}
	bwGBps := Median(bwRates) / 1e9
	bwCV := CV(bwRates)

	// ---- 延迟（Sattolo 指针追逐） ----
	latRates := make([]float64, 0, 4)
	for round := 0; round < 4; round++ {
		ns, ok := runLatency(ctx, arrBytes, 500*time.Millisecond)
		if !ok {
			r.Status = "aborted"
			r.Reasons = append(r.Reasons, string(guard.ReasonTimeout))
			return r
		}
		if round > 0 {
			latRates = append(latRates, ns)
		}
	}
	latNS := Median(latRates)
	latCV := CV(latRates)

	// 评分：带宽 60%（copy+triad 几何平均），延迟 40%（ref 100ns）
	bwScore := saturate(bwGBps/10.0, 1800)      // 参考：10 GB/s 得 30%
	latScore := saturate(100.0/latNS*1.0, 1200) // 延迟 100ns 得 30%
	r.Score = bwScore + latScore
	if r.Score > 3000 {
		r.Score = 3000
	}

	r.Value = round2(bwGBps)
	r.Percent = r.Score * 100 / 3000
	if bwCV > 8 || latCV > 8 {
		r.Stable = false
		r.Comparable = false
		r.Reasons = append(r.Reasons, string(guard.ReasonUnstable))
	}

	r.Items = []ItemResult{
		{ID: "bandwidth", Unit: "GB/s", Median: round2(bwGBps), CV: round2(bwCV), Samples: 3, Scored: true},
		{ID: "latency", Unit: "ns", Median: round2(latNS), CV: round2(latCV), Samples: 3, Scored: true},
	}
	r.ElapsedMS = time.Since(start).Milliseconds()
	return r
}

// runBandwidth 固定时长内做 copy + triad 两种模式，返回 GB/s。
func runBandwidth(ctx context.Context, arrBytes int64, dur time.Duration) (float64, bool) {
	n := arrBytes / 8 // 每个元素 8 字节
	a := make([]float64, n)
	b := make([]float64, n)
	defer func() { a, b = nil, nil; runtime.GC() }()
	rand.New(rand.NewSource(42)).Read(float64Bytes(a))
	for i := range b {
		b[i] = float64(i) * 0.001
	}

	var wg sync.WaitGroup
	var totalBytes atomic.Uint64
	threads := runtime.NumCPU()
	if threads > 8 {
		threads = 8
	}
	deadline := time.Now().Add(dur)
	for t := 0; t < threads; t++ {
		wg.Add(1)
		go func(t int) {
			defer wg.Done()
			chunk := n / int64(threads)
			lo := int64(t) * chunk
			hi := lo + chunk
			if t == threads-1 {
				hi = n
			}
			local := uint64(0)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				// copy 模式
				copy(b[lo:hi], a[lo:hi])
				local += uint64(hi-lo) * 8
				// triad 模式
				for i := lo; i < hi; i++ {
					b[i] = a[i]*1.01 + 0.5
				}
				local += uint64(hi-lo) * 8 * 2
			}
			totalBytes.Add(local)
		}(t)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return 0, false
	}
	elapsed := time.Since(deadline.Add(-dur)).Seconds()
	if elapsed <= 0 {
		elapsed = float64(dur) / float64(time.Second)
	}
	return float64(totalBytes.Load()) / elapsed, true
}

// runLatency Sattolo 单环随机排列 + 指针追逐，返回每次访问的平均纳秒。
func runLatency(ctx context.Context, arrBytes int64, dur time.Duration) (float64, bool) {
	n := arrBytes / 8
	idx := make([]int64, n)
	for i := int64(0); i < n; i++ {
		idx[i] = i
	}
	// Sattolo 算法：生成单环
	r := rand.New(rand.NewSource(7))
	for i := n - 1; i > 0; i-- {
		j := r.Int63n(i)
		idx[i], idx[j] = idx[j], idx[i]
	}
	// 指针追逐：每次访问依赖上次结果
	var sum int64
	chase := func(count int64) {
		var k int64 = 0
		for c := int64(0); c < count; c++ {
			k = idx[k]
			sum += k
		}
	}
	_ = chase
	deadline := time.Now().Add(dur)
	var k int64 = 0
	count := int64(0)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, false
		default:
		}
		k = idx[k]
		sum += k
		count++
	}
	sink.Add(uint64(sum & 0xffff))
	elapsed := time.Since(deadline.Add(-dur)).Seconds()
	if elapsed <= 0 {
		elapsed = float64(dur) / float64(time.Second)
	}
	return elapsed / float64(count) * 1e9, true
}

// float64Bytes 把 []float64 当作原始字节（用于填充随机数据）。
func float64Bytes(vals []float64) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&vals[0])), len(vals)*8)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

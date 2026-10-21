package bench

import (
	"context"
	"math"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"gmssh-cli-plugin/internal/guard"
	"gmssh-cli-plugin/internal/platform"
)

// DiskProbe 执行磁盘维度测量：顺序写/读 + 4K 随机读 QD1/QD16 + 4K 随机写 + fsync，
// 内部约 10 秒。O_DIRECT 优先，不支持时降级 O_SYNC 且读项不计分。
func DiskProbe(ctx context.Context, dir string, pf guard.Preflight) Result {
	r := Result{Key: "disk", Max: 3000, Weight: 3000, Unit: "MB/s", Profile: "0.9.0-preview", Status: "ok", Stable: true, Comparable: true}
	start := time.Now()
	// 所有提前返回路径都记录真实耗时（aborted/refused 也报 elapsed_ms）
	defer func() {
		if r.ElapsedMS == 0 {
			r.ElapsedMS = time.Since(start).Milliseconds()
		}
	}()

	if !pf.OK {
		r.Status = "refused"
		for _, x := range pf.Reasons {
			r.Reasons = append(r.Reasons, string(x))
		}
		return r
	}
	if dir == "" {
		dir = guard.DefaultDiskDir()
	}

	// 文件大小：默认 256MiB，空间不足缩小到 64MiB，再小跳过
	fileBytes := int64(256 * 1024 * 1024)
	total, free, err := platform.DiskUsage(dir)
	if err == nil && total > 0 {
		for fileBytes > 64*1024*1024 && free < uint64(fileBytes*4) {
			fileBytes /= 2
		}
	}
	if fileBytes < 64*1024*1024 {
		r.Status = "refused"
		r.Reasons = append(r.Reasons, string(guard.ReasonNoSpace))
		return r
	}

	// 匿名文件：O_TMPFILE 或创建即 unlink，强杀无残留
	f, err := platform.OpenAnonymousFile(dir, 0o600)
	if err != nil {
		r.Status = "refused"
		r.Reasons = append(r.Reasons, string(guard.ReasonBadDisk))
		return r
	}
	defer f.Close()

	// 尽力 NOCOW（btrfs）
	platform.SetFileNoCOW(f)

	// O_DIRECT 尝试
	df, derr := platform.OpenDirect(f)
	direct := derr == nil
	var wf, rf *os.File
	if direct {
		wf, rf = df, df
	} else {
		wf, rf = f, f
		r.Comparable = false
		r.Reasons = append(r.Reasons, "no_odirect")
	}
	_ = wf
	_ = rf

	// 填充数据（不可压缩），顺序写兼任填充
	buf := make([]byte, 1024*1024) // 1MiB
	rand.New(rand.NewSource(42)).Read(buf)

	// ---- 顺序写（填充文件） ----
	// 每轮上限 2.5s：慢盘写不满就按实际写入量算吞吐，绝不拖到 ctx 超时
	writeRates := make([]float64, 0, 3)
	filled := int64(0)
	for round := 0; round < 3; round++ {
		rate, ok := seqWrite(ctx, wf, fileBytes, buf, direct, round == 0, 2500*time.Millisecond)
		if !ok {
			r.Status = "aborted"
			r.Reasons = append(r.Reasons, string(guard.ReasonTimeout))
			return r
		}
		if round > 0 {
			writeRates = append(writeRates, rate)
		}
	}
	// 实际填充量 = 3 轮写满后的进度；慢盘未满时缩小后续随机 IO 范围
	filled, _ = f.Seek(0, 1)
	if filled < 64*1024*1024 {
		filled = 64 * 1024 * 1024
	}
	if filled > fileBytes {
		filled = fileBytes
	}
	seqW := Median(writeRates) / 1e6 // MB/s

	// ---- 顺序读（读已填充范围） ----
	readRates := make([]float64, 0, 3)
	for round := 0; round < 3; round++ {
		rate, ok := seqRead(ctx, rf, filled, buf, direct)
		if !ok {
			r.Status = "aborted"
			r.Reasons = append(r.Reasons, string(guard.ReasonTimeout))
			return r
		}
		if round > 0 {
			readRates = append(readRates, rate)
		}
	}
	seqR := Median(readRates) / 1e6

	// ---- 4K 随机读 QD1 / QD16（用实际填充范围） ----
	rr1 := randIO(ctx, rf, filled, 4096, 1, true, direct, 800*time.Millisecond)
	rr16 := randIO(ctx, rf, filled, 4096, 16, true, direct, 800*time.Millisecond)
	// ---- 4K 随机写 QD1（64MiB 上限） ----
	rw1 := randIO(ctx, wf, filled, 4096, 1, false, direct, 800*time.Millisecond)
	// ---- fsync 延迟（p50） ----
	fsyncNS := fsyncLatency(ctx, wf)

	if !direct {
		// 降级：读项命中页缓存不计分
		rr1.Scored = false
		rr16.Scored = false
		seqR = 0
	}

	// 评分（临时参考值，校准后替换）
	// 顺序写 20%、顺序读 20%、4K RD QD1 15%、4K RD QD16 15%、4K WR QD1 10%、fsync 20%
	score := 0.0
	scoredW := 0.0
	add := func(v float64, ref float64, w int) {
		if v <= 0 {
			return
		}
		score += saturateF(v/ref, float64(w))
		scoredW += float64(w)
	}
	if direct {
		add(seqW, 500, 600) // 500 MB/s 得 30%
		add(seqR, 500, 600)
		add(rr1.IOPS, 3000, 450)
		add(rr16.IOPS, 20000, 450)
		add(rw1.IOPS, 2000, 300)
	} else {
		add(seqW, 300, 600)
		add(rw1.IOPS, 1500, 300)
	}
	add(1e9/fsyncNS, 100, 600) // fsync: 10ms 得 30%
	if scoredW > 0 {
		// 未计分项按比例放大到 3000
		r.Score = int(score * 3000 / scoredW)
	}
	if r.Score > 3000 {
		r.Score = 3000
	}

	// 前端兼容：value 用顺序写速度
	r.Value = round2(seqW)
	r.Percent = r.Score * 100 / 3000

	r.ElapsedMS = time.Since(start).Milliseconds()
	r.Items = []ItemResult{
		{ID: "seq_write", Unit: "MB/s", Median: round2(seqW), CV: 0, Samples: 2, Scored: true},
		{ID: "seq_read", Unit: "MB/s", Median: round2(seqR), CV: 0, Samples: 2, Scored: direct},
		{ID: "rand_read_qd1", Unit: "IOPS", Median: round2(rr1.IOPS), CV: 0, Samples: 1, Scored: direct},
		{ID: "rand_read_qd16", Unit: "IOPS", Median: round2(rr16.IOPS), CV: 0, Samples: 1, Scored: direct},
		{ID: "rand_write_qd1", Unit: "IOPS", Median: round2(rw1.IOPS), CV: 0, Samples: 1, Scored: true},
		{ID: "fsync", Unit: "ms", Median: round2(fsyncNS / 1e6), CV: 0, Samples: 1, Scored: true},
	}
	return r
}

// seqWrite 顺序写。first 为 true 时兼任填充（从头写满 fileBytes），否则在文件内循环写。
// writeDur 限制单轮写入时长（慢盘自动写不满就返回已写吞吐，不触发 ctx 超时）。
func seqWrite(ctx context.Context, f *os.File, fileBytes int64, buf []byte, direct bool, first bool, writeDur time.Duration) (float64, bool) {
	if _, err := f.Seek(0, 0); err != nil {
		return 0, false
	}
	written := int64(0)
	start := time.Now()
	deadline := start.Add(writeDur)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, false
		default:
		}
		n, err := f.Write(buf)
		if err != nil {
			break
		}
		written += int64(n)
		if !first && written >= fileBytes {
			f.Seek(0, 0)
			written = 0
		}
		if first && written >= fileBytes {
			break // 填充完成
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0, false
	}
	// 慢盘写不满：按实际写入量算吞吐（部分填充也计分，后续读项缩小范围）
	if written == 0 {
		return 0, false
	}
	return float64(written) / elapsed, true
}

// seqRead 顺序读。
func seqRead(ctx context.Context, f *os.File, fileBytes int64, buf []byte, direct bool) (float64, bool) {
	if _, err := f.Seek(0, 0); err != nil {
		return 0, false
	}
	read := int64(0)
	start := time.Now()
	for read < fileBytes {
		select {
		case <-ctx.Done():
			return 0, false
		default:
		}
		n, err := f.Read(buf)
		if err != nil && n == 0 {
			break
		}
		read += int64(n)
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0, false
	}
	return float64(fileBytes) / elapsed, true
}

// IOStat 是随机 IO 子项结果。
type IOStat struct {
	IOPS   float64
	MBps   float64
	Scored bool
}

// randIO 4K 随机读写，qd 个 goroutine 各自同步 pread/pwrite。
func randIO(ctx context.Context, f *os.File, fileBytes int64, block int64, qd int, isRead bool, direct bool, dur time.Duration) IOStat {
	if !direct && isRead {
		return IOStat{}
	}
	buf := make([]byte, int(block)*qd)
	rand.New(rand.NewSource(3)).Read(buf)
	var ops atomic.Uint64
	var bytes atomic.Uint64
	var wg sync.WaitGroup
	deadline := time.Now().Add(dur)
	for t := 0; t < qd; t++ {
		wg.Add(1)
		go func(t int) {
			defer wg.Done()
			src := rand.New(rand.NewSource(int64(t) + 99))
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				off := (src.Int63n(fileBytes / block)) * block
				if isRead {
					if _, err := f.ReadAt(buf[t*int(block):(t+1)*int(block)], off); err != nil {
						return
					}
				} else {
					if _, err := f.WriteAt(buf[t*int(block):(t+1)*int(block)], off); err != nil {
						return
					}
				}
				ops.Add(1)
				bytes.Add(uint64(block))
			}
		}(t)
	}
	wg.Wait()
	secs := time.Since(deadline.Add(-dur)).Seconds()
	if secs <= 0 {
		secs = float64(dur) / float64(time.Second)
	}
	return IOStat{IOPS: float64(ops.Load()) / secs, MBps: float64(bytes.Load()) / secs / 1e6, Scored: true}
}

// fsyncLatency 返回 fdatasync p50 延迟（纳秒）。
func fsyncLatency(ctx context.Context, f *os.File) float64 {
	buf := make([]byte, 4096)
	latencies := make([]float64, 0, 32)
	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0
		default:
		}
		if _, err := f.WriteAt(buf, 0); err != nil {
			break
		}
		s := time.Now()
		if err := f.Sync(); err != nil {
			break
		}
		latencies = append(latencies, float64(time.Since(s).Nanoseconds()))
	}
	return Median(latencies)
}

// saturateF 浮点版饱和曲线。
func saturateF(ratio float64, weight float64) float64 {
	if ratio <= 0 {
		return 0
	}
	k := 0.357
	return weight * (1 - expNeg(k*ratio))
}

func expNeg(x float64) float64 {
	return math.Exp(-x)
}

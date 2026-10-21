package bench

import (
	"compress/flate"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"math"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gmssh-cli-plugin/internal/guard"
	"gmssh-cli-plugin/internal/probe"
)

// sink 防止编译器把无副作用负载优化掉。
var sink atomic.Uint64

// CPUProbe 执行 CPU 维度测量：单核 3s + 多核 4s，内部约 8 秒。
// 每项 1 轮预热 + 3 轮正式，取中位数。
func CPUProbe(ctx context.Context, budget int64, pf guard.Preflight) Result {
	r := Result{Key: "cpu", Max: 4000, Weight: 4000, Unit: "ops/s", Profile: "0.9.0-preview", Status: "ok", Stable: true, Comparable: true}
	start := time.Now()

	if !pf.OK {
		r.Status = "refused"
		for _, x := range pf.Reasons {
			r.Reasons = append(r.Reasons, string(x))
		}
		return r
	}

	cores := probe.CPUCount()
	if cores <= 0 {
		cores = 1
	}

	// ---- 单核 ----
	singleRates := make([]float64, 0, 4)
	for round := 0; round < 4; round++ { // 1 预热 + 3 正式
		rate, ok := runSingle(ctx, 700*time.Millisecond)
		if !ok {
			r.Status = "aborted"
			r.Reasons = append(r.Reasons, string(guard.ReasonTimeout))
			return r
		}
		if round > 0 {
			singleRates = append(singleRates, rate)
		}
		if round == 0 {
			runtime.GC()
		}
	}
	singleRate := Median(singleRates)
	singleCV := CV(singleRates)

	// ---- 多核 ----
	multiRates := make([]float64, 0, 4)
	threads := cores
	for round := 0; round < 4; round++ {
		rate, ok := runMulti(ctx, threads, 700*time.Millisecond)
		if !ok {
			r.Status = "aborted"
			r.Reasons = append(r.Reasons, string(guard.ReasonTimeout))
			return r
		}
		if round > 0 {
			multiRates = append(multiRates, rate)
		}
	}
	multiRate := Median(multiRates)
	multiCV := CV(multiRates)

	// 简单线性映射：单核 40%、多核 60%。参考值在 profile 校准后替换。
	// 临时：单核 2000 ops/s 得 50%，多核 per-core 2000 ops/s 得 50%。
	scoreSingle := saturate(singleRate/2000.0, 1600)
	scoreMulti := saturate(multiRate/(2000.0*float64(threads)), 2400)
	r.Score = scoreSingle + scoreMulti
	if r.Score > 4000 {
		r.Score = 4000
	}

	// 前端兼容：value 用综合吞吐（单核+多核平均），unit 保持 ops/s
	r.Value = (singleRate + multiRate/float64(threads)) / 2
	r.Percent = r.Score * 100 / 4000
	if singleCV > 8 || multiCV > 8 {
		r.Stable = false
		r.Comparable = false
		r.Reasons = append(r.Reasons, string(guard.ReasonUnstable))
	}

	r.Items = []ItemResult{
		{ID: "single", Unit: "ops/s", Median: singleRate, CV: singleCV, Samples: 3, Scored: true},
		{ID: "multi", Unit: "ops/s", Median: multiRate, CV: multiCV, Samples: 3, Scored: true},
	}
	r.ElapsedMS = time.Since(start).Milliseconds()
	return r
}

// runSingle 单核确定性负载：整数/浮点/SHA256+AES/deflate/JSON 五个内核轮流跑。
func runSingle(ctx context.Context, dur time.Duration) (float64, bool) {
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	kernels := []func(){kernelInt, kernelFloat, kernelCrypto, kernelDeflate, kernelJSON}
	seed := int64(42)
	count := 0
	deadline := time.Now().Add(dur)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, false
		default:
		}
		kernels[count%len(kernels)]()
		count++
		seed += 1
	}
	elapsed := time.Since(deadline.Add(-dur)).Seconds()
	if elapsed <= 0 {
		elapsed = float64(dur) / float64(time.Second)
	}
	return float64(count) / elapsed, true
}

// runMulti 多核工作队列：每线程固定工作块，取完即补。
func runMulti(ctx context.Context, threads int, dur time.Duration) (float64, bool) {
	kernels := []func(){kernelInt, kernelFloat, kernelCrypto, kernelDeflate, kernelJSON}
	var total atomic.Uint64
	var wg sync.WaitGroup
	deadline := time.Now().Add(dur)
	for t := 0; t < threads; t++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			local := 0
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				kernels[seed%len(kernels)]()
				seed++
				local++
			}
			total.Add(uint64(local))
		}(t * 7919)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return 0, false
	}
	elapsed := time.Since(deadline.Add(-dur)).Seconds()
	if elapsed <= 0 {
		elapsed = float64(dur) / float64(time.Second)
	}
	return float64(total.Load()) / elapsed, true
}

// ---- 确定性负载内核（固定种子，防优化 sink） ----

func kernelInt() {
	var x uint64 = 0x9e3779b97f4a7c15
	for i := 0; i < 20000; i++ {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		x *= 0x2545F4914F6CDD1D
	}
	sink.Add(x & 1)
}

func kernelFloat() {
	a := float64(0.1)
	b := float64(0.2)
	for i := 0; i < 4000; i++ {
		a = float64(a*b + a) // 显式转换阻止 FMA 融合
		b = float64(b*0.99 + 0.01)
	}
	sink.Add(uint64(a * 1e6))
}

func kernelCrypto() {
	data := make([]byte, 1024)
	rand.New(rand.NewSource(1)).Read(data)
	key := make([]byte, 32)
	h := sha256.Sum256(data)
	copy(key, h[:])
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	out := gcm.Seal(nil, nonce, data, nil)
	sink.Add(uint64(out[0]))
}

func kernelDeflate() {
	data := []byte("The quick brown fox jumps over the lazy dog. 0123456789 abcdefghijklmnopqrstuvwxyz")
	var buf bytesBuffer
	w, _ := flate.NewWriter(&buf, flate.BestSpeed)
	w.Write(data)
	w.Close()
	sink.Add(uint64(buf.Len()))
}

func kernelJSON() {
	type payload struct {
		Name  string    `json:"name"`
		Value []float64 `json:"value"`
		Flag  bool      `json:"flag"`
	}
	vals := make([]float64, 32)
	for i := range vals {
		vals[i] = float64(i) * 1.5
	}
	p := payload{Name: "bench", Value: vals, Flag: true}
	b, _ := json.Marshal(p)
	var out payload
	json.Unmarshal(b, &out)
	sink.Add(uint64(len(b)))
}

// bytesBuffer 最小字节缓冲，避免引入 bytes 包（其实直接 import bytes 更简单）。
type bytesBuffer struct{ b []byte }

func (w *bytesBuffer) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }
func (w *bytesBuffer) Len() int                    { return len(w.b) }

// saturate 饱和曲线：w * (1 - e^(-k*x))，k=0.357，x=1 得 30%。
func saturate(ratio float64, weight int) int {
	if ratio <= 0 {
		return 0
	}
	k := 0.357
	v := float64(weight) * (1 - math.Exp(-k*ratio))
	return int(v + 0.5)
}

var _ = strconv.Itoa

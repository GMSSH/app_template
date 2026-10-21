package bench

import (
	"context"
	"os"
	"testing"
	"time"

	"gmssh-cli-plugin/internal/guard"
)

// 预算不足时内存 probe 必须拒绝而不是硬跑。
func TestMemProbeRefusedOnTinyBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := MemProbe(ctx, 4*1024*1024, okPreflight())
	if res.Status != "refused" {
		t.Fatalf("status = %s, want refused", res.Status)
	}
}

// 预算充足时应在数秒内完成，且返回可解析的结构。
func TestMemProbeRunsWithBudget(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	res := MemProbe(ctx, 256*1024*1024, okPreflight())
	if res.Status != "ok" && res.Status != "refused" {
		t.Fatalf("unexpected status %s", res.Status)
	}
	if res.ElapsedMS <= 0 {
		t.Fatalf("elapsed_ms = %d, want > 0", res.ElapsedMS)
	}
	if res.Weight != 3000 || res.Max != 3000 {
		t.Fatalf("weight/max wrong: %d/%d", res.Weight, res.Max)
	}
}

// 目录不可写/不存在时磁盘 probe 必须拒绝。
func TestDiskProbeRefusedOnBadDir(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res := DiskProbe(ctx, "/nonexistent-dir-for-score-test", okPreflight())
	if res.Status != "refused" {
		t.Fatalf("status = %s, want refused", res.Status)
	}
}

// 磁盘 probe 结束后测试目录无任何残留文件（O_TMPFILE/unlink 生效）。
func TestDiskProbeLeavesNoResidue(t *testing.T) {
	dir, err := os.MkdirTemp("", "sb-residue")
	if err != nil {
		t.Skip("no temp dir")
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = DiskProbe(ctx, dir, okPreflight())

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := ""
		for _, e := range entries {
			names += e.Name() + ","
		}
		t.Fatalf("residue files left: %s", names)
	}
}

// CPU probe 在超时 ctx 下应快速返回 aborted。
func TestCPUProbeAbortsOnCanceledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := CPUProbe(ctx, 0, okPreflight())
	if res.Status == "ok" {
		t.Fatalf("status = ok, want aborted")
	}
	if res.ElapsedMS > 2000 {
		t.Fatalf("aborted too slowly: %d ms", res.ElapsedMS)
	}
}

func okPreflight() guard.Preflight {
	return guard.Preflight{OK: true}
}

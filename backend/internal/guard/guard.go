// Package guard 是跑分的安全核心：预检、资源预算、跨进程互斥与循环内检查。
// 所有 bench 的资源申请必须经过 Budget；预检不通过必须返回 refused 原因。
package guard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gmssh-cli-plugin/internal/platform"
	"gmssh-cli-plugin/internal/probe"
)

// Reason 是拒绝/中止/降级的原因枚举，前端据此展示提示。
type Reason string

const (
	ReasonOK          Reason = "ok"
	ReasonBusy        Reason = "busy"         // 锁被占用，有其他 probe 在跑
	ReasonHighLoad    Reason = "high_load"    // loadavg 过高
	ReasonHighPSI     Reason = "high_psi"     // PSI 压力过高
	ReasonHighTemp    Reason = "high_temp"    // 温度过高
	ReasonLowMemory   Reason = "low_memory"   // 内存预算不足
	ReasonBadDisk     Reason = "bad_disk_dir" // 磁盘目录不合规
	ReasonNoSpace     Reason = "no_space"     // 磁盘空间不足
	ReasonTimeout     Reason = "timeout"      // 内部超时中止
	ReasonUnsupported Reason = "unsupported"  // 平台不支持
	ReasonUnstable    Reason = "unstable"     // 样本 CV 过大，不计分
	ReasonUnknown     Reason = "unknown"
)

// Preflight 预检结果。
type Preflight struct {
	OK      bool
	Reasons []Reason
	Detail  map[string]string
}

// Budget 资源预算。所有大内存、大文件申请都经过这里。
type Budget struct {
	MemLimitBytes int64 // 本次允许使用的最大内存
	// Disk 相关
	DiskDir       string
	FileSizeBytes int64
	AllowedFS     map[string]bool
	WarmMax       int64 // 磁盘写总量上限
}

// Lock 是跨进程互斥锁（flock）。同一个用户数据目录下同一时刻只允许一个 probe。
type Lock struct {
	f *os.File
}

var (
	lockMu    sync.Mutex
	lockOwner string
)

// AcquireLock 非阻塞获取互斥锁。拿不到返回 ReasonBusy。
func AcquireLock(dir string) (*Lock, error) {
	lockMu.Lock()
	defer lockMu.Unlock()
	if lockOwner != "" {
		return nil, errors.New(string(ReasonBusy))
	}
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, ".sbench.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New(string(ReasonBusy))
	}
	lockOwner = path
	return &Lock{f: f}, nil
}

// Release 释放锁。
func (l *Lock) Release() {
	lockMu.Lock()
	defer lockMu.Unlock()
	if l != nil && l.f != nil {
		_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
		_ = l.f.Close()
	}
	if lockOwner != "" {
		lockOwner = ""
	}
}

// PreflightCPU 检查 CPU 相关预检。
// force=true 时跳过 loadavg/PSI 拒绝（用户确认后继续跑，结果标 degraded）。
func PreflightCPU(force bool) Preflight {
	p := Preflight{OK: true, Detail: map[string]string{}}
	cores := probe.CPUCount()
	if cores <= 0 {
		cores = 1
	}
	load := probe.LoadAvg1()
	p.Detail["load1"] = strconv.FormatFloat(load, 'f', 2, 64)
	if !force && load > float64(cores)*0.5 {
		p.OK = false
		p.Reasons = append(p.Reasons, ReasonHighLoad)
	}
	if psi := probe.PSI("cpu"); psi >= 0 {
		p.Detail["psi_cpu"] = strconv.FormatFloat(psi, 'f', 2, 64)
		if !force && psi > 20 {
			p.OK = false
			p.Reasons = append(p.Reasons, ReasonHighPSI)
		}
	}
	if temp := probe.ThermalMaxC(); temp >= 0 {
		p.Detail["temp"] = strconv.FormatFloat(temp, 'f', 1, 64)
		if temp >= 85*0.9 {
			p.OK = false
			p.Reasons = append(p.Reasons, ReasonHighTemp)
		}
	}
	return p
}

// PreflightDisk 检查磁盘目录合规性与空间。
func PreflightDisk(dir string, wantBytes int64) Preflight {
	p := Preflight{OK: true, Detail: map[string]string{}}
	if dir == "" {
		dir = defaultDiskDir()
	}
	p.Detail["dir"] = dir
	// 目录检查
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		p.OK = false
		p.Reasons = append(p.Reasons, ReasonBadDisk)
		return p
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		p.OK = false
		p.Reasons = append(p.Reasons, ReasonBadDisk)
		return p
	}
	for _, bad := range []string{"/proc", "/sys", "/dev", "/boot", "/etc", "/usr", "/var/lib"} {
		if abs == bad || strings.HasPrefix(abs, bad+"/") {
			p.OK = false
			p.Reasons = append(p.Reasons, ReasonBadDisk)
			return p
		}
	}
	if abs == "/" {
		p.OK = false
		p.Reasons = append(p.Reasons, ReasonBadDisk)
		return p
	}
	// 文件系统白名单
	fsType, _, err := platform.FSInfo(dir)
	if err != nil {
		p.OK = false
		p.Reasons = append(p.Reasons, ReasonBadDisk)
		return p
	}
	p.Detail["fs"] = fsType
	allowed := map[string]bool{"ext4": true, "xfs": true, "btrfs": true, "zfs": true, "f2fs": true}
	if !allowed[fsType] {
		p.OK = false
		p.Reasons = append(p.Reasons, ReasonBadDisk)
		return p
	}
	// 空间检查
	total, free, err := platform.DiskUsage(dir)
	if err != nil {
		p.OK = false
		p.Reasons = append(p.Reasons, ReasonNoSpace)
		return p
	}
	p.Detail["free"] = fmt.Sprintf("%dMB", free/1024/1024)
	if free < uint64(wantBytes*4) || free < total/10 {
		p.OK = false
		p.Reasons = append(p.Reasons, ReasonNoSpace)
	}
	return p
}

// PreflightMem 计算内存预算并设置 Go 内存上限。返回可用预算字节；不足返回 0。
func PreflightMem() (int64, Preflight) {
	p := Preflight{OK: true, Detail: map[string]string{}}
	avail := probe.MemAvailableKB()
	total := probe.MemTotalKB()
	cg := probe.CgroupMemoryLimitKB()
	if avail <= 0 && total <= 0 && cg <= 0 {
		// 读不到任何内存信息（非 Linux 或容器受限）：给一个保守的 64MiB
		const minBudget = 64 * 1024 * 1024
		debug.SetMemoryLimit(minBudget)
		p.Detail["budget"] = strconv.FormatInt(minBudget, 10)
		return minBudget, p
	}
	var limit int64 = 256 * 1024 * 1024 // 默认 256MiB 上限
	if avail > 0 {
		if v := avail * 1024 / 10; v < limit {
			limit = v
		}
	}
	if cg > 0 {
		if v := cg * 1024 / 4; v < limit {
			limit = v
		}
	}
	// 保证测试后系统可用内存不低于 max(总内存20%, 512MiB)
	if avail > 0 {
		keep := total * 1024 / 5
		if keep < 512*1024*1024 {
			keep = 512 * 1024 * 1024
		}
		if v := avail*1024 - keep; v < limit && v > 0 {
			limit = v
		}
	}
	if limit < 64*1024*1024 {
		p.OK = false
		p.Reasons = append(p.Reasons, ReasonLowMemory)
		return 0, p
	}
	debug.SetMemoryLimit(limit)
	p.Detail["budget"] = strconv.FormatInt(limit, 10)
	return limit, p
}

func defaultDiskDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return os.TempDir()
	}
	return home
}

// DefaultDiskDir 返回磁盘测试默认目录。
func DefaultDiskDir() string { return defaultDiskDir() }

// Watch 是循环内检查器：温度、IO 压力、超时。每 checkEvery 检查一次。
type Watch struct {
	ctx      context.Context
	cancel   context.CancelFunc
	LastTemp float64
	LastPSI  float64
	Aborted  Reason
}

// NewWatch 创建带硬超时的检查器。timeout 是硬上限。
func NewWatch(timeout time.Duration) *Watch {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return &Watch{ctx: ctx, cancel: cancel}
}

// Ctx 返回受超时控制的 context。
func (w *Watch) Ctx() context.Context { return w.ctx }

// Check 在测试循环内顺带调用；返回 true 表示应中止。
func (w *Watch) Check(forceTemp bool) (stop bool, reason Reason) {
	select {
	case <-w.ctx.Done():
		return true, ReasonTimeout
	default:
	}
	if temp := probe.ThermalMaxC(); temp >= 0 {
		w.LastTemp = temp
		if temp >= 85 {
			return true, ReasonHighTemp
		}
	}
	if psi := probe.PSI("io"); psi >= 0 {
		w.LastPSI = psi
		if psi > 40 {
			return true, ReasonHighPSI
		}
	}
	return false, ""
}

// Done 释放资源。
func (w *Watch) Done() {
	if w.cancel != nil {
		w.cancel()
	}
}

var _ = filepath.Join

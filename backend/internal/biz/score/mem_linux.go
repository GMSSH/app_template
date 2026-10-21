//go:build linux

package score

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// memTotalKB 返回系统总内存（KB）。优先读 /proc/meminfo；失败时退回
// syscall.Sysinfo，不依赖 /proc 挂载，飞牛等常规 Linux 发行版和精简容器都能覆盖。
func memTotalKB() int {
	if kb := memTotalKBFromProc(); kb > 0 {
		return kb
	}
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err == nil {
		kb := uint64(info.Totalram) * uint64(info.Unit) / 1024
		if kb > 0 {
			return int(kb)
		}
	}
	return 0
}

func memTotalKBFromProc() int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		n, _ := strconv.Atoi(fields[1])
		return n
	}
	return 0
}

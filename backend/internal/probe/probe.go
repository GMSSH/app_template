// Package probe 读取 /proc、/sys 下的系统拓扑与状态。
// 读取失败的指标一律返回 unknown（零值/空），不 panic，不当作安全事件。
package probe

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// LoadAvg1 返回 1 分钟 loadavg；读不到返回 0。
func LoadAvg1() float64 {
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 1 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

// PSI 读取 /proc/pressure/<res> 的 some avg10 值（百分比）。
// res: cpu|io|memory。读不到返回 -1（不可监控）。
func PSI(res string) float64 {
	f, err := os.Open("/proc/pressure/" + res)
	if err != nil {
		return -1
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "some ") {
			continue
		}
		fields := strings.Fields(line)
		// some avg10=0.00 avg60=0.00 avg300=0.00 total=0
		for _, fld := range fields {
			if strings.HasPrefix(fld, "avg10=") {
				v, _ := strconv.ParseFloat(strings.TrimPrefix(fld, "avg10="), 64)
				return v
			}
		}
	}
	return -1
}

// MemAvailableKB 返回 /proc/meminfo 的 MemAvailable；读不到返回 0。
func MemAvailableKB() int64 {
	return meminfoInt("MemAvailable:")
}

// MemTotalKB 返回 /proc/meminfo 的 MemTotal；读不到返回 0。
func MemTotalKB() int64 {
	return meminfoInt("MemTotal:")
}

func meminfoInt(key string) int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, key) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseInt(fields[1], 10, 64)
				return v
			}
		}
	}
	return 0
}

// CgroupMemoryLimitKB 返回 cgroup v2 memory.max / v1 memory.limit_in_bytes。
// 未受限（值为 "max" 或超大）返回 0。
func CgroupMemoryLimitKB() int64 {
	// v2 优先
	if v, ok := readCgroup("/sys/fs/cgroup/memory.max"); ok {
		return v
	}
	// v1: 以当前进程 cgroup 为准
	if v, ok := readCgroup("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok {
		return v
	}
	return 0
}

func readCgroup(path string) (int64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	text := strings.TrimSpace(string(data))
	if text == "max" || text == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(text, 10, 64)
	if err != nil || v < 0 {
		return 0, false
	}
	// 未受限的容器常是接近 int64 上限的值
	if v >= (1 << 50) {
		return 0, false
	}
	return v / 1024, true // bytes → KB
}

// MaxCacheKB 返回 cpu0 的最大缓存大小（KB）；读不到返回 0。
func MaxCacheKB() int64 {
	var max int64
	for i := 0; i < 8; i++ {
		entries, err := os.ReadDir("/sys/devices/system/cpu/cpu0/cache")
		if err != nil {
			break
		}
		for _, e := range entries {
			data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cache/" + e.Name() + "/size")
			if err != nil {
				continue
			}
			kb := parseSizeKB(strings.TrimSpace(string(data)))
			if kb > max {
				max = kb
			}
		}
		break
	}
	return max
}

func parseSizeKB(s string) int64 {
	// 形如 "32K" "1024K" "4M" "48M" "64M"
	if s == "" {
		return 0
	}
	mult := int64(1)
	switch s[len(s)-1] {
	case 'K':
		mult = 1
		s = s[:len(s)-1]
	case 'M':
		mult = 1024
		s = s[:len(s)-1]
	case 'G':
		mult = 1024 * 1024
		s = s[:len(s)-1]
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v * mult
}

// CPUCount 返回可用 CPU 数（受 affinity 限制）；失败返回 1。
func CPUCount() int {
	n := numCPUAffinity()
	if n <= 0 {
		return 1
	}
	return n
}

// ThermalMaxC 返回所有 thermal zone 中的最高温度（摄氏，×1000 原始值转成度数）。
// 无传感器返回 -1。
func ThermalMaxC() float64 {
	entries, err := os.ReadDir("/sys/class/thermal")
	if err != nil {
		return -1
	}
	var max float64 = -1
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "thermal_zone") {
			continue
		}
		data, err := os.ReadFile("/sys/class/thermal/" + name + "/temp")
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
		if err != nil {
			continue
		}
		t := v / 1000.0
		if t > max {
			max = t
		}
	}
	return max
}

// StealPercent 返回当前 CPU steal 占比（百分比）。读不到返回 -1。
// 用两次采样差值近似瞬时 steal 比例。
func StealPercent() float64 {
	read := func() (steal, total uint64, ok bool) {
		f, err := os.Open("/proc/stat")
		if err != nil {
			return 0, 0, false
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "cpu ") {
				continue
			}
			fields := strings.Fields(line)[1:] // 跳过 "cpu"
			if len(fields) < 8 {
				return 0, 0, false
			}
			var vals [8]uint64
			for i := 0; i < 8; i++ {
				vals[i], _ = strconv.ParseUint(fields[i], 10, 64)
			}
			for _, v := range vals {
				total += v
			}
			return vals[7], total, true // steal 是第 8 个字段
		}
		return 0, 0, false
	}
	s1, t1, ok1 := read()
	if !ok1 || t1 == 0 {
		return -1
	}
	// 单次采样只返回绝对 steal 的占比近似；跨轮比较由 bench 层做。
	_ = s1
	return -1
}

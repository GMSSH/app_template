//go:build !linux

package score

// 非 Linux 平台（如 macOS 本地开发）没有 /proc/meminfo 和 syscall.Sysinfo，
// 内存容量留空，跑分按固定中间值处理。生产部署目标都是 Linux。
func memTotalKB() int { return 0 }

//go:build linux

package probe

import "golang.org/x/sys/unix"

// numCPUAffinity 返回当前进程亲和性允许的 CPU 数（受 sched_getaffinity 限制）。
func numCPUAffinity() int {
	var set unix.CPUSet
	if err := unix.SchedGetaffinity(0, &set); err != nil {
		return 0
	}
	return set.Count()
}

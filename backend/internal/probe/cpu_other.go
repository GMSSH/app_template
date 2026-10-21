//go:build !linux

package probe

import "runtime"

func numCPUAffinity() int { return runtime.NumCPU() }

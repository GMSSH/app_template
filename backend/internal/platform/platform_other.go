//go:build !linux

package platform

import (
	"os"
)

// SetLowPriority 非 Linux 平台无操作。
func SetLowPriority() error { return nil }

// OpenAnonymousFile 非 Linux 平台回退：创建即 unlink，进程强杀后内核自动回收空间。
func OpenAnonymousFile(dir string, mode os.FileMode) (*os.File, error) {
	f, err := os.CreateTemp(dir, ".sbench-*")
	if err != nil {
		return nil, err
	}
	_ = os.Remove(f.Name()) // 目录项立即消失，句柄仍可读写
	return f, nil
}

// OpenDirect 非 Linux 平台不支持 O_DIRECT。
func OpenDirect(f *os.File) (*os.File, error) { return nil, ErrUnsupported }

// FSInfo 非 Linux 平台返回 unknown/4096（测量层在非 Linux 会直接跳过）。
func FSInfo(path string) (string, int64, error) { return "unknown", 4096, nil }

// DiskUsage 非 Linux 平台返回 0。
func DiskUsage(path string) (uint64, uint64, error) { return 0, 0, ErrUnsupported }

// SetFileNoCOW 非 Linux 平台无操作。
func SetFileNoCOW(f *os.File) {}

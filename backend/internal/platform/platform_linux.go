//go:build linux

package platform

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// SetLowPriority 让本进程给业务进程让路：nice 19 + ioprio idle（尽力而为）。
func SetLowPriority() error {
	if err := unix.Setpriority(unix.PRIO_PROCESS, 0, 19); err != nil {
		return err
	}
	// ioprio_set(IOPRIO_WHO_PROCESS, 0, IOPRIO_CLASS_IDLE<<13)。
	// 仅 BFQ 调度器生效，失败忽略。直接 syscall，避免依赖 x/sys 的导出符号。
	_, _, _ = unix.Syscall(unix.SYS_IOPRIO_SET, 1, 0, uintptr(3<<13))
	return nil
}

// OpenAnonymousFile 在 dir 下创建一个匿名临时文件（O_TMPFILE），
// 进程被强杀后空间由内核自动回收，不残留目录项。
// 不支持 O_TMPFILE 的文件系统回退为 O_CREAT 后立即 unlink。
func OpenAnonymousFile(dir string, mode os.FileMode) (*os.File, error) {
	fd, err := unix.Open(dir, unix.O_TMPFILE|unix.O_RDWR|unix.O_CLOEXEC, uint32(mode.Perm()))
	if err == nil {
		return os.NewFile(uintptr(fd), dir+"/.#sbench"), nil
	}
	// 回退：创建后立刻 unlink
	f, cerr := os.CreateTemp(dir, ".sbench-*")
	if cerr != nil {
		return nil, err
	}
	if uerr := os.Remove(f.Name()); uerr != nil {
		f.Close()
		return nil, uerr
	}
	return f, nil
}

// OpenDirect 打开 path 并尽量设置 O_DIRECT。目录 fd 上的 O_TMPFILE 无法
// 再以 O_DIRECT 打开，因此需要 /proc/self/fd 路径；失败时调用方降级。
func OpenDirect(f *os.File) (*os.File, error) {
	fd, err := unix.Open("/proc/self/fd/"+itoa(int(f.Fd())), unix.O_RDWR|unix.O_DIRECT|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "direct:"+f.Name()), nil
}

// FSInfo 返回路径所在文件系统的类型名与逻辑块大小（读 sysfs，失败返回 0）。
func FSInfo(path string) (fsType string, logicalBlockSize int64, err error) {
	var st syscall.Statfs_t
	if err = syscall.Statfs(path, &st); err != nil {
		return "", 0, err
	}
	fsType = fsTypeName(st.Type)
	logicalBlockSize = int64(st.Bsize)
	if logicalBlockSize <= 0 {
		logicalBlockSize = 4096
	}
	return fsType, logicalBlockSize, nil
}

func fsTypeName(magic int64) string {
	switch magic {
	case 0xEF53:
		return "ext4"
	case 0x58465342:
		return "xfs"
	case 0x9123683E:
		return "btrfs"
	case 0x2FC12FC1:
		return "zfs"
	case 0xF2F52010:
		return "f2fs"
	case 0x01021994:
		return "tmpfs"
	case 0x794C7630:
		return "overlay"
	case 0x6969:
		return "nfs"
	case 0xFF534D42:
		return "cifs"
	case 0x65735546:
		return "fuse"
	default:
		return "unknown"
	}
}

// DiskUsage 返回路径所在分区的总字节与可用字节。
func DiskUsage(path string) (total, free uint64, err error) {
	var st syscall.Statfs_t
	if err = syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	bs := uint64(st.Bsize)
	if bs == 0 {
		bs = 4096
	}
	return st.Blocks * bs, st.Bavail * bs, nil
}

// SetFileNoCOW 在 btrfs 上尽力为文件设置 NOCOW（ioctl FS_IOC_SETFLAGS），失败忽略。
func SetFileNoCOW(f *os.File) {
	const (
		fsIocSetFlags = 0x40086602
		fsNoCOWFlag   = 0x00800000
		fsFlagUserMod = 0x000fffff
	)
	if f == nil {
		return
	}
	// 简单起见：只尝试 ioctl，返回值忽略（调用方已容忍失败）。
	_ = unix.IoctlSetInt(int(f.Fd()), fsIocSetFlags, fsNoCOWFlag&fsFlagUserMod)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

var _ = errors.Is

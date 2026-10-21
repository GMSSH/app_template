// Package platform 封装平台相关的低层能力。
// 平台符号（O_DIRECT、ioprio_set、setpriority 等）只允许出现在本包。
// Linux 特性通过 build tag 隔离，非 Linux 平台一律降级返回 unsupported。
package platform

import "errors"

// ErrUnsupported 表示当前平台不支持该能力（调用方自行降级）。
var ErrUnsupported = errors.New("platform: unsupported")

package toolbox

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// maxOutput 单次命令最多保留的 stdout 字节数。运维命令输出可能极大（如 dig +trace），
// 截断而不是让整段塞进 JSON 信封。超出时 Truncated=true，调用方需如实告知用户。
const maxOutput = 256 * 1024

// Lookup 记录本进程内某命令是否存在，避免每次检测都探测一遍 PATH。
var (
	lookupMu    sync.Mutex
	lookupCache = map[string]bool{}
)

// Has 判断系统是否安装了指定命令。用于给用户「缺少依赖」的明确提示，
// 绝不在检测过程中尝试安装任何东西。
func Has(name string) bool {
	lookupMu.Lock()
	defer lookupMu.Unlock()
	if v, ok := lookupCache[name]; ok {
		return v
	}
	_, err := exec.LookPath(name)
	lookupCache[name] = err == nil
	return err == nil
}

// Result 是一次外部命令执行的完整事实：标准输出、标准错误、退出码、是否超时。
type Result struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	TimedOut  bool   `json:"timed_out"`
	Truncated bool   `json:"truncated"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

// Run 用「可执行文件 + 参数数组」执行命令，不经过 shell，
// 因此用户输入永远无法变成命令拼接或管道。
func RunCmd(timeout time.Duration, name string, args ...string) (Result, error) {
	return RunCmdIn(timeout, "", name, args...)
}

// RunCmdIn 与 RunCmd 相同，但可以从 stdin 喂入数据（例如把 PEM 交给 openssl x509）。
func RunCmdIn(timeout time.Duration, stdin string, name string, args ...string) (Result, error) {
	if !Has(name) {
		return Result{ExitCode: -1}, ErrCommandMissing{Name: name}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start).Milliseconds()

	out := stdout.String()
	truncated := false
	if len(out) > maxOutput {
		out = out[:maxOutput]
		truncated = true
	}

	res := Result{
		Stdout:    out,
		Stderr:    stderr.String(),
		ExitCode:  cmd.ProcessState.ExitCode(),
		TimedOut:  errors.Is(ctx.Err(), context.DeadlineExceeded),
		Truncated: truncated,
		ElapsedMS: elapsed,
	}

	// 超时是「检测结论」而不是框架错误：调用方据此返回 timed_out 状态。
	if res.TimedOut {
		return res, nil
	}
	if err != nil && res.ExitCode == 0 {
		res.ExitCode = -1
	}
	return res, nil
}

// ErrCommandMissing 表示本机缺少某个检测所需的命令。
type ErrCommandMissing struct{ Name string }

func (e ErrCommandMissing) Error() string { return "命令不存在: " + e.Name }

// OutLines 把 stdout 切成去空行的行列表，供解析器逐个匹配。
func OutLines(s string) []string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

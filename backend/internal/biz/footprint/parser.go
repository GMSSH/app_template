package footprint

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AuthMethod 认证方式
type AuthMethod string

const (
	MethodPassword  AuthMethod = "password"
	MethodPublicKey AuthMethod = "publickey"
	MethodUnknown   AuthMethod = "unknown"
)

// AuthResult 认证结果
type AuthResult string

const (
	ResultSuccess AuthResult = "success"
	ResultFailed  AuthResult = "failed"
)

// RawEvent 解析后的单条认证事件
type RawEvent struct {
	Timestamp int64      // 毫秒时间戳
	TimeStr   string     // 格式化服务器本地时间
	IP        string     // 来源 IP
	User      string     // 尝试登录的用户名
	Method    AuthMethod // password / publickey / unknown
	Result    AuthResult // success / failed
	IsInvalid bool       // 是否为 Invalid user
	Raw       string     // 日志原文
}

// 正则表达式匹配常见 SSH 日志
var (
	// IP 正则
	ipRegex = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`)

	// Failed password for [invalid user] <user> from <ip> port <port> ssh2
	// Accepted password for <user> from <ip> port <port> ssh2
	// Accepted publickey for <user> from <ip> port <port> ssh2: RSA SHA256:...
	reFailedPassword   = regexp.MustCompile(`Failed password for (?:invalid user )?(\S+) from (\S+) port (\d+)`)
	reAcceptedPassword = regexp.MustCompile(`Accepted password for (\S+) from (\S+) port (\d+)`)
	reAcceptedPubKey   = regexp.MustCompile(`Accepted publickey for (\S+) from (\S+) port (\d+)`)
	reInvalidUser      = regexp.MustCompile(`Invalid user (\S+) from (\S+) port (\d+)`)
	reAuthFailure      = regexp.MustCompile(`authentication failure;.*rhost=(\S+)(?:.*user=(\S+))?`)
)

// parseSyslogTime 解析标准 syslog 时间：
// 1. RFC3339 / ISO8601: "2026-10-23T22:30:15.123456+08:00"
// 2. Syslog 格式: "Oct 23 22:30:15"
func parseSyslogTime(line string, now time.Time, loc *time.Location) (time.Time, string) {
	// 尝试 ISO8601/RFC3339
	if len(line) >= 19 {
		prefix := line[:19]
		if t, err := time.ParseInLocation("2006-01-02T15:04:05", prefix, loc); err == nil {
			return t, t.In(loc).Format("2006-01-02 15:04:05")
		}
	}

	// 尝试标准 syslog 格式 "Oct 23 22:30:15"
	if len(line) >= 15 {
		prefix := line[:15]
		t, err := time.ParseInLocation("Jan _2 15:04:05", prefix, loc)
		if err == nil {
			// syslog 通常不带年份，默认当年
			year := now.Year()
			parsed := time.Date(year, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc)
			// 如果计算出的时间比当前时间超出较大跨度（例如日志在去年12月，当前是1月），回退一年
			if parsed.After(now.Add(24 * time.Hour)) {
				parsed = parsed.AddDate(-1, 0, 0)
			}
			return parsed, parsed.In(loc).Format("2006-01-02 15:04:05")
		}
	}

	return time.Time{}, ""
}

// unescapeJournal 还原 journalctl 输出的 C 风格转义。
// journalctl -o short-iso 会把非 ASCII 字节转成 \346\202\250 这类八进制，
// 不清洗会让中文/特殊字符用户名在界面上显示成乱码。
func unescapeJournal(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		// \xNN 十六进制
		if s[i+1] == 'x' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+2:i+4], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		// \NNN 八进制
		if s[i+1] >= '0' && s[i+1] <= '7' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		switch s[i+1] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte(c)
		}
		i++
	}
	return b.String()
}

// cleanUser 清理提取的用户名：去掉转义残留与控制字符，限制展示长度
func cleanUser(u string) string {
	u = unescapeJournal(u)
	u = strings.TrimSpace(u)
	u = strings.Trim(u, "'\"")
	u = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, u)
	if u == "" {
		return "unknown"
	}
	if r := []rune(u); len(r) > 32 {
		u = string(r[:32]) + "…"
	}
	return u
}

// cleanIP 校验并提取标准 IPv4
func cleanIP(ipStr string) string {
	ipStr = strings.TrimSpace(ipStr)
	match := ipRegex.FindString(ipStr)
	return match
}

// parseLine 解析单行 SSH 日志
func parseLine(line string, now time.Time, loc *time.Location) *RawEvent {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// 必须包含 ssh/sshd 或者典型关键字
	if !strings.Contains(line, "ssh") &&
		!strings.Contains(line, "Failed password") &&
		!strings.Contains(line, "Accepted") &&
		!strings.Contains(line, "Invalid user") {
		return nil
	}

	t, timeStr := parseSyslogTime(line, now, loc)
	if t.IsZero() {
		// 未能解析时间的不纳入统计
		return nil
	}

	// 1. Failed password
	if m := reFailedPassword.FindStringSubmatch(line); len(m) >= 3 {
		user := cleanUser(m[1])
		ip := cleanIP(m[2])
		if ip == "" {
			return nil
		}
		isInvalid := strings.Contains(line, "invalid user")
		return &RawEvent{
			Timestamp: t.UnixMilli(),
			TimeStr:   timeStr,
			IP:        ip,
			User:      user,
			Method:    MethodPassword,
			Result:    ResultFailed,
			IsInvalid: isInvalid,
			Raw:       line,
		}
	}

	// 2. Accepted publickey
	if m := reAcceptedPubKey.FindStringSubmatch(line); len(m) >= 3 {
		user := cleanUser(m[1])
		ip := cleanIP(m[2])
		if ip == "" {
			return nil
		}
		return &RawEvent{
			Timestamp: t.UnixMilli(),
			TimeStr:   timeStr,
			IP:        ip,
			User:      user,
			Method:    MethodPublicKey,
			Result:    ResultSuccess,
			IsInvalid: false,
			Raw:       line,
		}
	}

	// 3. Accepted password
	if m := reAcceptedPassword.FindStringSubmatch(line); len(m) >= 3 {
		user := cleanUser(m[1])
		ip := cleanIP(m[2])
		if ip == "" {
			return nil
		}
		return &RawEvent{
			Timestamp: t.UnixMilli(),
			TimeStr:   timeStr,
			IP:        ip,
			User:      user,
			Method:    MethodPassword,
			Result:    ResultSuccess,
			IsInvalid: false,
			Raw:       line,
		}
	}

	// 4. Invalid user (某些发行版可能只有 Invalid user xxx from yyy，之后紧跟 pam 错误而无 Failed password)
	if m := reInvalidUser.FindStringSubmatch(line); len(m) >= 3 {
		user := cleanUser(m[1])
		ip := cleanIP(m[2])
		if ip == "" {
			return nil
		}
		return &RawEvent{
			Timestamp: t.UnixMilli(),
			TimeStr:   timeStr,
			IP:        ip,
			User:      user,
			Method:    MethodUnknown,
			Result:    ResultFailed,
			IsInvalid: true,
			Raw:       line,
		}
	}

	return nil
}

// readJournalLogs 通过 journalctl 读取 ssh 日志
func readJournalLogs(sinceStr string, maxLines int) ([]string, error) {
	path, err := exec.LookPath("journalctl")
	if err != nil {
		return nil, fmt.Errorf("journalctl not found")
	}

	// 兼容不同发行版的 ssh 服务名 (ssh 或 sshd)
	// journalctl -u ssh -u sshd --since "..." --no-pager -o short-iso -n maxLines
	args := []string{"-u", "ssh", "-u", "sshd", "--no-pager", "-o", "short-iso"}
	if sinceStr != "" {
		args = append(args, "--since", sinceStr)
	}
	if maxLines > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", maxLines))
	}

	cmd := exec.Command(path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("exec journalctl failed: %v, stderr: %s", err, stderr.String())
	}

	var lines []string
	scanner := bufio.NewScanner(&stdout)
	// 允许长行
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines, scanner.Err()
}

// readFileLogs 从指定文件读取日志，支持普通文本和 .gz 轮转压缩文件
func readFileLogs(filePath string, maxLines int) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reader io.Reader = f
	if strings.HasSuffix(filePath, ".gz") {
		gzReader, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	var lines []string
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	return lines, scanner.Err()
}

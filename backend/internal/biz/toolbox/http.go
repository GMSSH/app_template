package toolbox

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CurlTiming 是 curl 的 --write-out 原始值（单位秒，全部是「从请求开始算起」的累计值）。
// 注意：这些数值不能直接当各阶段耗时相加，必须先做差。
type curlRaw struct {
	httpCode    int
	remoteIP    string
	redirectURL string
	redirects   int
	namelookup  float64
	connect     float64
	appconnect  float64
	starttrans  float64
	total       float64
	sizeDl      int64
	contentType string
	protocol    string
	errMsg      string
}

const curlFormat = "\n__WM__%{http_code}|%{remote_ip}|%{redirect_url}|%{num_redirects}|%{time_namelookup}|%{time_connect}|%{time_appconnect}|%{time_starttransfer}|%{time_total}|%{size_download}|%{content_type}|%{http_version}"

// HTTPHeader 是响应头的一条记录，保持注册局返回的原始顺序。
type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPStage 是做过差之后的各阶段独立耗时（毫秒）。
type HTTPStage struct {
	DNSMS   int64 `json:"dns_ms"`
	TCPMS   int64 `json:"tcp_ms"`
	TLSMS   int64 `json:"tls_ms"`
	TTFBMS  int64 `json:"ttfb_ms"`
	TotalMS int64 `json:"total_ms"`
}

// HTTPResult 是一次 HTTP 请求的完整结果。
type HTTPResult struct {
	URL          string       `json:"url"`
	Method       string       `json:"method"`
	StatusCode   int          `json:"status_code"`
	RemoteIP     string       `json:"remote_ip"`
	RedirectURL  string       `json:"redirect_url"`
	Redirects    int          `json:"redirects"`
	ContentType  string       `json:"content_type"`
	Protocol     string       `json:"protocol"`
	Headers      []HTTPHeader `json:"headers"`
	SizeBytes    int64        `json:"size_bytes"`
	Stage        HTTPStage    `json:"stage"`
	TLS          bool         `json:"tls"`
	TimedOut     bool         `json:"timed_out"`
	ErrorKind    string       `json:"error_kind"`
	ErrorDetail  string       `json:"error_detail"`
	CommandError string       `json:"command_error,omitempty"`
}

// curlErrorKind 把 curl 的退出码翻译成用户能看懂的失败类别。
func curlErrorKind(code int) (string, string) {
	switch code {
	case 6:
		return "dns_failed", "无法解析域名（DNS 查询失败）"
	case 7:
		return "connect_failed", "无法建立 TCP 连接（目标端口拒绝或网络不通）"
	case 28:
		return "timeout", "请求超时"
	case 35:
		return "tls_failed", "TLS 握手失败"
	case 51, 60:
		return "cert_invalid", "证书校验失败（证书无效、过期或域名不匹配）"
	case 52:
		return "empty_reply", "服务器返回空响应"
	case 56:
		return "recv_failed", "接收数据失败（连接被重置）"
	case 5:
		return "proxy_failed", "代理不可用"
	default:
		return "curl_failed", fmt.Sprintf("curl 执行失败（退出码 %d）", code)
	}
}

// FetchHTTP 用 curl 发一次请求并给出分阶段耗时。
// --insecure 不启用：证书问题必须暴露出来，不能静默通过。
func FetchHTTP(target Target, method string, timeout time.Duration, followMax int) (HTTPResult, error) {
	if !Has("curl") {
		return HTTPResult{CommandError: "当前服务器缺少 curl，无法进行 HTTP 检测"}, nil
	}
	if method != "HEAD" {
		method = "GET"
	}

	seconds := int(timeout / time.Second)
	if seconds < 1 {
		seconds = 1
	}

	// -D - 把响应头打到 stdout，-o /dev/null 丢弃正文：
	// 只取「状态码 + 响应头 + 计时」，不下载整页内容。
	args := []string{
		"-sS",
		"-o", "/dev/null",
		"-D", "-",
		"-X", method,
		"--max-time", strconv.Itoa(seconds),
		"--connect-timeout", strconv.Itoa(seconds),
		"-A", "GMSSH-WebmasterTools/1.0",
		"-w", curlFormat,
	}
	if followMax > 0 {
		args = append(args, "-L", "--max-redirs", strconv.Itoa(followMax))
	}
	args = append(args, target.URL)

	res, _ := RunCmd(timeout+3*time.Second, "curl", args...)

	out := HTTPResult{URL: target.URL, Method: method, TLS: target.Scheme == "https"}

	if res.TimedOut {
		out.TimedOut = true
		out.ErrorKind = "timeout"
		out.ErrorDetail = fmt.Sprintf("超过 %d 秒未完成", seconds)
		return out, nil
	}

	raw, ok := parseCurlWriteOut(res.Stdout)
	if !ok {
		if res.ExitCode != 0 {
			out.ErrorKind, out.ErrorDetail = curlErrorKind(res.ExitCode)
			return out, nil
		}
		out.ErrorKind = "no_output"
		out.ErrorDetail = "curl 未返回计时数据"
		return out, nil
	}

	out.StatusCode = raw.httpCode
	out.RemoteIP = raw.remoteIP
	out.RedirectURL = raw.redirectURL
	out.Redirects = raw.redirects
	out.ContentType = raw.contentType
	out.Protocol = raw.protocol
	out.Headers = parseResponseHeaders(res.Stdout)
	out.SizeBytes = raw.sizeDl
	out.Stage = computeStage(raw)

	if res.ExitCode != 0 {
		out.ErrorKind, out.ErrorDetail = curlErrorKind(res.ExitCode)
	}
	return out, nil
}

// computeStage 把累计计时做差，得到各阶段真正消耗的时间。
// TLS 阶段只有 HTTPS 才有（http 下 time_appconnect 为 0）。
// parseResponseHeaders 从 curl 的 -D - 输出里取最后一个 header 块。
// 跟随重定向时会有多个块，只有最后一块是最终响应。
func parseResponseHeaders(s string) []HTTPHeader {
	if i := strings.Index(s, "__WM__"); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	blocks := strings.Split(s, "\n\n")

	var block []string
	for i := len(blocks) - 1; i >= 0; i-- {
		lines := strings.Split(strings.TrimSpace(blocks[i]), "\n")
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "HTTP/") {
			block = lines
			break
		}
	}

	out := make([]HTTPHeader, 0, len(block))
	for _, line := range block[1:] {
		j := strings.Index(line, ":")
		if j <= 0 {
			continue
		}
		value := strings.TrimSpace(line[j+1:])
		if value == "" {
			continue
		}
		out = append(out, HTTPHeader{Name: strings.TrimSpace(line[:j]), Value: value})
		if len(out) >= 40 {
			break
		}
	}
	return out
}

func computeStage(r curlRaw) HTTPStage {
	ms := func(v float64) int64 {
		if v <= 0 {
			return 0
		}
		return int64(v*1000 + 0.5)
	}
	dns := ms(r.namelookup)
	tcp := ms(r.connect - r.namelookup)
	tls := ms(r.appconnect - r.connect)
	if r.appconnect <= 0 {
		tls = 0
	}
	ttfb := ms(r.starttrans - maxF(r.connect, r.appconnect))
	total := ms(r.total)
	if tcp < 0 {
		tcp = 0
	}
	if tls < 0 {
		tls = 0
	}
	if ttfb < 0 {
		ttfb = 0
	}
	return HTTPStage{DNSMS: dns, TCPMS: tcp, TLSMS: tls, TTFBMS: ttfb, TotalMS: total}
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func parseCurlWriteOut(s string) (curlRaw, bool) {
	var r curlRaw
	i := strings.Index(s, "__WM__")
	if i < 0 {
		return r, false
	}
	line := s[i+len("__WM__"):]
	if j := strings.IndexAny(line, "\n"); j >= 0 {
		line = line[:j]
	}
	parts := strings.Split(line, "|")
	if len(parts) < 11 {
		return r, false
	}
	if len(parts) >= 12 {
		r.protocol = "HTTP/" + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[11]), "HTTP/"))
	}
	r.httpCode, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	r.remoteIP = parts[1]
	r.redirectURL = parts[2]
	r.redirects, _ = strconv.Atoi(strings.TrimSpace(parts[3]))
	r.namelookup, _ = strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
	r.connect, _ = strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
	r.appconnect, _ = strconv.ParseFloat(strings.TrimSpace(parts[6]), 64)
	r.starttrans, _ = strconv.ParseFloat(strings.TrimSpace(parts[7]), 64)
	r.total, _ = strconv.ParseFloat(strings.TrimSpace(parts[8]), 64)
	r.sizeDl, _ = strconv.ParseInt(strings.TrimSpace(parts[9]), 10, 64)
	r.contentType = parts[10]
	return r, true
}

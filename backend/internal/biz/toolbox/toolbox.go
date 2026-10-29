package toolbox

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
)

// Run 是站长百宝箱的统一入口：一个 action 一次检测，返回一份结果。
// 每个工具都只在服务器本机执行只读命令（ping/dig/curl/openssl）或纯 TCP 连接，
// 不改动任何系统配置。
func Run(ctx *middleware.Context, in input.Input) (any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Str("action")))
	if action == "" {
		action = "probe"
	}

	timeout := time.Duration(clampInt(in.Int("timeout"), 1, 60)) * time.Second

	if action == "probe" {
		return Probe(), nil
	}

	raw := strings.TrimSpace(in.Str("target"))
	if raw == "" {
		return nil, fmt.Errorf("请提供检测目标（域名、IP 或完整网址）")
	}

	switch action {
	case "dns":
		host, err := ParseHostOnly(raw)
		if err != nil {
			return nil, err
		}
		server := strings.TrimSpace(in.Str("server"))
		if server != "" {
			if _, err := ParseHostOnly(server); err != nil {
				return nil, fmt.Errorf("DNS 服务器地址不合法：%s", server)
			}
		}
		recordType := strings.ToUpper(strings.TrimSpace(in.Str("type")))
		if recordType == "ALL" {
			return QueryDNSAll(host, server), nil
		}
		return QueryDNS(host, recordType, server)

	case "ping":
		host, err := ParseHostOnly(raw)
		if err != nil {
			return nil, err
		}
		count := clampInt(in.Int("count"), 1, 30)
		return RunPing(host, count, time.Second, timeout)

	case "port":
		host, err := ParseHostOnly(raw)
		if err != nil {
			return nil, err
		}
		// 端口检测并发进行，单端口等待超过 5 秒没有意义，还容易顶满宿主 30 秒执行超时
		dial := timeout
		if dial > 5*time.Second {
			dial = 5 * time.Second
		}
		return ScanCommonPorts(host, dial), nil

	case "http":
		target, usedDefault, err := parseForHTTP(raw, in.Str("port"))
		if err != nil {
			return nil, err
		}
		res, err := FetchHTTP(target, "GET", timeout, 10)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"url": res.URL, "method": res.Method, "status_code": res.StatusCode,
			"remote_ip": res.RemoteIP, "redirect_url": res.RedirectURL,
			"redirects": res.Redirects, "content_type": res.ContentType,
			"protocol": res.Protocol, "headers": res.Headers,
			"size_bytes": res.SizeBytes, "stage": res.Stage, "tls": res.TLS,
			"timed_out": res.TimedOut, "error_kind": res.ErrorKind,
			"error_detail":        res.ErrorDetail,
			"used_default_scheme": usedDefault, "host": target.Host, "port": target.Port,
			"scheme": target.Scheme,
		}, nil

	case "ssl":
		host, err := ParseHostOnly(raw)
		if err != nil {
			return nil, err
		}
		port, err := portOrDefault(in.Str("port"), 443)
		if err != nil {
			return nil, err
		}
		return CheckSSL(host, port, timeout)

	case "whois":
		domain, err := ParseHostOnly(raw)
		if err != nil {
			return nil, err
		}
		return QueryWhois(domain, timeout), nil
	}

	return nil, fmt.Errorf("不支持的操作：%s", action)
}

// Probe 报告当前服务器上有哪些检测依赖可用。界面/排障时用它解释「为什么某项检测失败」。
func Probe() map[string]any {
	tools := []map[string]any{}
	for _, name := range []string{"ping", "dig", "nslookup", "getent", "curl", "openssl", "whois"} {
		tools = append(tools, map[string]any{"name": name, "available": Has(name)})
	}
	return map[string]any{
		"version": "0.1.0",
		"tools":   tools,
		"ports":   defaultPorts,
	}
}

// parseForHTTP 把输入解析成 HTTP 目标，并告知是否补了默认协议。
func parseForHTTP(raw, portStr string) (Target, bool, error) {
	usedDefault := false
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") &&
		!strings.HasPrefix(lower, "http:") && !strings.HasPrefix(lower, "https:") {
		usedDefault = true
	}

	// 端口参数只在输入里没带端口时生效，避免与 URL 内的端口冲突
	if p := strings.TrimSpace(portStr); p != "" && !hasExplicitPort(trimmed) {
		if _, err := ParsePort(p); err != nil {
			return Target{}, usedDefault, err
		}
		trimmed = strings.TrimSuffix(trimmed, "/") + ":" + p
	}

	t, err := ParseTarget(trimmed, "https")
	return t, usedDefault, err
}

func hasExplicitPort(s string) bool {
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/"); i >= 0 {
		s = s[:i]
	}
	if strings.HasPrefix(s, "[") {
		return strings.Contains(s, "]:")
	}
	return strings.Contains(s, ":")
}

func portOrDefault(s string, def int) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return def, nil
	}
	return ParsePort(s)
}

func parseInterval(s string) time.Duration {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v <= 0 {
		return time.Second
	}
	if v > 5 {
		v = 5
	}
	return time.Duration(v * float64(time.Second))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

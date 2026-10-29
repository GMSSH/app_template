package toolbox

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Target 是「用户想检测什么」的规范化结果。所有检测都只接受这个结构，
// 而不是原始输入字符串，保证任何输入都无法影响命令行形态。
type Target struct {
	Input   string `json:"input"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Scheme  string `json:"scheme"`
	URL     string `json:"url"`
	IsIP    bool   `json:"is_ip"`
	IsIPv6  bool   `json:"is_ipv6"`
	HasPort bool   `json:"has_port"`
	Path    string `json:"path"`
}

var hostRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9_-]{0,61}[a-zA-Z0-9])?)*$`)

// ParseTarget 解析域名 / IP / URL。缺协议时按提示的默认协议补全，
// 并把「用了默认协议」这件事写进结果，界面据此提示用户。
func ParseTarget(raw string, defaultScheme string) (Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Target{}, fmt.Errorf("请输入域名、IP 或完整网址")
	}
	if len(raw) > 2048 {
		return Target{}, fmt.Errorf("输入过长，最多 2048 个字符")
	}
	if strings.ContainsAny(raw, " \t\r\n\x00") {
		return Target{}, fmt.Errorf("目标中不能包含空格或控制字符")
	}

	// 容错处理：用户漏输斜杠写成 http:example.com 或 https:/example.com
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http:") && !strings.HasPrefix(lower, "http://") {
		raw = "http://" + strings.TrimPrefix(strings.TrimPrefix(raw[5:], "/"), "/")
	} else if strings.HasPrefix(lower, "https:") && !strings.HasPrefix(lower, "https://") {
		raw = "https://" + strings.TrimPrefix(strings.TrimPrefix(raw[6:], "/"), "/")
	}

	scheme := defaultScheme
	rest := raw

	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
		if u.Scheme != "http" && u.Scheme != "https" {
			return Target{}, fmt.Errorf("仅支持 http / https 协议，当前为 %s", u.Scheme)
		}
		if u.User != nil {
			return Target{}, fmt.Errorf("不支持带账号密码的网址")
		}
		scheme = u.Scheme
		p := u.Port()
		port := 0
		if p != "" {
			if port, err = parsePort(p); err != nil {
				return Target{}, err
			}
		}
		host := u.Hostname()
		if err := checkHost(host); err != nil {
			return Target{}, err
		}
		if port == 0 {
			port = defaultPort(scheme)
		}
		path := u.RequestURI()
		if path == "" {
			path = "/"
		}
		return Target{
			Input: raw, Host: host, Port: port, Scheme: scheme,
			URL:  scheme + "://" + u.Host + path,
			IsIP: net.ParseIP(host) != nil, IsIPv6: isV6(host),
			HasPort: p != "", Path: path,
		}, nil
	}

	// 没有协议：允许 host[:port][/path]
	hostPart := rest
	path := "/"
	if i := strings.IndexAny(rest, "/"); i >= 0 {
		hostPart = rest[:i]
		path = rest[i:]
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	port := 0
	hasPort := false
	if h, p, err := net.SplitHostPort(hostPart); err == nil {
		hostPart = h
		if port, err = parsePort(p); err != nil {
			return Target{}, err
		}
		hasPort = true
	}
	if err := checkHost(hostPart); err != nil {
		return Target{}, err
	}
	if port == 0 {
		port = defaultPort(scheme)
	}
	hostForURL := hostPart
	if isV6(hostPart) {
		hostForURL = "[" + hostPart + "]"
	}
	urlHost := hostForURL
	if hasPort {
		urlHost = hostForURL + ":" + strconv.Itoa(port)
	}
	return Target{
		Input: raw, Host: hostPart, Port: port, Scheme: scheme,
		URL:  scheme + "://" + urlHost + path,
		IsIP: net.ParseIP(hostPart) != nil, IsIPv6: isV6(hostPart),
		HasPort: hasPort, Path: path,
	}, nil
}

func checkHost(host string) error {
	if host == "" {
		return fmt.Errorf("无法识别主机名，请输入有效域名或 IP")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if len(host) > 253 {
		return fmt.Errorf("域名过长")
	}
	labels := strings.Split(host, ".")
	for _, l := range labels {
		if l == "" {
			return fmt.Errorf("域名格式错误：存在连续的点")
		}
	}
	if !hostRe.MatchString(host) {
		return fmt.Errorf("域名格式不合法：%s", host)
	}
	if len(labels) < 2 && host != "localhost" {
		return fmt.Errorf("请输入完整域名（如 example.com）或 IP 地址")
	}
	return nil
}

func parsePort(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("端口必须是数字")
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("端口必须在 1-65535 范围内")
	}
	return n, nil
}

// ParsePort 供只关心端口参数的命令（TCP 检测）复用。
func ParsePort(s string) (int, error) { return parsePort(s) }

func defaultPort(scheme string) int {
	if scheme == "https" {
		return 443
	}
	return 80
}

func isV6(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.To4() == nil
}

// ParseHostOnly 用于 DNS / PING / TCP / 路由追踪这类只需要主机的命令。
func ParseHostOnly(raw string) (string, error) {
	t, err := ParseTarget(raw, "https")
	if err != nil {
		return "", err
	}
	return t.Host, nil
}

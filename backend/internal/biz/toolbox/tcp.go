package toolbox

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// TCPResult 是一次端口连通性探测的结果。
type TCPResult struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Connected bool   `json:"connected"`
	ElapsedMS int64  `json:"elapsed_ms"`
	RemoteIP  string `json:"remote_ip"`
	ErrorKind string `json:"error_kind"`
	ErrorText string `json:"error_text"`
}

// DialTCP 直接发起 TCP 连接。失败原因在可确定范围内分类：
// refused（端口未监听）、timeout（无响应）、unreachable（路由不可达）、dns（域名解析失败）。
func DialTCP(host string, port int, timeout time.Duration) (TCPResult, error) {
	out := TCPResult{Host: host, Port: port}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	out.ElapsedMS = time.Since(start).Milliseconds()

	if err != nil {
		out.Connected = false
		classifyDialError(err, &out)
		return out, nil
	}
	defer conn.Close()

	out.Connected = true
	if ra := conn.RemoteAddr(); ra != nil {
		if h, _, e := net.SplitHostPort(ra.String()); e == nil {
			out.RemoteIP = h
		}
	}
	return out, nil
}

func classifyDialError(err error, out *TCPResult) {
	msg := err.Error()
	low := strings.ToLower(msg)

	switch {
	case strings.Contains(low, "connection refused"):
		out.ErrorKind = "refused"
		out.ErrorText = "连接被拒绝：目标端口没有服务在监听"
	case strings.Contains(low, "i/o timeout"), strings.Contains(low, "timed out"):
		out.ErrorKind = "timeout"
		out.ErrorText = "连接超时：目标无响应，可能被防火墙丢包"
	case strings.Contains(low, "no route to host"):
		out.ErrorKind = "unreachable"
		out.ErrorText = "无法路由到目标主机"
	case strings.Contains(low, "network is unreachable"):
		out.ErrorKind = "unreachable"
		out.ErrorText = "本机没有到达该地址的网络路由"
	case strings.Contains(low, "no such host"):
		out.ErrorKind = "dns"
		out.ErrorText = "域名解析失败：找不到该主机"
	default:
		out.ErrorKind = "failed"
		out.ErrorText = msg
	}
}

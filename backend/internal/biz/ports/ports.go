package ports

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
)

type PortEntry struct {
	Protocol         string       `json:"protocol"`
	Port             int          `json:"port"`
	Address          string       `json:"address"`
	State            string       `json:"state"`
	PID              *int         `json:"pid"`
	ProcessName      string       `json:"processName"`
	User             string       `json:"user"`
	Command          string       `json:"command"`
	WorkingDirectory string       `json:"workingDirectory"`
	OwnerType        string       `json:"ownerType"`
	OwnerName        string       `json:"ownerName"`
	ScannedAt        string       `json:"scannedAt"`
	Service          *ServiceMeta `json:"service,omitempty"`
}

var (
	ssUsersRe   = regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)
	netstatPidRe = regexp.MustCompile(`(\d+)/([^\s]+)`)
)

func Run(_ *middleware.Context, in input.Input) (any, error) {
	action := strings.ToLower(strings.TrimSpace(in.Str("action")))
	if action == "" { action = "scan" }
	switch action {
	case "scan": return handleScan()
	case "kill": return handleKill(in.Int("pid"))
	case "release": return handleRelease(in.Int("port"), in.Str("proto"), in.Int("pid"))
	default: return nil, fmt.Errorf("unknown action: %s", action)
	}
}

func handleScan() ([]PortEntry, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	if entries, err := scanWithSS(now); err == nil && len(entries) > 0 { enrichEntries(entries); return entries, nil }
	if entries, err := scanWithNetstat(now); err == nil && len(entries) > 0 { enrichEntries(entries); return entries, nil }
	if entries, err := scanWithLsof(now); err == nil && len(entries) > 0 { enrichEntries(entries); return entries, nil }
	if entries, err := scanWithProc(now); err == nil && len(entries) > 0 { enrichEntries(entries); return entries, nil }
	return []PortEntry{}, nil
}

func isSystemCoreService(name, cmd string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	if lowerName == "ga_main" || lowerName == "gm_main" || lowerName == "sshd" || lowerName == "gm-agent" {
		return true
	}
	// 精确匹配命令行中的二进制文件，避免包含 GMSSH- 前缀的应用容器被误伤
	for _, f := range strings.Fields(cmd) {
		base := strings.ToLower(f)
		if idx := strings.LastIndex(base, "/"); idx >= 0 {
			base = base[idx+1:]
		}
		if base == "ga_main" || base == "gm_main" || base == "sshd" || base == "gm-agent" {
			return true
		}
	}
	return false
}

func handleKill(pid int) (map[string]any, error) {
	if pid <= 1 { return nil, fmt.Errorf("invalid pid: %d", pid) }
	// Critical Safety Check: Prevent killing sshd, gm_main, ga_main, gmssh or PID 1
	entries, _ := handleScan()
	for _, e := range entries {
		if e.PID != nil && *e.PID == pid {
			if e.Port == 22 || isSystemCoreService(e.ProcessName, e.Command) {
				return nil, fmt.Errorf("安全保护：SSH (22) 与 GMSSH 宿主底层核心服务受保护，禁止终止！")
			}
		}
	}
	proc, err := os.FindProcess(pid)
	if err != nil { return nil, fmt.Errorf("process %d not found: %w", pid, err) }
	_ = proc.Signal(syscall.SIGTERM)
	time.Sleep(200 * time.Millisecond)
	_ = proc.Signal(syscall.SIGKILL)
	return map[string]any{"success": true, "pid": pid, "message": fmt.Sprintf("Process %d terminated", pid)}, nil
}

func handleRelease(port int, proto string, pid int) (map[string]any, error) {
	if port == 22 {
		return nil, fmt.Errorf("安全保护：SSH 端口 (22) 为远程连接核心通道，禁止释放！")
	}
	// Check if target port belongs to GMSSH core services
	entries, _ := handleScan()
	for _, e := range entries {
		if e.Port == port && isSystemCoreService(e.ProcessName, e.Command) {
			return nil, fmt.Errorf("安全保护：GMSSH 宿主服务监听端口受保护，禁止释放！")
		}
	}
	if pid > 1 { return handleKill(pid) }
	if port < 1 || port > 65535 { return nil, fmt.Errorf("invalid port: %d", port) }
	entries, _ = handleScan()
	for _, e := range entries {
		if e.Port == port && (proto == "" || strings.EqualFold(e.Protocol, proto)) {
			if e.PID != nil && *e.PID > 1 { return handleKill(*e.PID) }
		}
	}
	if runtime.GOOS == "linux" {
		_ = exec.Command("fuser", "-k", fmt.Sprintf("%d/%s", port, strings.ToLower(proto))).Run()
	}
	return map[string]any{"success": true, "port": port, "message": fmt.Sprintf("Port %d released", port)}, nil
}

func scanWithSS(scannedAt string) ([]PortEntry, error) {
	cmd := exec.Command("ss", "-H", "-lntup")
	out, err := cmd.Output()
	if err != nil { return nil, err }
	var list []PortEntry
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 4 { continue }
		proto := strings.ToLower(fields[0])
		if !strings.HasPrefix(proto, "tcp") && !strings.HasPrefix(proto, "udp") { continue }
		if strings.HasPrefix(proto, "tcp") { proto = "tcp" } else { proto = "udp" }
		state := fields[1]
		localAddr := fields[3]
		host, port, ok := splitHostPort(localAddr)
		if !ok { continue }
		var pid *int
		procName := ""
		if len(fields) >= 5 {
			tail := strings.Join(fields[4:], " ")
			if m := ssUsersRe.FindStringSubmatch(tail); len(m) == 3 {
				procName = m[1]
				if p, err := strconv.Atoi(m[2]); err == nil { pid = &p }
			}
		}
		list = append(list, PortEntry{Protocol: proto, Port: port, Address: host, State: state, PID: pid, ProcessName: procName, ScannedAt: scannedAt})
	}
	return list, nil
}

func scanWithNetstat(scannedAt string) ([]PortEntry, error) {
	cmd := exec.Command("netstat", "-lntup")
	out, err := cmd.Output()
	if err != nil { return nil, err }
	var list []PortEntry
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 4 { continue }
		proto := strings.ToLower(fields[0])
		if !strings.HasPrefix(proto, "tcp") && !strings.HasPrefix(proto, "udp") { continue }
		if strings.HasPrefix(proto, "tcp") { proto = "tcp" } else { proto = "udp" }
		localAddr := fields[3]
		host, port, ok := splitHostPort(localAddr)
		if !ok { continue }
		state := "LISTEN"
		if proto == "udp" { state = "UNCONN" }
		var pid *int
		procName := ""
		for _, f := range fields[4:] {
			if m := netstatPidRe.FindStringSubmatch(f); len(m) == 3 {
				if p, err := strconv.Atoi(m[1]); err == nil { pid = &p; procName = m[2] }
			}
		}
		list = append(list, PortEntry{Protocol: proto, Port: port, Address: host, State: state, PID: pid, ProcessName: procName, ScannedAt: scannedAt})
	}
	return list, nil
}

func scanWithLsof(scannedAt string) ([]PortEntry, error) {
	cmd := exec.Command("lsof", "-i", "-P", "-n")
	out, err := cmd.Output()
	if err != nil { return nil, err }
	var list []PortEntry
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 8 { continue }
		nodeName := fields[len(fields)-1]
		state := "LISTEN"
		nameField := nodeName
		if nodeName == "(LISTEN)" {
			state = "LISTEN"
			nameField = fields[len(fields)-2]
		} else if strings.EqualFold(fields[4], "IPv4") || strings.EqualFold(fields[4], "IPv6") {
			if !strings.Contains(fields[len(fields)-1], "->") { state = "UNCONN" } else { continue }
		}
		proto := strings.ToLower(fields[7])
		if !strings.HasPrefix(proto, "tcp") && !strings.HasPrefix(proto, "udp") { continue }
		if strings.HasPrefix(proto, "tcp") {
			proto = "tcp"
			if nodeName != "(LISTEN)" { continue }
		} else { proto = "udp" }
		host, port, ok := splitHostPort(nameField)
		if !ok { continue }
		var pid *int
		if p, err := strconv.Atoi(fields[1]); err == nil { pid = &p }
		list = append(list, PortEntry{Protocol: proto, Port: port, Address: host, State: state, PID: pid, ProcessName: fields[0], User: fields[2], ScannedAt: scannedAt})
	}
	return list, nil
}

func scanWithProc(scannedAt string) ([]PortEntry, error) {
	var list []PortEntry
	for _, p := range []string{"tcp", "tcp6", "udp", "udp6"} {
		b, err := os.ReadFile("/proc/net/" + p)
		if err != nil { continue }
		proto := "tcp"
		if strings.HasPrefix(p, "udp") { proto = "udp" }
		scanner := bufio.NewScanner(bytes.NewReader(b))
		for scanner.Scan() {
			fields := strings.Fields(strings.TrimSpace(scanner.Text()))
			if len(fields) < 4 || fields[0] == "sl" { continue }
			if proto == "tcp" && fields[3] != "0A" { continue }
			addrParts := strings.Split(fields[1], ":")
			if len(addrParts) != 2 { continue }
			portHex, err := strconv.ParseInt(addrParts[1], 16, 64)
			if err != nil || portHex < 1 || portHex > 65535 { continue }
			list = append(list, PortEntry{Protocol: proto, Port: int(portHex), Address: "0.0.0.0", State: "LISTEN", ScannedAt: scannedAt})
		}
	}
	return list, nil
}

func enrichEntries(entries []PortEntry) {
	for i := range entries {
		e := &entries[i]
		if e.PID != nil && *e.PID > 0 {
			enrichProcess(e, *e.PID)
		} else if e.ProcessName != "" {
			classifyByName(e)
		} else {
			e.OwnerType = "unknown"
			e.OwnerName = "Unknown Service"
		}
		// Attach multi-tier service metadata
		meta := inspectService(e.ProcessName, e.PID, e.Command, e.Port, e.Protocol, e.WorkingDirectory)
		e.Service = &meta
		if meta.Category == "container" {
			e.OwnerType = "docker"
			if meta.ServiceName != "" { e.OwnerName = meta.ServiceName }
		} else if meta.Category == "system" {
			e.OwnerType = "systemd"
			if meta.ServiceName != "" { e.OwnerName = meta.ServiceName }
		} else if meta.ServiceName != "" && (e.OwnerName == "" || e.OwnerName == "Unknown Service") {
			e.OwnerName = meta.ServiceName
		}
	}
}

func enrichProcess(e *PortEntry, pid int) {
	procDir := fmt.Sprintf("/proc/%d", pid)
	if fi, err := os.Stat(procDir); err == nil {
		if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
			u, err := user.LookupId(strconv.Itoa(int(stat.Uid)))
			if err == nil { e.User = u.Username } else { e.User = strconv.Itoa(int(stat.Uid)) }
		}
	}
	if cwd, err := os.Readlink(procDir + "/cwd"); err == nil { e.WorkingDirectory = cwd }
	if cmdline, err := os.ReadFile(procDir + "/cmdline"); err == nil && len(cmdline) > 0 {
		args := strings.Split(string(cmdline), "\x00")
		var clean []string
		for _, a := range args { if a != "" { clean = append(clean, a) } }
		if len(clean) > 0 {
			e.Command = strings.Join(clean, " ")
			if e.ProcessName == "" { e.ProcessName = filepath.Base(clean[0]) }
		}
	}
	if e.ProcessName == "" {
		if comm, err := os.ReadFile(procDir + "/comm"); err == nil { e.ProcessName = strings.TrimSpace(string(comm)) }
	}
	if cgroup, err := os.ReadFile(procDir + "/cgroup"); err == nil {
		cgText := string(cgroup)
		if strings.Contains(cgText, "docker") || strings.Contains(cgText, "containerd") {
			e.OwnerType = "docker"
			e.OwnerName = extractContainerName(e.ProcessName, cgText)
			return
		}
		if strings.Contains(cgText, "system.slice") {
			e.OwnerType = "systemd"
			e.OwnerName = extractSystemdService(cgText)
			return
		}
	}
	classifyByName(e)
}

func classifyByName(e *PortEntry) {
	name := strings.ToLower(e.ProcessName)
	switch {
	case strings.Contains(name, "docker") || strings.Contains(name, "containerd") || strings.Contains(name, "dockerd"):
		e.OwnerType = "docker"
		e.OwnerName = "Docker Service"
	case strings.Contains(name, "systemd") || strings.Contains(name, "journald") || strings.Contains(name, "sshd"):
		e.OwnerType = "systemd"
		e.OwnerName = e.ProcessName + ".service"
	case strings.Contains(name, "daemon") || strings.HasSuffix(name, "d"):
		e.OwnerType = "process"
		e.OwnerName = e.ProcessName
	default:
		e.OwnerType = "process"
		e.OwnerName = e.ProcessName
	}
}

func extractContainerName(pname, cgroup string) string {
	lines := strings.Split(cgroup, "\n")
	for _, l := range lines {
		if idx := strings.Index(l, "docker/"); idx >= 0 {
			id := l[idx+7:]
			if len(id) > 12 { return "Container (" + id[:12] + ")" }
			return "Container (" + id + ")"
		}
		if idx := strings.Index(l, "docker-"); idx >= 0 {
			id := l[idx+7:]
			if len(id) > 12 { return "Container (" + id[:12] + ")" }
			return "Container (" + id + ")"
		}
	}
	return "Docker: " + pname
}

func extractSystemdService(cgroup string) string {
	lines := strings.Split(cgroup, "\n")
	for _, l := range lines {
		if idx := strings.Index(l, "system.slice/"); idx >= 0 {
			unit := l[idx+13:]
			if slash := strings.Index(unit, "/"); slash >= 0 { unit = unit[:slash] }
			return unit
		}
	}
	return "systemd.service"
}

func splitHostPort(local string) (string, int, bool) {
	local = strings.TrimSpace(local)
	idx := strings.LastIndex(local, ":")
	if idx < 0 { return "", 0, false }
	port, err := strconv.Atoi(local[idx+1:])
	if err != nil || port < 1 || port > 65535 { return "", 0, false }
	host := strings.Trim(local[:idx], "[]")
	if host == "*" || host == "" || host == "::" { host = "0.0.0.0" }
	return host, port, true
}
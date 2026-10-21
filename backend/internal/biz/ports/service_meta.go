package ports

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gmssh-cli-plugin/internal/i18n"
)

// DockerInfo carries real container metadata
type DockerInfo struct {
	ContainerID   string `json:"containerId,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	Image         string `json:"image,omitempty"`
	InternalPort  string `json:"internalPort,omitempty"`
	Cmd           string `json:"cmd,omitempty"`
}

// SystemdInfo carries real systemd properties
type SystemdInfo struct {
	UnitName         string `json:"unitName,omitempty"`
	Description      string `json:"description,omitempty"`
	ActiveState      string `json:"activeState,omitempty"`
	SubState         string `json:"subState,omitempty"`
	MainPID          int    `json:"mainPid,omitempty"`
	ExecStart        string `json:"execStart,omitempty"`
	FragmentPath     string `json:"fragmentPath,omitempty"`
	WorkingDirectory string `json:"workingDirectory,omitempty"`
	MemoryCurrent    string `json:"memoryCurrent,omitempty"`
	CPUUsageNSec     string `json:"cpuUsageNSec,omitempty"`
}

// ServiceMeta defines service category and details
type ServiceMeta struct {
	Category    string       `json:"category"`    // web | db | container | manager | unmanaged | system
	ServiceName string       `json:"serviceName"` // display name, e.g. "Nginx", "MySQL", "gmssh-web"
	Summary     string       `json:"summary"`     // factual description or empty
	Uptime      string       `json:"uptime,omitempty"`
	MemoryRSS   string       `json:"memoryRss,omitempty"`
	Docker      *DockerInfo  `json:"docker,omitempty"`
	Systemd     *SystemdInfo `json:"systemd,omitempty"`
}

// Known services dictionary with accurate, factual summaries (never fabricate for unknown)
var knownSummaries = map[string]struct {
	category string
	name     string
	summary  string
}{
	"nginx":         {"web", "Nginx", "svc.nginx.summary"},
	"httpd":         {"web", "Apache HTTP Server", "svc.apache.summary"},
	"apache2":       {"web", "Apache HTTP Server", "svc.apache.summary"},
	"caddy":         {"web", "Caddy", "svc.caddy.summary"},
	"traefik":       {"web", "Traefik", "svc.traefik.summary"},
	"openresty":     {"web", "OpenResty", "svc.openresty.summary"},

	"mysqld":        {"db", "MySQL", "svc.mysql.summary"},
	"mysql":         {"db", "MySQL", "svc.mysql.summary"},
	"mariadbd":      {"db", "MariaDB", "svc.mariadb.summary"},
	"postgres":      {"db", "PostgreSQL", "svc.postgres.summary"},
	"redis-server":  {"db", "Redis", "svc.redis.summary"},
	"redis":         {"db", "Redis", "svc.redis.summary"},
	"mongod":        {"db", "MongoDB", "svc.mongodb.summary"},
	"elasticsearch": {"db", "Elasticsearch", "svc.elasticsearch.summary"},
	"clickhouse-ser": {"db", "ClickHouse", "svc.clickhouse.summary"},

	"pm2":           {"manager", "PM2", "svc.pm2.summary"},
	"supervisord":   {"manager", "Supervisor", "svc.supervisor.summary"},

	"node":          {"unmanaged", "Node.js", "svc.node.summary"},
	"python":        {"unmanaged", "Python", "svc.python.summary"},
	"python3":       {"unmanaged", "Python 3", "svc.python.summary"},
	"java":          {"unmanaged", "Java", "svc.java.summary"},

	"sshd":          {"system", "OpenSSH SSHD", "svc.sshd.summary"},
	"systemd-resolv": {"system", "systemd-resolved", "svc.systemd_resolved.summary"},
	"chronyd":       {"system", "Chrony", "svc.chrony.summary"},
}

// inspectService resolves category, factual summary, docker metadata, and systemd metadata
func inspectService(pname string, pid *int, cmd string, port int, proto string, cwd string) ServiceMeta {
	lowerName := strings.ToLower(pname)
	lowerCmd := strings.ToLower(cmd)

	meta := ServiceMeta{
		Category:    "unmanaged",
		ServiceName: pname,
	}

	// 1. Check if Docker container (e.g. docker-proxy or container cgroup)
	if strings.Contains(lowerName, "docker") || strings.Contains(lowerCmd, "docker-proxy") {
		meta.Category = "container"
		meta.Summary = i18n.T("svc.docker_proxy.summary")
		dInfo := inspectDocker(port, proto, cmd)
		if dInfo != nil {
			meta.Docker = dInfo
			if dInfo.ContainerName != "" {
				meta.ServiceName = dInfo.ContainerName
			}
		}
		if meta.ServiceName == "" {
			meta.ServiceName = i18n.T("svc.docker_container.name")
		}
	} else if matched, ok := knownSummaries[lowerName]; ok {
		meta.Category = matched.category
		meta.ServiceName = matched.name
		meta.Summary = i18n.T(matched.summary)
		// Smart refinement: if process is python/node/java/ruby but command is serving HTTP, classify as web!
		if meta.Category == "unmanaged" {
			if strings.Contains(lowerCmd, "http.server") || strings.Contains(lowerCmd, "flask") ||
				strings.Contains(lowerCmd, "django") || strings.Contains(lowerCmd, "uvicorn") ||
				strings.Contains(lowerCmd, "gunicorn") || strings.Contains(lowerCmd, "fastapi") ||
				strings.Contains(lowerCmd, "express") || strings.Contains(lowerCmd, "koa") ||
				strings.Contains(lowerCmd, "next") || strings.Contains(lowerCmd, "nuxt") ||
				strings.Contains(lowerCmd, "vite") || strings.Contains(lowerCmd, "webpack") ||
				strings.Contains(lowerCmd, "serve") || strings.Contains(lowerCmd, "tornado") {
				meta.Category = "web"
				siteName := extractSiteName(cmd, cwd, matched.name)
				if siteName != "" {
					meta.ServiceName = siteName
					meta.Summary = i18n.T("svc.web_app.summary", matched.name)
				} else {
					meta.Summary = i18n.T("svc.web_http.summary", matched.name)
				}
			}
		}
	} else {
		// Detect by command keywords
		for k, v := range knownSummaries {
			if strings.Contains(lowerCmd, k) {
				meta.Category = v.category
				meta.ServiceName = v.name
				meta.Summary = i18n.T(v.summary)
				break
			}
		}
		// Also check web server keywords
		if meta.Category == "unmanaged" {
			if strings.Contains(lowerCmd, "http.server") || strings.Contains(lowerCmd, "flask") ||
				strings.Contains(lowerCmd, "django") || strings.Contains(lowerCmd, "uvicorn") ||
				strings.Contains(lowerCmd, "gunicorn") || strings.Contains(lowerCmd, "fastapi") ||
				strings.Contains(lowerCmd, "express") || strings.Contains(lowerCmd, "koa") {
				meta.Category = "web"
				siteName := extractSiteName(cmd, cwd, pname)
				if siteName != "" {
					meta.ServiceName = siteName
					meta.Summary = i18n.T("svc.web_app_generic.summary", pname)
				} else {
					meta.Summary = i18n.T("svc.web_http_generic.summary")
				}
			}
		}
	}

	// 2. Fetch Systemd Info if managed by systemd
	if pid != nil && *pid > 0 {
		meta.Uptime = getProcUptime(*pid)
		meta.MemoryRSS = getProcMemoryRSS(*pid)

		sysInfo := inspectSystemd(*pid)
		if sysInfo != nil {
			meta.Systemd = sysInfo
			if meta.Category == "unmanaged" {
				meta.Category = "system"
			}
		}
	}

	return meta
}

// inspectDocker inspects container by port or command line
func inspectDocker(port int, proto string, cmdline string) *DockerInfo {
	info := &DockerInfo{}

	// Extract internal port or container ID if available in docker-proxy cmd
	// e.g. /usr/bin/docker-proxy -proto tcp -host-ip 0.0.0.0 -host-port 8080 -container-ip 172.17.0.2 -container-port 80
	if strings.Contains(cmdline, "-container-port") {
		parts := strings.Fields(cmdline)
		for i, part := range parts {
			if part == "-container-port" && i+1 < len(parts) {
				info.InternalPort = parts[i+1]
			}
		}
	}

	// Query docker cli: docker ps --format '{{.ID}}	{{.Names}}	{{.Image}}	{{.Ports}}	{{.Command}}'
	out, err := exec.Command("docker", "ps", "--no-trunc", "--format", "{{.ID}}	{{.Names}}	{{.Image}}	{{.Ports}}	{{.Command}}").Output()
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(out))
		portStr := fmt.Sprintf(":%d->", port)
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Split(line, "	")
			if len(fields) >= 4 {
				portsField := fields[3]
				if strings.Contains(portsField, portStr) || (port > 0 && strings.Contains(portsField, fmt.Sprintf(":%d", port))) {
					id := fields[0]
					if len(id) > 12 {
						id = id[:12]
					}
					info.ContainerID = id
					info.ContainerName = fields[1]
					info.Image = fields[2]
					if len(fields) >= 5 {
						info.Cmd = strings.Trim(fields[4], "\"'")
					}
					return info
				}
			}
		}
	}

	if info.ContainerName == "" && info.ContainerID == "" && info.InternalPort == "" {
		return nil
	}
	return info
}

// inspectSystemd queries systemctl for the unit controlling pid
func inspectSystemd(pid int) *SystemdInfo {
	// 1. Get unit name from systemd-cgls or /proc/<pid>/cgroup
	unitName := ""
	cgroup, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err == nil {
		lines := strings.Split(string(cgroup), "\n")
		for _, l := range lines {
			if idx := strings.Index(l, "system.slice/"); idx >= 0 {
				unit := l[idx+13:]
				if slash := strings.Index(unit, "/"); slash >= 0 {
					unit = unit[:slash]
				}
				unitName = unit
				break
			}
		}
	}
	if unitName == "" {
		return nil
	}

	info := &SystemdInfo{UnitName: unitName}

	// 2. Query systemctl show
	// Properties: Description,ActiveState,SubState,MainPID,ExecStart,FragmentPath,WorkingDirectory,MemoryCurrent,CPUUsageNSec
	cmd := exec.Command("systemctl", "show", unitName,
		"-p", "Description",
		"-p", "ActiveState",
		"-p", "SubState",
		"-p", "MainPID",
		"-p", "ExecStart",
		"-p", "FragmentPath",
		"-p", "WorkingDirectory",
		"-p", "MemoryCurrent",
		"-p", "CPUUsageNSec",
	)
	out, err := cmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			eq := strings.Index(line, "=")
			if eq <= 0 {
				continue
			}
			k, v := line[:eq], line[eq+1:]
			switch k {
			case "Description":
				info.Description = v
			case "ActiveState":
				info.ActiveState = v
			case "SubState":
				info.SubState = v
			case "MainPID":
				if p, e := strconv.Atoi(v); e == nil {
					info.MainPID = p
				}
			case "ExecStart":
				info.ExecStart = cleanExecStart(v)
			case "FragmentPath":
				info.FragmentPath = v
			case "WorkingDirectory":
				info.WorkingDirectory = v
			case "MemoryCurrent":
				if v != "[not set]" && v != "18446744073709551615" {
					if bytesVal, err := strconv.ParseInt(v, 10, 64); err == nil && bytesVal > 0 {
						info.MemoryCurrent = formatBytes(bytesVal)
					}
				}
			case "CPUUsageNSec":
				if v != "[not set]" {
					if nsec, err := strconv.ParseInt(v, 10, 64); err == nil && nsec > 0 {
						info.CPUUsageNSec = fmt.Sprintf("%.2fs", float64(nsec)/1e9)
					}
				}
			}
		}
	}

	return info
}

func cleanExecStart(raw string) string {
	// systemd outputs: { path=/usr/sbin/nginx ; argv[]=/usr/sbin/nginx -g daemon on; master_process on; ... }
	if idx := strings.Index(raw, "argv[]="); idx >= 0 {
		tail := raw[idx+7:]
		if semi := strings.Index(tail, " ;"); semi >= 0 {
			return strings.TrimSpace(tail[:semi])
		}
		if closeB := strings.LastIndex(tail, " }"); closeB >= 0 {
			return strings.TrimSpace(tail[:closeB])
		}
		return strings.TrimSpace(tail)
	}
	return raw
}

func getProcUptime(pid int) string {
	// Read /proc/<pid>/stat start_time or stat modification time
	fi, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err != nil {
		return ""
	}
	dur := time.Since(fi.ModTime())
	if dur < time.Minute {
		return i18n.T("svc.uptime.seconds", int(dur.Seconds()))
	}
	if dur < time.Hour {
		return i18n.T("svc.uptime.minutes", int(dur.Minutes()))
	}
	if dur < 24*time.Hour {
		h := int(dur.Hours())
		m := int(dur.Minutes()) % 60
		return i18n.T("svc.uptime.hours_minutes", h, m)
	}
	d := int(dur.Hours()) / 24
	h := int(dur.Hours()) % 24
	return i18n.T("svc.uptime.days_hours", d, h)
}

func getProcMemoryRSS(pid int) string {
	// Read /proc/<pid>/status VmRSS
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					return formatBytes(kb * 1024)
				}
			}
		}
	}
	return ""
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}


// extractSiteName tries to extract real project/site name from cwd or cmdline
func extractSiteName(cmd, cwd, fallback string) string {
	// 1. From cwd: e.g. /home/ubuntu/web9999 -> "web9999"
	cleanCwd := strings.TrimRight(cwd, "/")
	if cleanCwd != "" && cleanCwd != "/root" && cleanCwd != "/home" && !strings.HasPrefix(cleanCwd, "/tmp") && cleanCwd != "/" {
		parts := strings.Split(cleanCwd, "/")
		last := parts[len(parts)-1]
		if last != "" && !strings.EqualFold(last, "ubuntu") && !strings.EqualFold(last, "root") && !strings.EqualFold(last, "home") {
			return last
		}
	}
	// 2. From script in command: e.g. "python3 app.py", "node server.js"
	fields := strings.Fields(cmd)
	for _, f := range fields {
		if strings.HasSuffix(f, ".py") || strings.HasSuffix(f, ".js") || strings.HasSuffix(f, ".ts") || strings.HasSuffix(f, ".jar") {
			base := f
			if idx := strings.LastIndex(base, "/"); idx >= 0 {
				base = base[idx+1:]
			}
			return base
		}
	}
	return ""
}

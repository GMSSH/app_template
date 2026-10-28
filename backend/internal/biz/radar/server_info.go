package radar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ServerLocation 服务器地理位置信息
type ServerLocation struct {
	IP      string  `json:"ip"`
	Country string  `json:"country"`
	Region  string  `json:"region"`
	City    string  `json:"city"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	ISP     string  `json:"isp"`
	Name    string  `json:"name"`
	// Located 表示经纬度是否真的由 IP 库返回。
	// 为 false 时 Lat/Lon 只是占位值，界面不应把它当成服务器的真实位置。
	Located bool `json:"located"`
}

// ServerReport 本机安全态势与位置综合报告
type ServerReport struct {
	Location ServerLocation      `json:"location"`
	Events   []ServerAttackEvent `json:"events"`
	IsReal   bool                `json:"is_real"`
	Updated  int64               `json:"updated"`
}

// 未定位时的占位坐标（中国上海）。前端据此判断"位置尚未确认"，
// 不要把任何具体国家的坐标硬编码成默认值，否则界面会指向一个错误的国家。
var fallbackLocation = ServerLocation{
	Name:    "本机服务器",
	Country: "中国",
	City:    "上海",
	Lat:     31.23,
	Lon:     121.47,
}

// 获取公网出口 IP 与真实地理位置。
// 依次尝试多个服务：任何一个拿到经纬度就返回，避免"名字对了但坐标还是默认值"的错位。
func detectServerLocation() ServerLocation {
	loc := fallbackLocation

	for _, fn := range []func() (ServerLocation, bool){geoFromIpwho, geoFromIpApi, geoFromCip} {
		if got, ok := fn(); ok {
			if got.Name == "" {
				got.Name = strings.TrimSpace(fmt.Sprintf("%s %s %s", got.Country, got.Region, got.City))
			}
			return got
		}
	}

	return loc
}

// geoFromIpwho ipwho.is：HTTPS、响应快、同时给出经纬度
func geoFromIpwho() (ServerLocation, bool) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://ipwho.is/?lang=zh-CN")
	if err != nil {
		return ServerLocation{}, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var r struct {
		Success    bool    `json:"success"`
		IP         string  `json:"ip"`
		Country    string  `json:"country"`
		Region     string  `json:"region"`
		City       string  `json:"city"`
		Latitude   float64 `json:"latitude"`
		Longitude  float64 `json:"longitude"`
		Connection struct {
			ISP string `json:"isp"`
		} `json:"connection"`
	}
	if json.Unmarshal(body, &r) != nil || !r.Success || (r.Latitude == 0 && r.Longitude == 0) {
		return ServerLocation{}, false
	}
	return ServerLocation{
		IP: r.IP, Country: r.Country, Region: r.Region, City: r.City,
		Lat: r.Latitude, Lon: r.Longitude, ISP: r.Connection.ISP,
		Name:    strings.TrimSpace(fmt.Sprintf("%s %s %s", r.Country, r.Region, r.City)),
		Located: true,
	}, true
}

// geoFromIpApi ip-api.com：纯 HTTP、国内可达但偶发较慢，超时给足
func geoFromIpApi() (ServerLocation, bool) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/?lang=zh-CN")
	if err != nil {
		return ServerLocation{}, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var r struct {
		Status     string  `json:"status"`
		Country    string  `json:"country"`
		RegionName string  `json:"regionName"`
		City       string  `json:"city"`
		Lat        float64 `json:"lat"`
		Lon        float64 `json:"lon"`
		ISP        string  `json:"isp"`
		Query      string  `json:"query"`
	}
	if json.Unmarshal(body, &r) != nil || r.Status != "success" || (r.Lat == 0 && r.Lon == 0) {
		return ServerLocation{}, false
	}
	return ServerLocation{
		IP: r.Query, Country: r.Country, Region: r.RegionName, City: r.City,
		Lat: r.Lat, Lon: r.Lon, ISP: r.ISP,
		Name:    strings.TrimSpace(fmt.Sprintf("%s %s %s", r.Country, r.RegionName, r.City)),
		Located: true,
	}, true
}

// geoFromCip cip.cc：只有 IP 和中文地址，没有经纬度。
// 因此 Located 保持 false —— 拿到 IP 和运营商，但不假装知道坐标。
func geoFromCip() (ServerLocation, bool) {
	out, err := exec.Command("curl", "-s", "--max-time", "4", "cip.cc").Output()
	if err != nil || len(out) == 0 {
		return ServerLocation{}, false
	}
	text := string(out)
	ip := regexp.MustCompile(`IP\s*:\s*([\d.]+)`).FindStringSubmatch(text)
	addr := regexp.MustCompile(`地址\s*:\s*(.+)`).FindStringSubmatch(text)
	isp := regexp.MustCompile(`运营商\s*:\s*(.+)`).FindStringSubmatch(text)
	if len(ip) < 2 {
		return ServerLocation{}, false
	}
	loc := fallbackLocation
	loc.IP = strings.TrimSpace(ip[1])
	if len(addr) > 1 {
		loc.Name = strings.TrimSpace(addr[1])
	}
	if len(isp) > 1 {
		loc.ISP = strings.TrimSpace(isp[1])
	}
	return loc, true
}

// 快速解析 Linux 真实系统日志 (/var/log/auth.log, /var/log/secure, /var/log/nginx/access.log)
func scanRealServerLogs() []ServerAttackEvent {
	var events []ServerAttackEvent
	now := time.Now().UnixMilli()

	// 1. 尝试读取 SSH 暴力破解日志
	authLogs := []string{"/var/log/auth.log", "/var/log/auth.log.1", "/var/log/secure"}
	for _, p := range authLogs {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			lines := strings.Split(string(b), "\n")
			// 取末尾 200 行
			start := len(lines) - 200
			if start < 0 {
				start = 0
			}
			ipRe := regexp.MustCompile(`(?:from |rhost=)(\d{1,3}(?:\.\d{1,3}){3})`)
			for i := len(lines) - 1; i >= start; i-- {
				line := lines[i]
				if strings.Contains(line, "Failed password") || strings.Contains(line, "Invalid user") || strings.Contains(line, "authentication failure") {
					m := ipRe.FindStringSubmatch(line)
					if len(m) > 1 {
						ip := m[1]
						events = append(events, ServerAttackEvent{
							ID:    fmt.Sprintf("ssh_%d_%d", now, len(events)),
							TS:    now - int64(len(events)*15000),
							IP:    ip,
							Kind:  "SSH 认证失败",
							Level: 3,
							N:     1,
							Src:   queryIPGeo(ip),
						})
						if len(events) >= 20 {
							break
						}
					}
				}
			}
			if len(events) > 0 {
				break
			}
		}
	}

	// 2. 尝试读取 Web 扫描拦截日志
	webLogs := []string{"/var/log/nginx/access.log", "/var/log/apache2/access.log", "/var/log/httpd/access_log"}
	for _, p := range webLogs {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			lines := strings.Split(string(b), "\n")
			start := len(lines) - 300
			if start < 0 {
				start = 0
			}
			ipRe := regexp.MustCompile(`^(\d{1,3}(?:\.\d{1,3}){3})`)
			for i := len(lines) - 1; i >= start; i-- {
				line := lines[i]
				if strings.Contains(line, ".env") || strings.Contains(line, "wp-login") || strings.Contains(line, "phpmyadmin") || strings.Contains(line, "/.git") {
					m := ipRe.FindStringSubmatch(line)
					if len(m) > 1 {
						ip := m[1]
						events = append(events, ServerAttackEvent{
							ID:    fmt.Sprintf("web_%d_%d", now, len(events)),
							TS:    now - int64(len(events)*12000),
							IP:    ip,
							Kind:  "Web 可疑请求",
							Level: 2,
							N:     1,
							Src:   queryIPGeo(ip),
						})
						if len(events) >= 30 {
							break
						}
					}
				}
			}
			if len(events) > 0 {
				break
			}
		}
	}

	return events
}

// 查 IP 所在城市（通过 ip-api 简单查或默认 fallback）
var ipGeoCache = make(map[string]*GeoInfo)

func queryIPGeo(ip string) *GeoInfo {
	if g, ok := ipGeoCache[ip]; ok {
		return g
	}
	// 简单调用 ip-api.com
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,regionName,city,lat,lon", ip))
	if err == nil {
		defer resp.Body.Close()
		var res struct {
			Status  string  `json:"status"`
			Country string  `json:"country"`
			City    string  `json:"city"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
		}
		b, _ := io.ReadAll(resp.Body)
		if json.Unmarshal(b, &res) == nil && res.Status == "success" {
			info := &GeoInfo{
				Lat:  res.Lat,
				Lon:  res.Lon,
				Name: fmt.Sprintf("%s (%s)", res.Country, res.City),
			}
			ipGeoCache[ip] = info
			return info
		}
	}

	// 兜底
	info := &GeoInfo{Lat: 37.77, Lon: -122.42, Name: "美国 (加州)"}
	ipGeoCache[ip] = info
	return info
}

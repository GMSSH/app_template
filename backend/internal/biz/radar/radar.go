package radar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
)

// ServerAttackEvent 代表本机捕获的攻击记录
type ServerAttackEvent struct {
	ID    string   `json:"id"`
	TS    int64    `json:"ts"`
	IP    string   `json:"ip"`
	Kind  string   `json:"kind"`
	Level int      `json:"level"`
	N     int      `json:"n"`
	Src   *GeoInfo `json:"src,omitempty"`
}

type GeoInfo struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Name string  `json:"name"`
}

// GlobalAttackEvent 代表全球传感器上报记录
type GlobalAttackEvent struct {
	ID      string   `json:"id"`
	TS      int64    `json:"ts"`
	IP      string   `json:"ip"`
	Kind    string   `json:"kind"`
	Level   int      `json:"level"`
	Reports int      `json:"reports"`
	Targets int      `json:"targets"`
	Src     *GeoInfo `json:"src,omitempty"`
}

// HeatCountry 代表国家热区统计项
type HeatCountry struct {
	CC    string  `json:"cc"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type HeatResult struct {
	At     int64         `json:"at"`
	Metric string        `json:"metric"`
	Range  string        `json:"range"`
	List   []HeatCountry `json:"list"`
	Mock   bool          `json:"mock"`
	MS     int64         `json:"ms"`
}

// PathPair 代表攻击源国家到目标国家的路径
type PathPair struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	FromName string  `json:"fromName"`
	ToName   string  `json:"toName"`
	Value    float64 `json:"value"`
}

type PathResult struct {
	At    int64      `json:"at"`
	Range string     `json:"range"`
	List  []PathPair `json:"list"`
	Mock  bool       `json:"mock"`
	MS    int64      `json:"ms"`
}

func getRadarToken() string {
	paths := []string{"/etc/gm-radar/radar.token", "/opt/gm-radar/radar.token"}
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil {
			t := strings.TrimSpace(string(b))
			if t != "" {
				return t
			}
		}
	}
	if envTok := strings.TrimSpace(os.Getenv("RADAR_TOKEN")); envTok != "" {
		return envTok
	}
	// 不内置任何 token：源码会进 Git 仓库，硬编码等于公开泄漏。
	// 未配置时 fetchCloudflareRadar 会返回错误，前端自动回退到模拟热区数据。
	return ""
}

func fetchCloudflareRadar(path string, rng string) ([]byte, int64, error) {
	tok := getRadarToken()
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/radar/attacks/layer3/%s?dateRange=%s&format=JSON", path, rng)
	t0 := time.Now()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	ms := time.Since(t0).Milliseconds()
	if err != nil {
		return nil, ms, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ms, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ms, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return body, ms, nil
}

// Run 统一业务入口：返回 CLI 信封中的 data
func Run(ctx *middleware.Context, in input.Input) (any, error) {
	action := in.Str("action")
	if action == "" {
		action = "server"
	}

	switch action {
	case "server":
		loc := detectServerLocation()
		realEvents := scanRealServerLogs()
		isReal := len(realEvents) > 0
		events := realEvents
		if len(events) == 0 {
			events = getServerMockEvents()
		}
		return ServerReport{
			Location: loc,
			Events:   events,
			IsReal:   isReal,
			Updated:  time.Now().UnixMilli(),
		}, nil
	case "global":
		return getGlobalMockEvents(), nil
	case "heat":
		metric := in.Str("metric")
		if metric == "" {
			metric = "target"
		}
		rng := in.Str("range")
		if rng == "" {
			rng = "3d"
		}

		// 优先尝试 Cloudflare Radar 真实数据
		body, ms, err := fetchCloudflareRadar(fmt.Sprintf("top/locations/%s", metric), rng)
		if err == nil {
			var resp struct {
				Success bool `json:"success"`
				Result  struct {
					Top0 []struct {
						TargetCountryAlpha2 string  `json:"targetCountryAlpha2"`
						TargetCountryName   string  `json:"targetCountryName"`
						OriginCountryAlpha2 string  `json:"originCountryAlpha2"`
						OriginCountryName   string  `json:"originCountryName"`
						Value               float64 `json:"value,string"`
					} `json:"top_0"`
				} `json:"result"`
			}
			if jsonErr := json.Unmarshal(body, &resp); jsonErr == nil && resp.Success && len(resp.Result.Top0) > 0 {
				list := make([]HeatCountry, 0, len(resp.Result.Top0))
				for _, item := range resp.Result.Top0 {
					cc := item.TargetCountryAlpha2
					name := item.TargetCountryName
					if metric == "origin" {
						cc = item.OriginCountryAlpha2
						name = item.OriginCountryName
					}
					if cc != "" {
						list = append(list, HeatCountry{
							CC:    cc,
							Name:  name,
							Value: item.Value,
						})
					}
				}
				if len(list) > 0 {
					return HeatResult{
						At:     time.Now().UnixMilli(),
						Metric: metric,
						Range:  rng,
						List:   list,
						Mock:   false,
						MS:     ms,
					}, nil
				}
			}
		}

		// 失败时降级兜底模拟数据
		return getMockHeat(metric, rng), nil

	case "paths":
		rng := in.Str("range")
		if rng == "" {
			rng = "3d"
		}

		// 优先尝试 Cloudflare Radar 真实数据
		body, ms, err := fetchCloudflareRadar("top/attacks", rng)
		if err == nil {
			var resp struct {
				Success bool `json:"success"`
				Result  struct {
					Top0 []struct {
						OriginCountryAlpha2 string  `json:"originCountryAlpha2"`
						OriginCountryName   string  `json:"originCountryName"`
						TargetCountryAlpha2 string  `json:"targetCountryAlpha2"`
						TargetCountryName   string  `json:"targetCountryName"`
						Value               float64 `json:"value,string"`
					} `json:"top_0"`
				} `json:"result"`
			}
			if jsonErr := json.Unmarshal(body, &resp); jsonErr == nil && resp.Success && len(resp.Result.Top0) > 0 {
				list := make([]PathPair, 0, len(resp.Result.Top0))
				for _, item := range resp.Result.Top0 {
					if item.OriginCountryAlpha2 != "" && item.TargetCountryAlpha2 != "" {
						list = append(list, PathPair{
							From:     item.OriginCountryAlpha2,
							To:       item.TargetCountryAlpha2,
							FromName: item.OriginCountryName,
							ToName:   item.TargetCountryName,
							Value:    item.Value,
						})
					}
				}
				if len(list) > 0 {
					return PathResult{
						At:    time.Now().UnixMilli(),
						Range: rng,
						List:  list,
						Mock:  false,
						MS:    ms,
					}, nil
				}
			}
		}

		// 失败时降级兜底模拟数据
		return getMockPaths(rng), nil

	default:
		return getServerMockEvents(), nil
	}
}

func getServerMockEvents() []ServerAttackEvent {
	now := time.Now().UnixMilli()
	return []ServerAttackEvent{
		{
			ID:    "s_1",
			TS:    now - 12000,
			IP:    "198.51.100.23",
			Kind:  "SSH 认证失败",
			Level: 3,
			N:     64,
			Src:   &GeoInfo{Lat: 37.77, Lon: -122.42, Name: "美国 (旧金山)"},
		},
		{
			ID:    "s_2",
			TS:    now - 28000,
			IP:    "203.0.113.88",
			Kind:  "Web 可疑请求",
			Level: 2,
			N:     18,
			Src:   &GeoInfo{Lat: 52.52, Lon: 13.40, Name: "德国 (柏林)"},
		},
		{
			ID:    "s_3",
			TS:    now - 45000,
			IP:    "198.51.100.104",
			Kind:  "PHP 漏洞探测",
			Level: 3,
			N:     52,
			Src:   &GeoInfo{Lat: 55.75, Lon: 37.62, Name: "俄罗斯 (莫斯科)"},
		},
		{
			ID:    "s_4",
			TS:    now - 90000,
			IP:    "192.0.2.14",
			Kind:  "Fail2ban 封禁",
			Level: 3,
			N:     80,
			Src:   &GeoInfo{Lat: -23.55, Lon: -46.63, Name: "巴西 (圣保罗)"},
		},
		{
			ID:    "s_5",
			TS:    now - 140000,
			IP:    "203.0.113.19",
			Kind:  "SSH 认证失败",
			Level: 1,
			N:     5,
			Src:   &GeoInfo{Lat: 28.61, Lon: 77.20, Name: "印度 (新德里)"},
		},
	}
}

func getGlobalMockEvents() []GlobalAttackEvent {
	now := time.Now().UnixMilli()
	return []GlobalAttackEvent{
		{
			ID:      "g_1",
			TS:      now - 5000,
			IP:      "198.51.100.23",
			Kind:    "DShield 来源统计",
			Level:   3,
			Reports: 1420,
			Targets: 85,
			Src:     &GeoInfo{Lat: 37.77, Lon: -122.42, Name: "United States"},
		},
		{
			ID:      "g_2",
			TS:      now - 15000,
			IP:      "203.0.113.88",
			Kind:    "DShield 来源统计",
			Level:   3,
			Reports: 1180,
			Targets: 64,
			Src:     &GeoInfo{Lat: 52.52, Lon: 13.40, Name: "Germany"},
		},
		{
			ID:      "g_3",
			TS:      now - 30000,
			IP:      "198.51.100.104",
			Kind:    "DShield 来源统计",
			Level:   2,
			Reports: 890,
			Targets: 41,
			Src:     &GeoInfo{Lat: 55.75, Lon: 37.62, Name: "Russia"},
		},
		{
			ID:      "g_4",
			TS:      now - 60000,
			IP:      "192.0.2.14",
			Kind:    "DShield 来源统计",
			Level:   2,
			Reports: 720,
			Targets: 33,
			Src:     &GeoInfo{Lat: -23.55, Lon: -46.63, Name: "Brazil"},
		},
		{
			ID:      "g_5",
			TS:      now - 120000,
			IP:      "203.0.113.19",
			Kind:    "DShield 来源统计",
			Level:   1,
			Reports: 360,
			Targets: 18,
			Src:     &GeoInfo{Lat: 28.61, Lon: 77.20, Name: "India"},
		},
	}
}

func getMockHeat(metric string, rng string) HeatResult {
	type pair struct {
		name string
		val  float64
	}
	m := map[string]pair{
		"US": {"United States", 18.4},
		"CN": {"China", 15.2},
		"RU": {"Russia", 11.7},
		"NL": {"Netherlands", 7.9},
		"BR": {"Brazil", 6.1},
		"IN": {"India", 5.4},
		"DE": {"Germany", 4.8},
		"VN": {"Vietnam", 3.9},
		"IR": {"Iran", 3.2},
		"UA": {"Ukraine", 2.7},
		"ID": {"Indonesia", 2.4},
		"TR": {"Turkey", 2.1},
		"SG": {"Singapore", 1.8},
		"KR": {"Korea", 1.5},
		"GB": {"United Kingdom", 1.3},
		"FR": {"France", 1.2},
		"JP": {"Japan", 1.1},
		"CA": {"Canada", 1.0},
		"AU": {"Australia", 0.9},
	}

	list := make([]HeatCountry, 0, len(m))
	for cc, p := range m {
		list = append(list, HeatCountry{
			CC:    cc,
			Name:  p.name,
			Value: p.val,
		})
	}

	return HeatResult{
		At:     time.Now().UnixMilli(),
		Metric: metric,
		Range:  rng,
		List:   list,
		Mock:   true,
		MS:     12,
	}
}

func getMockPaths(rng string) PathResult {
	type rawPath struct {
		from string
		to   string
		val  float64
	}
	raw := []rawPath{
		{"CN", "US", 6.1},
		{"RU", "UA", 5.4},
		{"US", "CN", 4.7},
		{"NL", "DE", 3.6},
		{"BR", "US", 3.1},
		{"IR", "IL", 2.8},
		{"IN", "US", 2.4},
		{"VN", "SG", 2.0},
		{"RU", "US", 1.9},
		{"TR", "DE", 1.6},
		{"US", "GB", 1.5},
		{"KR", "JP", 1.3},
		{"FR", "BE", 1.1},
		{"ID", "AU", 0.9},
		{"ZA", "NG", 0.7},
	}

	list := make([]PathPair, 0, len(raw))
	for _, p := range raw {
		list = append(list, PathPair{
			From:     p.from,
			To:       p.to,
			FromName: p.from,
			ToName:   p.to,
			Value:    p.val,
		})
	}

	return PathResult{
		At:    time.Now().UnixMilli(),
		Range: rng,
		List:  list,
		Mock:  true,
		MS:    8,
	}
}

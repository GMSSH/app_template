package inspector

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"gmssh-cli-plugin/internal/biz/domain/security"
	"gmssh-cli-plugin/internal/biz/domain/types"
)

type HopItem struct {
	HopIndex   int    `json:"hopIndex"`
	URL        string `json:"url"`
	StatusCode int    `json:"statusCode"`
	StatusText string `json:"statusText"`
	Location   string `json:"location,omitempty"`
	DurationMs int64  `json:"durationMs"`
}

type RedirectData struct {
	InitialURL string    `json:"initialUrl"`
	FinalURL   string    `json:"finalUrl"`
	HopCount   int       `json:"hopCount"`
	Hops       []HopItem `json:"hops"`
	HasLoop    bool      `json:"hasLoop"`
	Summary    string    `json:"summary"`
}

func InspectRedirect(ctx context.Context, domain string) (*types.ModuleEnvelope, error) {
	start := time.Now()
	envelope := &types.ModuleEnvelope{
		Module:    "redirect",
		Domain:    domain,
		Status:    types.StatusSuccess,
		StartedAt: start,
		Source:    "HTTP Redirect Chain Follower",
	}

	startURL := "http://" + domain
	currURL := startURL
	visited := make(map[string]bool)

	var hops []HopItem
	maxHops := 10
	hasLoop := false

	transport := &http.Transport{
		DialContext:         security.SafeDialContext(5 * time.Second),
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Do not follow automatically; we follow step by step manually
			return http.ErrUseLastResponse
		},
	}

	for i := 0; i < maxHops; i++ {
		if visited[currURL] {
			hasLoop = true
			break
		}
		visited[currURL] = true

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, currURL, nil)
		if err != nil {
			break
		}
		req.Header.Set("User-Agent", "GMSSH-DomainRedirect/1.0")

		stepStart := time.Now()
		resp, err := client.Do(req)
		stepMs := time.Since(stepStart).Milliseconds()

		if err != nil {
			// Try GET if HEAD fails
			req.Method = http.MethodGet
			resp, err = client.Do(req)
		}

		if err != nil {
			hops = append(hops, HopItem{
				HopIndex:   i + 1,
				URL:        currURL,
				StatusCode: 0,
				StatusText: fmt.Sprintf("连接失败: %v", err),
				DurationMs: stepMs,
			})
			break
		}
		_ = resp.Body.Close()

		loc := resp.Header.Get("Location")
		hop := HopItem{
			HopIndex:   i + 1,
			URL:        currURL,
			StatusCode: resp.StatusCode,
			StatusText: resp.Status,
			Location:   loc,
			DurationMs: stepMs,
		}
		hops = append(hops, hop)

		// Check if redirect status
		if resp.StatusCode >= 300 && resp.StatusCode < 400 && loc != "" {
			// Parse relative location
			parsedBase, err := url.Parse(currURL)
			if err == nil {
				parsedLoc, err := url.Parse(loc)
				if err == nil {
					currURL = parsedBase.ResolveReference(parsedLoc).String()
					continue
				}
			}
			currURL = loc
		} else {
			// Terminal response reached
			break
		}
	}

	finalURL := currURL
	if len(hops) > 0 {
		finalURL = hops[len(hops)-1].URL
	}

	summary := "直接访问（无跳转）"
	if len(hops) > 1 {
		summary = fmt.Sprintf("经过 %d 次跳转到达最终地址", len(hops)-1)
	}
	if hasLoop {
		summary = "检测到重定向循环（Redirect Loop）"
		envelope.Status = types.StatusWarning
	}

	envelope.Data = &RedirectData{
		InitialURL: startURL,
		FinalURL:   finalURL,
		HopCount:   len(hops),
		Hops:       hops,
		HasLoop:    hasLoop,
		Summary:    summary,
	}

	envelope.FinishedAt = time.Now()
	envelope.DurationMs = time.Since(start).Milliseconds()
	return envelope, nil
}

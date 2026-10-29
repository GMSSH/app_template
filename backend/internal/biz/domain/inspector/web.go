package inspector

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"time"

	"gmssh-cli-plugin/internal/biz/domain/security"
	"gmssh-cli-plugin/internal/biz/domain/types"
)

type LatencyDetail struct {
	DNSLookupMs    int64 `json:"dnsLookupMs"`
	TCPConnectMs   int64 `json:"tcpConnectMs"`
	TLSHandshakeMs int64 `json:"tlsHandshakeMs"`
	TTFBMs         int64 `json:"ttfbMs"`
	TotalMs        int64 `json:"totalMs"`
}

type WebData struct {
	URL         string        `json:"url"`
	FinalURL    string        `json:"finalUrl"`
	StatusCode  int           `json:"statusCode"`
	StatusText  string        `json:"statusText"`
	Proto       string        `json:"proto"`
	Server      string        `json:"server,omitempty"`
	ContentType string        `json:"contentType,omitempty"`
	Latency     LatencyDetail `json:"latency"`
	ResolvedIP  string        `json:"resolvedIP"`
}

func InspectWeb(ctx context.Context, domain string) (*types.ModuleEnvelope, error) {
	start := time.Now()
	envelope := &types.ModuleEnvelope{
		Module:    "web",
		Domain:    domain,
		Status:    types.StatusSuccess,
		StartedAt: start,
		Source:    "HTTPS / HTTP Client Probe",
	}

	targetURL := "https://" + domain
	res, err := probeHTTP(ctx, targetURL, domain)
	if err != nil {
		// Fallback to plain HTTP if HTTPS fails
		httpTarget := "http://" + domain
		resHTTP, errHTTP := probeHTTP(ctx, httpTarget, domain)
		if errHTTP == nil {
			res = resHTTP
			err = nil
		}
	}

	if err != nil {
		envelope.Status = types.StatusError
		envelope.Error = fmt.Sprintf("web probe failed: %v", err)
		envelope.FinishedAt = time.Now()
		envelope.DurationMs = time.Since(start).Milliseconds()
		return envelope, nil
	}

	if res.StatusCode >= 400 && res.StatusCode < 500 {
		envelope.Status = types.StatusWarning
	} else if res.StatusCode >= 500 {
		envelope.Status = types.StatusError
	}

	envelope.Data = res
	envelope.FinishedAt = time.Now()
	envelope.DurationMs = time.Since(start).Milliseconds()
	return envelope, nil
}

func probeHTTP(ctx context.Context, targetURL, domain string) (*WebData, error) {
	var dnsStart, dnsDone, connStart, connDone, tlsStart, tlsDone, firstByte time.Time
	var resolvedIP string

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dnsDone = time.Now()
			for _, ip := range info.Addrs {
				if security.IsPublicIP(ip.IP) {
					resolvedIP = ip.IP.String()
					break
				}
			}
		},
		ConnectStart: func(_, _ string) { connStart = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			connDone = time.Now()
			if host, _, e := net.SplitHostPort(addr); e == nil && resolvedIP == "" {
				resolvedIP = host
			}
		},
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			tlsDone = time.Now()
		},
		GotFirstResponseByte: func() { firstByte = time.Now() },
	}

	transport := &http.Transport{
		DialContext:         security.SafeDialContext(5 * time.Second),
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{
			ServerName: domain,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) GMSSH-DomainInspector/1.0")
	req.Header.Set("Accept", "*/*")

	overallStart := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Drain a small part of body
	_, _ = io.CopyN(io.Discard, resp.Body, 8192)

	totalMs := time.Since(overallStart).Milliseconds()

	latency := LatencyDetail{
		TotalMs: totalMs,
	}
	if !dnsStart.IsZero() && !dnsDone.IsZero() {
		latency.DNSLookupMs = dnsDone.Sub(dnsStart).Milliseconds()
	}
	if !connStart.IsZero() && !connDone.IsZero() {
		latency.TCPConnectMs = connDone.Sub(connStart).Milliseconds()
	}
	if !tlsStart.IsZero() && !tlsDone.IsZero() {
		latency.TLSHandshakeMs = tlsDone.Sub(tlsStart).Milliseconds()
	}
	if !overallStart.IsZero() && !firstByte.IsZero() {
		latency.TTFBMs = firstByte.Sub(overallStart).Milliseconds()
	}

	return &WebData{
		URL:         targetURL,
		FinalURL:    resp.Request.URL.String(),
		StatusCode:  resp.StatusCode,
		StatusText:  resp.Status,
		Proto:       resp.Proto,
		Server:      resp.Header.Get("Server"),
		ContentType: resp.Header.Get("Content-Type"),
		Latency:     latency,
		ResolvedIP:  resolvedIP,
	}, nil
}

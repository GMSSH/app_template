package inspector

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"gmssh-cli-plugin/internal/biz/domain/security"
	"gmssh-cli-plugin/internal/biz/domain/types"
)

type SSLData struct {
	Subject        string   `json:"subject"`
	Issuer         string   `json:"issuer"`
	ValidFrom      string   `json:"validFrom"`
	ValidTo        string   `json:"validTo"`
	RemainingDays  int      `json:"remainingDays"`
	DNSNames       []string `json:"dnsNames"`
	SerialNumber   string   `json:"serialNumber"`
	SignatureAlg   string   `json:"signatureAlg"`
	CipherSuite    string   `json:"cipherSuite"`
	TLSVersion     string   `json:"tlsVersion"`
	ChainLength    int      `json:"chainLength"`
	ChainIssuers   []string `json:"chainIssuers"`
	Verified       bool     `json:"verified"`
	VerifyError    string   `json:"verifyError,omitempty"`
	StatusLabel    string   `json:"statusLabel"` // 正常, 即将过期, 已过期, 不受信任, 域名不匹配
	RawCertificate string   `json:"rawCertificate,omitempty"`
}

func InspectSSL(ctx context.Context, domain string) (*types.ModuleEnvelope, error) {
	start := time.Now()
	envelope := &types.ModuleEnvelope{
		Module:    "ssl",
		Domain:    domain,
		Status:    types.StatusSuccess,
		StartedAt: start,
		Source:    "TLS 1.2/1.3 Handshake Probe",
	}

	dialer := security.SafeDialContext(5 * time.Second)
	rawConn, err := dialer(ctx, "tcp", domain+":443")
	if err != nil {
		envelope.Status = types.StatusError
		envelope.Error = fmt.Sprintf("failed to connect to %s:443: %v", domain, err)
		envelope.FinishedAt = time.Now()
		envelope.DurationMs = time.Since(start).Milliseconds()
		return envelope, nil
	}
	defer rawConn.Close()

	tlsConfig := &tls.Config{
		ServerName: domain,
		// We set InsecureSkipVerify to inspect the cert even if invalid, but verify manually
		InsecureSkipVerify: true,
	}

	tlsConn := tls.Client(rawConn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		envelope.Status = types.StatusError
		envelope.Error = fmt.Sprintf("TLS handshake failed: %v", err)
		envelope.FinishedAt = time.Now()
		envelope.DurationMs = time.Since(start).Milliseconds()
		return envelope, nil
	}
	defer tlsConn.Close()

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		envelope.Status = types.StatusError
		envelope.Error = "no peer certificate provided by server"
		envelope.FinishedAt = time.Now()
		envelope.DurationMs = time.Since(start).Milliseconds()
		return envelope, nil
	}

	cert := state.PeerCertificates[0]

	now := time.Now()
	remainingDays := int(cert.NotAfter.Sub(now).Hours() / 24)

	// Manual verification
	opts := x509.VerifyOptions{
		DNSName:       domain,
		Intermediates: x509.NewCertPool(),
	}
	for _, c := range state.PeerCertificates[1:] {
		opts.Intermediates.AddCert(c)
	}

	verified := true
	var verifyErrStr string
	if _, err := cert.Verify(opts); err != nil {
		verified = false
		verifyErrStr = err.Error()
	}

	// Status label
	statusLabel := "正常"
	envelope.Status = types.StatusSuccess

	if now.After(cert.NotAfter) {
		statusLabel = "已过期"
		envelope.Status = types.StatusError
	} else if now.Before(cert.NotBefore) {
		statusLabel = "尚未生效"
		envelope.Status = types.StatusError
	} else if !verified {
		if strings.Contains(strings.ToLower(verifyErrStr), "name") {
			statusLabel = "域名不匹配"
			envelope.Status = types.StatusWarning
		} else {
			statusLabel = "证书不可信"
			envelope.Status = types.StatusWarning
		}
	} else if remainingDays <= 30 {
		statusLabel = "即将过期"
		envelope.Status = types.StatusWarning
	}

	var chainIssuers []string
	for _, c := range state.PeerCertificates {
		chainIssuers = append(chainIssuers, c.Subject.CommonName)
	}

	rawPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}))

	data := &SSLData{
		Subject:        cert.Subject.CommonName,
		Issuer:         cert.Issuer.CommonName,
		ValidFrom:      cert.NotBefore.Format("2006-01-02 15:04:05"),
		ValidTo:        cert.NotAfter.Format("2006-01-02 15:04:05"),
		RemainingDays:  remainingDays,
		DNSNames:       cert.DNSNames,
		SerialNumber:   cert.SerialNumber.String(),
		SignatureAlg:   cert.SignatureAlgorithm.String(),
		CipherSuite:    tls.CipherSuiteName(state.CipherSuite),
		TLSVersion:     tlsVersionToString(state.Version),
		ChainLength:    len(state.PeerCertificates),
		ChainIssuers:   chainIssuers,
		Verified:       verified,
		VerifyError:    verifyErrStr,
		StatusLabel:    statusLabel,
		RawCertificate: rawPEM,
	}

	if data.Issuer == "" {
		data.Issuer = cert.Issuer.Organization[0]
	}
	if data.Subject == "" && len(data.DNSNames) > 0 {
		data.Subject = data.DNSNames[0]
	}

	envelope.Data = data
	envelope.FinishedAt = time.Now()
	envelope.DurationMs = time.Since(start).Milliseconds()
	return envelope, nil
}

func tlsVersionToString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}

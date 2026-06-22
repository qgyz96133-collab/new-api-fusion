package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	utls "github.com/refraction-networking/utls"
)

var rotatingFingerprints = []utls.ClientHelloID{
	utls.HelloChrome_Auto,
	utls.HelloFirefox_Auto,
	utls.HelloSafari_Auto,
	utls.HelloEdge_Auto,
}

var rotatingFingerprintNames = []string{
	"Chrome", "Firefox", "Safari", "Edge",
}

var fingerprintIndex uint64

func GetNextFingerprint() (utls.ClientHelloID, string) {
	idx := atomic.AddUint64(&fingerprintIndex, 1) - 1
	i := int(idx) % len(rotatingFingerprints)
	return rotatingFingerprints[i], rotatingFingerprintNames[i]
}

func NewRotatingUTLSTransport(connectTimeout, idleTimeout time.Duration) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: 30 * time.Second,
	}

	dialTLSFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}

		rawConn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, fmt.Errorf("dial: %w", err)
		}

		helloID, name := GetNextFingerprint()
		_ = name

		// Use http/1.1 only in ALPN to prevent HTTP/2 negotiation
		tlsConn := utls.UClient(rawConn, &utls.Config{
			ServerName: host,
			NextProtos: []string{"http/1.1"},
		}, helloID)

		if err := tlsConn.HandshakeContext(ctx); err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("utls handshake (%s): %w", name, err)
		}

		return tlsConn, nil
	}

	return &http.Transport{
		DialTLSContext:        dialTLSFunc,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       idleTimeout,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     false,
	}
}

func NewRotatingUTLSHttpClient(userAgent string, timeout time.Duration) *http.Client {
	transport := NewRotatingUTLSTransport(30*time.Second, 120*time.Second)

	return &http.Client{
		Transport: &headerInjectTransport{
			base: transport,
			headers: map[string]string{
				"User-Agent":       userAgent,
				"X-Amz-User-Agent": userAgent,
			},
		},
		Timeout: timeout,
	}
}

type headerInjectTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerInjectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

// suppress unused import warning
var _ = tls.VersionTLS12

package common

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
)

// UTLSTransport provides an HTTP transport with Chrome TLS fingerprint
// Ported from AIClient2API's tls-sidecar/main.go
// Uses uTLS to emulate Chrome's TLS fingerprint for anti-detection

// FingerprintType represents the browser fingerprint to use
type FingerprintType int

const (
	FingerprintChrome    FingerprintType = iota
	FingerprintFirefox
	FingerprintSafari
	FingerprintEdge
)

var fingerprintMap = map[FingerprintType]utls.ClientHelloID{
	FingerprintChrome:  utls.HelloChrome_Auto,
	FingerprintFirefox: utls.HelloFirefox_Auto,
	FingerprintSafari:  utls.HelloSafari_Auto,
	FingerprintEdge:    utls.HelloEdge_Auto,
}

// UTransportConfig holds uTLS transport configuration
type UTransportConfig struct {
	Fingerprint    FingerprintType
	ProxyURL       string
	ConnectTimeout time.Duration
	IdleTimeout    time.Duration
}

// DefaultUTransportConfig returns sensible defaults
func DefaultUTransportConfig() UTransportConfig {
	return UTransportConfig{
		Fingerprint:    FingerprintChrome,
		ConnectTimeout: 30 * time.Second,
		IdleTimeout:    120 * time.Second,
	}
}

// NewUTLSTransport creates an HTTP transport with uTLS fingerprinting
func NewUTLSTransport(cfg UTransportConfig) *http.Transport {
	helloID, ok := fingerprintMap[cfg.Fingerprint]
	if !ok {
		helloID = utls.HelloChrome_Auto
	}

	dialer := &net.Dialer{
		Timeout:   cfg.ConnectTimeout,
		KeepAlive: 30 * time.Second,
	}

	// Create the base dial function with optional proxy
	var proxyDialer proxyDialFunc
	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err == nil {
			proxyDialer = createProxyDialer(proxyURL, dialer)
		}
	}

	dialTLSFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Extract host from addr
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}

		var rawConn net.Conn
		if proxyDialer != nil {
			rawConn, err = proxyDialer(ctx, network, addr)
		} else {
			rawConn, err = dialer.DialContext(ctx, network, addr)
		}
		if err != nil {
			return nil, fmt.Errorf("dial: %w", err)
		}

		// Wrap with uTLS
		tlsConn := utls.UClient(rawConn, &utls.Config{
			ServerName: host,
		}, helloID)

		if err := tlsConn.HandshakeContext(ctx); err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("utls handshake: %w", err)
		}

		return tlsConn, nil
	}

	return &http.Transport{
		DialTLSContext:        dialTLSFunc,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       cfg.IdleTimeout,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}

// NewStandardTransport creates a standard HTTP transport without uTLS
// (fallback for when uTLS is not needed)
func NewStandardTransport(cfg UTransportConfig) *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   cfg.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       cfg.IdleTimeout,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}

// Transport cache for reuse
var (
	utlsTransportCache   map[string]*http.Transport
	utlsTransportCacheMu sync.Mutex
)

// GetUTLSTransport returns a cached uTLS transport for the given config
func GetUTLSTransport(cfg UTransportConfig) *http.Transport {
	key := fmt.Sprintf("%d:%s", cfg.Fingerprint, cfg.ProxyURL)

	utlsTransportCacheMu.Lock()
	defer utlsTransportCacheMu.Unlock()

	if utlsTransportCache == nil {
		utlsTransportCache = make(map[string]*http.Transport)
	}

	if t, ok := utlsTransportCache[key]; ok {
		return t
	}

	t := NewUTLSTransport(cfg)
	utlsTransportCache[key] = t
	return t
}

// proxyDialFunc is the signature for proxy dial functions
type proxyDialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// createProxyDialer creates a dial function that routes through a proxy
func createProxyDialer(proxyURL *url.URL, dialer *net.Dialer) proxyDialFunc {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		proxyAddr := proxyURL.Host
		if proxyURL.Scheme == "http" || proxyURL.Scheme == "https" {
			// HTTP CONNECT proxy
			conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
			if err != nil {
				return nil, err
			}

			// Send CONNECT request
			connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)
			if proxyURL.User != nil {
				auth := proxyURL.User.String()
				connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth)
			}
			connectReq += "\r\n"

			if _, err := conn.Write([]byte(connectReq)); err != nil {
				conn.Close()
				return nil, err
			}

			// Read response (simple check)
			buf := make([]byte, 12)
			if _, err := conn.Read(buf); err != nil {
				conn.Close()
				return nil, fmt.Errorf("proxy connect failed")
			}

			return conn, nil
		}

		// Direct connection (no proxy)
		return dialer.DialContext(ctx, network, addr)
	}
}

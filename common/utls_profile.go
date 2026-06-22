package common

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	utls "github.com/refraction-networking/utls"
)

// TLSProfile contains custom TLS fingerprint configuration.
// Ported from sub2api's pkg/tlsfingerprint/dialer.go Profile struct.
type TLSProfile struct {
	Name                string   `json:"name"`
	CipherSuites        string   `json:"cipher_suites"`       // JSON []uint16
	Curves              string   `json:"curves"`               // JSON []uint16
	SignatureAlgorithms string   `json:"signature_algorithms"` // JSON []uint16
	ALPNProtocols       string   `json:"alpn_protocols"`      // JSON []string
	EnableGREASE        bool     `json:"enable_grease"`
	Extensions          string   `json:"extensions"` // JSON []uint16 extension IDs
}

// --- Preset profiles for common use cases ---

// PresetNodeJS mimics Node.js 24.x TLS fingerprint (for Claude Code compatibility)
var PresetNodeJS = &TLSProfile{
	Name:         "nodejs-24",
	EnableGREASE: true,
}

// PresetChrome131 mimics Chrome 131 TLS fingerprint
var PresetChrome131 = &TLSProfile{
	Name:         "chrome-131",
	EnableGREASE: true,
}

// PresetClaudeCode mimics Claude Code client specifically
var PresetClaudeCode = &TLSProfile{
	Name:         "claude-code",
	EnableGREASE: true,
}

// Default TLS fingerprint values from Node.js 24.x / Claude Code
var (
	DefaultNodeJSCipherSuites = []uint16{
		0x1301, 0x1302, 0x1303, // TLS 1.3
		0xc02b, 0xc02f, 0xc02c, 0xc030, // ECDHE + AES-GCM
		0xcca9, 0xcca8, // ECDHE + ChaCha20
		0xc009, 0xc013, 0xc00a, 0xc014, // ECDHE + AES-CBC
		0x009c, 0x009d, // RSA + AES-GCM
		0x002f, 0x0035, // RSA + AES-CBC
	}

	DefaultNodeJSCurves = []utls.CurveID{
		utls.X25519,
		utls.CurveP256,
		utls.CurveP384,
	}

	DefaultNodeJSSignatureAlgorithms = []utls.SignatureScheme{
		0x0403, // ecdsa_secp256r1_sha256
		0x0804, // rsa_pss_rsae_sha256
		0x0401, // rsa_pkcs1_sha256
		0x0503, // ecdsa_secp384r1_sha384
		0x0805, // rsa_pss_rsae_sha384
		0x0501, // rsa_pkcs1_sha384
		0x0806, // rsa_pss_rsae_sha512
		0x0601, // rsa_pkcs1_sha512
		0x0201, // rsa_pkcs1_sha1
	}

	DefaultExtensionOrder = []uint16{
		0,     // server_name
		5,     // status_request
		10,    // supported_groups
		11,    // ec_point_formats
		13,    // signature_algorithms
		16,    // alpn
		18,    // signed_certificate_timestamp
		23,    // extended_master_secret
		35,    // session_ticket
		43,    // supported_versions
		45,    // psk_key_exchange_modes
		50,    // signature_algorithms_cert
		51,    // key_share
		0xfe0d, // encrypted_client_hello
		0xff01, // renegotiation_info
	}
)

// BuildClientHelloSpec creates a uTLS ClientHelloSpec from a TLSProfile
func BuildClientHelloSpec(profile *TLSProfile) *utls.ClientHelloSpec {
	cipherSuites := DefaultNodeJSCipherSuites
	curves := DefaultNodeJSCurves
	pointFormats := []uint8{0} // uncompressed
	sigAlgorithms := DefaultNodeJSSignatureAlgorithms
	alpnProtocols := []string{"http/1.1"}
	supportedVersions := []uint16{utls.VersionTLS13, utls.VersionTLS12}
	keyShareGroups := []utls.CurveID{utls.X25519}
	enableGREASE := false
	extOrder := DefaultExtensionOrder

	if profile != nil {
		enableGREASE = profile.EnableGREASE
	}

	// Build extensions
	extensions := make([]utls.TLSExtension, 0, len(extOrder)+2)

	if enableGREASE {
		extensions = append(extensions, &utls.UtlsGREASEExtension{})
	}

	for _, id := range extOrder {
		switch id {
		case 0:
			extensions = append(extensions, &utls.SNIExtension{})
		case 5:
			extensions = append(extensions, &utls.StatusRequestExtension{})
		case 10:
			extensions = append(extensions, &utls.SupportedCurvesExtension{Curves: curves})
		case 11:
			extensions = append(extensions, &utls.SupportedPointsExtension{SupportedPoints: pointFormats})
		case 13:
			extensions = append(extensions, &utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: sigAlgorithms})
		case 16:
			extensions = append(extensions, &utls.ALPNExtension{AlpnProtocols: alpnProtocols})
		case 18:
			extensions = append(extensions, &utls.SCTExtension{})
		case 23:
			extensions = append(extensions, &utls.ExtendedMasterSecretExtension{})
		case 35:
			extensions = append(extensions, &utls.SessionTicketExtension{})
		case 43:
			extensions = append(extensions, &utls.SupportedVersionsExtension{Versions: supportedVersions})
		case 45:
			extensions = append(extensions, &utls.PSKKeyExchangeModesExtension{Modes: []uint8{uint8(utls.PskModeDHE)}})
		case 50:
			extensions = append(extensions, &utls.SignatureAlgorithmsCertExtension{SupportedSignatureAlgorithms: sigAlgorithms})
		case 51:
			keyShares := make([]utls.KeyShare, len(keyShareGroups))
			for i, g := range keyShareGroups {
				keyShares[i] = utls.KeyShare{Group: g}
			}
			extensions = append(extensions, &utls.KeyShareExtension{KeyShares: keyShares})
		case 0xfe0d:
			extensions = append(extensions, &utls.GREASEEncryptedClientHelloExtension{})
		case 0xff01:
			extensions = append(extensions, &utls.RenegotiationInfoExtension{})
		default:
			extensions = append(extensions, &utls.GenericExtension{Id: id})
		}
	}

	if enableGREASE {
		extensions = append(extensions, &utls.UtlsGREASEExtension{})
	}

	return &utls.ClientHelloSpec{
		CipherSuites:       cipherSuites,
		CompressionMethods: []uint8{0},
		Extensions:         extensions,
		TLSVersMax:         utls.VersionTLS13,
		TLSVersMin:         utls.VersionTLS10,
	}
}

// ProfileTransport creates an HTTP transport with a custom TLS profile
func ProfileTransport(profile *TLSProfile, proxyURL string, connectTimeout, idleTimeout time.Duration) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: 30 * time.Second,
	}

	var proxyDial func(ctx context.Context, network, addr string) (net.Conn, error)
	if proxyURL != "" {
		pURL, err := url.Parse(proxyURL)
		if err == nil {
			proxyDial = createProxyDialer(pURL, dialer)
		}
	}

	spec := BuildClientHelloSpec(profile)

	dialTLSFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}

		var rawConn net.Conn
		if proxyDial != nil {
			rawConn, err = proxyDial(ctx, network, addr)
		} else {
			rawConn, err = dialer.DialContext(ctx, network, addr)
		}
		if err != nil {
			return nil, fmt.Errorf("dial: %w", err)
		}

		tlsConn := utls.UClient(rawConn, &utls.Config{
			ServerName: host,
		}, utls.HelloCustom)

		if err := tlsConn.ApplyPreset(spec); err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("apply preset: %w", err)
		}

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
		IdleConnTimeout:       idleTimeout,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}

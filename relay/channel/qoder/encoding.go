package qoder

import (
	"encoding/base64"
)

// Qoder WAF-bypass body encoding
// Ported from 9router/src/lib/qoder/encoding.js
//
// Algorithm:
// 1. base64-encode the plaintext bytes (standard alphabet)
// 2. Rearrange: split into thirds, reorder as [tail][mid][head]
// 3. Substitute each character via a custom alphabet mapping

const (
	qoderStdAlphabet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	qoderCustomAlphabet = "_doRTgHZBKcGVjlvpC,@aFSx#DPuNJme&i*MzLOEn)sUrthbf%Y^w.(kIQyXqWA!"
)

// qoderS2C is the standard-to-custom substitution table
var qoderS2C [128]byte

func init() {
	for i := range qoderS2C {
		qoderS2C[i] = byte(i) // identity for non-alphabet chars
	}
	for i := 0; i < 64; i++ {
		qoderS2C[qoderStdAlphabet[i]] = qoderCustomAlphabet[i]
	}
	qoderS2C['='] = '$'
}

// QoderEncodeBody encodes plaintext bytes using Qoder's WAF-bypass scheme.
func QoderEncodeBody(plaintext []byte) string {
	std := base64.StdEncoding.EncodeToString(plaintext)
	n := len(std)
	if n == 0 {
		return ""
	}
	a := n / 3

	// Rearrange: [tail][mid][head]
	rearranged := std[n-a:] + std[a:n-a] + std[:a]

	out := make([]byte, n)
	for i := 0; i < n; i++ {
		c := rearranged[i]
		if c < 128 {
			out[i] = qoderS2C[c]
		} else {
			out[i] = c
		}
	}
	return string(out)
}

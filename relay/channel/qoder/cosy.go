package qoder

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Qoder COSY signing constants (from 9router/src/lib/qoder/constants.js)
const (
	QoderIDEVersion   = "1.0.0"
	QoderClientType   = "5"
	QoderDataPolicy   = "disagree"
	QoderLoginVersion = "v2"
	QoderMachineOS    = "x86_64_linux"
	QoderMachineType  = "5"

	QoderChatURL = "https://api3.qoder.sh/algo/api/v2/service/pro/sse/agent_chat_generation?FetchKeys=llm_model_result&AgentId=agent_common"
	QoderChatURLEncoded = QoderChatURL + "&Encode=1"
	QoderModelListURL = "https://api3.qoder.sh/algo/api/v2/model/list"

	// RSA public key for COSY encryption
	QoderRSAPublicKey = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDA8iMH5c02LilrsERw9t6Pv5Nc
4k6Pz1EaDicBMpdpxKduSZu5OANqUq8er4GM95omAGIOPOh+Nx0spthYA2BqGz+l
6HRkPJ7S236FZz73In/KVuLnwI8JJ2CbuJap8kvheCCZpmAWpb/cPx/3Vr/J6I17
XcW+ML9FoCI6AOvOzwIDAQAB
-----END PUBLIC KEY-----`
)

// Creds holds Qoder authentication credentials
type Creds struct {
	UserID    string `json:"user_id"`
	AuthToken string `json:"auth_token"` // dt-... token
	Name      string `json:"name"`
	Email     string `json:"email"`
	MachineID string `json:"machine_id"`
}

// ParseCredsFromKey parses the channel key field as JSON credentials
// Format: {"user_id":"...","auth_token":"dt-...","name":"...","email":"...","machine_id":"..."}
// Or simple: just the dt-... token string (requires user_id to be set separately)
func ParseCredsFromKey(key string) (*Creds, error) {
	// Try JSON format first
	var creds Creds
	if err := json.Unmarshal([]byte(key), &creds); err == nil && creds.AuthToken != "" {
		if creds.MachineID == "" {
			creds.MachineID = uuid.New().String()
		}
		return &creds, nil
	}

	// Fallback: treat as raw token (user_id must be stored in channel's Other field)
	if strings.HasPrefix(key, "dt-") {
		return &Creds{
			AuthToken: key,
			MachineID: uuid.New().String(),
		}, nil
	}

	return nil, fmt.Errorf("invalid Qoder credentials: expected JSON or dt-... token")
}

// BuildCosyHeaders generates COSY-signed headers for a Qoder API request
func BuildCosyHeaders(body []byte, requestURL string, creds *Creds) (map[string]string, error) {
	if creds.UserID == "" {
		return nil, fmt.Errorf("cosy: user_id is empty")
	}
	if creds.AuthToken == "" {
		return nil, fmt.Errorf("cosy: auth_token is empty")
	}

	// Encrypt user info
	cosyKey, info, err := encryptUserInfo(creds)
	if err != nil {
		return nil, fmt.Errorf("cosy: encrypt user info: %w", err)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	requestID := uuid.New().String()

	// Build payload
	payloadMap := map[string]string{
		"version":    "v1",
		"requestId":  requestID,
		"info":       info,
		"cosyVersion": QoderIDEVersion,
		"ideVersion": "",
	}
	payloadJSON, _ := json.Marshal(payloadMap)
	payloadB64 := base64.StdEncoding.EncodeToString(payloadJSON)

	// Compute signature
	sigPath := computeSigPath(requestURL)
	sigInput := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", payloadB64, cosyKey, timestamp, string(body), sigPath)
	sig := md5Hex([]byte(sigInput))

	machineID := creds.MachineID
	if machineID == "" {
		machineID = uuid.New().String()
	}

	bodyHash := md5Hex(body)
	bodyLength := strconv.Itoa(len(body))

	return map[string]string{
		"Authorization":          fmt.Sprintf("Bearer COSY.%s.%s", payloadB64, sig),
		"Content-Type":           "application/json",
		"Cosy-Key":              cosyKey,
		"Cosy-User":             creds.UserID,
		"Cosy-Date":             timestamp,
		"Cosy-Version":          QoderIDEVersion,
		"Cosy-Machineid":        machineID,
		"Cosy-Machinetoken":     machineID,
		"Cosy-Machinetype":      QoderMachineType,
		"Cosy-Machineos":        QoderMachineOS,
		"Cosy-Clienttype":       QoderClientType,
		"Cosy-Clientip":         "127.0.0.1",
		"Cosy-Bodyhash":         bodyHash,
		"Cosy-Bodylength":       bodyLength,
		"Cosy-Sigpath":          sigPath,
		"Cosy-Data-Policy":      QoderDataPolicy,
		"Cosy-Organization-Id":  "",
		"Cosy-Organization-Tags": "",
		"Login-Version":         QoderLoginVersion,
		"X-Request-Id":          uuid.New().String(),
		"User-Agent":            "Go-http-client/2.0",
		"Accept":                "text/event-stream",
	}, nil
}

func encryptUserInfo(creds *Creds) (cosyKey, info string, err error) {
	// Generate AES key (first 16 chars of UUID)
	aesKey := uuid.New().String()[:16]

	// Build user info JSON
	userInfo := map[string]string{
		"uid":                  creds.UserID,
		"security_oauth_token": creds.AuthToken,
		"name":                 creds.Name,
		"aid":                  "",
		"email":                creds.Email,
	}
	plaintext, err := json.Marshal(userInfo)
	if err != nil {
		return "", "", err
	}

	// AES-128-CBC encrypt
	infoB64, err := aesEncryptCBCBase64(plaintext, []byte(aesKey))
	if err != nil {
		return "", "", err
	}

	// RSA encrypt the AES key
	cosyKeyB64, err := rsaEncryptBase64([]byte(aesKey))
	if err != nil {
		return "", "", err
	}

	return cosyKeyB64, infoB64, nil
}

func aesEncryptCBCBase64(plaintext, key []byte) (string, error) {
	if len(key) != 16 {
		return "", fmt.Errorf("AES key must be 16 bytes, got %d", len(key))
	}

	// IV = key bytes (matches qodercli behavior)
	iv := key[:16]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// PKCS7 padding
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	encrypted := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encrypted, padded)

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func rsaEncryptBase64(data []byte) (string, error) {
	block, _ := pem.Decode([]byte(QoderRSAPublicKey))
	if block == nil {
		return "", fmt.Errorf("failed to parse RSA public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("not an RSA public key")
	}

	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, data)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func md5Hex(data []byte) string {
	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash)
}

func computeSigPath(requestURL string) string {
	u, err := url.Parse(requestURL)
	if err != nil {
		return ""
	}
	path := u.Path
	if strings.HasPrefix(path, "/algo") {
		return path[len("/algo"):]
	}
	return path
}

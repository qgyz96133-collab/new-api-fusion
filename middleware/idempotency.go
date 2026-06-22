package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// IdempotencyRecord stores a cached response for replay
type IdempotencyRecord struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	CreatedAt  time.Time
	TTL        time.Duration
}

// IsExpired checks if the record has expired
func (r *IdempotencyRecord) IsExpired() bool {
	return time.Since(r.CreatedAt) > r.TTL
}

// IdempotencyStore is an in-memory store for idempotency records
type IdempotencyStore struct {
	mu      sync.RWMutex
	records map[string]*IdempotencyRecord
}

var globalIdempotencyStore = &IdempotencyStore{
	records: make(map[string]*IdempotencyRecord),
}

// Get retrieves a record by key
func (s *IdempotencyStore) Get(key string) (*IdempotencyRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.records[key]
	if !ok || rec.IsExpired() {
		return nil, false
	}
	return rec, true
}

// Set stores a record
func (s *IdempotencyStore) Set(key string, rec *IdempotencyRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[key] = rec
}

// Cleanup removes expired records
func (s *IdempotencyStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, rec := range s.records {
		if rec.IsExpired() {
			delete(s.records, key)
		}
	}
}

func init() {
	// Periodic cleanup every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			globalIdempotencyStore.Cleanup()
		}
	}()
}

// Idempotency middleware prevents duplicate charges by replaying cached responses
// for requests with the same Idempotency-Key header
func Idempotency() gin.HandlerFunc {
	return func(c *gin.Context) {
		idempotencyKey := c.GetHeader("Idempotency-Key")
		if idempotencyKey == "" {
			c.Next()
			return
		}

		// Build cache key from idempotency key + method + path
		cacheKey := buildIdempotencyCacheKey(c, idempotencyKey)

		// Check for cached response
		if rec, ok := globalIdempotencyStore.Get(cacheKey); ok {
			// Replay the cached response
			common.SysLog(fmt.Sprintf("[Idempotency] replay: key=%s method=%s path=%s",
				idempotencyKey[:min(16, len(idempotencyKey))], c.Request.Method, c.Request.URL.Path))

			c.Header("X-Idempotency-Replayed", "true")
			for k, v := range rec.Headers {
				c.Header(k, v)
			}
			c.Data(rec.StatusCode, "application/json", rec.Body)
			c.Abort()
			return
		}

		// Wrap the response writer to capture the response
		wrapper := &responseCapture{
			ResponseWriter: c.Writer,
			headers:        make(map[string]string),
			statusCode:     http.StatusOK,
		}
		c.Writer = wrapper

		c.Next()

		// Cache successful responses (2xx)
		if wrapper.statusCode >= 200 && wrapper.statusCode < 300 {
			globalIdempotencyStore.Set(cacheKey, &IdempotencyRecord{
				StatusCode: wrapper.statusCode,
				Headers:    wrapper.headers,
				Body:       wrapper.body.Bytes(),
				CreatedAt:  time.Now(),
				TTL:        24 * time.Hour, // 24 hour TTL
			})
			common.SysLog(fmt.Sprintf("[Idempotency] cached: key=%s status=%d",
				idempotencyKey[:min(16, len(idempotencyKey))], wrapper.statusCode))
		}
	}
}

// responseCapture wraps gin.ResponseWriter to capture the response
type responseCapture struct {
	gin.ResponseWriter
	body       bytes.Buffer
	headers    map[string]string
	statusCode int
}

func (r *responseCapture) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseCapture) Write(data []byte) (int, error) {
	r.body.Write(data)
	// Capture important headers
	for _, key := range []string{"Content-Type", "X-Request-Id"} {
		if v := r.ResponseWriter.Header().Get(key); v != "" {
			r.headers[key] = v
		}
	}
	return r.ResponseWriter.Write(data)
}

func (r *responseCapture) WriteJSON(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	r.body.Write(data)
	r.ResponseWriter.Header().Set("Content-Type", "application/json")
	r.headers["Content-Type"] = "application/json"
	_, err = r.ResponseWriter.Write(data)
	return err
}

func buildIdempotencyCacheKey(c *gin.Context, idempotencyKey string) string {
	raw := c.Request.Method + ":" + c.Request.URL.Path + ":" + idempotencyKey
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// IdempotencyStatus returns the current store size (for monitoring)
func IdempotencyStatus() map[string]interface{} {
	globalIdempotencyStore.mu.RLock()
	defer globalIdempotencyStore.mu.RUnlock()
	return map[string]interface{}{
		"cached_records": len(globalIdempotencyStore.records),
	}
}

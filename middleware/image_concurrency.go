package middleware

import (
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// ImageConcurrencyLimiter limits concurrent image generation requests
// Ported from sub2api's image_concurrency_limiter.go

var (
	activeImageRequests int64
	maxImageConcurrency int64 = 5 // default
	imageConcurrencyMu  sync.Mutex
)

// SetMaxImageConcurrency configures the max concurrent image requests
func SetMaxImageConcurrency(max int) {
	imageConcurrencyMu.Lock()
	defer imageConcurrencyMu.Unlock()
	maxImageConcurrency = int64(max)
}

// GetImageConcurrencyStatus returns current concurrency info
func GetImageConcurrencyStatus() (int64, int64) {
	return atomic.LoadInt64(&activeImageRequests), maxImageConcurrency
}

// ImageConcurrencyLimit middleware
func ImageConcurrencyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		current := atomic.AddInt64(&activeImageRequests, 1)
		if current > maxImageConcurrency {
			atomic.AddInt64(&activeImageRequests, -1)
			common.SysLog("[ImageConcurrency] limit reached, rejecting request")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"message": "Too many concurrent image generation requests. Please try again later.",
					"type":    "rate_limit_error",
					"code":    "image_concurrency_limit",
				},
			})
			c.Abort()
			return
		}

		defer atomic.AddInt64(&activeImageRequests, -1)
		c.Next()
	}
}

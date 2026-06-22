package controller

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ConsoleLogSSE streams gateway logs to admin dashboard in real-time
// Ported from 9router's /translator/console-logs/stream

var (
	logSubscribers   = make(map[string]chan string)
	logSubscribersMu sync.RWMutex
	logBuffer        []string
	logBufferMu      sync.RWMutex
	maxLogBuffer     = 500
)

// PublishConsoleLog broadcasts a log line to all SSE subscribers
func PublishConsoleLog(line string) {
	timestamped := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), line)

	logBufferMu.Lock()
	logBuffer = append(logBuffer, timestamped)
	if len(logBuffer) > maxLogBuffer {
		logBuffer = logBuffer[len(logBuffer)-maxLogBuffer:]
	}
	logBufferMu.Unlock()

	logSubscribersMu.RLock()
	defer logSubscribersMu.RUnlock()
	for _, ch := range logSubscribers {
		select {
		case ch <- timestamped:
		default: // skip slow subscribers
		}
	}
}

// ConsoleLogStream SSE endpoint
func ConsoleLogStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	id := fmt.Sprintf("sub-%d", time.Now().UnixNano())
	ch := make(chan string, 64)

	logSubscribersMu.Lock()
	logSubscribers[id] = ch
	logSubscribersMu.Unlock()

	defer func() {
		logSubscribersMu.Lock()
		delete(logSubscribers, id)
		logSubscribersMu.Unlock()
		close(ch)
	}()

	// Send buffer first
	logBufferMu.RLock()
	for _, line := range logBuffer {
		fmt.Fprintf(c.Writer, "data: %s\n\n", line)
	}
	c.Writer.Flush()
	logBufferMu.RUnlock()

	// Stream new logs
	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case line := <-ch:
			_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", line)
			if err != nil {
				return
			}
			c.Writer.(interface{ Flush() }).Flush()
		}
	}
}

// GetConsoleLogBuffer returns recent log buffer
func GetConsoleLogBuffer(c *gin.Context) {
	logBufferMu.RLock()
	lines := make([]string, len(logBuffer))
	copy(lines, logBuffer)
	logBufferMu.RUnlock()

	c.JSON(200, gin.H{"success": true, "data": lines})
}

// ensure io is used
var _ = io.Discard

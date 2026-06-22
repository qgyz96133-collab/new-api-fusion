package common

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LogWriterMu protects concurrent access to gin.DefaultWriter/gin.DefaultErrorWriter
// during log file rotation. Acquire RLock when reading/writing through the writers,
// acquire Lock when swapping writers and closing old files.
var LogWriterMu sync.RWMutex

// ConsoleLogPublisher is an optional callback to publish logs to the Ops Dashboard SSE stream.
// It is set by the controller package to avoid a circular import.
var ConsoleLogPublisher func(line string)

func SysLog(s string) {
	t := time.Now()
	formatted := fmt.Sprintf("[SYS] %v | %s", t.Format("2006/01/02 - 15:04:05"), s)
	LogWriterMu.RLock()
	_, _ = fmt.Fprintf(gin.DefaultWriter, "%s \n", formatted)
	LogWriterMu.RUnlock()
	if ConsoleLogPublisher != nil {
		ConsoleLogPublisher(formatted)
	}
}

func SysError(s string) {
	t := time.Now()
	formatted := fmt.Sprintf("[SYS] %v | %s", t.Format("2006/01/02 - 15:04:05"), s)
	LogWriterMu.RLock()
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "%s \n", formatted)
	LogWriterMu.RUnlock()
	if ConsoleLogPublisher != nil {
		ConsoleLogPublisher(formatted)
	}
}

func FatalLog(v ...any) {
	t := time.Now()
	LogWriterMu.RLock()
	_, _ = fmt.Fprintf(gin.DefaultErrorWriter, "[FATAL] %v | %v \n", t.Format("2006/01/02 - 15:04:05"), v)
	LogWriterMu.RUnlock()
	os.Exit(1)
}

func LogStartupSuccess(startTime time.Time, port string) {
	duration := time.Since(startTime)
	durationMs := duration.Milliseconds()

	// Get network IPs
	networkIps := GetNetworkIps()

	LogWriterMu.RLock()
	defer LogWriterMu.RUnlock()

	fmt.Fprintf(gin.DefaultWriter, "\n")
	fmt.Fprintf(gin.DefaultWriter, "  \033[32m%s %s\033[0m  ready in %d ms\n", SystemName, Version, durationMs)
	fmt.Fprintf(gin.DefaultWriter, "\n")

	if !IsRunningInContainer() {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mLocal:\033[0m   http://localhost:%s/\n", port)
	}

	for _, ip := range networkIps {
		fmt.Fprintf(gin.DefaultWriter, "  ➜  \033[1mNetwork:\033[0m http://%s:%s/\n", ip, port)
	}

	fmt.Fprintf(gin.DefaultWriter, "\n")
}

//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func main() {
	dbPath := "data/new-api.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// 查找包含 HTML 的配置项
	rows, err := db.Query("SELECT key, value FROM options WHERE value LIKE '<%'")
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}
	defer rows.Close()

	var htmlOptions []struct {
		Key   string
		Value string
	}

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		htmlOptions = append(htmlOptions, struct {
			Key   string
			Value string
		}{key, value})
		fmt.Printf("Found HTML in key '%s':\n%s\n\n", key, value[:min(200, len(value))])
	}

	if len(htmlOptions) == 0 {
		fmt.Println("No HTML content found in options table")
		return
	}

	fmt.Printf("\nFound %d options with HTML content\n", len(htmlOptions))
	fmt.Println("Cleaning up...")

	// 清理这些记录
	for _, opt := range htmlOptions {
		// 根据 key 的类型决定如何处理
		var newValue string
		if strings.Contains(opt.Key, "Ratio") || strings.Contains(opt.Key, "Method") {
			newValue = "{}" // JSON 对象
		} else if strings.Contains(opt.Key, "Enabled") {
			newValue = "false" // 布尔值
		} else if strings.Contains(opt.Key, "Count") || strings.Contains(opt.Key, "Limit") {
			newValue = "0" // 数字
		} else {
			newValue = "" // 空字符串
		}

		_, err := db.Exec("UPDATE options SET value = ? WHERE key = ?", newValue, opt.Key)
		if err != nil {
			log.Printf("Failed to update key '%s': %v", opt.Key, err)
		} else {
			fmt.Printf("✓ Cleaned: %s -> '%s'\n", opt.Key, newValue)
		}
	}

	fmt.Println("\nCleanup completed!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

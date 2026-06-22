package model

import (
	"encoding/json"
	"time"
)

// ChannelMonitor defines a scheduled channel health check.
// Ported from sub2api's channel_monitor schema (simplified for GORM).
type ChannelMonitor struct {
	Id              int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string    `json:"name" gorm:"type:varchar(100);not null"`
	ChannelId       int       `json:"channel_id" gorm:"index;not null"`
	Models          string    `json:"models" gorm:"type:text"`           // JSON []string - models to test
	TestModel       string    `json:"test_model" gorm:"type:varchar(100)"` // primary model for display
	Enabled         bool      `json:"enabled" gorm:"default:true"`
	CheckInterval   int       `json:"check_interval" gorm:"default:300"`  // seconds between checks
	LastCheckedAt   *time.Time `json:"last_checked_at"`
	APIKeyEncrypted string    `json:"-" gorm:"type:text"`                 // encrypted API key for testing
	Note            string    `json:"note" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (ChannelMonitor) TableName() string {
	return "channel_monitors"
}

// ParseModels parses the JSON models field
func (m *ChannelMonitor) ParseModels() []string {
	if m.Models == "" {
		return nil
	}
	var models []string
	json.Unmarshal([]byte(m.Models), &models)
	return models
}

// SetModels sets the models field from a slice
func (m *ChannelMonitor) SetModels(models []string) {
	data, _ := json.Marshal(models)
	m.Models = string(data)
}

// ChannelMonitorHistory records individual check results
type ChannelMonitorHistory struct {
	Id        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	MonitorId int64     `json:"monitor_id" gorm:"index:idx_monitor_model;not null"`
	Model     string    `json:"model" gorm:"type:varchar(100);index:idx_monitor_model"`
	Success   bool      `json:"success"`
	LatencyMs int       `json:"latency_ms"`
	StatusCode int      `json:"status_code"`
	Message   string    `json:"message" gorm:"type:text"`
	CheckedAt time.Time `json:"checked_at" gorm:"index"`
}

func (ChannelMonitorHistory) TableName() string {
	return "channel_monitor_histories"
}

// --- CRUD ---

func ListChannelMonitors() ([]ChannelMonitor, error) {
	var monitors []ChannelMonitor
	err := DB.Order("id ASC").Find(&monitors).Error
	return monitors, err
}

func GetChannelMonitor(id int64) (*ChannelMonitor, error) {
	var monitor ChannelMonitor
	err := DB.First(&monitor, id).Error
	return &monitor, err
}

func CreateChannelMonitor(monitor *ChannelMonitor) error {
	return DB.Create(monitor).Error
}

func UpdateChannelMonitor(monitor *ChannelMonitor) error {
	return DB.Save(monitor).Error
}

func DeleteChannelMonitor(id int64) error {
	DB.Where("monitor_id = ?", id).Delete(&ChannelMonitorHistory{})
	return DB.Delete(&ChannelMonitor{}, id).Error
}

func ListEnabledMonitors() ([]ChannelMonitor, error) {
	var monitors []ChannelMonitor
	err := DB.Where("enabled = ?", true).Find(&monitors).Error
	return monitors, err
}

func MarkMonitorChecked(id int64) {
	now := time.Now()
	DB.Model(&ChannelMonitor{}).Where("id = ?", id).Update("last_checked_at", now)
}

func InsertMonitorHistoryBatch(rows []ChannelMonitorHistory) error {
	if len(rows) == 0 {
		return nil
	}
	return DB.Create(&rows).Error
}

func ListMonitorHistory(monitorId int64, model string, limit int) ([]ChannelMonitorHistory, error) {
	var history []ChannelMonitorHistory
	tx := DB.Where("monitor_id = ?", monitorId)
	if model != "" {
		tx = tx.Where("model = ?", model)
	}
	err := tx.Order("checked_at DESC").Limit(limit).Find(&history).Error
	return history, err
}

// DeleteHistoryBefore removes old history entries
func DeleteMonitorHistoryBefore(before time.Time) (int64, error) {
	result := DB.Where("checked_at < ?", before).Delete(&ChannelMonitorHistory{})
	return result.RowsAffected, result.Error
}

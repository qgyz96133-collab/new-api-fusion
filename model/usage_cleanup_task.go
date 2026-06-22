package model

import (
	"context"
	"encoding/json"
	"time"
)

// UsageCleanupTask tracks usage log cleanup tasks.
// Ported from sub2api's usage_cleanup_task schema.
type UsageCleanupTask struct {
	Id          int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	Status      string     `json:"status" gorm:"type:varchar(20);default:'pending';index"` // pending/running/succeeded/failed/canceled
	Filters     string     `json:"filters" gorm:"type:text"`                                // JSON-encoded UsageCleanupFilters
	CreatedBy   int64      `json:"created_by"`
	DeletedRows int64      `json:"deleted_rows" gorm:"default:0"`
	ErrorMsg    *string    `json:"error_msg" gorm:"type:text"`
	CanceledBy  *int64     `json:"canceled_by"`
	CanceledAt  *time.Time `json:"canceled_at"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (UsageCleanupTask) TableName() string {
	return "usage_cleanup_tasks"
}

// UsageCleanupFilters defines the filter conditions for cleanup
type UsageCleanupFilters struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	UserId    *int      `json:"user_id,omitempty"`
	TokenId   *int      `json:"token_id,omitempty"`
	ChannelId *int      `json:"channel_id,omitempty"`
	GroupName *string   `json:"group_name,omitempty"`
	ModelName *string   `json:"model_name,omitempty"`
}

// ParseFilters parses the JSON filters field
func (t *UsageCleanupTask) ParseFilters() (*UsageCleanupFilters, error) {
	if t.Filters == "" {
		return nil, nil
	}
	var f UsageCleanupFilters
	if err := json.Unmarshal([]byte(t.Filters), &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// SetFilters sets the filters field from a struct
func (t *UsageCleanupTask) SetFilters(f UsageCleanupFilters) error {
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	t.Filters = string(data)
	return nil
}

// --- Constants ---

const (
	CleanupStatusPending   = "pending"
	CleanupStatusRunning   = "running"
	CleanupStatusSucceeded = "succeeded"
	CleanupStatusFailed    = "failed"
	CleanupStatusCanceled  = "canceled"
)

// --- CRUD ---

func CreateUsageCleanupTask(task *UsageCleanupTask) error {
	return DB.Create(task).Error
}

func GetUsageCleanupTask(id int64) (*UsageCleanupTask, error) {
	var task UsageCleanupTask
	err := DB.First(&task, id).Error
	return &task, err
}

func ListUsageCleanupTasks(page, pageSize int) ([]UsageCleanupTask, int64, error) {
	var tasks []UsageCleanupTask
	var total int64
	DB.Model(&UsageCleanupTask{}).Count(&total)
	offset := (page - 1) * pageSize
	err := DB.Order("id DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error
	return tasks, total, err
}

func UpdateUsageCleanupTask(task *UsageCleanupTask) error {
	return DB.Save(task).Error
}

func ClaimNextPendingCleanupTask() (*UsageCleanupTask, error) {
	var task UsageCleanupTask
	// Find the oldest pending task, or a stale running task (>30 min)
	staleThreshold := time.Now().Add(-30 * time.Minute)
	err := DB.Where("status = ? OR (status = ? AND updated_at < ?)",
		CleanupStatusPending, CleanupStatusRunning, staleThreshold).
		Order("id ASC").
		First(&task).Error
	if err != nil {
		return nil, err
	}
	// Claim it
	now := time.Now()
	task.Status = CleanupStatusRunning
	task.StartedAt = &now
	DB.Save(&task)
	return &task, nil
}

func CancelUsageCleanupTask(id int64, canceledBy int64) error {
	now := time.Now()
	return DB.Model(&UsageCleanupTask{}).Where("id = ? AND status IN (?, ?)",
		id, CleanupStatusPending, CleanupStatusRunning).
		Updates(map[string]interface{}{
			"status":      CleanupStatusCanceled,
			"canceled_by": canceledBy,
			"canceled_at": now,
			"updated_at":  now,
		}).Error
}

// DeleteLogsByFilters deletes usage logs matching the given filters in batches
func DeleteLogsByFilters(filters *UsageCleanupFilters, batchSize int) (int64, error) {
	tx := LOG_DB.Model(&Log{})

	if !filters.StartTime.IsZero() {
		tx = tx.Where("created_at >= ?", filters.StartTime.Unix())
	}
	if !filters.EndTime.IsZero() {
		tx = tx.Where("created_at <= ?", filters.EndTime.Unix())
	}
	if filters.UserId != nil && *filters.UserId > 0 {
		tx = tx.Where("user_id = ?", *filters.UserId)
	}
	if filters.TokenId != nil && *filters.TokenId > 0 {
		tx = tx.Where("token_id = ?", *filters.TokenId)
	}
	if filters.ChannelId != nil && *filters.ChannelId > 0 {
		tx = tx.Where("channel_id = ?", *filters.ChannelId)
	}
	if filters.GroupName != nil && *filters.GroupName != "" {
		tx = tx.Where(commonGroupCol+" = ?", *filters.GroupName)
	}
	if filters.ModelName != nil && *filters.ModelName != "" {
		tx = tx.Where("model_name = ?", *filters.ModelName)
	}

	// Delete in batches with context cancellation support
	var totalDeleted int64
	ctx := context.Background()
	for {
		// Check cancellation every 10 batches
		if totalDeleted > 0 && totalDeleted%(int64(batchSize)*10) == 0 {
			select {
			case <-ctx.Done():
				return totalDeleted, ctx.Err()
			default:
			}
		}
		result := tx.Limit(batchSize).Delete(&Log{})
		if result.Error != nil {
			return totalDeleted, result.Error
		}
		totalDeleted += result.RowsAffected
		if result.RowsAffected < int64(batchSize) {
			break
		}
	}
	return totalDeleted, nil
}

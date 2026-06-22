package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	cleanupBatchSize       = 5000
	cleanupMaxRangeDays    = 90
	cleanupWorkerInterval  = 30 * time.Second
	cleanupTaskTimeout     = 30 * time.Minute
)

// StartUsageCleanupWorker starts a background goroutine that processes cleanup tasks
func StartUsageCleanupWorker() {
	go func() {
		common.SysLog("[UsageCleanup] worker started")
		ticker := time.NewTicker(cleanupWorkerInterval)
		defer ticker.Stop()

		for range ticker.C {
			processNextCleanupTask()
		}
	}()
}

func processNextCleanupTask() {
	task, err := model.ClaimNextPendingCleanupTask()
	if err != nil {
		return // No pending tasks
	}

	common.SysLog(fmt.Sprintf("[UsageCleanup] processing task %d", task.Id))

	filters, err := task.ParseFilters()
	if err != nil {
		markTaskFailed(task, fmt.Sprintf("invalid filters: %v", err))
		return
	}

	// Validate filters
	if err := validateCleanupFilters(filters); err != nil {
		markTaskFailed(task, err.Error())
		return
	}

	// Execute cleanup with timeout
	done := make(chan struct{})
	var deletedRows int64
	var deleteErr error

	go func() {
		deletedRows, deleteErr = model.DeleteLogsByFilters(filters, cleanupBatchSize)
		close(done)
	}()

	select {
	case <-done:
		if deleteErr != nil {
			markTaskFailed(task, fmt.Sprintf("delete error: %v", deleteErr))
			return
		}
		markTaskSucceeded(task, deletedRows)
	case <-time.After(cleanupTaskTimeout):
		markTaskFailed(task, "task timed out after 30 minutes")
	}
}

func validateCleanupFilters(filters *model.UsageCleanupFilters) error {
	if filters == nil {
		return fmt.Errorf("filters cannot be nil")
	}
	if filters.StartTime.IsZero() || filters.EndTime.IsZero() {
		return fmt.Errorf("start_time and end_time are required")
	}
	if filters.EndTime.Before(filters.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	delta := filters.EndTime.Sub(filters.StartTime)
	if delta > time.Duration(cleanupMaxRangeDays)*24*time.Hour {
		return fmt.Errorf("date range exceeds %d days maximum", cleanupMaxRangeDays)
	}
	return nil
}

func markTaskSucceeded(task *model.UsageCleanupTask, deletedRows int64) {
	now := time.Now()
	task.Status = model.CleanupStatusSucceeded
	task.DeletedRows = deletedRows
	task.FinishedAt = &now
	model.UpdateUsageCleanupTask(task)
	common.SysLog(fmt.Sprintf("[UsageCleanup] task %d succeeded: deleted %d rows", task.Id, deletedRows))
}

func markTaskFailed(task *model.UsageCleanupTask, errMsg string) {
	now := time.Now()
	task.Status = model.CleanupStatusFailed
	task.ErrorMsg = &errMsg
	task.FinishedAt = &now
	model.UpdateUsageCleanupTask(task)
	common.SysLog(fmt.Sprintf("[UsageCleanup] task %d failed: %s", task.Id, errMsg))
}

// CreateCleanupTask validates and creates a new cleanup task
func CreateCleanupTask(filters model.UsageCleanupFilters, createdBy int64) (*model.UsageCleanupTask, error) {
	if err := validateCleanupFilters(&filters); err != nil {
		return nil, err
	}
	task := &model.UsageCleanupTask{
		Status:    model.CleanupStatusPending,
		CreatedBy: createdBy,
	}
	if err := task.SetFilters(filters); err != nil {
		return nil, fmt.Errorf("failed to serialize filters: %v", err)
	}
	if err := model.CreateUsageCleanupTask(task); err != nil {
		return nil, err
	}
	common.SysLog(fmt.Sprintf("[UsageCleanup] task %d created by user %d", task.Id, createdBy))
	return task, nil
}

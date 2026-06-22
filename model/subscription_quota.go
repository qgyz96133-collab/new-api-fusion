package model

import (
	"fmt"
	"sync"
	"time"
)

// SubscriptionQuota tracks per-token daily/weekly/monthly usage limits
type SubscriptionQuota struct {
	ID        int   `json:"id" gorm:"primaryKey;autoIncrement"`
	TokenID   int   `json:"token_id" gorm:"uniqueIndex;not null"`
	UserID    int   `json:"user_id" gorm:"index;not null"`

	// Daily limits
	DailyLimit     int64 `json:"daily_limit" gorm:"default:0"`      // 0 = unlimited
	DailyUsed      int64 `json:"daily_used" gorm:"default:0"`
	DailyResetAt   int64 `json:"daily_reset_at" gorm:"default:0"`   // unix timestamp

	// Weekly limits
	WeeklyLimit    int64 `json:"weekly_limit" gorm:"default:0"`
	WeeklyUsed     int64 `json:"weekly_used" gorm:"default:0"`
	WeeklyResetAt  int64 `json:"weekly_reset_at" gorm:"default:0"`

	// Monthly limits
	MonthlyLimit   int64 `json:"monthly_limit" gorm:"default:0"`
	MonthlyUsed    int64 `json:"monthly_used" gorm:"default:0"`
	MonthlyResetAt int64 `json:"monthly_reset_at" gorm:"default:0"`

	// RPM (requests per minute) limit
	RPMLimit       int   `json:"rpm_limit" gorm:"default:0"`       // 0 = unlimited

	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func (SubscriptionQuota) TableName() string {
	return "subscription_quotas"
}

// In-memory RPM tracking
var (
	rpmCounters   = make(map[int]*rpmCounter) // tokenID -> counter
	rpmCountersMu sync.Mutex
)

type rpmCounter struct {
	count     int
	windowStart time.Time
}

// GetSubscriptionQuota returns the quota record for a token, creating if needed
func GetSubscriptionQuota(tokenID, userID int) (*SubscriptionQuota, error) {
	var quota SubscriptionQuota
	err := DB.Where("token_id = ?", tokenID).First(&quota).Error
	if err != nil {
		// Create default record
		quota = SubscriptionQuota{TokenID: tokenID, UserID: userID}
		if createErr := DB.Create(&quota).Error; createErr != nil {
			return nil, createErr
		}
	}
	return &quota, nil
}

// CheckQuota checks if a request would exceed quota limits
func (q *SubscriptionQuota) CheckQuota(estimatedCost int64) (bool, string) {
	now := time.Now()

	// Auto-reset daily if needed
	if q.DailyResetAt > 0 && now.Unix() >= q.DailyResetAt {
		q.DailyUsed = 0
		q.DailyResetAt = nextDayReset().Unix()
	}

	// Auto-reset weekly if needed
	if q.WeeklyResetAt > 0 && now.Unix() >= q.WeeklyResetAt {
		q.WeeklyUsed = 0
		q.WeeklyResetAt = nextWeekReset().Unix()
	}

	// Auto-reset monthly if needed
	if q.MonthlyResetAt > 0 && now.Unix() >= q.MonthlyResetAt {
		q.MonthlyUsed = 0
		q.MonthlyResetAt = nextMonthReset().Unix()
	}

	// Check daily
	if q.DailyLimit > 0 && q.DailyUsed+estimatedCost > q.DailyLimit {
		return false, fmt.Sprintf("daily limit exceeded: %d/%d", q.DailyUsed, q.DailyLimit)
	}

	// Check weekly
	if q.WeeklyLimit > 0 && q.WeeklyUsed+estimatedCost > q.WeeklyLimit {
		return false, fmt.Sprintf("weekly limit exceeded: %d/%d", q.WeeklyUsed, q.WeeklyLimit)
	}

	// Check monthly
	if q.MonthlyLimit > 0 && q.MonthlyUsed+estimatedCost > q.MonthlyLimit {
		return false, fmt.Sprintf("monthly limit exceeded: %d/%d", q.MonthlyUsed, q.MonthlyLimit)
	}

	return true, ""
}

// RecordUsage records usage after a successful request
func (q *SubscriptionQuota) RecordUsage(cost int64) error {
	q.DailyUsed += cost
	q.WeeklyUsed += cost
	q.MonthlyUsed += cost
	return DB.Model(q).Updates(map[string]interface{}{
		"daily_used":   q.DailyUsed,
		"weekly_used":  q.WeeklyUsed,
		"monthly_used": q.MonthlyUsed,
		"daily_reset_at":   q.DailyResetAt,
		"weekly_reset_at":  q.WeeklyResetAt,
		"monthly_reset_at": q.MonthlyResetAt,
	}).Error
}

// CheckRPM checks if the token has exceeded its RPM limit
func CheckRPM(tokenID, rpmLimit int) bool {
	if rpmLimit <= 0 {
		return true // unlimited
	}

	rpmCountersMu.Lock()
	defer rpmCountersMu.Unlock()

	counter, ok := rpmCounters[tokenID]
	if !ok || time.Since(counter.windowStart) >= time.Minute {
		rpmCounters[tokenID] = &rpmCounter{count: 1, windowStart: time.Now()}
		return true
	}

	counter.count++
	return counter.count <= rpmLimit
}

// SetSubscriptionQuota creates or updates a quota record
func SetSubscriptionQuota(quota *SubscriptionQuota) error {
	return DB.Save(quota).Error
}

// GetSubscriptionQuotasByUser returns all quotas for a user
func GetSubscriptionQuotasByUser(userID int) ([]*SubscriptionQuota, error) {
	var quotas []*SubscriptionQuota
	err := DB.Where("user_id = ?", userID).Find(&quotas).Error
	return quotas, err
}

func nextDayReset() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}

func nextWeekReset() time.Time {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 { weekday = 7 }
	daysUntilMonday := 8 - weekday
	return time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 0, 0, 0, 0, now.Location())
}

func nextMonthReset() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
}

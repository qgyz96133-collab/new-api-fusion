package model

import (
	"time"
)

// WebSearchProvider represents a configured web search provider.
// Ported from sub2api's websearch_config.go (simplified).
type WebSearchProvider struct {
	Id           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Type         string    `json:"type" gorm:"type:varchar(20);not null"` // brave, tavily, searxng
	Name         string    `json:"name" gorm:"type:varchar(100)"`
	APIKey       string    `json:"api_key" gorm:"type:text"`
	BaseURL      string    `json:"base_url" gorm:"type:varchar(255)"`
	QuotaLimit   int64     `json:"quota_limit" gorm:"default:0"` // 0 = unlimited
	QuotaUsed    int64     `json:"quota_used" gorm:"default:0"`
	ProxyId      *int64    `json:"proxy_id"`
	Status       string    `json:"status" gorm:"type:varchar(20);default:'active'"`
	ExpiresAt    *int64    `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (WebSearchProvider) TableName() string {
	return "websearch_providers"
}

func ListWebSearchProviders() ([]WebSearchProvider, error) {
	var providers []WebSearchProvider
	err := DB.Order("id ASC").Find(&providers).Error
	// Mask API keys in response
	for i := range providers {
		if len(providers[i].APIKey) > 8 {
			providers[i].APIKey = providers[i].APIKey[:4] + "****" + providers[i].APIKey[len(providers[i].APIKey)-4:]
		} else if providers[i].APIKey != "" {
			providers[i].APIKey = "****"
		}
	}
	return providers, err
}

func GetWebSearchProvider(id int64) (*WebSearchProvider, error) {
	var provider WebSearchProvider
	err := DB.First(&provider, id).Error
	return &provider, err
}

func CreateWebSearchProvider(provider *WebSearchProvider) error {
	return DB.Create(provider).Error
}

func UpdateWebSearchProvider(provider *WebSearchProvider) error {
	return DB.Save(provider).Error
}

func DeleteWebSearchProvider(id int64) error {
	return DB.Delete(&WebSearchProvider{}, id).Error
}

func GetActiveWebSearchProvider(providerType string) (*WebSearchProvider, error) {
	var provider WebSearchProvider
	err := DB.Where("type = ? AND status = ?", providerType, "active").First(&provider).Error
	if err != nil {
		return nil, err
	}
	// Check quota
	if provider.QuotaLimit > 0 && provider.QuotaUsed >= provider.QuotaLimit {
		return nil, nil // quota exceeded
	}
	return &provider, nil
}

func IncrementWebSearchUsage(id int64) {
	DB.Model(&WebSearchProvider{}).Where("id = ?", id).
		UpdateColumn("quota_used", DB.Raw("quota_used + 1"))
}

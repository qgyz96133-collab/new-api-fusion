package model

import (
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// TLSFingerprintProfile stores TLS fingerprint templates for anti-detection
type TLSFingerprintProfile struct {
	ID          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);uniqueIndex;not null"`
	Description string `json:"description" gorm:"type:varchar(512)"`

	// TLS configuration
	JA3Hash       string `json:"ja3_hash" gorm:"type:varchar(64)"`       // JA3 fingerprint hash
	JA4Hash       string `json:"ja4_hash" gorm:"type:varchar(64)"`       // JA4 fingerprint hash
	UserAgent     string `json:"user_agent" gorm:"type:varchar(512)"`    // Browser User-Agent
	CipherSuites  string `json:"cipher_suites" gorm:"type:text"`         // JSON array of cipher suite IDs
	ALPNProtocols string `json:"alpn_protocols" gorm:"type:varchar(256)"` // comma-separated ALPN protocols
	TLSVersion    string `json:"tls_version" gorm:"type:varchar(16)"`    // e.g. "1.3", "1.2"

	// Request headers to inject
	Headers string `json:"headers" gorm:"type:text"` // JSON map of additional headers

	// Routing
	ChannelIDs string `json:"channel_ids" gorm:"type:text"` // comma-separated channel IDs that use this profile

	// Status
	Enabled  bool   `json:"enabled" gorm:"default:true"`
	Priority int    `json:"priority" gorm:"default:0"` // higher = preferred
	Status   string `json:"status" gorm:"type:varchar(32);default:'active'"` // active, suspended, expired

	CreatedAt int64 `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64 `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TLSFingerprintProfile) TableName() string {
	return "tls_fingerprint_profiles"
}

// GetAllTLSProfiles returns all TLS fingerprint profiles
func GetAllTLSProfiles() ([]*TLSFingerprintProfile, error) {
	var profiles []*TLSFingerprintProfile
	err := DB.Order("priority DESC, id ASC").Find(&profiles).Error
	return profiles, err
}

// GetTLSProfileByID returns a profile by ID
func GetTLSProfileByID(id int) (*TLSFingerprintProfile, error) {
	var profile TLSFingerprintProfile
	err := DB.First(&profile, id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetActiveTLSProfileForChannel returns the best active profile for a channel
func GetActiveTLSProfileForChannel(channelID int) (*TLSFingerprintProfile, error) {
	var profiles []*TLSFingerprintProfile
	err := DB.Where("enabled = ? AND status = ?", true, "active").
		Order("priority DESC").
		Find(&profiles).Error
	if err != nil {
		return nil, err
	}

	// Find a profile that matches this channel or has no channel restriction
	for _, p := range profiles {
		if p.ChannelIDs == "" || p.ChannelIDs == "*" {
			return p, nil
		}
		// Check if channel ID is in the list
		for _, id := range splitInts(p.ChannelIDs) {
			if id == channelID {
				return p, nil
			}
		}
	}

	return nil, gorm.ErrRecordNotFound
}

// CreateTLSProfile creates a new TLS fingerprint profile
func CreateTLSProfile(profile *TLSFingerprintProfile) error {
	return DB.Create(profile).Error
}

// UpdateTLSProfile updates an existing profile
func UpdateTLSProfile(profile *TLSFingerprintProfile) error {
	return DB.Save(profile).Error
}

// DeleteTLSProfile deletes a profile by ID
func DeleteTLSProfile(id int) error {
	return DB.Delete(&TLSFingerprintProfile{}, id).Error
}

// splitInts splits a comma-separated string into a slice of ints
func splitInts(s string) []int {
	parts := strings.Split(s, ",")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if v, err := strconv.Atoi(p); err == nil {
			result = append(result, v)
		}
	}
	return result
}

package model

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrorPassthroughRule defines a rule for controlling how upstream errors
// are returned to clients. Ported from sub2api's error_passthrough_rule schema.
type ErrorPassthroughRule struct {
	Id              int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name            string    `json:"name" gorm:"type:varchar(100);not null"`
	Enabled         bool      `json:"enabled" gorm:"default:true;index"`
	Priority        int       `json:"priority" gorm:"default:0;index"`
	ErrorCodes      string    `json:"error_codes" gorm:"type:text"`          // JSON array of int
	Keywords        string    `json:"keywords" gorm:"type:text"`             // JSON array of string
	MatchMode       string    `json:"match_mode" gorm:"type:varchar(10);default:'any'"` // "any" or "all"
	Platforms       string    `json:"platforms" gorm:"type:text"`            // JSON array of string
	PassthroughCode bool      `json:"passthrough_code" gorm:"default:true"`
	ResponseCode    *int      `json:"response_code"`
	PassthroughBody bool      `json:"passthrough_body" gorm:"default:true"`
	CustomMessage   *string   `json:"custom_message" gorm:"type:text"`
	SkipMonitoring  bool      `json:"skip_monitoring" gorm:"default:false"`
	Description     *string   `json:"description" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (ErrorPassthroughRule) TableName() string {
	return "error_passthrough_rules"
}

// ParseErrorCodes parses the JSON error_codes field
func (r *ErrorPassthroughRule) ParseErrorCodes() []int {
	if r.ErrorCodes == "" {
		return nil
	}
	var codes []int
	if err := json.Unmarshal([]byte(r.ErrorCodes), &codes); err != nil {
		return nil
	}
	return codes
}

// ParseKeywords parses the JSON keywords field
func (r *ErrorPassthroughRule) ParseKeywords() []string {
	if r.Keywords == "" {
		return nil
	}
	var keywords []string
	if err := json.Unmarshal([]byte(r.Keywords), &keywords); err != nil {
		return nil
	}
	return keywords
}

// ParsePlatforms parses the JSON platforms field
func (r *ErrorPassthroughRule) ParsePlatforms() []string {
	if r.Platforms == "" {
		return nil
	}
	var platforms []string
	if err := json.Unmarshal([]byte(r.Platforms), &platforms); err != nil {
		return nil
	}
	return platforms
}

// SetErrorCodes sets the error_codes field from a slice
func (r *ErrorPassthroughRule) SetErrorCodes(codes []int) {
	data, _ := json.Marshal(codes)
	r.ErrorCodes = string(data)
}

// SetKeywords sets the keywords field from a slice
func (r *ErrorPassthroughRule) SetKeywords(keywords []string) {
	data, _ := json.Marshal(keywords)
	r.Keywords = string(data)
}

// SetPlatforms sets the platforms field from a slice
func (r *ErrorPassthroughRule) SetPlatforms(platforms []string) {
	data, _ := json.Marshal(platforms)
	r.Platforms = string(data)
}

// cachedRule holds pre-computed matching data
type cachedRule struct {
	*ErrorPassthroughRule
	lowerKeywords  []string
	lowerPlatforms []string
	errorCodeSet   map[int]struct{}
}

// ErrorPassthroughService manages rule matching with in-memory cache
type ErrorPassthroughService struct {
	mu         sync.RWMutex
	cachedRules []cachedRule
}

var globalErrorPassthroughService = &ErrorPassthroughService{}

// GetErrorPassthroughService returns the global service instance
func GetErrorPassthroughService() *ErrorPassthroughService {
	return globalErrorPassthroughService
}

// ReloadRules loads all enabled rules from DB and builds cache
func (s *ErrorPassthroughService) ReloadRules() error {
	var rules []ErrorPassthroughRule
	if err := DB.Where("enabled = ?", true).Order("priority ASC, id ASC").Find(&rules).Error; err != nil {
		return err
	}

	cached := make([]cachedRule, 0, len(rules))
	for i := range rules {
		r := &rules[i]
		cr := cachedRule{ErrorPassthroughRule: r}

		// Pre-compute lowercase keywords
		for _, kw := range r.ParseKeywords() {
			cr.lowerKeywords = append(cr.lowerKeywords, strings.ToLower(kw))
		}

		// Pre-compute lowercase platforms
		for _, p := range r.ParsePlatforms() {
			cr.lowerPlatforms = append(cr.lowerPlatforms, strings.ToLower(p))
		}

		// Pre-compute error code set
		cr.errorCodeSet = make(map[int]struct{})
		for _, code := range r.ParseErrorCodes() {
			cr.errorCodeSet[code] = struct{}{}
		}

		cached = append(cached, cr)
	}

	s.mu.Lock()
	s.cachedRules = cached
	s.mu.Unlock()
	return nil
}

// MatchRule finds the first matching rule for the given error
func (s *ErrorPassthroughService) MatchRule(platform string, upstreamStatus int, responseBody []byte) *ErrorPassthroughRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.cachedRules) == 0 {
		return nil
	}

	bodyStr := string(responseBody)
	bodyLower := ""
	bodyLowerDone := false
	platformLower := strings.ToLower(platform)

	for i := range s.cachedRules {
		rule := &s.cachedRules[i]

		// Check platform filter
		if len(rule.lowerPlatforms) > 0 {
			found := false
			for _, p := range rule.lowerPlatforms {
				if p == platformLower {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if s.matches(rule, upstreamStatus, bodyStr, &bodyLower, &bodyLowerDone) {
			return rule.ErrorPassthroughRule
		}
	}
	return nil
}

func (s *ErrorPassthroughService) matches(rule *cachedRule, statusCode int, body string, bodyLower *string, bodyLowerDone *bool) bool {
	hasErrorCodes := len(rule.errorCodeSet) > 0
	hasKeywords := len(rule.lowerKeywords) > 0

	if !hasErrorCodes && !hasKeywords {
		return false
	}

	codeMatch := false
	if hasErrorCodes {
		_, codeMatch = rule.errorCodeSet[statusCode]
	}

	if rule.MatchMode == "all" {
		if !hasErrorCodes || !hasKeywords {
			return codeMatch && hasErrorCodes && !hasKeywords
		}
		if !codeMatch {
			return false
		}
		return s.containsAnyKeyword(rule, body, bodyLower, bodyLowerDone)
	}

	// "any" mode (default)
	if codeMatch {
		return true
	}
	if hasKeywords {
		return s.containsAnyKeyword(rule, body, bodyLower, bodyLowerDone)
	}
	return false
}

func (s *ErrorPassthroughService) containsAnyKeyword(rule *cachedRule, body string, bodyLower *string, bodyLowerDone *bool) bool {
	// Limit body scan to 8KB
	scanBody := body
	if len(scanBody) > 8192 {
		scanBody = scanBody[:8192]
	}

	if !*bodyLowerDone {
		bl := strings.ToLower(scanBody)
		*bodyLower = bl
		*bodyLowerDone = true
	}

	for _, kw := range rule.lowerKeywords {
		if strings.Contains(*bodyLower, kw) {
			return true
		}
	}
	return false
}

// --- CRUD operations ---

func GetAllErrorPassthroughRules() ([]ErrorPassthroughRule, error) {
	var rules []ErrorPassthroughRule
	err := DB.Order("priority ASC, id ASC").Find(&rules).Error
	return rules, err
}

func GetErrorPassthroughRule(id int64) (*ErrorPassthroughRule, error) {
	var rule ErrorPassthroughRule
	err := DB.First(&rule, id).Error
	return &rule, err
}

func CreateErrorPassthroughRule(rule *ErrorPassthroughRule) error {
	return DB.Create(rule).Error
}

func UpdateErrorPassthroughRule(rule *ErrorPassthroughRule) error {
	return DB.Save(rule).Error
}

func DeleteErrorPassthroughRule(id int64) error {
	return DB.Delete(&ErrorPassthroughRule{}, id).Error
}

// Sort rules by priority
type byPriority []ErrorPassthroughRule
func (a byPriority) Len() int           { return len(a) }
func (a byPriority) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byPriority) Less(i, j int) bool { return a[i].Priority < a[j].Priority }

func init() {
	sort.Sort(byPriority(nil)) // ensure import used
}

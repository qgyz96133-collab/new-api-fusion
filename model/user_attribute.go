package model

import "gorm.io/gorm"

// UserAttributeDefinition defines a custom attribute field
type UserAttributeDefinition struct {
	ID          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name        string `json:"name" gorm:"type:varchar(128);uniqueIndex;not null"` // e.g., "department", "employee_id"
	DisplayName string `json:"display_name" gorm:"type:varchar(256)"`
	Type        string `json:"type" gorm:"type:varchar(32);default:'string'"` // string, number, boolean, select
	Required    bool   `json:"required" gorm:"default:false"`
	Options     string `json:"options" gorm:"type:text"` // JSON array for select type
	DefaultVal  string `json:"default_value" gorm:"type:varchar(512)"`
	Sortable    bool   `json:"sortable" gorm:"default:false"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (UserAttributeDefinition) TableName() string { return "user_attribute_definitions" }

// UserAttributeValue stores a user's custom attribute value
type UserAttributeValue struct {
	ID             int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int    `json:"user_id" gorm:"index;not null"`
	DefinitionID   int    `json:"definition_id" gorm:"index;not null"`
	Value          string `json:"value" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (UserAttributeValue) TableName() string { return "user_attribute_values" }

// GetUserAttributeDefinitions returns all definitions
func GetUserAttributeDefinitions() ([]UserAttributeDefinition, error) {
	var defs []UserAttributeDefinition
	err := DB.Order("id ASC").Find(&defs).Error
	return defs, err
}

// CreateUserAttributeDefinition creates a new attribute definition
func CreateUserAttributeDefinition(def *UserAttributeDefinition) error {
	return DB.Create(def).Error
}

// DeleteUserAttributeDefinition deletes a definition and all values
func DeleteUserAttributeDefinition(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		tx.Where("definition_id = ?", id).Delete(&UserAttributeValue{})
		return tx.Delete(&UserAttributeDefinition{}, id).Error
	})
}

// GetUserAttributes returns all attributes for a user
func GetUserAttributes(userID int) (map[string]string, error) {
	var values []UserAttributeValue
	err := DB.Where("user_id = ?", userID).Find(&values).Error
	if err != nil {
		return nil, err
	}

	// Load definitions for name lookup
	defs, _ := GetUserAttributeDefinitions()
	defMap := make(map[int]string)
	for _, d := range defs {
		defMap[d.ID] = d.Name
	}

	result := make(map[string]string)
	for _, v := range values {
		if name, ok := defMap[v.DefinitionID]; ok {
			result[name] = v.Value
		}
	}
	return result, nil
}

// SetUserAttribute sets a custom attribute value for a user
func SetUserAttribute(userID int, definitionID int, value string) error {
	var existing UserAttributeValue
	err := DB.Where("user_id = ? AND definition_id = ?", userID, definitionID).First(&existing).Error
	if err == nil {
		existing.Value = value
		return DB.Save(&existing).Error
	}
	return DB.Create(&UserAttributeValue{
		UserID:       userID,
		DefinitionID: definitionID,
		Value:        value,
	}).Error
}

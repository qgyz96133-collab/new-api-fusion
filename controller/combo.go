package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/gin-gonic/gin"
)

// GetCombos returns all registered model combos
func GetCombos(c *gin.Context) {
	combos := relay.GetAllCombos()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    combos,
	})
}

// UpdateCombo creates or updates a model combo
func UpdateCombo(c *gin.Context) {
	var req struct {
		Name   string   `json:"name" binding:"required"`
		Models []string `json:"models" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	if len(req.Models) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Combo must have at least 2 models",
		})
		return
	}

	relay.RegisterCombo(req.Name, req.Models)

	// Persist to DB as an option
	comboJSON, _ := json.Marshal(relay.GetAllCombos())
	if err := model.UpdateOption("model_combos", string(comboJSON)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to persist combo: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Combo updated successfully",
	})
}

// DeleteCombo removes a model combo
func DeleteCombo(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Combo name is required",
		})
		return
	}

	relay.RemoveCombo(name)

	// Persist to DB
	comboJSON, _ := json.Marshal(relay.GetAllCombos())
	model.UpdateOption("model_combos", string(comboJSON))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Combo deleted",
	})
}

// LoadCombosFromDB loads combos from DB on startup
func LoadCombosFromDB() {
	common.OptionMapRWMutex.RLock()
	data := common.OptionMap["model_combos"]
	common.OptionMapRWMutex.RUnlock()
	var err error
	if data == "" {
		err = fmt.Errorf("not found")
	}
	if err != nil || data == "" {
		return
	}
	relay.LoadCombosFromJSON(data)
}

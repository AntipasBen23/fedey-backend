package handlers

// Plan management endpoints.
//
// GET  /v1/plan         — returns the current plan ("free" | "pro")
// POST /v1/plan/upgrade — upgrades the plan to "pro"
//                         requires { "upgradeCode": "<value of UPGRADE_SECRET env var>" }
//
// The upgradeCode approach lets you manually activate pro for users now and
// swap in a Stripe webhook later without changing the API contract.

import (
	"net/http"
	"os"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
)

// getOrCreatePrefs returns the single UserPreferences row, creating it if absent.
func getOrCreatePrefs() (*models.UserPreferences, error) {
	var prefs models.UserPreferences
	result := database.DB.Order("id asc").First(&prefs)
	if result.Error != nil {
		// No row yet — create one with defaults
		prefs = models.UserPreferences{Plan: "free"}
		if err := database.DB.Create(&prefs).Error; err != nil {
			return nil, err
		}
	}
	return &prefs, nil
}

// GetPlanHandler returns the user's current plan.
// GET /v1/plan
func GetPlanHandler(c *gin.Context) {
	prefs, err := getOrCreatePrefs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load preferences"})
		return
	}
	plan := prefs.Plan
	if plan == "" {
		plan = "free"
	}
	c.JSON(http.StatusOK, gin.H{"plan": plan})
}

// UpgradePlanHandler upgrades the plan to "pro".
// POST /v1/plan/upgrade  { "upgradeCode": "..." }
//
// Set UPGRADE_SECRET env var to any secret string.
// If UPGRADE_SECRET is empty, upgrades are disabled until it is set.
func UpgradePlanHandler(c *gin.Context) {
	secret := os.Getenv("UPGRADE_SECRET")
	if secret == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Pro upgrades are not yet configured. Set UPGRADE_SECRET on the server.",
		})
		return
	}

	var body struct {
		UpgradeCode string `json:"upgradeCode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.UpgradeCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "upgradeCode is required"})
		return
	}
	if body.UpgradeCode != secret {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid upgrade code"})
		return
	}

	prefs, err := getOrCreatePrefs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load preferences"})
		return
	}

	if err := database.DB.Model(prefs).Update("plan", "pro").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade plan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"plan": "pro", "message": "Upgraded to Pro successfully."})
}

// IsPro is a helper used by other handlers to check the active plan.
func IsPro() bool {
	if database.DB == nil {
		return false
	}
	prefs, err := getOrCreatePrefs()
	if err != nil {
		return false
	}
	return prefs.Plan == "pro"
}

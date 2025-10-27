package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/thenaveensharma/telehook/internal/database"
)

type AnalyticsHandler struct {
	db *database.DB
}

func NewAnalyticsHandler(db *database.DB) *AnalyticsHandler {
	return &AnalyticsHandler{db: db}
}

// GetAnalytics returns comprehensive analytics data for the authenticated user
// GET /api/user/analytics?range=24h|7d|30d
func (h *AnalyticsHandler) GetAnalytics(c *fiber.Ctx) error {
	// Get user ID from context (set by auth middleware)
	userID, ok := c.Locals("user_id").(int)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Get time range from query parameter (default: 24h)
	timeRange := c.Query("range", "24h")

	// Validate time range
	validRanges := map[string]bool{
		"24h": true,
		"7d":  true,
		"30d": true,
	}

	if !validRanges[timeRange] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid time range. Must be 24h, 7d, or 30d",
		})
	}

	// Get analytics from database
	analytics, err := h.db.GetAnalytics(c.Context(), userID, timeRange)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch analytics",
		})
	}

	return c.JSON(analytics)
}

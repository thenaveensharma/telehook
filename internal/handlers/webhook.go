package handlers

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/thenaveensharma/telehook/internal/database"
	"github.com/thenaveensharma/telehook/internal/models"
	"github.com/thenaveensharma/telehook/internal/queue"
	"github.com/thenaveensharma/telehook/internal/telegram"
)

type WebhookHandler struct {
	db    *database.DB
	bot   *telegram.Bot
	queue *queue.AlertQueue
}

func NewWebhookHandler(db *database.DB, bot *telegram.Bot, alertQueue *queue.AlertQueue) *WebhookHandler {
	return &WebhookHandler{
		db:    db,
		bot:   bot,
		queue: alertQueue,
	}
}

func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	// Get webhook token from URL parameter
	tokenStr := c.Params("token")
	if tokenStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "webhook token is required",
		})
	}

	// Parse token as UUID
	token, err := uuid.Parse(tokenStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid webhook token format",
		})
	}

	// Get user by webhook token
	user, err := h.db.GetUserByWebhookToken(context.Background(), token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid webhook token",
		})
	}

	// Parse JSON payload
	var payload models.WebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid JSON payload",
		})
	}

	// Ensure message is not empty
	if payload.Message == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "message field is required",
		})
	}

	// Parse message to extract channel identifier
	channelIdentifier, messageContent := parseMessageWithIdentifier(payload.Message)

	// If no identifier found, return error (new behavior)
	if channelIdentifier == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "channel identifier not found. Message format: '<content>\\n----\\n<identifier>'",
		})
	}

	// Look up channel configuration
	channel, err := h.db.GetTelegramChannelByIdentifier(context.Background(), user.ID, channelIdentifier)
	if err != nil {
		log.Printf("Channel identifier '%s' not found for user %d: %v", channelIdentifier, user.ID, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "channel identifier not found or inactive",
			"identifier": channelIdentifier,
			"hint":       "Please configure this channel identifier in your dashboard",
		})
	}

	// Get bot token for this channel
	bot, err := h.db.GetBotByID(context.Background(), channel.BotID)
	if err != nil {
		log.Printf("Bot not found for channel %d: %v", channel.ID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "bot configuration not found",
		})
	}

	// Get priority from payload (default to normal)
	priority := 3 // Normal priority
	if payload.Priority > 0 {
		priority = payload.Priority
	}

	// Create payload map for alert
	payloadMap := map[string]interface{}{
		"message":    messageContent,
		"priority":   priority,
		"identifier": channelIdentifier,
	}
	if payload.Data != nil {
		payloadMap["data"] = payload.Data
	}

	// Create alert with channel routing information
	alert := &queue.Alert{
		ID:         uuid.New().String(),
		UserID:     user.ID,
		Username:   user.Username,
		Payload:    payloadMap,
		Priority:   priority,
		MaxRetries: 3,
		CreatedAt:  time.Now(),
		// Store routing info in metadata
		BotToken:  bot.BotToken,
		ChannelID: channel.ChannelID,
		DBChannelID: channel.ID,
	}

	// Enqueue the alert
	if err := h.queue.Enqueue(alert); err != nil {
		log.Printf("Error enqueuing alert: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "alert queue is full, please try again later",
		})
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"message":    "alert queued successfully",
		"alert_id":   alert.ID,
		"channel":    channel.ChannelName,
		"identifier": channelIdentifier,
	})
}

func (h *WebhookHandler) GetQueueStats(c *fiber.Ctx) error {
	stats := h.queue.GetStats()
	return c.JSON(stats)
}

func (h *WebhookHandler) GetWebhookInfo(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	username := c.Locals("username").(string)

	// Get user to retrieve webhook token
	user, err := h.db.GetUserByEmail(context.Background(), c.Locals("email").(string))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve user information",
		})
	}

	// Get recent webhook logs
	logs, err := h.db.GetUserWebhookLogs(context.Background(), userID, 10)
	if err != nil {
		log.Printf("Error getting webhook logs: %v", err)
		logs = make([]models.WebhookLog, 0)
	}

	webhookURL := c.BaseURL() + "/api/webhook/" + user.WebhookToken.String()

	return c.JSON(fiber.Map{
		"username":     username,
		"webhook_url":  webhookURL,
		"webhook_token": user.WebhookToken,
		"recent_logs":  logs,
	})
}

// parseMessageWithIdentifier parses a message in the format:
// "content\n----\nidentifier"
// Returns the identifier and the content (without the separator and identifier)
func parseMessageWithIdentifier(message string) (identifier string, content string) {
	// Split by "----"
	parts := strings.Split(message, "----")

	if len(parts) < 2 {
		// No separator found, return empty identifier
		return "", message
	}

	// Content is everything before "----"
	content = strings.TrimSpace(parts[0])

	// Identifier is everything after "----" (trimmed)
	identifier = strings.TrimSpace(parts[1])

	return identifier, content
}

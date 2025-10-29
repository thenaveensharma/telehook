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

	// Parse message to extract optional channel identifier
	channelIdentifier, messageContent := parseMessageWithIdentifier(payload.Message)
	log.Printf("[Webhook] User: %d, Original msg len: %d, Cleaned msg len: %d, Identifier: '%s'",
		user.ID, len(payload.Message), len(messageContent), channelIdentifier)

	// Log preview of cleaned message
	previewLen := 100
	if len(messageContent) < previewLen {
		previewLen = len(messageContent)
	}
	log.Printf("[Webhook] Cleaned message preview: %s", messageContent[:previewLen])

	var channel *models.TelegramChannel

	// If identifier provided, use specific channel; otherwise use default
	if channelIdentifier != "" {
		// Look up channel by identifier
		channel, err = h.db.GetTelegramChannelByIdentifier(context.Background(), user.ID, channelIdentifier)
		if err != nil {
			log.Printf("Channel identifier '%s' not found for user %d: %v", channelIdentifier, user.ID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":      "channel identifier not found or inactive",
				"identifier": channelIdentifier,
				"hint":       "Please configure this channel identifier in your dashboard",
			})
		}
	} else {
		// Use default channel (first active channel)
		channel, err = h.db.GetDefaultTelegramChannel(context.Background(), user.ID)
		if err != nil {
			log.Printf("No active channel found for user %d: %v", user.ID, err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "no active channel configured",
				"hint":  "Please configure a Telegram channel in your dashboard",
			})
		}
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
		"message":  messageContent,
		"priority": priority,
	}
	if channelIdentifier != "" {
		payloadMap["identifier"] = channelIdentifier
	}
	if payload.Data != nil {
		payloadMap["data"] = payload.Data
	}

	// Create alert with channel routing information
	alert := &queue.Alert{
		ID:          uuid.New().String(),
		UserID:      user.ID,
		Username:    user.Username,
		Payload:     payloadMap,
		Priority:    priority,
		MaxRetries:  3,
		CreatedAt:   time.Now(),
		BotToken:    bot.BotToken,
		ChannelID:   channel.ChannelID,
		DBChannelID: channel.ID,
	}

	// Enqueue the alert
	if err := h.queue.Enqueue(alert); err != nil {
		log.Printf("Error enqueuing alert: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "alert queue is full, please try again later",
		})
	}

	response := fiber.Map{
		"success":  true,
		"message":  "alert queued successfully",
		"alert_id": alert.ID,
		"channel":  channel.ChannelName,
	}
	if channelIdentifier != "" {
		response["identifier"] = channelIdentifier
	}

	return c.JSON(response)
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
		"username":      username,
		"webhook_url":   webhookURL,
		"webhook_token": user.WebhookToken,
		"recent_logs":   logs,
	})
}

// parseMessageWithIdentifier parses a message in the format:
// "content\n----\nidentifier"
// Returns the identifier and the content (without the separator and identifier)
// If no identifier found, returns empty string and the original message
func parseMessageWithIdentifier(message string) (identifier string, content string) {
	// Look for the pattern "\n----\n" to avoid matching dashes in content
	separator := "\n----\n"
	idx := strings.LastIndex(message, separator)

	if idx == -1 {
		// No separator found, return empty identifier and original message
		return "", message
	}

	// Content is everything before the separator
	content = strings.TrimSpace(message[:idx])

	// Identifier is everything after the separator (trimmed)
	identifier = strings.TrimSpace(message[idx+len(separator):])

	// Validate identifier (should be a single word/token, not multiple lines)
	if strings.Contains(identifier, "\n") || len(identifier) > 50 {
		// If identifier contains newlines or is too long, it's probably not an identifier
		// Return the full message instead
		return "", message
	}

	return identifier, content
}

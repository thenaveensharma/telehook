package handlers

import (
	"context"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/thenaveensharma/telehook/internal/database"
	"github.com/thenaveensharma/telehook/internal/models"
	"github.com/thenaveensharma/telehook/internal/telegram"
)

type TelegramConfigHandler struct {
	db *database.DB
}

func NewTelegramConfigHandler(db *database.DB) *TelegramConfigHandler {
	return &TelegramConfigHandler{
		db: db,
	}
}

// ============================================================================
// Bot Management Endpoints
// ============================================================================

func (h *TelegramConfigHandler) CreateBot(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	var req models.CreateBotRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.BotToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bot_token is required",
		})
	}

	// Validate bot token by attempting to get bot username
	botUsername, err := telegram.GetBotUsername(req.BotToken)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid bot token or cannot connect to Telegram API",
		})
	}

	// Create bot in database
	bot, err := h.db.CreateTelegramBot(context.Background(), userID, req.BotToken, botUsername, req.IsDefault)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "bot token already exists",
			})
		}
		log.Printf("Error creating bot: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create bot",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"bot":     bot,
	})
}

func (h *TelegramConfigHandler) GetBots(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	bots, err := h.db.GetUserTelegramBots(context.Background(), userID)
	if err != nil {
		log.Printf("Error getting bots: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve bots",
		})
	}

	if bots == nil {
		bots = []models.TelegramBot{}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"bots":    bots,
	})
}

func (h *TelegramConfigHandler) GetBot(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	botID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid bot ID",
		})
	}

	bot, err := h.db.GetTelegramBot(context.Background(), botID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "bot not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"bot":     bot,
	})
}

func (h *TelegramConfigHandler) UpdateBot(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	botID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid bot ID",
		})
	}

	var req models.UpdateBotRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// If token is being updated, validate it
	botUsername := ""
	if req.BotToken != "" {
		username, err := telegram.GetBotUsername(req.BotToken)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid bot token or cannot connect to Telegram API",
			})
		}
		botUsername = username
	}

	bot, err := h.db.UpdateTelegramBot(context.Background(), botID, userID, req.BotToken, botUsername, req.IsDefault)
	if err != nil {
		log.Printf("Error updating bot: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update bot",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"bot":     bot,
	})
}

func (h *TelegramConfigHandler) DeleteBot(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	botID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid bot ID",
		})
	}

	err = h.db.DeleteTelegramBot(context.Background(), botID, userID)
	if err != nil {
		log.Printf("Error deleting bot: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to delete bot",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "bot deleted successfully",
	})
}

// ============================================================================
// Channel Management Endpoints
// ============================================================================

func (h *TelegramConfigHandler) CreateChannel(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	var req models.CreateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.BotID == 0 || req.Identifier == "" || req.ChannelID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bot_id, identifier, and channel_id are required",
		})
	}

	// Verify bot belongs to user
	_, err := h.db.GetTelegramBot(context.Background(), req.BotID, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bot not found or not owned by user",
		})
	}

	// Create channel
	channel, err := h.db.CreateTelegramChannel(
		context.Background(),
		userID,
		req.BotID,
		req.Identifier,
		req.ChannelID,
		req.ChannelName,
		req.Description,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "identifier already exists for this user",
			})
		}
		log.Printf("Error creating channel: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create channel",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"channel": channel,
	})
}

func (h *TelegramConfigHandler) GetChannels(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	channels, err := h.db.GetUserTelegramChannels(context.Background(), userID)
	if err != nil {
		log.Printf("Error getting channels: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve channels",
		})
	}

	if channels == nil {
		channels = []models.TelegramChannel{}
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"channels": channels,
	})
}

func (h *TelegramConfigHandler) GetChannel(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	channelID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid channel ID",
		})
	}

	channel, err := h.db.GetTelegramChannel(context.Background(), channelID, userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "channel not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"channel": channel,
	})
}

func (h *TelegramConfigHandler) UpdateChannel(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	channelID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid channel ID",
		})
	}

	var req models.UpdateChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// If bot_id is being updated, verify it belongs to user
	if req.BotID != 0 {
		_, err := h.db.GetTelegramBot(context.Background(), req.BotID, userID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "bot not found or not owned by user",
			})
		}
	}

	channel, err := h.db.UpdateTelegramChannel(context.Background(), channelID, userID, req)
	if err != nil {
		log.Printf("Error updating channel: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update channel",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"channel": channel,
	})
}

func (h *TelegramConfigHandler) DeleteChannel(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)
	channelID, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid channel ID",
		})
	}

	err = h.db.DeleteTelegramChannel(context.Background(), channelID, userID)
	if err != nil {
		log.Printf("Error deleting channel: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to delete channel",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "channel deleted successfully",
	})
}

// GetBotsWithChannels returns all bots with their associated channels
func (h *TelegramConfigHandler) GetBotsWithChannels(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(int)

	bots, err := h.db.GetUserTelegramBots(context.Background(), userID)
	if err != nil {
		log.Printf("Error getting bots: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to retrieve bots",
		})
	}

	result := make([]models.BotWithChannels, 0, len(bots))

	for _, bot := range bots {
		channels, err := h.db.GetBotChannels(context.Background(), bot.ID, userID)
		if err != nil {
			log.Printf("Error getting channels for bot %d: %v", bot.ID, err)
			channels = []models.TelegramChannel{}
		}

		result = append(result, models.BotWithChannels{
			Bot:      bot,
			Channels: channels,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

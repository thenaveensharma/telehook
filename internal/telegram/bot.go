package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/time/rate"
)

type Bot struct {
	api            *tgbotapi.BotAPI
	channelID      string
	botLimiter     *rate.Limiter // Per-bot rate limiter (30 msg/sec)
	channelLimiter *rate.Limiter // Per-channel rate limiter (20 msg/min)
}

// BotManager manages multiple bot instances per user
type BotManager struct {
	bots            map[string]*tgbotapi.BotAPI // token -> bot instance
	botLimiters     map[string]*rate.Limiter    // token -> rate limiter (30 msg/sec per bot)
	channelLimiters map[string]*rate.Limiter    // channelID -> rate limiter (20 msg/min per channel)
	mu              sync.RWMutex
}

var globalBotManager = &BotManager{
	bots:            make(map[string]*tgbotapi.BotAPI),
	botLimiters:     make(map[string]*rate.Limiter),
	channelLimiters: make(map[string]*rate.Limiter),
}

// NewBot creates a bot instance using environment variables (legacy support)
func NewBot() (*Bot, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN not set in environment")
	}

	channelID := os.Getenv("TELEGRAM_CHANNEL_ID")
	if channelID == "" {
		return nil, fmt.Errorf("TELEGRAM_CHANNEL_ID not set in environment")
	}

	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	log.Printf("Telegram bot authorized as: %s", botAPI.Self.UserName)

	return &Bot{
		api:       botAPI,
		channelID: channelID,
	}, nil
}

// NewBotWithToken creates a bot instance with a specific token and channel
func NewBotWithToken(token, channelID string) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("bot token is required")
	}

	if channelID == "" {
		return nil, fmt.Errorf("channel ID is required")
	}

	botAPI, botLimiter, channelLimiter, err := globalBotManager.GetOrCreateBot(token, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		api:            botAPI,
		channelID:      channelID,
		botLimiter:     botLimiter,
		channelLimiter: channelLimiter,
	}, nil
}

// GetOrCreateBot retrieves or creates a bot instance with rate limiters
func (bm *BotManager) GetOrCreateBot(token string, channelID string) (*tgbotapi.BotAPI, *rate.Limiter, *rate.Limiter, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Get or create bot
	bot, exists := bm.bots[token]
	if !exists {
		var err error
		bot, err = tgbotapi.NewBotAPI(token)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create bot API: %w", err)
		}
		bm.bots[token] = bot
		log.Printf("New Telegram bot authorized: %s", bot.Self.UserName)
	}

	// Get or create bot rate limiter (30 messages per second)
	botLimiter, exists := bm.botLimiters[token]
	if !exists {
		// Allow 30 requests per second with burst of 5
		botLimiter = rate.NewLimiter(rate.Limit(30), 5)
		bm.botLimiters[token] = botLimiter
	}

	// Get or create channel rate limiter (60 messages per minute = 1 per second)
	channelLimiter, exists := bm.channelLimiters[channelID]
	if !exists {
		// Allow 1 message per second (60/min) with burst of 5
		// This is conservative and safe, well below bot limit of 30/sec
		channelLimiter = rate.NewLimiter(rate.Limit(1), 5)
		bm.channelLimiters[channelID] = channelLimiter
	}

	return bot, botLimiter, channelLimiter, nil
}

// GetBotUsername retrieves the username of a bot by token
func GetBotUsername(token string) (string, error) {
	botAPI, _, _, err := globalBotManager.GetOrCreateBot(token, "")
	if err != nil {
		return "", err
	}
	return botAPI.Self.UserName, nil
}

func (b *Bot) SendMessage(text string) (string, error) {
	// Wait for bot-level rate limit (30 msg/sec)
	if b.botLimiter != nil {
		if err := b.botLimiter.Wait(context.Background()); err != nil {
			return "", fmt.Errorf("bot rate limit error: %w", err)
		}
	}

	// Wait for channel-level rate limit (20 msg/min)
	if b.channelLimiter != nil {
		if err := b.channelLimiter.Wait(context.Background()); err != nil {
			return "", fmt.Errorf("channel rate limit error: %w", err)
		}
	}

	msg := tgbotapi.NewMessageToChannel(b.channelID, text)
	msg.ParseMode = "HTML"

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	response := map[string]interface{}{
		"message_id": sentMsg.MessageID,
		"chat_id":    sentMsg.Chat.ID,
		"date":       sentMsg.Date,
	}

	responseJSON, _ := json.Marshal(response)
	return string(responseJSON), nil
}

func (b *Bot) SendFormattedWebhookMessage(username string, payload map[string]interface{}) (string, error) {
	message := ""

	// Check if there's a custom message field
	if msg, ok := payload["message"].(string); ok && msg != "" {
		message = fmt.Sprintf("%s\n\n", msg)
	}

	// Add data if present
	if data, ok := payload["data"].(map[string]interface{}); ok && len(data) > 0 {
		dataJSON, err := json.MarshalIndent(data, "", "  ")
		if err == nil {
			message += fmt.Sprintf("<pre>%s</pre>", string(dataJSON))
		}
	} else if len(payload) > 1 || (len(payload) == 1 && payload["message"] == nil) {
		// If there's no message or data fields, send the entire payload
		message += "<b>Payload:</b>\n"
		payloadJSON, err := json.MarshalIndent(payload, "", "  ")
		if err == nil {
			message += fmt.Sprintf("<pre>%s</pre>", string(payloadJSON))
		}
	}

	return b.SendMessage(message)
}

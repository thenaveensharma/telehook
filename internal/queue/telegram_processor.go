package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/thenaveensharma/telehook/internal/database"
	"github.com/thenaveensharma/telehook/internal/telegram"
)

// TelegramProcessor implements AlertProcessor for Telegram
type TelegramProcessor struct {
	bot *telegram.Bot
	db  *database.DB
	ruleEngine *RuleEngine
}

// NewTelegramProcessor creates a new Telegram alert processor
func NewTelegramProcessor(bot *telegram.Bot, db *database.DB) *TelegramProcessor {
	return &TelegramProcessor{
		bot:        bot,
		db:         db,
		ruleEngine: NewRuleEngine(30 * time.Second), // 30 second dedup window
	}
}

// ProcessAlert processes a single alert
func (tp *TelegramProcessor) ProcessAlert(ctx context.Context, alert *Alert) error {
	// Apply rules
	allowed, reason := tp.ruleEngine.ProcessAlert(alert)
	if !allowed {
		log.Printf("Alert %s blocked: %s", alert.ID, reason)
		_ = tp.db.CreateWebhookLog(ctx, alert.UserID, alert.Payload, reason, "filtered")
		return nil // Not an error, just filtered
	}

	// Use per-alert bot token and channel if provided (multi-channel mode)
	var botInstance *telegram.Bot
	var err error

	if alert.BotToken != "" && alert.ChannelID != "" {
		// Multi-channel mode: create bot instance with alert's token and channel
		botInstance, err = telegram.NewBotWithToken(alert.BotToken, alert.ChannelID)
		if err != nil {
			log.Printf("Failed to create bot instance for alert %s: %v", alert.ID, err)
			_ = tp.db.CreateWebhookLog(ctx, alert.UserID, alert.Payload, err.Error(), "failed")
			return fmt.Errorf("failed to create bot instance: %w", err)
		}
	} else {
		// Legacy mode: use global bot
		if tp.bot == nil {
			return fmt.Errorf("telegram bot not configured")
		}
		botInstance = tp.bot
	}

	// Send to Telegram
	response, err := botInstance.SendFormattedWebhookMessage(alert.Username, alert.Payload)
	if err != nil {
		_ = tp.db.CreateWebhookLog(ctx, alert.UserID, alert.Payload, err.Error(), "failed")
		return err
	}

	// Log success
	_ = tp.db.CreateWebhookLog(ctx, alert.UserID, alert.Payload, response, "success")
	log.Printf("Alert %s processed successfully for user %d to channel %s", alert.ID, alert.UserID, alert.ChannelID)

	return nil
}

// ProcessBatch processes multiple alerts in a batch
func (tp *TelegramProcessor) ProcessBatch(ctx context.Context, alerts []*Alert) error {
	if len(alerts) == 0 {
		return nil
	}

	log.Printf("Processing batch of %d alerts", len(alerts))

	successCount := 0
	errorCount := 0

	for _, alert := range alerts {
		if err := tp.ProcessAlert(ctx, alert); err != nil {
			errorCount++
			log.Printf("Batch: Failed to process alert %s: %v", alert.ID, err)
		} else {
			successCount++
		}
	}

	log.Printf("Batch complete: %d succeeded, %d failed", successCount, errorCount)

	if errorCount > 0 && successCount == 0 {
		return fmt.Errorf("all alerts in batch failed")
	}

	return nil
}

// AddCustomRule adds a custom rule to the processor
func (tp *TelegramProcessor) AddCustomRule(rule *AlertRule) {
	tp.ruleEngine.AddRule(rule)
}

// InitializeDefaultRules sets up default alert rules
func (tp *TelegramProcessor) InitializeDefaultRules() {
	for _, rule := range DefaultRules() {
		tp.ruleEngine.AddRule(rule)
	}
	log.Println("Default alert rules initialized")
}

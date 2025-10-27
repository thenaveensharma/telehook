package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	WebhookToken uuid.UUID `json:"webhook_token"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type WebhookLog struct {
	ID               int       `json:"id"`
	UserID           int       `json:"user_id"`
	Payload          string    `json:"payload"`
	TelegramResponse string    `json:"telegram_response,omitempty"`
	Status           string    `json:"status"`
	SentAt           time.Time `json:"sent_at"`
}

type SignupRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token        string    `json:"token"`
	User         User      `json:"user"`
	WebhookToken uuid.UUID `json:"webhook_token"`
}

type WebhookPayload struct {
	Message  string                 `json:"message"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Priority int                    `json:"priority,omitempty"` // 1=urgent, 2=high, 3=normal, 4=low
}

type QueueStats struct {
	Processed   int64 `json:"processed"`
	Failed      int64 `json:"failed"`
	Retried     int64 `json:"retried"`
	Batched     int64 `json:"batched"`
	CurrentSize int   `json:"current_size"`
}

// TelegramBot represents a user's Telegram bot configuration
type TelegramBot struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	BotToken    string    `json:"bot_token"`
	BotUsername string    `json:"bot_username,omitempty"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TelegramChannel represents a user's channel/group configuration with identifier
type TelegramChannel struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	BotID       int       `json:"bot_id"`
	Identifier  string    `json:"identifier"`  // Custom identifier like "tg", "alerts", "vip"
	ChannelID   string    `json:"channel_id"`  // Telegram channel ID or username
	ChannelName string    `json:"channel_name,omitempty"`
	Description string    `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Request/Response models for bot and channel management

type CreateBotRequest struct {
	BotToken  string `json:"bot_token" validate:"required"`
	IsDefault bool   `json:"is_default"`
}

type UpdateBotRequest struct {
	BotToken  string `json:"bot_token,omitempty"`
	IsDefault bool   `json:"is_default"`
}

type CreateChannelRequest struct {
	BotID       int    `json:"bot_id" validate:"required"`
	Identifier  string `json:"identifier" validate:"required"`
	ChannelID   string `json:"channel_id" validate:"required"`
	ChannelName string `json:"channel_name,omitempty"`
	Description string `json:"description,omitempty"`
}

type UpdateChannelRequest struct {
	BotID       int    `json:"bot_id,omitempty"`
	Identifier  string `json:"identifier,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
	Description string `json:"description,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

type BotWithChannels struct {
	Bot      TelegramBot       `json:"bot"`
	Channels []TelegramChannel `json:"channels"`
}

// ============================================================================
// Analytics Models
// ============================================================================

// AnalyticsSummary provides overall performance metrics
type AnalyticsSummary struct {
	TotalMessages    int     `json:"total_messages"`
	SuccessCount     int     `json:"success_count"`
	FailedCount      int     `json:"failed_count"`
	FilteredCount    int     `json:"filtered_count"`
	PendingCount     int     `json:"pending_count"`
	SuccessRate      float64 `json:"success_rate"`
	AvgPerHour       float64 `json:"avg_per_hour"`
	AvgPerDay        float64 `json:"avg_per_day"`
	PeakHour         int     `json:"peak_hour"`          // 0-23
	PeakHourCount    int     `json:"peak_hour_count"`
	LastMessageAt    *time.Time `json:"last_message_at,omitempty"`
}

// TimelineDataPoint represents messages at a specific time
type TimelineDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	SuccessCount int       `json:"success_count"`
	FailedCount  int       `json:"failed_count"`
	FilteredCount int      `json:"filtered_count"`
	TotalCount   int       `json:"total_count"`
}

// StatusDistribution shows breakdown by status
type StatusDistribution struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
	Percentage float64 `json:"percentage"`
}

// ChannelDistribution shows messages per channel
type ChannelDistribution struct {
	ChannelIdentifier string `json:"channel_identifier"`
	ChannelName       string `json:"channel_name,omitempty"`
	Count             int    `json:"count"`
	Percentage        float64 `json:"percentage"`
}

// PriorityDistribution shows messages per priority level
type PriorityDistribution struct {
	Priority   int     `json:"priority"`
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// AnalyticsResponse combines all analytics data
type AnalyticsResponse struct {
	Summary              AnalyticsSummary        `json:"summary"`
	Timeline             []TimelineDataPoint     `json:"timeline"`
	StatusDistribution   []StatusDistribution    `json:"status_distribution"`
	ChannelDistribution  []ChannelDistribution   `json:"channel_distribution,omitempty"`
	PriorityDistribution []PriorityDistribution  `json:"priority_distribution,omitempty"`
	TimeRange            string                  `json:"time_range"` // "24h", "7d", "30d"
}

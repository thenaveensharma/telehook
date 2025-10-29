package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/thenaveensharma/telehook/internal/models"
)

func (db *DB) CreateUser(ctx context.Context, username, email, passwordHash string) (*models.User, error) {
	var user models.User
	query := `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email, webhook_token, created_at, updated_at
	`

	err := db.Pool.QueryRow(ctx, query, username, email, passwordHash).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.WebhookToken,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := `
		SELECT id, username, email, password_hash, webhook_token, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	err := db.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.WebhookToken,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}

func (db *DB) GetUserByWebhookToken(ctx context.Context, token uuid.UUID) (*models.User, error) {
	var user models.User
	query := `
		SELECT id, username, email, password_hash, webhook_token, created_at, updated_at
		FROM users
		WHERE webhook_token = $1
	`

	err := db.Pool.QueryRow(ctx, query, token).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.WebhookToken,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get user by webhook token: %w", err)
	}

	return &user, nil
}

func (db *DB) CreateWebhookLog(ctx context.Context, userID int, payload map[string]interface{}, telegramResponse, status string) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		INSERT INTO webhook_logs (user_id, payload, telegram_response, status)
		VALUES ($1, $2, $3, $4)
	`

	_, err = db.Pool.Exec(ctx, query, userID, payloadJSON, telegramResponse, status)
	if err != nil {
		return fmt.Errorf("failed to create webhook log: %w", err)
	}

	return nil
}

func (db *DB) GetUserWebhookLogs(ctx context.Context, userID int, limit int) ([]models.WebhookLog, error) {
	query := `
		SELECT id, user_id, payload, telegram_response, status, sent_at
		FROM webhook_logs
		WHERE user_id = $1
		ORDER BY sent_at DESC
		LIMIT $2
	`

	rows, err := db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get webhook logs: %w", err)
	}
	defer rows.Close()

	var logs []models.WebhookLog
	for rows.Next() {
		var log models.WebhookLog
		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Payload,
			&log.TelegramResponse,
			&log.Status,
			&log.SentAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// ============================================================================
// Telegram Bot CRUD Operations
// ============================================================================

func (db *DB) CreateTelegramBot(ctx context.Context, userID int, botToken, botUsername string, isDefault bool) (*models.TelegramBot, error) {
	var bot models.TelegramBot

	// If this is set as default, unset other defaults for this user
	if isDefault {
		_, err := db.Pool.Exec(ctx, `UPDATE telegram_bots SET is_default = false WHERE user_id = $1`, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to unset other defaults: %w", err)
		}
	}

	query := `
		INSERT INTO telegram_bots (user_id, bot_token, bot_username, is_default)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, bot_token, bot_username, is_default, created_at, updated_at
	`

	err := db.Pool.QueryRow(ctx, query, userID, botToken, botUsername, isDefault).Scan(
		&bot.ID,
		&bot.UserID,
		&bot.BotToken,
		&bot.BotUsername,
		&bot.IsDefault,
		&bot.CreatedAt,
		&bot.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	return &bot, nil
}

func (db *DB) GetTelegramBot(ctx context.Context, botID, userID int) (*models.TelegramBot, error) {
	var bot models.TelegramBot
	query := `
		SELECT id, user_id, bot_token, bot_username, is_default, created_at, updated_at
		FROM telegram_bots
		WHERE id = $1 AND user_id = $2
	`

	err := db.Pool.QueryRow(ctx, query, botID, userID).Scan(
		&bot.ID,
		&bot.UserID,
		&bot.BotToken,
		&bot.BotUsername,
		&bot.IsDefault,
		&bot.CreatedAt,
		&bot.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get telegram bot: %w", err)
	}

	return &bot, nil
}

func (db *DB) GetUserTelegramBots(ctx context.Context, userID int) ([]models.TelegramBot, error) {
	query := `
		SELECT id, user_id, bot_token, bot_username, is_default, created_at, updated_at
		FROM telegram_bots
		WHERE user_id = $1
		ORDER BY is_default DESC, created_at DESC
	`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user telegram bots: %w", err)
	}
	defer rows.Close()

	var bots []models.TelegramBot
	for rows.Next() {
		var bot models.TelegramBot
		err := rows.Scan(
			&bot.ID,
			&bot.UserID,
			&bot.BotToken,
			&bot.BotUsername,
			&bot.IsDefault,
			&bot.CreatedAt,
			&bot.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan telegram bot: %w", err)
		}
		bots = append(bots, bot)
	}

	return bots, nil
}

func (db *DB) UpdateTelegramBot(ctx context.Context, botID, userID int, botToken, botUsername string, isDefault bool) (*models.TelegramBot, error) {
	// If this is set as default, unset other defaults for this user
	if isDefault {
		_, err := db.Pool.Exec(ctx, `UPDATE telegram_bots SET is_default = false WHERE user_id = $1 AND id != $2`, userID, botID)
		if err != nil {
			return nil, fmt.Errorf("failed to unset other defaults: %w", err)
		}
	}

	query := `
		UPDATE telegram_bots
		SET bot_token = COALESCE(NULLIF($1, ''), bot_token),
		    bot_username = COALESCE(NULLIF($2, ''), bot_username),
		    is_default = $3,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $4 AND user_id = $5
		RETURNING id, user_id, bot_token, bot_username, is_default, created_at, updated_at
	`

	var bot models.TelegramBot
	err := db.Pool.QueryRow(ctx, query, botToken, botUsername, isDefault, botID, userID).Scan(
		&bot.ID,
		&bot.UserID,
		&bot.BotToken,
		&bot.BotUsername,
		&bot.IsDefault,
		&bot.CreatedAt,
		&bot.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update telegram bot: %w", err)
	}

	return &bot, nil
}

func (db *DB) DeleteTelegramBot(ctx context.Context, botID, userID int) error {
	query := `DELETE FROM telegram_bots WHERE id = $1 AND user_id = $2`
	result, err := db.Pool.Exec(ctx, query, botID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete telegram bot: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("bot not found or not owned by user")
	}

	return nil
}

// ============================================================================
// Telegram Channel CRUD Operations
// ============================================================================

func (db *DB) CreateTelegramChannel(ctx context.Context, userID, botID int, identifier, channelID, channelName, description string) (*models.TelegramChannel, error) {
	var channel models.TelegramChannel
	query := `
		INSERT INTO telegram_channels (user_id, bot_id, identifier, channel_id, channel_name, description)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, bot_id, identifier, channel_id, channel_name, description, is_active, created_at, updated_at
	`

	err := db.Pool.QueryRow(ctx, query, userID, botID, identifier, channelID, channelName, description).Scan(
		&channel.ID,
		&channel.UserID,
		&channel.BotID,
		&channel.Identifier,
		&channel.ChannelID,
		&channel.ChannelName,
		&channel.Description,
		&channel.IsActive,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create telegram channel: %w", err)
	}

	return &channel, nil
}

func (db *DB) GetTelegramChannel(ctx context.Context, channelID, userID int) (*models.TelegramChannel, error) {
	var channel models.TelegramChannel
	query := `
		SELECT id, user_id, bot_id, identifier, channel_id, channel_name, description, is_active, created_at, updated_at
		FROM telegram_channels
		WHERE id = $1 AND user_id = $2
	`

	err := db.Pool.QueryRow(ctx, query, channelID, userID).Scan(
		&channel.ID,
		&channel.UserID,
		&channel.BotID,
		&channel.Identifier,
		&channel.ChannelID,
		&channel.ChannelName,
		&channel.Description,
		&channel.IsActive,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get telegram channel: %w", err)
	}

	return &channel, nil
}

func (db *DB) GetTelegramChannelByIdentifier(ctx context.Context, userID int, identifier string) (*models.TelegramChannel, error) {
	var channel models.TelegramChannel
	query := `
		SELECT id, user_id, bot_id, identifier, channel_id, channel_name, description, is_active, created_at, updated_at
		FROM telegram_channels
		WHERE user_id = $1 AND identifier = $2 AND is_active = true
	`

	err := db.Pool.QueryRow(ctx, query, userID, identifier).Scan(
		&channel.ID,
		&channel.UserID,
		&channel.BotID,
		&channel.Identifier,
		&channel.ChannelID,
		&channel.ChannelName,
		&channel.Description,
		&channel.IsActive,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get telegram channel by identifier: %w", err)
	}

	return &channel, nil
}

func (db *DB) GetUserTelegramChannels(ctx context.Context, userID int) ([]models.TelegramChannel, error) {
	query := `
		SELECT id, user_id, bot_id, identifier, channel_id, channel_name, description, is_active, created_at, updated_at
		FROM telegram_channels
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user telegram channels: %w", err)
	}
	defer rows.Close()

	var channels []models.TelegramChannel
	for rows.Next() {
		var channel models.TelegramChannel
		err := rows.Scan(
			&channel.ID,
			&channel.UserID,
			&channel.BotID,
			&channel.Identifier,
			&channel.ChannelID,
			&channel.ChannelName,
			&channel.Description,
			&channel.IsActive,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan telegram channel: %w", err)
		}
		channels = append(channels, channel)
	}

	return channels, nil
}

func (db *DB) GetBotChannels(ctx context.Context, botID, userID int) ([]models.TelegramChannel, error) {
	query := `
		SELECT id, user_id, bot_id, identifier, channel_id, channel_name, description, is_active, created_at, updated_at
		FROM telegram_channels
		WHERE bot_id = $1 AND user_id = $2
		ORDER BY created_at DESC
	`

	rows, err := db.Pool.Query(ctx, query, botID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bot channels: %w", err)
	}
	defer rows.Close()

	var channels []models.TelegramChannel
	for rows.Next() {
		var channel models.TelegramChannel
		err := rows.Scan(
			&channel.ID,
			&channel.UserID,
			&channel.BotID,
			&channel.Identifier,
			&channel.ChannelID,
			&channel.ChannelName,
			&channel.Description,
			&channel.IsActive,
			&channel.CreatedAt,
			&channel.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan channel: %w", err)
		}
		channels = append(channels, channel)
	}

	return channels, nil
}

func (db *DB) UpdateTelegramChannel(ctx context.Context, channelID, userID int, req models.UpdateChannelRequest) (*models.TelegramChannel, error) {
	query := `
		UPDATE telegram_channels
		SET bot_id = COALESCE(NULLIF($1, 0), bot_id),
		    identifier = COALESCE(NULLIF($2, ''), identifier),
		    channel_id = COALESCE(NULLIF($3, ''), channel_id),
		    channel_name = COALESCE(NULLIF($4, ''), channel_name),
		    description = COALESCE(NULLIF($5, ''), description),
		    is_active = COALESCE($6, is_active),
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $7 AND user_id = $8
		RETURNING id, user_id, bot_id, identifier, channel_id, channel_name, description, is_active, created_at, updated_at
	`

	var channel models.TelegramChannel
	err := db.Pool.QueryRow(ctx, query, req.BotID, req.Identifier, req.ChannelID, req.ChannelName, req.Description, req.IsActive, channelID, userID).Scan(
		&channel.ID,
		&channel.UserID,
		&channel.BotID,
		&channel.Identifier,
		&channel.ChannelID,
		&channel.ChannelName,
		&channel.Description,
		&channel.IsActive,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update telegram channel: %w", err)
	}

	return &channel, nil
}

func (db *DB) DeleteTelegramChannel(ctx context.Context, channelID, userID int) error {
	query := `DELETE FROM telegram_channels WHERE id = $1 AND user_id = $2`
	result, err := db.Pool.Exec(ctx, query, channelID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete telegram channel: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("channel not found or not owned by user")
	}

	return nil
}

// GetBotByID retrieves bot by ID for internal use
func (db *DB) GetBotByID(ctx context.Context, botID int) (*models.TelegramBot, error) {
	var bot models.TelegramBot
	query := `
		SELECT id, user_id, bot_token, bot_username, is_default, created_at, updated_at
		FROM telegram_bots
		WHERE id = $1
	`

	err := db.Pool.QueryRow(ctx, query, botID).Scan(
		&bot.ID,
		&bot.UserID,
		&bot.BotToken,
		&bot.BotUsername,
		&bot.IsDefault,
		&bot.CreatedAt,
		&bot.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get bot by ID: %w", err)
	}

	return &bot, nil
}

// GetDefaultTelegramChannel retrieves the first active channel for a user
func (db *DB) GetDefaultTelegramChannel(ctx context.Context, userID int) (*models.TelegramChannel, error) {
	var channel models.TelegramChannel
	query := `
		SELECT id, user_id, bot_id, identifier, channel_id, channel_name, description, is_active, created_at, updated_at
		FROM telegram_channels
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at ASC
		LIMIT 1
	`

	err := db.Pool.QueryRow(ctx, query, userID).Scan(
		&channel.ID,
		&channel.UserID,
		&channel.BotID,
		&channel.Identifier,
		&channel.ChannelID,
		&channel.ChannelName,
		&channel.Description,
		&channel.IsActive,
		&channel.CreatedAt,
		&channel.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get default telegram channel: %w", err)
	}

	return &channel, nil
}

// ============================================================================
// Analytics Queries
// ============================================================================

// GetAnalytics retrieves comprehensive analytics for a user within a time range
func (db *DB) GetAnalytics(ctx context.Context, userID int, timeRange string) (*models.AnalyticsResponse, error) {
	var response models.AnalyticsResponse
	response.TimeRange = timeRange

	// Calculate time boundaries
	var since time.Time
	now := time.Now()

	switch timeRange {
	case "24h":
		since = now.Add(-24 * time.Hour)
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
	case "30d":
		since = now.Add(-30 * 24 * time.Hour)
	default:
		since = now.Add(-24 * time.Hour)
		response.TimeRange = "24h"
	}

	// Get summary statistics
	summary, err := db.getAnalyticsSummary(ctx, userID, since, now)
	if err != nil {
		return nil, err
	}
	response.Summary = *summary

	// Get timeline data
	timeline, err := db.getAnalyticsTimeline(ctx, userID, since, now, timeRange)
	if err != nil {
		return nil, err
	}
	response.Timeline = timeline

	// Get status distribution
	statusDist, err := db.getAnalyticsByStatus(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	response.StatusDistribution = statusDist

	// Get channel distribution
	channelDist, err := db.getAnalyticsByChannel(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	response.ChannelDistribution = channelDist

	// Get priority distribution
	priorityDist, err := db.getAnalyticsByPriority(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	response.PriorityDistribution = priorityDist

	return &response, nil
}

// getAnalyticsSummary calculates overall statistics
func (db *DB) getAnalyticsSummary(ctx context.Context, userID int, since, until time.Time) (*models.AnalyticsSummary, error) {
	var summary models.AnalyticsSummary

	// Get total counts by status
	query := `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as success,
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) as failed,
			COALESCE(SUM(CASE WHEN status = 'filtered' THEN 1 ELSE 0 END), 0) as filtered,
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending,
			MAX(sent_at) as last_message
		FROM webhook_logs
		WHERE user_id = $1 AND sent_at >= $2 AND sent_at <= $3
	`

	var lastMsg *time.Time
	err := db.Pool.QueryRow(ctx, query, userID, since, until).Scan(
		&summary.TotalMessages,
		&summary.SuccessCount,
		&summary.FailedCount,
		&summary.FilteredCount,
		&summary.PendingCount,
		&lastMsg,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics summary: %w", err)
	}

	summary.LastMessageAt = lastMsg

	// Calculate success rate
	if summary.TotalMessages > 0 {
		summary.SuccessRate = float64(summary.SuccessCount) / float64(summary.TotalMessages) * 100
	}

	// Calculate averages
	hoursDiff := until.Sub(since).Hours()
	if hoursDiff > 0 {
		summary.AvgPerHour = float64(summary.TotalMessages) / hoursDiff
		summary.AvgPerDay = summary.AvgPerHour * 24
	}

	// Get peak hour
	peakQuery := `
		SELECT
			EXTRACT(HOUR FROM sent_at)::INTEGER as hour,
			COUNT(*) as count
		FROM webhook_logs
		WHERE user_id = $1 AND sent_at >= $2 AND sent_at <= $3
		GROUP BY hour
		ORDER BY count DESC
		LIMIT 1
	`

	err = db.Pool.QueryRow(ctx, peakQuery, userID, since, until).Scan(&summary.PeakHour, &summary.PeakHourCount)
	if err != nil && err.Error() != "no rows in result set" {
		// If no data, just leave peak values as 0
		if err.Error() != "no rows in result set" {
			return nil, fmt.Errorf("failed to get peak hour: %w", err)
		}
	}

	return &summary, nil
}

// getAnalyticsTimeline returns time-series data for charting
func (db *DB) getAnalyticsTimeline(ctx context.Context, userID int, since, until time.Time, timeRange string) ([]models.TimelineDataPoint, error) {
	// Determine grouping interval based on time range
	var interval string
	switch timeRange {
	case "24h":
		interval = "1 hour"
	case "7d":
		interval = "6 hours"
	case "30d":
		interval = "1 day"
	default:
		interval = "1 hour"
	}

	query := fmt.Sprintf(`
		SELECT
			date_trunc('hour', sent_at) +
			(EXTRACT(HOUR FROM sent_at)::INTEGER / CASE
				WHEN $4 = '24h' THEN 1
				WHEN $4 = '7d' THEN 6
				ELSE 24
			END) * INTERVAL '%s' as timestamp,
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) as success_count,
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) as failed_count,
			COALESCE(SUM(CASE WHEN status = 'filtered' THEN 1 ELSE 0 END), 0) as filtered_count,
			COUNT(*) as total_count
		FROM webhook_logs
		WHERE user_id = $1 AND sent_at >= $2 AND sent_at <= $3
		GROUP BY timestamp
		ORDER BY timestamp ASC
	`, interval)

	rows, err := db.Pool.Query(ctx, query, userID, since, until, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline data: %w", err)
	}
	defer rows.Close()

	var timeline []models.TimelineDataPoint
	for rows.Next() {
		var point models.TimelineDataPoint
		err := rows.Scan(
			&point.Timestamp,
			&point.SuccessCount,
			&point.FailedCount,
			&point.FilteredCount,
			&point.TotalCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan timeline data: %w", err)
		}
		timeline = append(timeline, point)
	}

	return timeline, nil
}

// getAnalyticsByStatus returns distribution of messages by status
func (db *DB) getAnalyticsByStatus(ctx context.Context, userID int, since time.Time) ([]models.StatusDistribution, error) {
	query := `
		SELECT
			status,
			COUNT(*) as count,
			(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER ()) as percentage
		FROM webhook_logs
		WHERE user_id = $1 AND sent_at >= $2
		GROUP BY status
		ORDER BY count DESC
	`

	rows, err := db.Pool.Query(ctx, query, userID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get status distribution: %w", err)
	}
	defer rows.Close()

	var distribution []models.StatusDistribution
	for rows.Next() {
		var dist models.StatusDistribution
		err := rows.Scan(&dist.Status, &dist.Count, &dist.Percentage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan status distribution: %w", err)
		}
		distribution = append(distribution, dist)
	}

	return distribution, nil
}

// getAnalyticsByChannel returns distribution of messages by channel
func (db *DB) getAnalyticsByChannel(ctx context.Context, userID int, since time.Time) ([]models.ChannelDistribution, error) {
	query := `
		SELECT
			COALESCE(
				(payload->>'identifier')::TEXT,
				'default'
			) as identifier,
			COUNT(*) as count,
			(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER ()) as percentage
		FROM webhook_logs
		WHERE user_id = $1 AND sent_at >= $2
		GROUP BY identifier
		ORDER BY count DESC
		LIMIT 10
	`

	rows, err := db.Pool.Query(ctx, query, userID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel distribution: %w", err)
	}
	defer rows.Close()

	var distribution []models.ChannelDistribution
	for rows.Next() {
		var dist models.ChannelDistribution
		err := rows.Scan(&dist.ChannelIdentifier, &dist.Count, &dist.Percentage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan channel distribution: %w", err)
		}

		// Get channel name from telegram_channels table if available
		var channelName string
		nameQuery := `
			SELECT channel_name
			FROM telegram_channels
			WHERE user_id = $1 AND identifier = $2 AND is_active = true
			LIMIT 1
		`
		err = db.Pool.QueryRow(ctx, nameQuery, userID, dist.ChannelIdentifier).Scan(&channelName)
		if err == nil && channelName != "" {
			dist.ChannelName = channelName
		}

		distribution = append(distribution, dist)
	}

	return distribution, nil
}

// getAnalyticsByPriority returns distribution of messages by priority
func (db *DB) getAnalyticsByPriority(ctx context.Context, userID int, since time.Time) ([]models.PriorityDistribution, error) {
	query := `
		SELECT
			COALESCE((payload->>'priority')::INTEGER, 3) as priority,
			COUNT(*) as count,
			(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER ()) as percentage
		FROM webhook_logs
		WHERE user_id = $1 AND sent_at >= $2
		GROUP BY priority
		ORDER BY priority ASC
	`

	rows, err := db.Pool.Query(ctx, query, userID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get priority distribution: %w", err)
	}
	defer rows.Close()

	priorityLabels := map[int]string{
		1: "Urgent",
		2: "High",
		3: "Normal",
		4: "Low",
	}

	var distribution []models.PriorityDistribution
	for rows.Next() {
		var dist models.PriorityDistribution
		err := rows.Scan(&dist.Priority, &dist.Count, &dist.Percentage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan priority distribution: %w", err)
		}
		dist.Label = priorityLabels[dist.Priority]
		distribution = append(distribution, dist)
	}

	return distribution, nil
}

// Helper function to split message and extract identifier
func splitMessage(message string) []string {
	parts := make([]string, 2)
	idx := -1

	// Look for ---- separator
	for i := 0; i < len(message)-3; i++ {
		if message[i:i+4] == "----" {
			idx = i
			break
		}
	}

	if idx == -1 {
		parts[0] = message
		parts[1] = ""
		return parts
	}

	parts[0] = message[:idx]
	if idx+4 < len(message) {
		parts[1] = message[idx+4:]
	} else {
		parts[1] = ""
	}

	// Trim whitespace and newlines
	for i := range parts {
		parts[i] = trimWhitespace(parts[i])
	}

	return parts
}

func trimWhitespace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\n' || s[start] == '\r' || s[start] == '\t') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\r' || s[end-1] == '\t') {
		end--
	}

	return s[start:end]
}

-- Migration: Multi-channel support with user-owned bots
-- Created: 2025-10-26

-- Table for storing user's Telegram bots
CREATE TABLE IF NOT EXISTS telegram_bots (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bot_token VARCHAR(255) NOT NULL,
    bot_username VARCHAR(255),
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, bot_token)
);

-- Table for storing user's Telegram channels/groups with identifiers
CREATE TABLE IF NOT EXISTS telegram_channels (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bot_id INTEGER NOT NULL REFERENCES telegram_bots(id) ON DELETE CASCADE,
    identifier VARCHAR(50) NOT NULL, -- e.g., "tg", "alerts", "vip"
    channel_id VARCHAR(255) NOT NULL, -- e.g., "@channel" or "-1001234567890"
    channel_name VARCHAR(255), -- Friendly name for display
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, identifier)
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_telegram_bots_user_id ON telegram_bots(user_id);
CREATE INDEX IF NOT EXISTS idx_telegram_channels_user_id ON telegram_channels(user_id);
CREATE INDEX IF NOT EXISTS idx_telegram_channels_identifier ON telegram_channels(user_id, identifier);
CREATE INDEX IF NOT EXISTS idx_telegram_channels_bot_id ON telegram_channels(bot_id);

-- Update webhook_logs to track which channel was used
ALTER TABLE webhook_logs
ADD COLUMN IF NOT EXISTS channel_id INTEGER REFERENCES telegram_channels(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_webhook_logs_channel_id ON webhook_logs(channel_id);

-- Add comment for documentation
COMMENT ON TABLE telegram_bots IS 'Stores user-owned Telegram bot tokens';
COMMENT ON TABLE telegram_channels IS 'Stores user channel configurations with custom identifiers';
COMMENT ON COLUMN telegram_channels.identifier IS 'Custom identifier used in message routing (e.g., tg, alerts, vip)';
COMMENT ON COLUMN telegram_channels.channel_id IS 'Telegram channel/group ID or username';

/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package utils

import "regexp"

var (
	TelegramMessageRegex = regexp.MustCompile(`^https://t\.me/([a-zA-Z0-9_]{4,})/(\d+)$`)
)

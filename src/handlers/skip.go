/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Ashok Shau
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/saregama_go_music
 */

package handlers

import (
	"suraj08832/tgmusic/src/core/cache"
	"suraj08832/tgmusic/src/vc"

	"github.com/amarnathcjd/gogram/telegram"
)

// skipHandler handles the /skip command.
func skipHandler(m *telegram.NewMessage) error {
	chatID := m.ChannelID()
	if !cache.ChatCache.IsActive(chatID) {
		_, _ = m.Reply("⏸ Nothing is playing.")
		return nil
	}

	_ = vc.Calls.PlayNext(chatID)
	return nil
}

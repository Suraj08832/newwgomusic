/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Ashok Shau
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/saregama_go_music
 */

package handlers

import (
	"suraj08832/tgmusic/src/core"
	"suraj08832/tgmusic/src/core/cache"
	"fmt"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
)

func handleVoiceChatMessage(m *telegram.NewMessage) error {
	if m.Action == nil {
		return nil
	}

	chatID := m.ChannelID()
	client := m.Client

	// Chat is not a Supergroup
	if m.Channel == nil {
		text := fmt.Sprintf(
			"This chat (%d) is not a supergroup yet.\n<b>⚠️ Please convert this chat to a supergroup and add me as admin.</b>\n\nIf you don't know how to convert, use this guide:\n🔗 https://te.legra.ph/How-to-Convert-a-Group-to-a-Supergroup-01-02\n\nIf you have any questions, join our support group:",
			chatID,
		)

		_, _ = client.SendMessage(chatID, text, &telegram.SendOptions{
			ReplyMarkup: core.AddMeMarkup(client.Me().Username),
			LinkPreview: false,
		})

		time.Sleep(1 * time.Second)
		_ = client.LeaveChannel(chatID)
		return nil
	}

	action, ok := m.Action.(*telegram.MessageActionGroupCall)
	if !ok {
		return telegram.ErrEndGroup
	}

	var message string

	if action.Duration == 0 {
		cache.ChatCache.ClearChat(chatID)
		message = "🎙️ Video chat started!\nUse /play <song name> to play music."
	} else {
		cache.ChatCache.ClearChat(chatID)
		logger.Info("Voice chat ended. Duration: %d seconds", action.Duration)
		message = "🎧 Video chat ended!\nAll queues cleared."
	}

	_, _ = m.Client.SendMessage(chatID, message)
	return telegram.ErrEndGroup
}

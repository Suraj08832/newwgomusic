/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package handlers

import (
	"fmt"
	"strings"

	"suraj08832/tgmusic/src/core/db"

	"github.com/amarnathcjd/gogram/telegram"
)

// autoplayHandler handles the /autoplay command.
// It toggles the personal autoplay preference for the user.
//
// Usage:
//   /autoplay           -> toggles current state
//   /autoplay on|enable -> explicitly enable
//   /autoplay off|disable -> explicitly disable
func autoplayHandler(m *telegram.NewMessage) error {
	ctx, cancel := db.Ctx()
	defer cancel()

	userID := m.SenderID()
	current := db.Instance.GetAutoplayStatus(ctx, userID)

	args := strings.ToLower(strings.TrimSpace(m.Args()))

	var newStatus bool
	var replyText string

	switch args {
	case "on", "enable":
		newStatus = true
		replyText = "✅ Autoplay has been <b>enabled</b> for you."
	case "off", "disable":
		newStatus = false
		replyText = "✅ Autoplay has been <b>disabled</b> for you."
	case "":
		newStatus = !current
		if newStatus {
			replyText = "✅ Autoplay is now <b>enabled</b> for you."
		} else {
			replyText = "✅ Autoplay is now <b>disabled</b> for you."
		}
	default:
		status := "disabled"
		if current {
			status = "enabled"
		}
		_, err := m.Reply(
			fmt.Sprintf("Usage: <code>/autoplay [on|off]</code>\nCurrent status: <b>%s</b>.", status),
		)
		if err != nil {
			return err
		}
		return telegram.ErrEndGroup
	}

	if err := db.Instance.SetAutoplayStatus(ctx, userID, newStatus); err != nil {
		_, _ = m.Reply("⚠️ Failed to update autoplay setting. Please try again later.")
		return err
	}

	_, err := m.Reply(replyText)
	return err
}



/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package ubot

func (ctx *Context) Unmute(chatId int64) (bool, error) {
	return ctx.binding.UnMute(chatId)
}

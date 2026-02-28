/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Ashok Shau
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/saregama_go_music
 */

package ubot

func (ctx *Context) Resume(chatId int64) (bool, error) {
	return ctx.binding.Resume(chatId)
}

/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package ubot

import "suraj08832/tgmusic/src/vc/ntgcalls"

func (ctx *Context) Time(chatId int64, streamMode ntgcalls.StreamMode) (uint64, error) {
	return ctx.binding.Time(chatId, streamMode)
}

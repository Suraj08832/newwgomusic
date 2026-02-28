/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Ashok Shau
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/saregama_go_music
 */

package ubot

import "ashokshau/tgmusic/src/vc/ntgcalls"

func (ctx *Context) Time(chatId int64, streamMode ntgcalls.StreamMode) (uint64, error) {
	return ctx.binding.Time(chatId, streamMode)
}

/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package ubot

import "suraj08832/tgmusic/src/vc/ntgcalls"

func (ctx *Context) Record(chatId int64, mediaDescription ntgcalls.MediaDescription) error {

	if ctx.binding.Calls()[chatId] == nil {
		err := ctx.Play(chatId, ntgcalls.MediaDescription{})
		if err != nil {
			return err
		}
	}
	return ctx.binding.SetStreamSources(chatId, ntgcalls.PlaybackStream, mediaDescription)
}

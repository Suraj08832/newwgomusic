/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package ubot

import "fmt"

func (ctx *Context) convertCallId(callId int64) (int64, error) {
	for chatId, inputCall := range ctx.inputCalls {
		if inputCall.ID == callId {
			return chatId, nil
		}
	}
	return 0, fmt.Errorf("call id %d not found", callId)
}

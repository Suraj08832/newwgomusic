/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package types

import (
	"time"

	tg "github.com/amarnathcjd/gogram/telegram"
)

type CallParticipantsCache struct {
	CallParticipants  map[int64]*tg.GroupCallParticipant
	LastMtprotoUpdate time.Time
}

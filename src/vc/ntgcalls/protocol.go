/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package ntgcalls

type Protocol struct {
	MinLayer     int32
	MaxLayer     int32
	UdpP2P       bool
	UdpReflector bool
	Versions     []string
}

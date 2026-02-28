/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package src

import (
	"suraj08832/tgmusic/config"
	"suraj08832/tgmusic/src/core/db"
	"suraj08832/tgmusic/src/handlers"
	"suraj08832/tgmusic/src/vc"
	"context"

	tg "github.com/amarnathcjd/gogram/telegram"
)

func Init(client *tg.Client) error {
	if err := db.InitDatabase(context.Background()); err != nil {
		return err
	}

	for _, session := range config.Conf.SessionStrings {
		_, err := vc.Calls.StartClient(config.Conf.ApiId, config.Conf.ApiHash, session)
		if err != nil {
			return err
		}
	}

	vc.Calls.RegisterHandlers(client)
	handlers.LoadModules(client)

	return nil
}

/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package vc

/*
#cgo linux LDFLAGS: -L . -lntgcalls -lm -lz
#cgo darwin LDFLAGS: -L . -lntgcalls -lc++ -lz -lbz2 -liconv -framework AVFoundation -framework AudioToolbox -framework CoreAudio -framework QuartzCore -framework CoreMedia -framework VideoToolbox -framework AppKit -framework Metal -framework MetalKit -framework OpenGL -framework IOSurface -framework ScreenCaptureKit

// Currently is supported only dynamically linked library on Windows due to
// https://github.com/golang/go/issues/63903
#cgo windows LDFLAGS: -L. -lntgcalls
#include "ntgcalls/ntgcalls.h"
#include "glibc_compatibility.h"
*/
import "C"

import (
	"suraj08832/tgmusic/config"
	"suraj08832/tgmusic/src/core"
	"suraj08832/tgmusic/src/utils"
	"suraj08832/tgmusic/src/vc/ntgcalls"
	"suraj08832/tgmusic/src/vc/sessions"
	"suraj08832/tgmusic/src/vc/ubot"
	"context"
	"crypto/rand"
	"time"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"suraj08832/tgmusic/src/core/cache"
	"suraj08832/tgmusic/src/core/db"
	"suraj08832/tgmusic/src/core/dl"

	tg "github.com/amarnathcjd/gogram/telegram"
)

const DefaultStreamURL = "https://t.me/FallenSongs/1295"

// getClientName selects an assistant client for a given chat. It prioritizes existing assignments from the database.
func (c *TelegramCalls) getClientName(chatID int64) (string, error) {
	c.mu.RLock()
	if len(c.availableClients) == 0 {
		c.mu.RUnlock()
		return "", fmt.Errorf("no clients are available")
	}
	availableClients := make([]string, len(c.availableClients))
	copy(availableClients, c.availableClients)
	c.mu.RUnlock()

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(availableClients))))
	if err != nil {
		log.Printf("[TelegramCalls] Could not generate a random number: %v", err)
		return availableClients[0], nil
	}
	newClient := availableClients[n.Int64()]

	ctx, cancel := db.Ctx()
	defer cancel()

	assignedClient, err := db.Instance.AssignAssistant(ctx, chatID, newClient)
	if err != nil {
		c.bot.Log.Info("[TelegramCalls] DB.AssignAssistant error: %v", err)
	}

	if assignedClient != "" {
		isAvailable := false
		for _, name := range availableClients {
			if name == assignedClient {
				isAvailable = true
				break
			}
		}

		if isAvailable {
			return assignedClient, nil
		}

		c.bot.Log.Info("[TelegramCalls] Assigned assistant %s is unavailable. Overwriting with %s.", assignedClient, newClient)
		if err = db.Instance.SetAssistant(ctx, chatID, newClient); err != nil {
			c.bot.Log.Info("[TelegramCalls] DB.SetAssistant error: %v", err)
		}
		return newClient, nil
	}

	if err = db.Instance.SetAssistant(ctx, chatID, newClient); err != nil {
		c.bot.Log.Info("[TelegramCalls] DB.SetAssistant error: %v", err)
	}

	c.bot.Log.Info("[TelegramCalls] An assistant has been set for chat %d -> %s", chatID, newClient)
	return newClient, nil
}

// GetGroupAssistant retrieves the ubot.Context for a given chat, which is used to interact with the voice call.
func (c *TelegramCalls) GetGroupAssistant(chatID int64) (*ubot.Context, error) {
	clientName, err := c.getClientName(chatID)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	call, ok := c.uBContext[clientName]
	if !ok {
		return nil, fmt.Errorf("no ntgcalls instance was found for %s", clientName)
	}
	return call, nil
}

// StartClient initializes a new userbot client and adds it to the pool of available assistants.
// It authenticates with Telegram using the provided API ID, API hash, and session string.
// The session type is determined by the configuration (pyrogram, telethon, or gogram).
func (c *TelegramCalls) StartClient(apiID int32, apiHash, stringSession string) (*ubot.Context, error) {
	c.mu.Lock()
	clientName := fmt.Sprintf("client%d", c.clientCounter)
	c.clientCounter++
	c.mu.Unlock()

	var sess *tg.Session
	var err error

	clientConfig := tg.ClientConfig{
		AppID:         apiID,
		AppHash:       apiHash,
		MemorySession: true,
		SessionName:   clientName,
		FloodHandler:  handleFlood,
		LogLevel:      tg.InfoLevel,
	}

	switch config.Conf.SessionType {
	case "telethon":
		sess, err = sessions.DecodeTelethonSessionString(stringSession)
		if err != nil {
			return nil, fmt.Errorf("failed to decode telethon session string for %s: %w", clientName, err)
		}
		clientConfig.StringSession = sess.Encode()
	case "pyrogram":
		sess, err = sessions.DecodePyrogramSessionString(stringSession)
		if err != nil {
			return nil, fmt.Errorf("failed to decode pyrogram session string for %s: %w", clientName, err)
		}
		clientConfig.StringSession = sess.Encode()
	case "gogram":
		clientConfig.StringSession = stringSession
	default:
		return nil, fmt.Errorf("unsupported session type: %s", config.Conf.SessionType)
	}

	mtProto, err := tg.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create the MTProto client: %w", err)
	}

	if err := mtProto.Start(); err != nil {
		return nil, fmt.Errorf("failed to start the client: %w", err)
	}

	if mtProto.Me().Bot {
		_ = mtProto.Stop()
		return nil, fmt.Errorf("the client %s is a bot", clientName)
	}

	call, err := ubot.NewInstance(mtProto)
	if err != nil {
		_ = mtProto.Stop()
		return nil, fmt.Errorf("failed to create the ubot instance: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.uBContext[clientName] = call
	c.clients[clientName] = mtProto
	c.availableClients = append(c.availableClients, clientName)

	mtProto.Logger.Info("[TelegramCalls] client %s has started successfully.", clientName)
	return call, nil
}

// StopAllClients gracefully stops all active userbot clients and their associated voice calls.
func (c *TelegramCalls) StopAllClients() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, call := range c.uBContext {
		call.Close()
	}

	for name, client := range c.clients {
		c.bot.Log.Info("[TelegramCalls] Stopping the client: %s", name)
		_ = client.Stop()
	}
}

// PlayMedia plays media in a voice chat using ffmpeg. It downloads the file if necessary
// and updates the cache and logger status.
func (c *TelegramCalls) PlayMedia(chatID int64, filePath string, video bool, ffmpegParameters string) error {
	call, err := c.GetGroupAssistant(chatID)
	if err != nil {
		return err
	}
	ctx, cancel := db.Ctx()
	defer cancel()

	if chatID < 0 {
		if err := c.joinAssistant(chatID, call.App.Me().ID); err != nil {
			cache.ChatCache.ClearChat(chatID)
			return err
		}
	} else {
		_, _ = call.App.ResolvePeer(chatID)
	}

	c.bot.Log.Debugf("Playing media in chat %d: %s", chatID, filePath)

	mediaDesc := getMediaDescription(filePath, video, ffmpegParameters)
	if err = call.Play(chatID, mediaDesc); err != nil {
		logger.Error("Failed to play the media: %v", err)
		cache.ChatCache.ClearChat(chatID)
		return fmt.Errorf("playback failed: %w", err)
	}

	if db.Instance.GetLoggerStatus(ctx, c.bot.Me().ID) {
		go sendLogger(c.bot, chatID, cache.ChatCache.GetPlayingTrack(chatID))
	}

	return nil
}

// downloadAndPrepareSong handles the download and preparation of a song for playback.
// It returns an error if the download or preparation fails.
func (c *TelegramCalls) downloadAndPrepareSong(song *utils.CachedTrack, reply *tg.NewMessage) error {
	if song.FilePath != "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	dlPath, err := dl.DownloadSong(ctx, song, c.bot)
	if err != nil {
		_, _ = reply.Edit("⚠️ Download failed. Skipping track...")
		return err
	}

	song.FilePath = dlPath
	if song.FilePath == "" {
		_, _ = reply.Edit("⚠️ Download failed. Skipping track...")
		return errors.New("download failed due to an empty file path")
	}

	return nil
}

// PlayNext plays the next song in the queue, handles looping, and notifies the chat when the queue is finished.
func (c *TelegramCalls) PlayNext(chatID int64) error {
	loop := cache.ChatCache.GetLoopCount(chatID)
	if loop > 0 {
		cache.ChatCache.SetLoopCount(chatID, loop-1)
		if currentsSong := cache.ChatCache.GetPlayingTrack(chatID); currentsSong != nil {
			return c.playSong(chatID, currentsSong)
		}
	}

	if nextSong := cache.ChatCache.GetUpcomingTrack(chatID); nextSong != nil {
		cache.ChatCache.RemoveCurrentSong(chatID)
		return c.playSong(chatID, nextSong)
	}

	// No upcoming song in the queue; try autoplay based on the current user's preference.
	if c.tryAutoplay(chatID) {
		if nextSong := cache.ChatCache.GetUpcomingTrack(chatID); nextSong != nil {
			cache.ChatCache.RemoveCurrentSong(chatID)
			return c.playSong(chatID, nextSong)
		}
	}

	cache.ChatCache.RemoveCurrentSong(chatID)
	return c.handleNoSong(chatID)
}

// tryAutoplay attempts to enqueue a related track when the queue is empty
// and the requester has enabled autoplay. It returns true if a new track was
// successfully added to the queue.
func (c *TelegramCalls) tryAutoplay(chatID int64) bool {
	currentSong := cache.ChatCache.GetPlayingTrack(chatID)
	if currentSong == nil || currentSong.UserID == 0 {
		return false
	}

	// Enforce a maximum number of autoplay songs in a row per "chain".
	// Each time the queue finishes and autoplay kicks in, we allow up to
	// 4 songs to be auto-added. After that, autoplay stops until the user
	// plays something manually again (which resets the counter).
	remaining := cache.ChatCache.GetAutoplayRemaining(chatID)
	if remaining == 0 {
		// Limit reached for this chain.
		return false
	}
	if remaining < 0 {
		// Autoplay hasn't started yet for this chain; initialize the limit.
		remaining = 4
	}

	ctx, cancel := db.Ctx()
	defer cancel()

	if !db.Instance.GetAutoplayStatus(ctx, currentSong.UserID) {
		return false
	}

	searchCtx, searchCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer searchCancel()

	langHint := detectAutoplayLanguageHint(currentSong.Name, currentSong.Channel)

	// Build a set of queries to try, prioritizing:
	//  1) Artist/singer name (to get other songs from that artist)
	//  2) Channel name
	//  3) Song name + artist (fallback)
	//  4) Song name alone (last resort)
	//  5) URL as a very last resort.
	var queries []string

	artist := strings.TrimSpace(currentSong.Channel)
	// Try to extract a cleaner artist name from patterns like "Vilen - Topic", "Vilen Official", etc.
	if artist != "" {
		for _, sep := range []string{"-", "|", "•", ","} {
			if idx := strings.Index(artist, sep); idx > 0 {
				artist = strings.TrimSpace(artist[:idx])
				break
			}
		}
	}

	if artist != "" {
		queries = append(queries, artist)
	}
	if artist != "" {
		queries = append(queries, artist+" songs")
	}
	// When we have a language hint (e.g. "bhojpuri", "hindi"), bias the query so
	// YouTube search stays in the same lane for autoplay.
	if langHint != "" && artist != "" {
		queries = append(queries, artist+" "+langHint+" song")
		queries = append(queries, artist+" "+langHint+" songs")
	}
	if currentSong.Channel != "" && currentSong.Channel != artist {
		queries = append(queries, currentSong.Channel)
	}
	// Title-based searches are more likely to return variants of the same song,
	// so we only use them as a fallback (and still apply strong de-dup filters).
	if currentSong.Name != "" && artist != "" {
		queries = append(queries, currentSong.Name+" "+artist)
	}
	if currentSong.Name != "" {
		queries = append(queries, currentSong.Name)
	}
	if langHint != "" && currentSong.Name != "" {
		queries = append(queries, currentSong.Name+" "+langHint+" song")
	}
	if currentSong.URL != "" {
		queries = append(queries, currentSong.URL)
	}
	queries = dedupeAutoplayQueries(queries)

	var next utils.MusicTrack
	baseTitleNorm := normalizeTitleForAutoplay(currentSong.Name)
	for _, q := range queries {
		if strings.TrimSpace(q) == "" {
			continue
		}

		wrapper := dl.NewDownloaderWrapper(q)
		tracks, err := wrapper.Search(searchCtx)
		if err != nil || tracks.Results == nil || len(tracks.Results) == 0 {
			continue
		}

		var candidates []utils.MusicTrack
		for _, t := range tracks.Results {
			if t.Id == "" {
				continue
			}

			// Filter out obvious non‑music content (full episodes, cartoons, very long
			// or very short clips, etc.) so autoplay stays focused on songs.
			if !isLikelyMusicTrack(t, langHint) {
				continue
			}

			// Skip the exact same video and anything we've already played recently
			// in this chat to avoid looping between the same few tracks.
			if t.Id == currentSong.TrackID || cache.ChatCache.WasPlayed(chatID, t.Id) {
				continue
			}

			// Also skip the same song in different uploads (lyrics, official video, etc.)
			candTitleNorm := normalizeTitleForAutoplay(t.Title)
			if baseTitleNorm != "" && candTitleNorm == baseTitleNorm {
				continue
			}
			// And skip any song whose normalized title was already played recently in this chat.
			if candTitleNorm != "" && cache.ChatCache.WasTitlePlayed(chatID, candTitleNorm) {
				continue
			}

			candidates = append(candidates, t)
		}

		if len(candidates) == 0 {
			continue
		}

		// Pick a truly random candidate from the filtered list so autoplay feels
		// less repetitive even when search results are similar every time.
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
		if err != nil {
			next = candidates[0]
		} else {
			next = candidates[n.Int64()]
		}

		// One autoplay slot is now being used for this chain.
		remaining--
		cache.ChatCache.SetAutoplayRemaining(chatID, remaining)
		break
	}

	if next.Id == "" {
		// No suitable related track found.
		return false
	}

	newTrack := &utils.CachedTrack{
		URL:       next.Url,
		Name:      next.Title,
		Loop:      0,
		User:      currentSong.User,
		UserID:    currentSong.UserID,
		FilePath:  "",
		Thumbnail: next.Thumbnail,
		TrackID:   next.Id,
		Duration:  next.Duration,
		Channel:   next.Channel,
		Views:     next.Views,
		IsVideo:   currentSong.IsVideo,
		Platform:  next.Platform,
	}

	cache.ChatCache.AddSong(chatID, newTrack)
	logger.Info("[Autoplay] Added related track %s (%s) for chat %d", newTrack.Name, newTrack.URL, chatID)
	return true
}

// handleNoSong manages the situation where there are no more songs in the queue by stopping the playback
// and sending a notification to the chat.
func (c *TelegramCalls) handleNoSong(chatID int64) error {
	_ = c.Stop(chatID)
	_, _ = c.bot.SendMessage(chatID, "🎵 Queue finished. Add more songs with /play.")
	return nil
}

// playSong downloads and plays a single song. It sends a message to the chat to indicate the download status
// and updates it with the song's information once playback begins.
func (c *TelegramCalls) playSong(chatID int64, song *utils.CachedTrack) error {
	reply, err := c.bot.SendMessage(chatID, fmt.Sprintf("Downloading %s...", song.Name))
	if err != nil {
		c.bot.Log.Info("[playSong] Failed to send message: %v", err)
		return err
	}

	if err = c.downloadAndPrepareSong(song, reply); err != nil {
		return c.PlayNext(chatID)
	}

	if err = c.PlayMedia(chatID, song.FilePath, song.IsVideo, ""); err != nil {
		_, err := reply.Edit(err.Error())
		return err
	}

	// Track this song in the chat's playback history to improve autoplay variety.
	if song.TrackID != "" {
		cache.ChatCache.MarkPlayed(chatID, song.TrackID)
	}
	if normTitle := normalizeTitleForAutoplay(song.Name); normTitle != "" {
		cache.ChatCache.MarkTitlePlayed(chatID, normTitle)
	}

	if song.Duration == 0 {
		song.Duration = utils.GetMediaDuration(song.FilePath)
	}

	text := fmt.Sprintf(
		"<b>Now Playing:</b>\n\n<b>Track:</b> <a href='%s'>%s</a>\n<b>Duration:</b> %s\n<b>By:</b> %s",
		song.URL,
		song.Name,
		utils.SecToMin(song.Duration),
		song.User,
	)

	// Generate thumbnail if it's a YouTube video
	var thumbPath string
	if song.Platform == utils.YouTube && song.TrackID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		playerUsername := c.bot.Me().Username
		if playerUsername == "" {
			playerUsername = "tgmusicbot"
		}
		if thumb, err := core.GetThumb(ctx, song.TrackID, playerUsername); err == nil {
			thumbPath = thumb
		}
	}

	sendOpts := &tg.SendOptions{
		ReplyMarkup: core.ControlButtons("play"),
	}
	if thumbPath != "" {
		sendOpts.Media = thumbPath
	}

	_, err = reply.Edit(text, sendOpts)
	if err != nil {
		c.bot.Log.Warn("[playSong] Failed to edit message: %v", err)
		return nil
	}

	return nil
}

// Stop halts media playback in a voice chat and clears the chat's cache.
func (c *TelegramCalls) Stop(chatId int64) error {
	call, err := c.GetGroupAssistant(chatId)
	if err != nil {
		return err
	}
	cache.ChatCache.ClearChat(chatId)
	err = call.Stop(chatId)
	if err != nil {
		c.bot.Log.Error("[Stop] Failed to stop the call: %v", err)
		return err
	}
	return nil
}

// Pause temporarily stops media playback in a voice chat.
// It returns true if the operation was successful, and an error otherwise.
func (c *TelegramCalls) Pause(chatId int64) (bool, error) {
	call, err := c.GetGroupAssistant(chatId)
	if err != nil {
		return false, err
	}

	res, err := call.Pause(chatId)
	if err != nil {
		c.bot.Log.Error("[Pause] Failed to pause the call: %v", err)
	}
	return res, err
}

// Resume continues a paused media playback in a voice chat.
// It returns true if the operation was successful, and an error otherwise.
func (c *TelegramCalls) Resume(chatId int64) (bool, error) {
	call, err := c.GetGroupAssistant(chatId)
	if err != nil {
		return false, err
	}
	res, err := call.Resume(chatId)
	if err != nil {
		c.bot.Log.Error("[Resume] Failed to resume the call: %v", err)
	}
	return res, err
}

// Mute silences the media playback in a voice chat.
// It returns true if the operation was successful, and an error otherwise.
func (c *TelegramCalls) Mute(chatId int64) (bool, error) {
	call, err := c.GetGroupAssistant(chatId)
	if err != nil {
		return false, err
	}
	res, err := call.Mute(chatId)
	if err != nil {
		c.bot.Log.Error("[Mute] Failed to mute the call: %v", err)
	}
	return res, err
}

// Unmute restores the audio of a muted media playback in a voice chat.
// It returns true if the operation was successful, and an error otherwise.
func (c *TelegramCalls) Unmute(chatId int64) (bool, error) {
	call, err := c.GetGroupAssistant(chatId)
	if err != nil {
		return false, err
	}
	res, err := call.Unmute(chatId)
	if err != nil {
		c.bot.Log.Error("[Unmute] Failed to unmute the call: %v", err)
	}
	return res, err
}

// PlayedTime retrieves the elapsed time of the current playback in a voice chat.
// It returns the elapsed time in seconds and an error if any.
func (c *TelegramCalls) PlayedTime(chatId int64) (uint64, error) {
	call, err := c.GetGroupAssistant(chatId)
	if err != nil {
		return 0, err
	}

	// TODO: Pass the streamMode.
	return call.Time(chatId, 0)
}

var urlRegex = regexp.MustCompile(`^https?://`)

// normalizeTitleForAutoplay tries to reduce titles like
// "Aur Iss Dil Mein - Lyrical", "Aur Iss Dil Mein (Lyrics)",
// "Aur Iss Dil Mein | Official Video" to a common base form so
// we can avoid picking multiple uploads of essentially the same song.
func normalizeTitleForAutoplay(s string) string {
	s = strings.ToLower(s)
	// Remove bracket/parenthesis content.
	brackets := regexp.MustCompile(`[\(\[\{].*?[\)\]\}]`)
	s = brackets.ReplaceAllString(s, " ")

	// Remove common noise words.
	noise := []string{
		"official video", "official lyric video", "official music video", "music video",
		"lyric video", "lyrics video", "lyrical", "lyrical video", "lyrics",
		"audio", "audio song", "full video", "full song", "video song",
		"remix", "mix", "edit", "version", "extended",
		"lofi", "lo-fi", "slowed", "reverb", "slowed + reverb", "slowed reverb",
		"8d", "8d audio", "bass boosted",
		"feat.", "ft.", "ltd.", "hd", "4k",
		"|", "•",
	}
	for _, w := range noise {
		s = strings.ReplaceAll(s, w, " ")
	}

	// Collapse extra spaces.
	spaceRe := regexp.MustCompile(`\s+`)
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// normalizeArtistForAutoplay cleans up channel/artist names like
// "Vilen - Topic", "Vilen Official", etc. so we can compare singers
// and avoid autoplaying back-to-back songs from the same artist.
func normalizeArtistForAutoplay(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}

	noise := []string{
		"official", "topic", "music", "channel",
		" - official", " - topic",
	}
	for _, w := range noise {
		s = strings.ReplaceAll(s, w, " ")
	}

	spaceRe := regexp.MustCompile(`\s+`)
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// isLikelyMusicTrack tries to detect if a search result looks like a normal song
// and not a full episode, cartoon block, gameplay, podcast, etc.
func isLikelyMusicTrack(t utils.MusicTrack, langHint string) bool {
	title := strings.ToLower(strings.TrimSpace(t.Title))
	channel := strings.ToLower(strings.TrimSpace(t.Channel))
	combined := title + " " + channel

	// Obvious non‑music keywords we want to avoid in autoplay.
	badKeywords := []string{
		"full episode", "episodes", "cartoon network", "cartoon", "anime episode",
		"gameplay", "walkthrough", "let's play", "highlights", "live stream",
		"podcast", "talk show", "behind the scenes", "interview",
		"commercials", "ads", "advertisement",
		// Movie / non-music
		"full movie", "movie clip", "movie scene", "movie scenes", "scene", "scenes", "dialogue", "dialogues",
		// News / conflict / violence (common autoplay drift issue)
		"news", "breaking", "live news", "update", "war", "battle", "fight", "fighting", "attack",
		"iran", "israel", "gaza", "palestine", "hamas", "ukraine", "russia",
		// Promos
		"trailer", "teaser",
	}
	for _, w := range badKeywords {
		if strings.Contains(combined, w) {
			return false
		}
	}

	// Autoplay needs to be stricter than manual playback: it should bias toward
	// "normal song length" to avoid drifting into long videos (movies, debates, etc.).
	// Default upper bound is 15 minutes, but if the global duration limit is set
	// lower, respect that.
	maxDur := int64(15 * 60)
	if config.Conf != nil && config.Conf.SongDurationLimit > 0 && config.Conf.SongDurationLimit < maxDur {
		maxDur = config.Conf.SongDurationLimit
	}
	if t.Duration > 0 && int64(t.Duration) > maxDur {
		return false
	}

	// Also skip ultra‑short clips that are unlikely to be full songs.
	if t.Duration > 0 && t.Duration < 45 {
		return false
	}

	// If we can infer language from the currently playing track (e.g. bhojpuri),
	// keep autoplay in the same language lane to avoid unrelated suggestions.
	if langHint != "" && !matchesAutoplayLanguageHint(langHint, t.Title, t.Channel) {
		return false
	}

	return true
}

func dedupeAutoplayQueries(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, q := range in {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		key := strings.ToLower(q)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, q)
	}
	return out
}

// detectAutoplayLanguageHint returns a short language tag to bias autoplay.
// It's intentionally heuristic-based and only returns a value when we're confident.
func detectAutoplayLanguageHint(title, channel string) string {
	s := strings.ToLower(strings.TrimSpace(title + " " + channel))
	type hint struct {
		tag   string
		words []string
	}
	hints := []hint{
		{tag: "bhojpuri", words: []string{"bhojpuri", "भोजपुरी"}},
		{tag: "hindi", words: []string{"hindi", "हिंदी"}},
		{tag: "punjabi", words: []string{"punjabi", "ਪੰਜਾਬੀ"}},
		{tag: "tamil", words: []string{"tamil", "தமிழ்"}},
		{tag: "telugu", words: []string{"telugu", "తెలుగు"}},
		{tag: "malayalam", words: []string{"malayalam", "മലയാളം"}},
		{tag: "kannada", words: []string{"kannada", "ಕನ್ನಡ"}},
		{tag: "bengali", words: []string{"bengali", "bangla", "বাংলা"}},
		{tag: "marathi", words: []string{"marathi", "मराठी"}},
		{tag: "gujarati", words: []string{"gujarati", "ગુજરાતી"}},
		{tag: "urdu", words: []string{"urdu", "اردو"}},
		{tag: "arabic", words: []string{"arabic", "العربية", "عربي"}},
		{tag: "persian", words: []string{"persian", "farsi", "فارسی"}},
		{tag: "hebrew", words: []string{"hebrew", "עברית"}},
	}
	for _, h := range hints {
		for _, w := range h.words {
			if strings.Contains(s, strings.ToLower(w)) {
				return h.tag
			}
		}
	}
	return ""
}

func matchesAutoplayLanguageHint(langHint, title, channel string) bool {
	combined := strings.ToLower(strings.TrimSpace(title + " " + channel))

	containsAny := func(needles []string) bool {
		for _, n := range needles {
			if strings.Contains(combined, strings.ToLower(n)) {
				return true
			}
		}
		return false
	}

	switch langHint {
	case "bhojpuri":
		return containsAny([]string{"bhojpuri", "भोजपुरी"})
	case "hindi":
		return containsAny([]string{"hindi", "हिंदी"})
	case "punjabi":
		return containsAny([]string{"punjabi", "ਪੰਜਾਬੀ"})
	case "tamil":
		return containsAny([]string{"tamil", "தமிழ்"})
	case "telugu":
		return containsAny([]string{"telugu", "తెలుగు"})
	case "malayalam":
		return containsAny([]string{"malayalam", "മലയാളം"})
	case "kannada":
		return containsAny([]string{"kannada", "ಕನ್ನಡ"})
	case "bengali":
		return containsAny([]string{"bengali", "bangla", "বাংলা"})
	case "marathi":
		return containsAny([]string{"marathi", "मराठी"})
	case "gujarati":
		return containsAny([]string{"gujarati", "ગુજરાતી"})
	case "urdu":
		return containsAny([]string{"urdu", "اردو"})
	case "arabic":
		return containsAny([]string{"arabic", "العربية", "عربي"})
	case "persian":
		return containsAny([]string{"persian", "farsi", "فارسی"})
	case "hebrew":
		return containsAny([]string{"hebrew", "עברית"})
	default:
		return true
	}
}

// SeekStream jumps to a specific time in the current media stream.
func (c *TelegramCalls) SeekStream(chatID int64, filePath string, toSeek, duration int, isVideo bool) error {
	if toSeek < 0 || duration <= 0 {
		return errors.New("invalid seek position or duration. The position must be positive and the duration must be greater than 0")
	}

	isURL := urlRegex.MatchString(filePath)
	_, err := os.Stat(filePath)
	isFile := err == nil

	var ffmpegParams string
	if isURL || !isFile {
		ffmpegParams = fmt.Sprintf("-ss %d -i %s -to %d", toSeek, filePath, duration)
	} else {
		ffmpegParams = fmt.Sprintf("-ss %d -to %d", toSeek, duration)
	}

	return c.PlayMedia(chatID, filePath, isVideo, ffmpegParams)
}

// ChangeSpeed modifies the playback speed of the current stream.
func (c *TelegramCalls) ChangeSpeed(chatID int64, speed float64) error {
	if speed < 0.5 || speed > 4.0 {
		return errors.New("invalid speed. Value must be between 0.5 and 4.0")
	}

	playingSong := cache.ChatCache.GetPlayingTrack(chatID)
	if playingSong == nil {
		return errors.New("🔇 Nothing is playing")
	}

	videoPTS := 1 / speed

	var audioFilterBuilder strings.Builder
	remaining := speed
	for remaining > 2.0 {
		audioFilterBuilder.WriteString("atempo=2.0,")
		remaining /= 2.0
	}
	for remaining < 0.5 {
		audioFilterBuilder.WriteString("atempo=0.5,")
		remaining /= 0.5
	}
	audioFilterBuilder.WriteString(fmt.Sprintf("atempo=%f", remaining))
	audioFilter := audioFilterBuilder.String()

	ffmpegFilters := fmt.Sprintf("-filter:v setpts=%f*PTS -filter:a %s", videoPTS, audioFilter)

	return c.PlayMedia(chatID, playingSong.FilePath, playingSong.IsVideo, ffmpegFilters)
}

// RegisterHandlers sets up the event handlers for the voice call client.
func (c *TelegramCalls) RegisterHandlers(client *tg.Client) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.bot = client
	logger = client.Log

	for _, call := range c.uBContext {

		//_, _ = call.App.UpdatesGetState()
		call.OnStreamEnd(func(chatID int64, streamType ntgcalls.StreamType, device ntgcalls.StreamDevice) {
			client.Log.Info("[TelegramCalls] The stream has ended in chat %d (type=%v, device=%v)", chatID, streamType, device)
			if streamType == ntgcalls.VideoStream {
				client.Log.Info("Ignoring video stream end for chat %d", chatID)
				return
			}

			if err := c.PlayNext(chatID); err != nil {
				client.Log.Error("[OnStreamEnd] Failed to play the song: %v", err)
			}
		})

		call.OnIncomingCall(func(ub *ubot.Context, chatID int64) {
			_, _ = ub.App.SendMessage(chatID, "Incoming call detected. Playing music...")
			msg, err := utils.GetMessage(c.bot, DefaultStreamURL)
			if err != nil {
				c.bot.Log.Info("[OnIncomingCall] Failed to get the message: %v", err)
				return
			}

			dCtx, dCancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer dCancel()
			filePath, err := msg.Download(&tg.DownloadOptions{FileName: filepath.Join(config.Conf.DownloadsDir, msg.File.Name), Ctx: dCtx})
			if err != nil {
				c.bot.Log.Info("[OnIncomingCall] Failed to download the message: %v", err)
				return
			}

			err = c.PlayMedia(chatID, filePath, false, "")
			if err != nil {
				c.bot.Log.Info("[OnIncomingCall] Failed to play the media: %v", err)
				return
			}

			return
		})

		//call.OnFrame(func(chatId int64, mode ntgcalls.StreamMode, device ntgcalls.StreamDevice, frames []ntgcalls.Frame) {
		//	c.bot.Log.Debug("Received frames for chatId: %d, mode: %v, device: %v", chatId, mode, device)
		//})

		_, _ = call.App.SendMessage(client.Me().Username, "/start")
		_, err := call.App.SendMessage(config.Conf.LoggerId, "Userbot started.")
		if err != nil {
			c.bot.Log.Info("[TelegramCalls - SendMessage] Failed to send message: %v", err)
		}
	}
}

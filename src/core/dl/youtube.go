/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package dl

import (
	"suraj08832/tgmusic/config"
	"suraj08832/tgmusic/src/core/db"
	"suraj08832/tgmusic/src/utils"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	tg "github.com/amarnathcjd/gogram/telegram"
)

// YouTubeData provides an interface for fetching track and playlist information from YouTube.
type YouTubeData struct {
	Query    string
	ApiUrl   string
	APIKey   string
	Patterns map[string]*regexp.Regexp
}

const (
	// Direct stream API URL used for play/download.
	fallbackAPIURL = "https://shrutibots.site"
)

var (
	youtubePatterns = map[string]*regexp.Regexp{
		"youtube":   regexp.MustCompile(`^(?:https?://)?(?:www\.)?youtube\.com/watch\?v=([\w-]{11})(?:[&#?].*)?$`),
		"youtu_be":  regexp.MustCompile(`^(?:https?://)?(?:www\.)?youtu\.be/([\w-]{11})(?:[?#].*)?$`),
		"yt_shorts": regexp.MustCompile(`^(?:https?://)?(?:www\.)?youtube\.com/shorts/([\w-]{11})(?:[?#].*)?$`),
		//"yt_live":   regexp.MustCompile(`^(?:https?://)?(?:www\.)?youtube\.com/live/([\w-]{11})(?:[?#].*)?$`),
	}

)

// NewYouTubeData initializes a YouTubeData instance with pre-compiled regex patterns and a cleaned query.
func NewYouTubeData(query string) *YouTubeData {
	return &YouTubeData{
		Query:    clearQuery(query),
		ApiUrl:   strings.TrimRight(config.Conf.ApiUrl, "/"),
		APIKey:   config.Conf.ApiKey,
		Patterns: youtubePatterns,
	}
}

// clearQuery removes extraneous URL parameters and fragments from a given query string.
func clearQuery(query string) string {
	query = strings.SplitN(query, "#", 2)[0]
	query = strings.SplitN(query, "&", 2)[0]
	return strings.TrimSpace(query)
}

// normalizeYouTubeURL converts various YouTube URL formats (e.g., youtu.be, shorts) into a standard watch URL.
func (y *YouTubeData) normalizeYouTubeURL(url string) string {
	var videoID string
	switch {
	case strings.Contains(url, "youtu.be/"):
		parts := strings.SplitN(strings.SplitN(url, "youtu.be/", 2)[1], "?", 2)
		videoID = strings.SplitN(parts[0], "#", 2)[0]
	case strings.Contains(url, "youtube.com/shorts/"):
		parts := strings.SplitN(strings.SplitN(url, "youtube.com/shorts/", 2)[1], "?", 2)
		videoID = strings.SplitN(parts[0], "#", 2)[0]
	default:
		return url
	}
	return "https://www.youtube.com/watch?v=" + videoID
}

// extractVideoID parses a YouTube URL and extracts the video ID.
func (y *YouTubeData) extractVideoID(url string) string {
	url = y.normalizeYouTubeURL(url)
	for _, pattern := range y.Patterns {
		if match := pattern.FindStringSubmatch(url); len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

// IsValid checks if the query string matches any of the known YouTube URL patterns.
func (y *YouTubeData) IsValid() bool {
	if y.Query == "" {
		log.Println("The query or patterns are empty.")
		return false
	}

	for _, pattern := range y.Patterns {
		if pattern.MatchString(y.Query) {
			return true
		}
	}
	return false
}

// GetInfo retrieves metadata for a track from YouTube.
// It returns a PlatformTracks object or an error if the information cannot be fetched.
func (y *YouTubeData) GetInfo(_ context.Context) (utils.PlatformTracks, error) {
	if !y.IsValid() {
		return utils.PlatformTracks{}, errors.New("the provided URL is invalid or the platform is not supported")
	}

	y.Query = y.normalizeYouTubeURL(y.Query)
	videoID := y.extractVideoID(y.Query)
	if videoID == "" {
		return utils.PlatformTracks{}, errors.New("unable to extract the video ID")
	}

	tracks, err := searchYouTube(videoID, 10)
	if err != nil {
		return utils.PlatformTracks{}, err
	}

	for _, track := range tracks {
		if track.Id == videoID {
			return utils.PlatformTracks{Results: []utils.MusicTrack{track}}, nil
		}
	}

	return utils.PlatformTracks{}, errors.New("no video results were found")
}

// Search performs a search for a track on YouTube.
func (y *YouTubeData) Search(_ context.Context) (utils.PlatformTracks, error) {
	tracks, err := searchYouTube(y.Query, 5)
	if err != nil {
		return utils.PlatformTracks{}, err
	}
	if len(tracks) == 0 {
		return utils.PlatformTracks{}, errors.New("no video results were found")
	}
	return utils.PlatformTracks{Results: tracks}, nil
}

// GetTrack retrieves detailed information for a single track.
func (y *YouTubeData) GetTrack(ctx context.Context) (utils.TrackInfo, error) {
	if y.Query == "" {
		return utils.TrackInfo{}, errors.New("the query is empty")
	}
	if !y.IsValid() {
		return utils.TrackInfo{}, errors.New("the provided URL is invalid or the platform is not supported")
	}

	// Fast-path for known YouTube URLs: extract ID directly and avoid
	// relying on search responses that can intermittently return empty.
	normalized := y.normalizeYouTubeURL(y.Query)
	videoID := y.extractVideoID(normalized)
	if videoID != "" {
		return utils.TrackInfo{
			Id:       videoID,
			URL:      normalized,
			Platform: utils.YouTube,
		}, nil
	}

	if y.ApiUrl != "" && y.APIKey != "" {
		if trackInfo, err := NewApiData(y.Query).GetTrack(ctx); err == nil {
			return trackInfo, nil
		}
	}

	getInfo, err := y.GetInfo(ctx)
	if err != nil {
		return utils.TrackInfo{}, err
	}
	if len(getInfo.Results) == 0 {
		return utils.TrackInfo{}, errors.New("no video results were found")
	}

	track := getInfo.Results[0]
	trackInfo := utils.TrackInfo{
		Id:       track.Id,
		URL:      track.Url,
		Platform: utils.YouTube,
	}

	return trackInfo, nil
}

// downloadTrack handles the download of a track from YouTube.
// It checks MongoDB cache first, then tries API, and caches to logger group.
func (y *YouTubeData) downloadTrack(ctx context.Context, info utils.TrackInfo, video bool) (string, error) {
	// Check MongoDB cache first
	dbCtx, cancel := db.Ctx()
	defer cancel()
	
	loggerLink, err := db.Instance.GetSongCache(dbCtx, info.Id, video)
	if err == nil && loggerLink != "" {
		// Try to download from logger group
		if bot := getBotFromContext(ctx); bot != nil {
			if filePath, err := downloadFromLogger(bot, loggerLink); err == nil {
				log.Printf("[YouTube] Cache hit for video ID: %s, using logger link: %s", info.Id, loggerLink)
				return filePath, nil
			}
			// If download from logger fails, continue with normal download
			log.Printf("[YouTube] Failed to download from logger cache, falling back to API download: %v", err)
		}
	}

	// Check if file already exists in downloads directory
	exts := []string{"mp3", "m4a", "webm", "opus"}
	if video {
		exts = []string{"mp4", "webm", "mkv"}
	}
	for _, ext := range exts {
		filePath := filepath.Join(config.Conf.DownloadsDir, fmt.Sprintf("%s.%s", info.Id, ext))
		if stat, err := os.Stat(filePath); err == nil && stat.Size() > 0 {
			return filePath, nil
		}
	}

	filePath, apiErr := downloadViaFallbackAPI(ctx, info.Id, video)
	if apiErr != nil || filePath == "" {
		return "", fmt.Errorf("direct stream download failed: %w", apiErr)
	}

	log.Printf("[YouTube] Successfully downloaded via API: %s", info.Id)
	
	// In background: Send downloaded file to logger group and cache for next time
	if bot := getBotFromContext(ctx); bot != nil && config.Conf.LoggerId != 0 {
		go func() {
			log.Printf("[YouTube] Background: Sending %s to logger group...", info.Id)
			if loggerLink, sendErr := sendToLoggerGroup(bot, filePath, info.Id, video); sendErr == nil {
				dbCtx2, cancel2 := db.Ctx()
				defer cancel2()
				if cacheErr := db.Instance.SetSongCache(dbCtx2, info.Id, loggerLink, video); cacheErr != nil {
					log.Printf("[YouTube] Failed to cache logger link: %v", cacheErr)
				} else {
					log.Printf("[YouTube] Cached logger link for video ID: %s (next time will use cache)", info.Id)
				}
			} else {
				log.Printf("[YouTube] Failed to send to logger group: %v", sendErr)
			}
		}()
	}

	return filePath, nil
}

// downloadViaFallbackAPI downloads audio/video using Shruti's direct stream API.
func downloadViaFallbackAPI(ctx context.Context, videoID string, isVideo bool) (string, error) {
	if videoID == "" || len(videoID) < 3 {
		return "", errors.New("invalid video ID")
	}

	mediaType := "audio"
	if isVideo {
		mediaType = "video"
	}

	// Check if file already exists
	ext := "mp3"
	if isVideo {
		ext = "mp4"
	}
	filePath := filepath.Join(config.Conf.DownloadsDir, fmt.Sprintf("%s.%s", videoID, ext))
	if stat, err := os.Stat(filePath); err == nil && stat.Size() > 0 {
		return filePath, nil
	}

	// Create downloads directory if it doesn't exist
	if err := os.MkdirAll(config.Conf.DownloadsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create downloads directory: %w", err)
	}

	streamURL := fmt.Sprintf("%s/stream/%s?type=%s", strings.TrimRight(fallbackAPIURL, "/"), url.QueryEscape(videoID), mediaType)

	timeout := 300 * time.Second
	if isVideo {
		timeout = 600 * time.Second
	}

	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create stream request: %w", err)
	}

	// Create client that doesn't follow redirects automatically
	client2 := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Return error to prevent automatic redirect, we'll handle it manually
			return http.ErrUseLastResponse
		},
	}
	resp2, err := client2.Do(req2)
	// Even if we get ErrUseLastResponse (due to redirect), resp2 is still valid
	if err != nil && !errors.Is(err, http.ErrUseLastResponse) {
		return "", fmt.Errorf("stream request failed: %w", err)
	}
	if resp2 == nil {
		return "", errors.New("stream request returned nil response")
	}
	defer resp2.Body.Close()

	// Handle 302 redirect (matching Python code)
	if resp2.StatusCode == http.StatusFound {
		redirectURL := resp2.Header.Get("Location")
		if redirectURL == "" {
			return "", errors.New("302 redirect but no Location header")
		}

		// Follow the redirect
		req3, err := http.NewRequestWithContext(ctx, http.MethodGet, redirectURL, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create redirect request: %w", err)
		}

		client3 := &http.Client{Timeout: timeout}
		resp3, err := client3.Do(req3)
		if err != nil {
			return "", fmt.Errorf("redirect request failed: %w", err)
		}
		defer resp3.Body.Close()

		if resp3.StatusCode != http.StatusOK {
			return "", fmt.Errorf("redirect returned status code: %d", resp3.StatusCode)
		}

		// Write file in chunks (16KB like Python code)
		outFile, err := os.Create(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to create file: %w", err)
		}
		defer outFile.Close()

		buffer := make([]byte, 16384)
		if _, err := io.CopyBuffer(outFile, resp3.Body, buffer); err != nil {
			os.Remove(filePath)
			return "", fmt.Errorf("failed to write file: %w", err)
		}

		// Verify file was written successfully
		if stat, err := os.Stat(filePath); err != nil || stat.Size() == 0 {
			os.Remove(filePath)
			return "", errors.New("downloaded file is empty or missing")
		}

		return filePath, nil
	} else if resp2.StatusCode == http.StatusOK {
		// Write file in chunks (16KB like Python code)
		outFile, err := os.Create(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to create file: %w", err)
		}
		defer outFile.Close()

		buffer := make([]byte, 16384)
		if _, err := io.CopyBuffer(outFile, resp2.Body, buffer); err != nil {
			os.Remove(filePath)
			return "", fmt.Errorf("failed to write file: %w", err)
		}

		// Verify file was written successfully
		if stat, err := os.Stat(filePath); err != nil || stat.Size() == 0 {
			os.Remove(filePath)
			return "", errors.New("downloaded file is empty or missing")
		}

		return filePath, nil
	}

	// Clean up on error
	if _, err := os.Stat(filePath); err == nil {
		os.Remove(filePath)
	}

	return "", fmt.Errorf("stream returned status code: %d", resp2.StatusCode)
}

// downloadWithApi downloads a track using the external API.
func (y *YouTubeData) downloadWithApi(ctx context.Context, videoID string, _ bool) (string, error) {
	videoUrl := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	api := NewApiData(videoUrl)
	track, err := api.GetTrack(ctx)
	if err != nil {
		return "", err
	}

	down, err := NewDownload(ctx, track)
	if err != nil {
		log.Println("Error creating download: " + err.Error())
		return "", err
	}

	return down.Process()
}

// getBotFromContext extracts the bot client from context if available.
func getBotFromContext(ctx context.Context) *tg.Client {
	if bot, ok := ctx.Value("bot").(*tg.Client); ok {
		return bot
	}
	return nil
}

// sendToLoggerGroup sends a file to the logger group and returns the message link.
func sendToLoggerGroup(bot *tg.Client, filePath string, videoID string, isVideo bool) (string, error) {
	if config.Conf.LoggerId == 0 {
		return "", errors.New("logger ID not configured")
	}

	caption := fmt.Sprintf("Audio ID: %s", videoID)
	if isVideo {
		caption = fmt.Sprintf("Video ID: %s", videoID)
	}

	// Send file using SendMessage with Media option
	msg, err := bot.SendMessage(config.Conf.LoggerId, caption, &tg.SendOptions{
		Media: filePath,
	})

	if err != nil {
		return "", fmt.Errorf("failed to send to logger group: %w", err)
	}

	// Generate message link
	// Format for private groups: https://t.me/c/{chat_id}/{message_id}
	// For private groups, remove -100 prefix if present
	chatID := config.Conf.LoggerId
	if chatID < 0 {
		// Remove -100 prefix for private groups (e.g., -1001234567890 -> -1234567890)
		chatIDStr := fmt.Sprintf("%d", chatID)
		if strings.HasPrefix(chatIDStr, "-100") {
			if parsedID, err := strconv.ParseInt(chatIDStr[4:], 10, 64); err == nil {
				chatID = -parsedID
			}
		}
	}
	
	loggerLink := fmt.Sprintf("https://t.me/c/%d/%d", chatID, msg.ID)
	
	return loggerLink, nil
}

// downloadFromLogger downloads a file from the logger group using the message link.
func downloadFromLogger(bot *tg.Client, loggerLink string) (string, error) {
	// Parse the logger link to extract chat ID and message ID
	// Format: https://t.me/c/{chat_id}/{message_id}
	parts := strings.Split(loggerLink, "/")
	if len(parts) < 2 {
		return "", errors.New("invalid logger link format")
	}

	// Extract message ID (last part)
	msgIDStr := parts[len(parts)-1]
	msgID, err := strconv.Atoi(msgIDStr)
	if err != nil {
		return "", fmt.Errorf("invalid message ID in logger link: %w", err)
	}

	// Get message from logger group
	msg, err := bot.GetMessageByID(config.Conf.LoggerId, int32(msgID))
	if err != nil {
		return "", fmt.Errorf("failed to get message from logger group: %w", err)
	}

	if msg.File == nil {
		return "", errors.New("logger message has no downloadable file")
	}

	// Download the file
	fileName := filepath.Base(msg.File.Name)
	if fileName == "" {
		ext := "mp3"
		if strings.Contains(loggerLink, "video") {
			ext = "mp4"
		}
		fileName = fmt.Sprintf("%d.%s", msgID, ext)
	}

	dst := filepath.Join(config.Conf.DownloadsDir, fileName)
	
	// Check if file already exists
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	path, err := msg.Download(&tg.DownloadOptions{
		FileName: dst,
		Ctx:      ctx,
	})
	if err != nil {
		return "", fmt.Errorf("failed to download from logger group: %w", err)
	}

	return path, nil
}

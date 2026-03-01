/*
 * TgMusicBot - Telegram Music Bot
 *  Copyright (c) 2025-2026 Your Name
 *
 *  Licensed under GNU GPL v3
 *  See https://github.com/Suraj08832/newgomusic
 */

package core

import (
	"suraj08832/tgmusic/src/core/dl"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	cacheDir     = "cache"
	thumbWidth   = 1280
	thumbHeight  = 720
	rectWidth    = 842
	rectHeight   = 412
	rectX        = (thumbWidth - rectWidth) / 2
	rectY        = 120
	titleY       = rectY + rectHeight + 40
	metaY        = titleY + 50
	padding      = 25
	defaultThumb = "https://telegra.ph/file/6543d0c0c4b0e0e0e0e0e0.png"
)

var neonColors = []color.RGBA{
	{0, 255, 255, 255},     // Cyan
	{255, 0, 255, 255},     // Magenta
	{0, 255, 128, 255},     // Green
	{255, 255, 0, 255},     // Yellow
	{255, 105, 180, 255},   // Pink
}

// GetThumb generates a thumbnail image for a YouTube video.
// Returns the path to the generated thumbnail file.
func GetThumb(ctx context.Context, videoid string, playerUsername string) (string, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%s_v5.png", videoid))
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	// Get video information
	yt := dl.NewYouTubeData(fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoid))
	tracks, err := yt.Search(ctx)
	if err != nil || len(tracks.Results) == 0 {
		return "", fmt.Errorf("failed to get video info: %w", err)
	}

	track := tracks.Results[0]
	title := cleanTitle(track.Title)
	thumbnail := track.Thumbnail
	if thumbnail == "" {
		thumbnail = defaultThumb
	}
	duration := formatDuration(track.Duration)
	views := track.Views
	if views == "" {
		views = "Unknown Views"
	}

	isLive := duration == "" || strings.ToLower(duration) == "live" || strings.ToLower(duration) == "live now"
	durationText := "Live"
	if !isLive && duration != "" {
		durationText = duration
	} else if duration == "" {
		durationText = "Unknown"
	}

	// Download thumbnail
	thumbPath := filepath.Join(cacheDir, fmt.Sprintf("thumb_%s.png", videoid))
	if err := downloadThumbnail(ctx, thumbnail, thumbPath); err != nil {
		log.Printf("[Thumbnail] Failed to download thumbnail: %v, using default", err)
		thumbnail = defaultThumb
		if err := downloadThumbnail(ctx, defaultThumb, thumbPath); err != nil {
			return "", fmt.Errorf("failed to download default thumbnail: %w", err)
		}
	}
	defer os.Remove(thumbPath)

	// Load and process base image
	baseImg, err := imaging.Open(thumbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open thumbnail: %w", err)
	}

	// Resize base image
	baseImg = imaging.Resize(baseImg, thumbWidth, thumbHeight, imaging.Lanczos)
	
	// Convert to RGBA for processing
	baseRGBA := imageToRGBA(baseImg)
	
	// Apply brightness adjustment
	for y := 0; y < baseRGBA.Bounds().Dy(); y++ {
		for x := 0; x < baseRGBA.Bounds().Dx(); x++ {
			c := baseRGBA.RGBAAt(x, y)
			c.R = uint8(float64(c.R) * 0.4)
			c.G = uint8(float64(c.G) * 0.4)
			c.B = uint8(float64(c.B) * 0.4)
			baseRGBA.SetRGBA(x, y, c)
		}
	}

	// Apply blur (simple box blur approximation)
	blurred := imaging.Blur(baseRGBA, 20)
	
	// Convert blurred image to RGBA
	rgba := imageToRGBA(blurred)

	// Load and resize thumbnail
	thumbImg, err := imaging.Open(thumbPath)
	if err != nil {
		return "", fmt.Errorf("failed to open thumbnail: %w", err)
	}
	thumbImg = imaging.Resize(thumbImg, rectWidth, rectHeight, imaging.Lanczos)

	// Choose random neon color
	neonColor := neonColors[rand.Intn(len(neonColors))]

	// Create glow effect
	glow := image.NewRGBA(image.Rect(0, 0, rectWidth+80, rectHeight+80))
	for r := 40; r > 0; r -= 4 {
		alpha := uint8(255 * r / 40 / 2)
		drawRect(glow, r, r, glow.Bounds().Dx()-r, glow.Bounds().Dy()-r, color.RGBA{
			R: neonColor.R,
			G: neonColor.G,
			B: neonColor.B,
			A: alpha,
		}, 4)
	}

	// Paste glow
	gx := rectX - 40
	gy := rectY - 40
	draw.Draw(rgba, image.Rect(gx, gy, gx+glow.Bounds().Dx(), gy+glow.Bounds().Dy()), glow, image.Point{}, draw.Over)

	// Paste thumbnail
	thumbRGBA := image.NewRGBA(thumbImg.Bounds())
	draw.Draw(thumbRGBA, thumbRGBA.Bounds(), thumbImg, image.Point{}, draw.Src)
	draw.Draw(rgba, image.Rect(rectX, rectY, rectX+rectWidth, rectY+rectHeight), thumbRGBA, image.Point{}, draw.Over)

	// Draw text
	drawText(rgba, title, thumbWidth/2, titleY, color.White, true)
	metaText := fmt.Sprintf("YouTube : %s | Time : %s | Player : @%s", views, durationText, playerUsername)
	drawText(rgba, metaText, thumbWidth/2, metaY, neonColor, false)

	// Draw watermarks
	yellow := color.RGBA{255, 255, 0, 255}
	raushanText := "Dev :- @wife_girlfriend_group"
	sonaliText := "@Urs_aarohi"
	drawText(rgba, raushanText, thumbWidth-padding, padding, yellow, false)
	drawText(rgba, sonaliText, padding, thumbHeight-50, yellow, false)

	// Save final image as PNG
	file, err := os.Create(cachePath)
	if err != nil {
		return "", fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, rgba); err != nil {
		return "", fmt.Errorf("failed to encode PNG: %w", err)
	}

	return cachePath, nil
}

// downloadThumbnail downloads a thumbnail image from a URL.
func downloadThumbnail(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download thumbnail: status %d", resp.StatusCode)
	}

	file, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// cleanTitle cleans and formats the title text.
func cleanTitle(title string) string {
	// Remove non-word characters and title case
	re := regexp.MustCompile(`\W+`)
	cleaned := re.ReplaceAllString(title, " ")
	words := strings.Fields(cleaned)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// formatDuration formats duration in seconds to MM:SS or HH:MM:SS format.
func formatDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}

// drawText draws text on an image at the specified position.
func drawText(img *image.RGBA, text string, x, y int, clr color.Color, isTitle bool) {
	face := basicfont.Face7x13
	if isTitle {
		// Use larger font for title (approximate 38px)
		face = basicfont.Face7x13
	}

	// Trim text to fit width
	maxWidth := 800
	if !isTitle {
		maxWidth = 1200
	}
	text = trimToWidth(text, face, maxWidth)

	point := fixed.Point26_6{
		X: fixed.Int26_6(x * 64),
		Y: fixed.Int26_6(y * 64),
	}

	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  point,
	}

	// Center text
	width := d.MeasureString(text)
	d.Dot.X = fixed.Int26_6((x - int(width.Ceil())/2) * 64)

	d.DrawString(text)
}

// trimToWidth trims text to fit within the specified width.
func trimToWidth(text string, face font.Face, maxWidth int) string {
	d := &font.Drawer{Face: face}
	width := d.MeasureString(text)
	if width.Ceil() <= maxWidth {
		return text
	}

	ellipsis := "…"

	for i := len(text) - 1; i > 0; i-- {
		trimmed := text[:i] + ellipsis
		w := d.MeasureString(trimmed)
		if w.Ceil() <= maxWidth {
			return trimmed
		}
	}

	return ellipsis
}

// drawRect draws a rectangle on an image.
func drawRect(img *image.RGBA, x1, y1, x2, y2 int, clr color.Color, width int) {
	for w := 0; w < width; w++ {
		// Top
		for x := x1; x < x2; x++ {
			img.Set(x, y1+w, clr)
		}
		// Bottom
		for x := x1; x < x2; x++ {
			img.Set(x, y2-w, clr)
		}
		// Left
		for y := y1; y < y2; y++ {
			img.Set(x1+w, y, clr)
		}
		// Right
		for y := y1; y < y2; y++ {
			img.Set(x2-w, y, clr)
		}
	}
}

// Helper function to convert image.Image to *image.RGBA
func imageToRGBA(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, image.Point{}, draw.Src)
	return rgba
}

// init initializes random seed
func init() {
	rand.Seed(time.Now().UnixNano())
}


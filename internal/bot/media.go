package bot

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type DownloadedMedia struct {
	FileName  string
	FilePath  string
	SizeBytes int64
	MediaType string // "photo", "document", "video", "audio", "voice"
}

// DownloadMessageMedia extracts attached file/photo/video/audio from Telegram message and saves it to workspace
func (r *Router) DownloadMessageMedia(msg *tgbotapi.Message, targetDir string) (*DownloadedMedia, error) {
	var fileID string
	var fileName string
	var mediaType string

	// Format: agy-bot-tt-bb-tahun_HHmmss (misal: agy-bot-21-09-2026_191851)
	timestamp := time.Now().Format("02-01-2006_150405")

	if len(msg.Photo) > 0 {
		// Pick highest resolution photo
		photo := msg.Photo[len(msg.Photo)-1]
		fileID = photo.FileID
		fileName = fmt.Sprintf("agy-bot-%s.jpg", timestamp)
		mediaType = "photo"
	} else if msg.Document != nil {
		fileID = msg.Document.FileID
		rawName := msg.Document.FileName
		if rawName == "" {
			fileName = fmt.Sprintf("agy-bot-%s.bin", timestamp)
		} else if strings.HasPrefix(rawName, "agy-bot-") {
			fileName = rawName
		} else {
			fileName = fmt.Sprintf("agy-bot-%s_%s", timestamp, rawName)
		}
		mediaType = "document"
	} else if msg.Video != nil {
		fileID = msg.Video.FileID
		rawName := msg.Video.FileName
		if rawName == "" {
			fileName = fmt.Sprintf("agy-bot-%s.mp4", timestamp)
		} else if strings.HasPrefix(rawName, "agy-bot-") {
			fileName = rawName
		} else {
			fileName = fmt.Sprintf("agy-bot-%s_%s", timestamp, rawName)
		}
		mediaType = "video"
	} else if msg.Audio != nil {
		fileID = msg.Audio.FileID
		rawName := msg.Audio.FileName
		if rawName == "" {
			fileName = fmt.Sprintf("agy-bot-%s.mp3", timestamp)
		} else if strings.HasPrefix(rawName, "agy-bot-") {
			fileName = rawName
		} else {
			fileName = fmt.Sprintf("agy-bot-%s_%s", timestamp, rawName)
		}
		mediaType = "audio"
	} else if msg.Voice != nil {
		fileID = msg.Voice.FileID
		fileName = fmt.Sprintf("agy-bot-voice-%s.ogg", timestamp)
		mediaType = "voice"
	} else {
		return nil, nil // No media
	}

	uploadDir := filepath.Join(targetDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		uploadDir = filepath.Join(os.TempDir(), "agy_uploads")
		_ = os.MkdirAll(uploadDir, 0755)
	}

	// Sanitize file name to avoid directory traversal
	cleanFileName := filepath.Base(strings.ReplaceAll(fileName, "..", ""))
	savePath := filepath.Join(uploadDir, cleanFileName)

	directURL, err := r.bot.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("gagal mendapatkan link unduh Telegram: %w", err)
	}

	resp, err := http.Get(directURL)
	if err != nil {
		return nil, fmt.Errorf("gagal mengunduh file dari Telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status HTTP unduh Telegram tidak valid: %d", resp.StatusCode)
	}

	out, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat file di server: %w", err)
	}
	defer out.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gagal menyimpan file: %w", err)
	}

	return &DownloadedMedia{
		FileName:  cleanFileName,
		FilePath:  savePath,
		SizeBytes: n,
		MediaType: mediaType,
	}, nil
}

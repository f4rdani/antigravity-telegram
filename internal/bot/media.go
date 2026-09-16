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

	if len(msg.Photo) > 0 {
		// Pick highest resolution photo
		photo := msg.Photo[len(msg.Photo)-1]
		fileID = photo.FileID
		fileName = fmt.Sprintf("photo_%d.jpg", time.Now().Unix())
		mediaType = "photo"
	} else if msg.Document != nil {
		fileID = msg.Document.FileID
		fileName = msg.Document.FileName
		if fileName == "" {
			fileName = fmt.Sprintf("doc_%d", time.Now().Unix())
		}
		mediaType = "document"
	} else if msg.Video != nil {
		fileID = msg.Video.FileID
		fileName = msg.Video.FileName
		if fileName == "" {
			fileName = fmt.Sprintf("video_%d.mp4", time.Now().Unix())
		}
		mediaType = "video"
	} else if msg.Audio != nil {
		fileID = msg.Audio.FileID
		fileName = msg.Audio.FileName
		if fileName == "" {
			fileName = fmt.Sprintf("audio_%d.mp3", time.Now().Unix())
		}
		mediaType = "audio"
	} else if msg.Voice != nil {
		fileID = msg.Voice.FileID
		fileName = fmt.Sprintf("voice_%d.ogg", time.Now().Unix())
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

// ===============================
// internal/services/upload.go
// ===============================

package services

import (
	"context"
	"fmt"
	"mime/multipart"
	"time"

	"weibaobe/internal/storage"

	"github.com/google/uuid"
)

type UploadService struct {
	r2Client *storage.R2Client
}

func NewUploadService(r2Client *storage.R2Client) *UploadService {
	return &UploadService{r2Client: r2Client}
}

func (s *UploadService) UploadFile(ctx context.Context, file multipart.File, filename, fileType string) (string, error) {
	// Generate unique filename
	ext := getFileExtension(filename)
	uniqueFilename := fmt.Sprintf("%s/%d_%s%s", fileType, time.Now().Unix(), uuid.New().String()[:8], ext)

	// Determine content type
	contentType := getContentType(fileType, ext)

	// Upload to R2
	err := s.r2Client.UploadFile(ctx, uniqueFilename, file, contentType)
	if err != nil {
		return "", err
	}

	// Return public URL
	return s.r2Client.GetPublicURL(uniqueFilename), nil
}

func getFileExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
	}
	return ""
}

func getContentType(fileType, ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/avi"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}

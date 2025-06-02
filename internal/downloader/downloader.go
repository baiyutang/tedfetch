package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/schollz/progressbar/v3"
)

// Downloader handles downloading of TED talk videos and subtitles
type Downloader struct {
	client *http.Client
	// Base directory for downloads
	baseDir    string
	maxRetries int
}

// New creates a new Downloader instance
func New(baseDir string) (*Downloader, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &Downloader{
		client:     &http.Client{},
		baseDir:    baseDir,
		maxRetries: 3,
	}, nil
}

// DownloadVideo downloads a video file with progress bar
func (d *Downloader) DownloadVideo(url, filename string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < d.maxRetries; attempt++ {
		// Create output file (truncate if exists)
		out, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}

		resp, err := d.client.Get(url)
		if err != nil {
			_ = out.Close()
			lastErr = fmt.Errorf("failed to get video: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if cerr := resp.Body.Close(); cerr != nil {
				fmt.Println("close response body error:", cerr)
			}
			_ = out.Close()
			lastErr = fmt.Errorf("bad status: %s", resp.Status)
			continue
		}

		bar := progressbar.DefaultBytes(
			resp.ContentLength,
			"Downloading video",
		)

		_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Println("close response body error:", cerr)
		}
		if err != nil {
			_ = out.Close()
			lastErr = fmt.Errorf("failed to download video: %w", err)
			continue
		}

		if cerr := out.Close(); cerr != nil {
			fmt.Println("close file error:", cerr)
		}

		return nil
	}

	return lastErr
}

// DownloadSubtitle downloads a subtitle file
func (d *Downloader) DownloadSubtitle(url, filename string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < d.maxRetries; attempt++ {
		out, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}

		resp, err := d.client.Get(url)
		if err != nil {
			_ = out.Close()
			lastErr = fmt.Errorf("failed to get subtitle: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if cerr := resp.Body.Close(); cerr != nil {
				fmt.Println("close response body error:", cerr)
			}
			_ = out.Close()
			lastErr = fmt.Errorf("bad status: %s", resp.Status)
			continue
		}

		bar := progressbar.DefaultBytes(
			resp.ContentLength,
			"Downloading subtitle",
		)

		_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Println("close response body error:", cerr)
		}
		if err != nil {
			_ = out.Close()
			lastErr = fmt.Errorf("failed to download subtitle: %w", err)
			continue
		}

		if cerr := out.Close(); cerr != nil {
			fmt.Println("close file error:", cerr)
		}

		return nil
	}

	return lastErr
}

// GetDownloadPath returns the full path for a download
func (d *Downloader) GetDownloadPath(talkTitle, format string) string {
	// Sanitize filename
	filename := sanitizeFilename(talkTitle)
	return filepath.Join(d.baseDir, filename, format)
}

// sanitizeFilename converts a string to a valid filename
func sanitizeFilename(s string) string {
	// Replace invalid characters with underscore
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := s
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	return result
}

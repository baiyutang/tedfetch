package downloader

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDownloader(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return mock content
		content := []byte("test content")
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		_, err := w.Write(content)
		assert.NoError(t, err)
	}))
	defer server.Close()

	// Create temporary directory for downloads
	tempDir, err := os.MkdirTemp("", "tedfetch_test")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("failed to remove temp directory: %v", err)
		}
	}()

	// Create downloader
	d, err := New(tempDir)
	assert.NoError(t, err)

	// Test video download
	t.Run("DownloadVideo", func(t *testing.T) {
		filename := d.GetDownloadPath("test_talk", "video.mp4")
		err := d.DownloadVideo(server.URL, filename)
		assert.NoError(t, err)

		// Verify file exists and has correct content
		content, err := os.ReadFile(filename)
		assert.NoError(t, err)
		assert.Equal(t, []byte("test content"), content)
	})

	// Test subtitle download
	t.Run("DownloadSubtitle", func(t *testing.T) {
		filename := d.GetDownloadPath("test_talk", "subtitle.srt")
		err := d.DownloadSubtitle(server.URL, filename)
		assert.NoError(t, err)

		// Verify file exists and has correct content
		content, err := os.ReadFile(filename)
		assert.NoError(t, err)
		assert.Equal(t, []byte("test content"), content)
	})

	// Test filename sanitization
	t.Run("GetDownloadPath", func(t *testing.T) {
		path := d.GetDownloadPath("test/talk:with*invalid?chars", "video.mp4")
		expected := filepath.Join(tempDir, "test_talk_with_invalid_chars", "video.mp4")
		assert.Equal(t, expected, path)
	})
}

func TestGetDownloadPath(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		filename string
		want     string
	}{
		{
			name:     "video file",
			title:    "The Story Behind the Mars Rovers",
			filename: "720p.mp4",
			want:     filepath.Join("test_downloads", "The Story Behind the Mars Rovers", "720p.mp4"),
		},
		{
			name:     "subtitle file",
			title:    "The Story Behind the Mars Rovers",
			filename: "en.srt",
			want:     filepath.Join("test_downloads", "The Story Behind the Mars Rovers", "en.srt"),
		},
		{
			name:     "title with special characters",
			title:    "Why We Need to Build in Space?",
			filename: "1080p.mp4",
			want:     filepath.Join("test_downloads", "Why We Need to Build in Space_", "1080p.mp4"),
		},
	}

	d, err := New("test_downloads")
	if err != nil {
		t.Fatalf("Failed to create downloader: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.GetDownloadPath(tt.title, tt.filename)
			if got != tt.want {
				t.Errorf("GetDownloadPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

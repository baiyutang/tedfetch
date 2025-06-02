package cmd

import (
	"fmt"
	"strings"

	"github.com/baiyutang/tedfetch/internal/downloader"
	"github.com/baiyutang/tedfetch/internal/parser"
	"github.com/spf13/cobra"
)

var (
	// downloadCmd represents the download command
	downloadCmd = &cobra.Command{
		Use:   "download",
		Short: "Download TED talk videos and subtitles",
		Long: `Download TED talk videos and subtitles. For example:
tedfetch download "The power of vulnerability" --quality 720p
tedfetch download https://www.ted.com/talks/ariel_ekblaw_how_to_build_in_space_for_life_on_earth --quality 720p --subtitle zh-CN`,
		RunE: runDownload,
	}

	// Flags
	quality  string
	subtitle string
	output   string
)

func init() {
	rootCmd.AddCommand(downloadCmd)

	// Add flags
	downloadCmd.Flags().StringVarP(&quality, "quality", "q", "720p", "Video quality (720p, 1080p)")
	downloadCmd.Flags().StringVarP(&subtitle, "subtitle", "s", "", "Subtitle language code (e.g., en, zh-CN). Leave empty to skip subtitle download")
	downloadCmd.Flags().StringVarP(&output, "output", "o", ".", "Output directory")
}

func runDownload(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a talk title or URL")
	}

	// Create parser
	p := parser.New()

	// Create downloader
	d, err := downloader.New(output)
	if err != nil {
		return fmt.Errorf("failed to create downloader: %w", err)
	}

	// Parse talk details
	var talk *parser.Talk
	var slug string
	if strings.HasPrefix(args[0], "http") {
		talk, err = p.ParseURL(args[0])
		slug = extractSlug(args[0])
	} else {
		talk, err = p.ParseTalkDetails(args[0])
		slug = extractSlug(talk.URL)
	}
	if err != nil {
		return fmt.Errorf("failed to parse talk details: %w", err)
	}

	// Get video URL for requested quality
	videoURL, ok := talk.VideoURLs[quality]
	if !ok {
		return fmt.Errorf("video quality %s not available", quality)
	}

	// Download video
	fmt.Printf("Downloading video (%s)...\n", quality)
	videoPath := d.GetDownloadPath(slug, fmt.Sprintf("%s.mp4", quality))
	if err := d.DownloadVideo(videoURL, videoPath); err != nil {
		return fmt.Errorf("failed to download video: %w", err)
	}

	// Download subtitle if requested
	if subtitle != "" {
		subtitleURL, ok := talk.SubtitleURLs[subtitle]
		if !ok {
			return fmt.Errorf("subtitle language %s not available", subtitle)
		}

		fmt.Printf("Downloading subtitle (%s)...\n", subtitle)
		subtitlePath := d.GetDownloadPath(slug, fmt.Sprintf("%s.srt", subtitle))
		if err := d.DownloadSubtitle(subtitleURL, subtitlePath); err != nil {
			return fmt.Errorf("failed to download subtitle: %w", err)
		}
		fmt.Printf("Subtitle: %s\n", subtitlePath)
	}

	fmt.Printf("\nDownload completed!\n")
	fmt.Printf("Video: %s\n", videoPath)

	return nil
}

// extractSlug extracts the slug from a TED talk URL
func extractSlug(url string) string {
	parts := strings.Split(strings.SplitN(url, "?", 2)[0], "/")
	slug := ""
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			slug = parts[i]
			break
		}
	}
	return slug
}

package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Talk represents a TED talk with its metadata
type Talk struct {
	Title         string
	Speaker       string
	URL           string
	Description   string
	Duration      string
	PublishedDate string
	Views         string
	// Video related fields
	VideoURLs    map[string]string // quality -> URL
	VideoFormats []VideoFormat     // Available video formats
	// Subtitle related fields
	SubtitleURLs map[string]string // language code -> URL
}

// VideoFormat represents a specific video format
type VideoFormat struct {
	Quality string // e.g., "1080p", "720p", "480p"
	URL     string // Direct download URL
	Size    int64  // File size in bytes
}

// Parser handles the parsing of TED talk pages
type Parser struct {
	client     *http.Client
	GraphqlURL string
	// Debug mode and response storage
	Debug        bool
	RawResponses map[string][]byte // Store raw responses for debugging
}

var baseURL = "https://www.ted.com"

// New creates a new Parser instance
func New() *Parser {
	return &Parser{
		client:       &http.Client{},
		GraphqlURL:   "https://www.ted.com/graphql",
		RawResponses: make(map[string][]byte),
	}
}

// SetDebug enables or disables debug mode
func (p *Parser) SetDebug(debug bool) {
	p.Debug = debug
}

// debugPrint prints debug information if debug mode is enabled
func (p *Parser) debugPrint(format string, args ...interface{}) {
	if p.Debug {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
	}
}

// storeRawResponse stores raw response for debugging
func (p *Parser) storeRawResponse(key string, data []byte) {
	if p.Debug {
		p.RawResponses[key] = data
	}
}

// GetRawResponse returns stored raw response
func (p *Parser) GetRawResponse(key string) []byte {
	return p.RawResponses[key]
}

// ParseTopic fetches and parses TED talks for a given topic or title
func (p *Parser) ParseTopic(query string, limit int) ([]Talk, error) {
	// If the query looks like a title, search by title
	if !strings.Contains(query, " ") {
		url := fmt.Sprintf("%s/talks?topics[]=%s", baseURL, query)
		return p.parseTalksList(url, limit)
	}

	// Otherwise, search by title
	url := fmt.Sprintf("%s/search?q=%s", baseURL, strings.ReplaceAll(query, " ", "+"))
	return p.parseTalksList(url, limit)
}

// parseTalksList fetches and parses the list of talks from a given URL
func (p *Parser) parseTalksList(url string, limit int) ([]Talk, error) {
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch talks list: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Println("close response body error:", cerr)
		}
	}()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var talks []Talk
	doc.Find(".media__message, .search__result").Each(func(i int, s *goquery.Selection) {
		if i >= limit {
			return
		}

		var titleLink *goquery.Selection
		if s.HasClass("search__result") {
			titleLink = s.Find("h3 a")
		} else {
			titleLink = s.Find(".media__message__title a")
		}

		title := strings.TrimSpace(titleLink.Text())
		speaker := strings.TrimSpace(s.Find(".media__message__speaker h4, .search__result__speaker").Text())
		url, _ := titleLink.Attr("href")
		if !strings.HasPrefix(url, "http") {
			url = baseURL + url
		}

		talk := Talk{
			Title:   title,
			Speaker: speaker,
			URL:     url,
		}

		// Parse individual talk page to get video and subtitle URLs
		if err := p.parseTalkDetails(&talk); err != nil {
			fmt.Printf("Warning: failed to parse talk details for %s: %v\n", url, err)
		}

		talks = append(talks, talk)
	})

	return talks, nil
}

// parseTalkDetails fetches and parses the individual talk page to get video and subtitle URLs
func (p *Parser) parseTalkDetails(talk *Talk) error {
	resp, err := p.client.Get(talk.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch talk page: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Println("close response body error:", cerr)
		}
	}()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse talk page: %w", err)
	}

	// Extract video URLs from the page's JSON data
	if err := p.extractVideoURLs(doc, talk); err != nil {
		return fmt.Errorf("failed to extract video URLs: %w", err)
	}

	// Extract subtitle URLs
	if err := p.extractSubtitleURLs(doc, talk); err != nil {
		return fmt.Errorf("failed to extract subtitle URLs: %w", err)
	}

	return nil
}

// extractVideoURLs extracts video download URLs from the page's JSON data
func (p *Parser) extractVideoURLs(doc *goquery.Document, talk *Talk) error {
	// Find the script tag containing video data
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "talkPage.init") {
			// Extract JSON data from the script
			start := strings.Index(text, "{")
			end := strings.LastIndex(text, "}")
			if start != -1 && end != -1 {
				jsonData := text[start : end+1]
				var data struct {
					PlayerData struct {
						Talks []struct {
							PlayerTalks []struct {
								Resources struct {
									H264 []struct {
										Quality string `json:"quality"`
										Size    int64  `json:"size"`
										URL     string `json:"file"`
									} `json:"h264"`
								} `json:"resources"`
							} `json:"player_talks"`
						} `json:"talks"`
					} `json:"playerData"`
				}

				if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
					p.debugPrint("Failed to parse video JSON data: %v", err)
					return
				}

				// Extract video formats
				if len(data.PlayerData.Talks) > 0 && len(data.PlayerData.Talks[0].PlayerTalks) > 0 {
					resources := data.PlayerData.Talks[0].PlayerTalks[0].Resources
					for _, h264 := range resources.H264 {
						talk.VideoFormats = append(talk.VideoFormats, VideoFormat{
							Quality: h264.Quality,
							URL:     h264.URL,
							Size:    h264.Size,
						})
						// Also add to VideoURLs map
						if talk.VideoURLs == nil {
							talk.VideoURLs = make(map[string]string)
						}
						talk.VideoURLs[h264.Quality] = h264.URL
					}
				}
			}
		}
	})
	return nil
}

// extractSubtitleURLs extracts subtitle download URLs from the page
func (p *Parser) extractSubtitleURLs(doc *goquery.Document, talk *Talk) error {
	talk.SubtitleURLs = make(map[string]string)

	// Find subtitle links in the page
	doc.Find("a[data-language]").Each(func(i int, s *goquery.Selection) {
		lang := s.AttrOr("data-language", "")
		url, exists := s.Attr("href")
		if exists && lang != "" {
			if !strings.HasPrefix(url, "http") {
				url = baseURL + url
			}
			talk.SubtitleURLs[lang] = url
		}
	})

	return nil
}

// ParseURL parses a TED talk page directly from its URL
func (p *Parser) ParseURL(url string) (*Talk, error) {
	// Extract slug from URL
	u := strings.SplitN(url, "?", 2)[0] // Remove query parameters
	parts := strings.Split(u, "/")
	slug := ""
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			slug = parts[i]
			break
		}
	}
	// Slug must not be empty, must not be a domain, and must be under /talks/
	if slug == "" || strings.Contains(slug, ".") || !strings.Contains(u, "/talks/") {
		return nil, fmt.Errorf("invalid TED talk URL")
	}
	p.debugPrint("Processing slug: %s", slug)

	// Try GraphQL first
	talk, err := p.parseWithGraphQL(slug, url)
	if err != nil {
		p.debugPrint("GraphQL parsing failed: %v", err)
		// Fallback to HTML parsing
		p.debugPrint("Falling back to HTML parsing")
		return p.parseWithHTML(url)
	}
	return talk, nil
}

// parseWithGraphQL attempts to parse using GraphQL API
func (p *Parser) parseWithGraphQL(slug, url string) (*Talk, error) {
	// Create GraphQL request
	query := `query shareLinks($slug: String!, $language: String) {
		videos(
			slug: [$slug]
			language: $language
			first: 1
			isPublished: [true, false]
			channel: ALL
		) {
			nodes {
				id
				canonicalUrl
				audioDownload
				nativeDownloads {
					low
					medium
					high
				}
				subtitledDownloads {
					low
					high
					internalLanguageCode
					languageName
				}
			}
		}
	}`

	// Create request body
	reqBody := map[string]interface{}{
		"operationName": "shareLinks",
		"variables": map[string]interface{}{
			"slug":     slug,
			"language": "en",
		},
		"query": query,
	}

	// Convert request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", p.GraphqlURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "https://www.ted.com")
	req.Header.Set("Referer", url)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("X-Operation-Name", "shareLinks")

	// Send request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Println("close response body error:", cerr)
		}
	}()

	// Read and store raw response
	rawResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	p.storeRawResponse("graphql_"+slug, rawResp)

	// Parse response
	var result struct {
		Data struct {
			Videos struct {
				Nodes []struct {
					NativeDownloads struct {
						Low    string `json:"low"`
						Medium string `json:"medium"`
						High   string `json:"high"`
					} `json:"nativeDownloads"`
					SubtitledDownloads []struct {
						Low                  string `json:"low"`
						High                 string `json:"high"`
						InternalLanguageCode string `json:"internalLanguageCode"`
						LanguageName         string `json:"languageName"`
					} `json:"subtitledDownloads"`
				} `json:"nodes"`
			} `json:"videos"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(rawResp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	if len(result.Data.Videos.Nodes) == 0 {
		return nil, fmt.Errorf("no video data found")
	}

	// Create talk
	talk := &Talk{
		URL: url,
	}

	// Extract video URLs from subtitledDownloads
	talk.VideoURLs = make(map[string]string)
	node := result.Data.Videos.Nodes[0]

	// Find English version for video URLs
	for _, sub := range node.SubtitledDownloads {
		if sub.InternalLanguageCode == "en" {
			talk.VideoURLs["720p"] = sub.Low
			talk.VideoURLs["1080p"] = sub.High
			break
		}
	}

	// Extract subtitle URLs
	talk.SubtitleURLs = make(map[string]string)
	for _, sub := range node.SubtitledDownloads {
		if sub.Low != "" {
			lang := strings.ToLower(sub.InternalLanguageCode)
			talk.SubtitleURLs[lang] = sub.Low
		}
	}

	// Get talk details using regular HTTP client
	resp, err = http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch talk page: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Println("close response body error:", cerr)
		}
	}()

	// Read and store raw HTML response
	rawHTML, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTML response: %w", err)
	}
	p.storeRawResponse("html_"+slug, rawHTML)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(rawHTML))
	if err != nil {
		return nil, fmt.Errorf("failed to parse talk page: %w", err)
	}

	// Extract title and speaker
	talk.Title = doc.Find("h1").First().Text()
	talk.Speaker = doc.Find("h2").First().Text()

	p.debugPrint("Successfully parsed talk: %s by %s", talk.Title, talk.Speaker)
	p.debugPrint("Available subtitles: %v", talk.SubtitleURLs)

	return talk, nil
}

// parseWithHTML attempts to parse using HTML as fallback
func (p *Parser) parseWithHTML(url string) (*Talk, error) {
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch talk page: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Println("close response body error:", cerr)
		}
	}()

	// Read and store raw HTML response
	rawHTML, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HTML response: %w", err)
	}
	p.storeRawResponse("html_fallback", rawHTML)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(rawHTML))
	if err != nil {
		return nil, fmt.Errorf("failed to parse talk page: %w", err)
	}

	talk := &Talk{
		URL: url,
	}

	// Extract title and speaker
	talk.Title = doc.Find("h1").First().Text()
	talk.Speaker = doc.Find("h2").First().Text()

	// Try to extract video URLs from page's JSON data
	if err := p.extractVideoURLs(doc, talk); err != nil {
		p.debugPrint("Failed to extract video URLs from HTML: %v", err)
	}

	// Try to extract subtitle URLs
	if err := p.extractSubtitleURLs(doc, talk); err != nil {
		p.debugPrint("Failed to extract subtitle URLs from HTML: %v", err)
	}

	p.debugPrint("Fallback HTML parsing completed for: %s", talk.Title)

	// If没有视频和字幕，返回 error 和 nil
	if len(talk.VideoURLs) == 0 && len(talk.SubtitleURLs) == 0 {
		return nil, fmt.Errorf("no video or subtitle data found")
	}

	return talk, nil
}

// ParseTalkDetails parses a talk's details page and returns the talk information
func (p *Parser) ParseTalkDetails(title string) (*Talk, error) {
	// Search for the talk first
	talks, err := p.ParseTopic(title, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to search for talk: %w", err)
	}

	if len(talks) == 0 {
		return nil, fmt.Errorf("no talk found with title: %s", title)
	}

	// Parse the talk's details page using URL
	return p.ParseURL(talks[0].URL)
}

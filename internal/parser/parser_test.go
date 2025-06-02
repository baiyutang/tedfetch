package parser

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseTopic(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle different request paths
		switch r.URL.Path {
		case "/talks":
			// Verify request
			assert.Equal(t, "education", r.URL.Query().Get("topics[]"))

			// Return mock HTML response for talks list
			html := `
			<div class="media__message">
				<div class="media__message__title">
					<h4><a href="/talks/john_doe_power_of_education">The power of education</a></h4>
				</div>
				<div class="media__message__speaker">
					<h4>John Doe</h4>
				</div>
			</div>
			<div class="media__message">
				<div class="media__message__title">
					<h4><a href="/talks/jane_smith_learning_digital">Learning in the digital age</a></h4>
				</div>
				<div class="media__message__speaker">
					<h4>Jane Smith</h4>
				</div>
			</div>`
			if _, err := w.Write([]byte(html)); err != nil {
				t.Errorf("failed to write: %v", err)
			}

		case "/talks/john_doe_power_of_education", "/talks/jane_smith_learning_digital":
			// Return mock HTML response for individual talk page
			html := `
			<script>
			talkPage.init({
				"playerData": {
					"talks": [{
						"player_talks": [{
							"resources": {
								"h264": [
									{
										"quality": "1080p",
										"size": 1000000,
										"file": "https://example.com/video/1080p.mp4"
									},
									{
										"quality": "720p",
										"size": 500000,
										"file": "https://example.com/video/720p.mp4"
									}
								]
							}
						}]
					}]
				}
			});
			</script>
			<div class="talk-subtitles">
				<a href="/talks/subtitles/en" data-language="en">English</a>
				<a href="/talks/subtitles/zh" data-language="zh">Chinese</a>
			</div>`
			if _, err := w.Write([]byte(html)); err != nil {
				t.Errorf("failed to write: %v", err)
			}

		default:
			t.Errorf("Unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	// Create parser with test server URL
	p := New()
	p.client = &http.Client{
		Timeout: 5 * time.Second,
	}

	// Override the base URL for testing
	originalURL := baseURL
	baseURL = server.URL
	defer func() { baseURL = originalURL }()

	// Test parsing
	talks, err := p.ParseTopic("education", 2)
	assert.NoError(t, err)
	assert.Len(t, talks, 2)

	// Verify first talk
	assert.Equal(t, "The power of education", talks[0].Title)
	assert.Equal(t, "John Doe", talks[0].Speaker)
	assert.Equal(t, server.URL+"/talks/john_doe_power_of_education", talks[0].URL)

	// Verify video formats
	assert.Len(t, talks[0].VideoFormats, 2)
	assert.Equal(t, "1080p", talks[0].VideoFormats[0].Quality)
	assert.Equal(t, "https://example.com/video/1080p.mp4", talks[0].VideoFormats[0].URL)
	assert.Equal(t, int64(1000000), talks[0].VideoFormats[0].Size)
	assert.Equal(t, "720p", talks[0].VideoFormats[1].Quality)
	assert.Equal(t, "https://example.com/video/720p.mp4", talks[0].VideoFormats[1].URL)
	assert.Equal(t, int64(500000), talks[0].VideoFormats[1].Size)

	// Verify subtitle URLs
	assert.Len(t, talks[0].SubtitleURLs, 2)
	assert.Equal(t, server.URL+"/talks/subtitles/en", talks[0].SubtitleURLs["en"])
	assert.Equal(t, server.URL+"/talks/subtitles/zh", talks[0].SubtitleURLs["zh"])

	// Verify second talk
	assert.Equal(t, "Learning in the digital age", talks[1].Title)
	assert.Equal(t, "Jane Smith", talks[1].Speaker)
	assert.Equal(t, server.URL+"/talks/jane_smith_learning_digital", talks[1].URL)
}

func TestParseURL_GraphQL(t *testing.T) {
	// mock GraphQL response with real data structure
	graphqlJSON := []byte(`{
		"data": {
			"videos": {
				"nodes": [
					{
						"id": "399",
						"canonicalUrl": "https://www.ted.com/talks/test_slug",
						"audioDownload": null,
						"nativeDownloads": {
							"low": null,
							"medium": null,
							"high": null
						},
						"subtitledDownloads": [
							{
								"low": "https://download.ted.com/talks/test-low-en.mp4",
								"high": "https://download.ted.com/talks/test-480p-en.mp4",
								"internalLanguageCode": "en",
								"languageName": "English"
							},
							{
								"low": "https://download.ted.com/talks/test-low-zh-cn.mp4",
								"high": "https://download.ted.com/talks/test-480p-zh-cn.mp4",
								"internalLanguageCode": "zh-cn",
								"languageName": "Chinese, Simplified"
							}
						]
					}
				]
			}
		}
	}`)

	// mock HTML response
	html := `<html><h1>Test Title</h1><h2>Test Speaker</h2></html>`

	// unified mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(graphqlJSON)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer mockServer.Close()

	// Create parser with debug mode enabled
	p := &Parser{
		client:       mockServer.Client(),
		GraphqlURL:   mockServer.URL + "/graphql",
		Debug:        true,
		RawResponses: make(map[string][]byte),
	}

	// patch baseURL to mockServer
	oldBaseURL := baseURL
	baseURL = mockServer.URL
	defer func() { baseURL = oldBaseURL }()

	talk, err := p.ParseURL(mockServer.URL + "/talks/test_slug")
	assert.NoError(t, err)
	assert.Equal(t, "Test Title", talk.Title)
	assert.Equal(t, "Test Speaker", talk.Speaker)

	// Verify video URLs
	assert.Equal(t, "https://download.ted.com/talks/test-low-en.mp4", talk.VideoURLs["720p"])
	assert.Equal(t, "https://download.ted.com/talks/test-480p-en.mp4", talk.VideoURLs["1080p"])

	// Verify subtitle URLs
	assert.Equal(t, "https://download.ted.com/talks/test-low-en.mp4", talk.SubtitleURLs["en"])
	assert.Equal(t, "https://download.ted.com/talks/test-low-zh-cn.mp4", talk.SubtitleURLs["zh-cn"])

	// Verify raw responses were stored
	assert.NotEmpty(t, p.GetRawResponse("graphql_test_slug"))
	assert.NotEmpty(t, p.GetRawResponse("html_test_slug"))
}

func TestParseURL_GraphQLFallback(t *testing.T) {
	// mock GraphQL error response
	graphqlJSON := []byte(`{
		"errors": [
			{
				"message": "Invalid slug",
				"extensions": {
					"code": "GRAPHQL_VALIDATION_FAILED"
				}
			}
		]
	}`)

	// mock HTML response with video data
	html := `
	<html>
		<h1>Test Title</h1>
		<h2>Test Speaker</h2>
		<script>
		talkPage.init({
			"playerData": {
				"talks": [{
					"player_talks": [{
						"resources": {
							"h264": [
								{
									"quality": "1080p",
									"size": 1000000,
									"file": "https://example.com/video/1080p.mp4"
								},
								{
									"quality": "720p",
									"size": 500000,
									"file": "https://example.com/video/720p.mp4"
								}
							]
						}
					}]
				}]
			}
		});
		</script>
		<div class="talk-subtitles">
			<a href="/talks/subtitles/en" data-language="en">English</a>
			<a href="/talks/subtitles/zh" data-language="zh">Chinese</a>
		</div>
	</html>`

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(graphqlJSON)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer mockServer.Close()

	// Create parser with debug mode enabled
	p := &Parser{
		client:       mockServer.Client(),
		GraphqlURL:   mockServer.URL + "/graphql",
		Debug:        true,
		RawResponses: make(map[string][]byte),
	}

	// patch baseURL to mockServer
	oldBaseURL := baseURL
	baseURL = mockServer.URL
	defer func() { baseURL = oldBaseURL }()

	talk, err := p.ParseURL(mockServer.URL + "/talks/test_slug")
	assert.NoError(t, err)
	assert.Equal(t, "Test Title", talk.Title)
	assert.Equal(t, "Test Speaker", talk.Speaker)

	// Verify video URLs from HTML fallback
	assert.Equal(t, "https://example.com/video/720p.mp4", talk.VideoURLs["720p"])
	assert.Equal(t, "https://example.com/video/1080p.mp4", talk.VideoURLs["1080p"])

	// Verify subtitle URLs from HTML fallback
	assert.Equal(t, mockServer.URL+"/talks/subtitles/en", talk.SubtitleURLs["en"])
	assert.Equal(t, mockServer.URL+"/talks/subtitles/zh", talk.SubtitleURLs["zh"])

	// Verify raw responses were stored
	assert.NotEmpty(t, p.GetRawResponse("graphql_test_slug"))
	assert.NotEmpty(t, p.GetRawResponse("html_fallback"))
}

func TestParseURL_NoVideoData(t *testing.T) {
	// mock empty GraphQL response
	graphqlJSON := []byte(`{
		"data": {
			"videos": {
				"nodes": []
			}
		}
	}`)

	// mock HTML response without video data
	html := `<html><h1>Test Title</h1><h2>Test Speaker</h2></html>`

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(graphqlJSON)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer mockServer.Close()

	// Create parser with debug mode enabled
	p := &Parser{
		client:       mockServer.Client(),
		GraphqlURL:   mockServer.URL + "/graphql",
		Debug:        true,
		RawResponses: make(map[string][]byte),
	}

	// patch baseURL to mockServer
	oldBaseURL := baseURL
	baseURL = mockServer.URL
	defer func() { baseURL = oldBaseURL }()

	talk, err := p.ParseURL(mockServer.URL + "/talks/test_slug")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no video or subtitle data found")
	assert.Nil(t, talk)
}

func TestParseURL_InvalidSlug(t *testing.T) {
	// mock empty GraphQL response
	graphqlJSON := []byte(`{
		"data": {
			"videos": {
				"nodes": []
			}
		}
	}`)

	// mock HTML response without video data
	html := `<html><h1>Test Title</h1><h2>Test Speaker</h2></html>`

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(graphqlJSON)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer mockServer.Close()

	// Create parser with debug mode enabled
	p := &Parser{
		client:       mockServer.Client(),
		GraphqlURL:   mockServer.URL + "/graphql",
		Debug:        true,
		RawResponses: make(map[string][]byte),
	}

	// patch baseURL to mockServer
	oldBaseURL := baseURL
	baseURL = mockServer.URL
	defer func() { baseURL = oldBaseURL }()

	// Test with completely invalid URL
	_, err := p.ParseURL("not-a-ted-url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid TED talk URL")

	// Test with URL that has no slug
	_, err = p.ParseURL("https://www.ted.com/")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid TED talk URL")
}

func TestParseURL_GraphQLError(t *testing.T) {
	// mock GraphQL error response
	graphqlJSON := []byte(`{
		"errors": [
			{
				"message": "Invalid slug",
				"extensions": {
					"code": "GRAPHQL_VALIDATION_FAILED"
				}
			}
		]
	}`)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(graphqlJSON)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	p := &Parser{client: mockServer.Client(), GraphqlURL: mockServer.URL + "/graphql"}
	oldBaseURL := baseURL
	baseURL = mockServer.URL
	defer func() { baseURL = oldBaseURL }()

	_, err := p.ParseURL(mockServer.URL + "/talks/test_slug")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no video or subtitle data found")
}

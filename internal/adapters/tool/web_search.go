package tool

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	UserAgent               = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	defaultGoogleNewsRSSURL = "https://news.google.com/rss/search"
	defaultWebSearchTimeout = 10 * time.Second
)

type WebSearchTool struct {
	maxResults int
	client     *http.Client
	endpoint   string
}

type SearchResult struct {
	Title       string
	URL         string
	Description string
	Source      string
	PublishedAt string
}

type googleNewsRSS struct {
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			PubDate     string `xml:"pubDate"`
			Description string `xml:"description"`
			Source      struct {
				Text string `xml:",chardata"`
				URL  string `xml:"url,attr"`
			} `xml:"source"`
		} `xml:"item"`
	} `xml:"channel"`
}

func NewWebSearchTool(apiKey string, maxResults int) *WebSearchTool {
	_ = apiKey
	if maxResults <= 0 {
		maxResults = 5
	}
	return &WebSearchTool{
		maxResults: maxResults,
		client:     newDefaultWebSearchClient(),
		endpoint:   defaultGoogleNewsRSSURL,
	}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "Search Google News without an API key. Returns titles, URLs, sources, and snippets."
}

func (t *WebSearchTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"query": {
					"type": "string",
					"description": "The search query"
				},
				"count": {
					"type": "integer",
					"description": "Results (1-10)",
					"minimum": 1,
					"maximum": 10
				}
			},
			"required": ["query"]
		}`),
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	query, _ := params["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return "", errors.New("web search tool: `query` is required")
	}

	count := t.maxResults
	if rawCount, ok := params["count"]; ok {
		parsed, err := intValue(rawCount)
		if err != nil {
			return "", fmt.Errorf("web search tool: invalid `count`: %w", err)
		}
		count = parsed
	}
	count = min(10, max(1, count))

	if t.client == nil {
		t.client = newDefaultWebSearchClient()
	}
	if strings.TrimSpace(t.endpoint) == "" {
		t.endpoint = defaultGoogleNewsRSSURL
	}

	results, err := t.searchGoogleNewsRSS(ctx, query, count)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return fmt.Sprintf("No results found for: %s", query), nil
	}
	return t.formatResults(query, results), nil
}

func (t *WebSearchTool) searchGoogleNewsRSS(ctx context.Context, query string, count int) ([]SearchResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, defaultWebSearchTimeout)
	defer cancel()

	req, err := t.buildRequest(reqCtx, query)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google news RSS HTTP error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var feed googleNewsRSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decode RSS: %w", err)
	}

	results := make([]SearchResult, 0, min(count, len(feed.Channel.Items)))
	for _, item := range feed.Channel.Items {
		result := SearchResult{
			Title:       strings.TrimSpace(item.Title),
			URL:         strings.TrimSpace(item.Link),
			Description: cleanHTML(item.Description),
			Source:      strings.TrimSpace(item.Source.Text),
			PublishedAt: strings.TrimSpace(item.PubDate),
		}
		if result.Title == "" || result.URL == "" {
			continue
		}
		results = append(results, result)
		if len(results) == count {
			break
		}
	}

	return results, nil
}

func (t *WebSearchTool) buildRequest(ctx context.Context, query string) (*http.Request, error) {
	p := url.Values{}
	p.Set("q", query)
	p.Set("hl", "en-US")
	p.Set("gl", "US")
	p.Set("ceid", "US:en")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.endpoint+"?"+p.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", UserAgent)
	req.Close = true
	return req, nil
}

func (t *WebSearchTool) formatResults(query string, results []SearchResult) string {
	lines := []string{fmt.Sprintf("Results for: %s", query), ""}
	for i, r := range results {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, r.Title))
		lines = append(lines, fmt.Sprintf("   %s", r.URL))
		meta := make([]string, 0, 2)
		if r.Source != "" {
			meta = append(meta, "Source: "+r.Source)
		}
		if r.PublishedAt != "" {
			meta = append(meta, "Published: "+r.PublishedAt)
		}
		if len(meta) > 0 {
			lines = append(lines, "   "+strings.Join(meta, " | "))
		}
		if r.Description != "" {
			lines = append(lines, fmt.Sprintf("   %s", r.Description))
		}
		if i < len(results)-1 {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func newDefaultWebSearchClient() *http.Client {
	return &http.Client{
		Timeout: defaultWebSearchTimeout,
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			ForceAttemptHTTP2: false,
		},
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func intValue(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, err
		}
		return int(n), nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, err
		}
		return n, nil
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}

func cleanHTML(text string) string {
	replacer := strings.NewReplacer("<br>", " ", "<br/>", " ", "<br />", " ")
	text = replacer.Replace(text)
	for {
		start := strings.Index(text, "<")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], ">")
		if end == -1 {
			break
		}
		text = text[:start] + " " + text[start+end+1:]
	}
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

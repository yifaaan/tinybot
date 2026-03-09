package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"tinybot/internal/ports"

	readability "github.com/go-shiori/go-readability"
)

const (
	defaultWebFetchMaxChars = 50000
	defaultWebFetchTimeout  = 30 * time.Second
)

var (
	webFetchScriptPattern     = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	webFetchStylePattern      = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	webFetchTagPattern        = regexp.MustCompile(`(?is)<[^>]+>`)
	webFetchLinkPattern       = regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	webFetchHeadingPattern    = regexp.MustCompile(`(?is)<h([1-6])[^>]*>(.*?)</h[1-6]>`)
	webFetchListItemPattern   = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`)
	webFetchBlockClosePattern = regexp.MustCompile(`(?is)</(p|div|section|article|main|blockquote|ul|ol)>`)
	webFetchLineBreakPattern  = regexp.MustCompile(`(?is)<(br|hr)\s*/?>`)
	webFetchSpacePattern      = regexp.MustCompile(`[ \t\f\v]+`)
	webFetchNewlinePattern    = regexp.MustCompile(`\n{3,}`)
	webFetchTitlePattern      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

type WebFetchTool struct {
	maxChars int
	client   *http.Client
}

type webFetchResult struct {
	URL       string `json:"url"`
	FinalURL  string `json:"finalUrl"`
	Status    int    `json:"status"`
	Extractor string `json:"extractor"`
	Truncated bool   `json:"truncated"`
	Length    int    `json:"length"`
	Text      string `json:"text"`
}

type webFetchError struct {
	Error string `json:"error"`
	URL   string `json:"url"`
}

func NewWebFetchTool(maxChars int) *WebFetchTool {
	if maxChars < 100 {
		maxChars = defaultWebFetchMaxChars
	}
	return &WebFetchTool{
		maxChars: maxChars,
		client:   newDefaultWebFetchClient(),
	}
}

func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

func (t *WebFetchTool) Description() string {
	return "Fetch URL and extract readable content (HTML to markdown/text)."
}

func (t *WebFetchTool) Spec() ports.ToolSpec {
	return ports.ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": {
					"type": "string",
					"description": "URL to fetch"
				},
				"extractMode": {
					"type": "string",
					"enum": ["markdown", "text"],
					"default": "markdown"
				},
				"maxChars": {
					"type": "integer",
					"minimum": 100
				}
			},
			"required": ["url"]
		}`),
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	rawURL, _ := params["url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("web fetch tool: `url` is required")
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return "", fmt.Errorf("web fetch tool: invalid `url`: %w", err)
	}

	extractMode := "markdown"
	if value, ok := params["extractMode"].(string); ok && strings.TrimSpace(value) != "" {
		extractMode = strings.ToLower(strings.TrimSpace(value))
	}
	if extractMode != "markdown" && extractMode != "text" {
		return "", errors.New("web fetch tool: `extractMode` must be 'markdown' or 'text'")
	}

	maxChars := t.maxChars
	if rawMaxChars, ok := params["maxChars"]; ok {
		parsed, err := intValue(rawMaxChars)
		if err != nil {
			return "", fmt.Errorf("web fetch tool: invalid `maxChars`: %w", err)
		}
		if parsed < 100 {
			return "", errors.New("web fetch tool: `maxChars` must be at least 100")
		}
		maxChars = parsed
	}
	if maxChars < 100 {
		maxChars = defaultWebFetchMaxChars
	}

	if t.client == nil {
		t.client = newDefaultWebFetchClient()
	}

	result, err := t.fetch(ctx, rawURL, extractMode, maxChars)
	if err != nil {
		return marshalJSON(webFetchError{Error: err.Error(), URL: rawURL}), nil
	}
	return marshalJSON(result), nil
}

func (t *WebFetchTool) fetch(ctx context.Context, rawURL string, extractMode string, maxChars int) (webFetchResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, defaultWebFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return webFetchResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json,text/plain,*/*")

	resp, err := t.client.Do(req)
	if err != nil {
		return webFetchResult{}, fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return webFetchResult{}, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return webFetchResult{}, fmt.Errorf("read response body: %w", err)
	}

	text, extractor := extractFetchedContent(resp, body, extractMode)
	runes := []rune(text)
	truncated := len(runes) > maxChars
	if truncated {
		runes = runes[:maxChars]
		text = string(runes)
	}

	return webFetchResult{
		URL:       rawURL,
		FinalURL:  resp.Request.URL.String(),
		Status:    resp.StatusCode,
		Extractor: extractor,
		Truncated: truncated,
		Length:    len(runes),
		Text:      text,
	}, nil
}

func extractFetchedContent(resp *http.Response, body []byte, extractMode string) (string, string) {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if isJSONContentType(contentType) {
		return prettyJSON(body), "json"
	}
	if isHTMLContentType(contentType) || looksLikeHTML(body) {
		text, extractor := extractHTMLContent(resp.Request.URL, body, extractMode)
		return text, extractor
	}
	return string(body), "raw"
}

func extractHTMLContent(pageURL *url.URL, body []byte, extractMode string) (string, string) {
	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err == nil {
		var content string
		if extractMode == "markdown" {
			content = htmlToMarkdown(article.Content)
		} else {
			content = normalizeExtractedText(article.TextContent)
			if content == "" {
				content = stripHTMLTags(article.Content)
			}
		}
		content = prependTitle(strings.TrimSpace(article.Title), content)
		if content != "" {
			return content, "readability"
		}
	}

	rawHTML := string(body)
	var content string
	if extractMode == "markdown" {
		content = htmlToMarkdown(rawHTML)
	} else {
		content = stripHTMLTags(rawHTML)
	}
	content = prependTitle(extractTitle(rawHTML), content)
	return content, "html"
}

func htmlToMarkdown(source string) string {
	text := webFetchScriptPattern.ReplaceAllString(source, "")
	text = webFetchStylePattern.ReplaceAllString(text, "")
	text = webFetchLinkPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := webFetchLinkPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return stripHTMLTags(match)
		}
		href := html.UnescapeString(strings.TrimSpace(parts[1]))
		label := stripHTMLTags(parts[2])
		if href == "" {
			return label
		}
		if label == "" {
			label = href
		}
		return fmt.Sprintf("[%s](%s)", label, href)
	})
	text = webFetchHeadingPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := webFetchHeadingPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return stripHTMLTags(match)
		}
		level := strings.TrimSpace(parts[1])
		heading := stripHTMLTags(parts[2])
		if heading == "" {
			return ""
		}
		return fmt.Sprintf("\n%s %s\n", strings.Repeat("#", len(level)), heading)
	})
	text = webFetchListItemPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := webFetchListItemPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return ""
		}
		item := stripHTMLTags(parts[1])
		if item == "" {
			return ""
		}
		return "\n- " + item
	})
	text = webFetchBlockClosePattern.ReplaceAllString(text, "\n\n")
	text = webFetchLineBreakPattern.ReplaceAllString(text, "\n")
	return stripHTMLTags(text)
}

func stripHTMLTags(source string) string {
	text := webFetchScriptPattern.ReplaceAllString(source, "")
	text = webFetchStylePattern.ReplaceAllString(text, "")
	text = webFetchTagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	return normalizeExtractedText(text)
}

func normalizeExtractedText(source string) string {
	text := strings.ReplaceAll(source, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = webFetchSpacePattern.ReplaceAllString(text, " ")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")
	text = webFetchNewlinePattern.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

func prependTitle(title string, content string) string {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" {
		return content
	}
	if content == "" {
		return "# " + title
	}
	return "# " + title + "\n\n" + content
}

func extractTitle(source string) string {
	parts := webFetchTitlePattern.FindStringSubmatch(source)
	if len(parts) < 2 {
		return ""
	}
	return stripHTMLTags(parts[1])
}

func isJSONContentType(contentType string) bool {
	return strings.Contains(contentType, "application/json") || strings.Contains(contentType, "+json")
}

func isHTMLContentType(contentType string) bool {
	return strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml+xml")
}

func looksLikeHTML(body []byte) bool {
	prefix := strings.ToLower(strings.TrimSpace(string(body[:min(256, len(body))])))
	return strings.HasPrefix(prefix, "<!doctype") || strings.HasPrefix(prefix, "<html")
}

func prettyJSON(body []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err == nil {
		return out.String()
	}
	return string(body)
}

func marshalJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		fallback := fmt.Sprintf(`{"error":"failed to marshal response: %s"}`, strings.ReplaceAll(err.Error(), `"`, `'`))
		return fallback
	}
	return string(data)
}

func newDefaultWebFetchClient() *http.Client {
	return &http.Client{
		Timeout: defaultWebFetchTimeout,
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			ForceAttemptHTTP2: false,
		},
	}
}

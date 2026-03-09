package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebSearchTool_Execute(t *testing.T) {
	t.Run("missing query", func(t *testing.T) {
		tool := NewWebSearchTool("ignored", 5)

		_, err := tool.Execute(context.Background(), map[string]any{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "query") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("searches Google News RSS and formats results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("q"); got != "OpenAI latest news" {
				t.Fatalf("q = %q, want %q", got, "OpenAI latest news")
			}
			if got := r.URL.Query().Get("hl"); got != "en-US" {
				t.Fatalf("hl = %q, want %q", got, "en-US")
			}
			w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <item>
      <title>Introducing GPT-5.4 - OpenAI</title>
      <link>https://news.google.com/rss/articles/test-1</link>
      <pubDate>Thu, 05 Mar 2026 20:40:54 GMT</pubDate>
      <description><![CDATA[<a href="https://news.google.com/rss/articles/test-1">Introducing GPT-5.4</a>&nbsp;&nbsp;<font color="#6f6f6f">OpenAI</font>]]></description>
      <source url="https://openai.com">OpenAI</source>
    </item>
    <item>
      <title>OpenAI robotics leader resigns - NPR</title>
      <link>https://news.google.com/rss/articles/test-2</link>
      <pubDate>Sun, 08 Mar 2026 20:44:11 GMT</pubDate>
      <description><![CDATA[<a href="https://news.google.com/rss/articles/test-2">OpenAI robotics leader resigns</a>&nbsp;&nbsp;<font color="#6f6f6f">NPR</font>]]></description>
      <source url="https://www.npr.org">NPR</source>
    </item>
  </channel>
</rss>`))
		}))
		defer server.Close()

		tool := NewWebSearchTool("ignored", 5)
		tool.endpoint = server.URL
		tool.client = server.Client()

		got, err := tool.Execute(context.Background(), map[string]any{
			"query": "OpenAI latest news",
			"count": 2,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(got, "Results for: OpenAI latest news") {
			t.Fatalf("missing header: %q", got)
		}
		if !strings.Contains(got, "1. Introducing GPT-5.4 - OpenAI") {
			t.Fatalf("missing first result: %q", got)
		}
		if !strings.Contains(got, "Source: OpenAI") {
			t.Fatalf("missing source metadata: %q", got)
		}
		if !strings.Contains(got, "Published: Thu, 05 Mar 2026 20:40:54 GMT") {
			t.Fatalf("missing published metadata: %q", got)
		}
		if !strings.Contains(got, "https://news.google.com/rss/articles/test-2") {
			t.Fatalf("missing second link: %q", got)
		}
	})

	t.Run("no results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel></channel></rss>`))
		}))
		defer server.Close()

		tool := NewWebSearchTool("ignored", 5)
		tool.endpoint = server.URL
		tool.client = server.Client()

		got, err := tool.Execute(context.Background(), map[string]any{"query": "nothing"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "No results found for: nothing" {
			t.Fatalf("got = %q", got)
		}
	})
}

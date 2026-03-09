package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebFetchTool_Execute(t *testing.T) {
	t.Run("missing url", func(t *testing.T) {
		tool := NewWebFetchTool(50000)

		_, err := tool.Execute(context.Background(), map[string]any{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "url") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid extractMode", func(t *testing.T) {
		tool := NewWebFetchTool(50000)

		_, err := tool.Execute(context.Background(), map[string]any{
			"url":         "https://example.com",
			"extractMode": "html",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "extractMode") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fetches JSON and follows redirects", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/json", http.StatusFound)
		})
		mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("User-Agent"); got != UserAgent {
				t.Fatalf("User-Agent = %q, want %q", got, UserAgent)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"tinybot","ok":true}`))
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		tool := NewWebFetchTool(50000)
		tool.client = server.Client()

		got, err := tool.Execute(context.Background(), map[string]any{
			"url": server.URL + "/redirect",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result webFetchResult
		if err := json.Unmarshal([]byte(got), &result); err != nil {
			t.Fatalf("unmarshal result: %v\nraw: %s", err, got)
		}
		if result.URL != server.URL+"/redirect" {
			t.Fatalf("url = %q", result.URL)
		}
		if !strings.HasSuffix(result.FinalURL, "/json") {
			t.Fatalf("finalUrl = %q", result.FinalURL)
		}
		if result.Status != http.StatusOK {
			t.Fatalf("status = %d", result.Status)
		}
		if result.Extractor != "json" {
			t.Fatalf("extractor = %q", result.Extractor)
		}
		if result.Truncated {
			t.Fatalf("truncated = true, want false")
		}
		if !strings.Contains(result.Text, "\"name\": \"tinybot\"") {
			t.Fatalf("formatted JSON missing expected key: %q", result.Text)
		}
	})

	t.Run("extracts HTML as markdown", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><title>Example Article</title></head>
  <body>
    <article>
      <h1>Example Article</h1>
      <p>First paragraph with a <a href="https://example.com/link">link</a>.</p>
      <ul>
        <li>Alpha</li>
        <li>Beta</li>
      </ul>
    </article>
  </body>
</html>`))
		}))
		defer server.Close()

		tool := NewWebFetchTool(50000)
		tool.client = server.Client()

		got, err := tool.Execute(context.Background(), map[string]any{
			"url":         server.URL,
			"extractMode": "markdown",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result webFetchResult
		if err := json.Unmarshal([]byte(got), &result); err != nil {
			t.Fatalf("unmarshal result: %v\nraw: %s", err, got)
		}
		if result.Extractor != "readability" {
			t.Fatalf("extractor = %q", result.Extractor)
		}
		if !strings.Contains(result.Text, "# Example Article") {
			t.Fatalf("missing title: %q", result.Text)
		}
		if !strings.Contains(result.Text, "First paragraph") {
			t.Fatalf("missing content: %q", result.Text)
		}
		if !strings.Contains(result.Text, "Alpha") || !strings.Contains(result.Text, "Beta") {
			t.Fatalf("missing list content: %q", result.Text)
		}
	})

	t.Run("extracts HTML as text", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!doctype html>
<html>
  <head><title>Plain Title</title></head>
  <body>
    <main>
      <p>Readable body with link text only.</p>
    </main>
  </body>
</html>`))
		}))
		defer server.Close()

		tool := NewWebFetchTool(50000)
		tool.client = server.Client()

		got, err := tool.Execute(context.Background(), map[string]any{
			"url":         server.URL,
			"extractMode": "text",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result webFetchResult
		if err := json.Unmarshal([]byte(got), &result); err != nil {
			t.Fatalf("unmarshal result: %v\nraw: %s", err, got)
		}
		if result.Extractor != "readability" {
			t.Fatalf("extractor = %q", result.Extractor)
		}
		if !strings.Contains(result.Text, "# Plain Title") {
			t.Fatalf("missing title: %q", result.Text)
		}
		if !strings.Contains(result.Text, "Readable body with link text only.") {
			t.Fatalf("missing body: %q", result.Text)
		}
		if strings.Contains(result.Text, "[") || strings.Contains(result.Text, "](") {
			t.Fatalf("expected text output, got markdown-like content: %q", result.Text)
		}
	})

	t.Run("truncates long responses", func(t *testing.T) {
		body := strings.Repeat("a", 160)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(body))
		}))
		defer server.Close()

		tool := NewWebFetchTool(50000)
		tool.client = server.Client()

		got, err := tool.Execute(context.Background(), map[string]any{
			"url":      server.URL,
			"maxChars": 120,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result webFetchResult
		if err := json.Unmarshal([]byte(got), &result); err != nil {
			t.Fatalf("unmarshal result: %v\nraw: %s", err, got)
		}
		if result.Extractor != "raw" {
			t.Fatalf("extractor = %q", result.Extractor)
		}
		if !result.Truncated {
			t.Fatalf("truncated = false, want true")
		}
		if result.Length != 120 {
			t.Fatalf("length = %d, want 120", result.Length)
		}
		if len([]rune(result.Text)) != 120 {
			t.Fatalf("text length = %d, want 120", len([]rune(result.Text)))
		}
	})

	t.Run("returns structured error for non-2xx responses", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusNotFound)
		}))
		defer server.Close()

		tool := NewWebFetchTool(50000)
		tool.client = server.Client()

		got, err := tool.Execute(context.Background(), map[string]any{
			"url": server.URL,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result webFetchError
		if err := json.Unmarshal([]byte(got), &result); err != nil {
			t.Fatalf("unmarshal result: %v\nraw: %s", err, got)
		}
		if result.URL != server.URL {
			t.Fatalf("url = %q", result.URL)
		}
		if !strings.Contains(result.Error, "404 Not Found") {
			t.Fatalf("error = %q", result.Error)
		}
	})
}

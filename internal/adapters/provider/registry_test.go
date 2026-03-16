package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"tinybot/internal/domain/model"
	"tinybot/internal/service/chat"
)

// fakeClient 是测试用的假 CompletionClient
type fakeClient struct{}

func (f *fakeClient) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	return model.LLMResponse{Content: "fake"}, nil
}

// TestRegistry_Create_Known —— 注册已知 kind，Create 应该成功
func TestRegistry_Create_Known(t *testing.T) {
	r := NewRegistry()
	r.Register("test", func(apiKey, apiBase, model string, enableThinking bool) (chat.CompletionClient, error) {
		return &fakeClient{}, nil
	})

	client, err := r.Create("test", "key", "base", "model", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// TestRegistry_Create_Unknown —— 未注册的 kind，Create 应该返回错误
func TestRegistry_Create_Unknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Create("nonexistent", "key", "base", "model", false)
	if err == nil {
		t.Fatal("expected error for unknown provider kind")
	}
	if !strings.Contains(err.Error(), "unknown provider kind") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestDefaultRegistry_HasBuiltins —— DefaultRegistry 应该包含预注册的 provider
func TestDefaultRegistry_HasBuiltins(t *testing.T) {
	r := DefaultRegistry()

	for _, kind := range []string{"openai", "qwen", "deepseek"} {
		_, err := r.Create(kind, "", "", "", false)
		if err == nil {
			t.Fatalf("expected error for missing API key when creating provider %q", kind)
		}
	}

	ollamaClient, err := r.Create("ollama", "", "", "", false)
	if err != nil {
		t.Fatalf("unexpected error creating ollama provider: %v", err)
	}
	if ollamaClient == nil {
		t.Fatal("expected non-nil ollama client")
	}
}

// TestRegistry_Register_Override —— 重复注册应该覆盖旧工厂
func TestRegistry_Register_Override(t *testing.T) {
	r := NewRegistry()
	factoryA := func(apiKey, apiBase, model string, enableThinking bool) (chat.CompletionClient, error) {
		return &fakeClient{}, nil
	}
	factoryB := func(apiKey, apiBase, model string, enableThinking bool) (chat.CompletionClient, error) {
		return &fakeClient{}, nil
	}
	r.Register("x", factoryA)
	r.Register("x", factoryB)
	b := r.factories["x"]
	if fmt.Sprintf("%p", b) != fmt.Sprintf("%p", factoryB) {
		t.Fatal("expected factoryB to override factoryA")
	}
}

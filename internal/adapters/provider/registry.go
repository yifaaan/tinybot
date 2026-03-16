package provider

import (
	"fmt"
	"tinybot/internal/service/chat"
)

// ProviderFactory 是创建 CompletionClient 的工厂函数类型。
//
// enableThinking 参数让每个 provider 自行决定是否/如何启用推理模式：
// - Qwen3：发送 enable_thinking=true，解析 reasoning_content
// - DeepSeek R1：同上
// - OpenAI / Ollama：忽略此参数（不支持或有自己的 reasoning_effort 机制）
type ProviderFactory func(apiKey, apiBase, model string, enableThinking bool) (chat.CompletionClient, error)

type Registry struct {
	factories map[string]ProviderFactory
}

func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ProviderFactory),
	}
}

func (r *Registry) Register(kind string, factory ProviderFactory) {
	r.factories[kind] = factory
}

func (r *Registry) Create(kind, apiKey, apiBase, model string, enableThinking bool) (chat.CompletionClient, error) {
	factory, ok := r.factories[kind]
	if !ok {
		return nil, fmt.Errorf("unknown provider kind: %q", kind)
	}
	return factory(apiKey, apiBase, model, enableThinking)
}

func DefaultRegistry() *Registry {
	r := NewRegistry()

	r.Register("openai", func(apiKey, apiBase, model string, enableThinking bool) (chat.CompletionClient, error) {
		return NewOpenAIProvider(apiKey, apiBase, model)
	})
	r.Register("qwen", func(apiKey, apiBase, model string, enableThinking bool) (chat.CompletionClient, error) {
		return NewQwenProvider(apiKey, apiBase, model, enableThinking)
	})
	r.Register("ollama", func(apiKey, apiBase, model string, enableThinking bool) (chat.CompletionClient, error) {
		return NewOllamaProvider(apiKey, apiBase, model)
	})
	r.Register("deepseek", func(apiKey, apiBase, model string, enableThinking bool) (chat.CompletionClient, error) {
		return NewDeepseekProvider(apiKey, apiBase, model, enableThinking)
	})
	return r
}

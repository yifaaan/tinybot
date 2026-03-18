package provider

import (
	"fmt"
	"tinybot/internal/service/chat"
)

type Options struct {
	EnableThinking   bool
	ReasoningEffort  string
	ReasoningSummary string
	TextVerbosity    string
}

type ProviderFactory func(apiKey, apiBase, model string, options Options) (chat.CompletionClient, error)

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

func (r *Registry) Create(kind, apiKey, apiBase, model string, options Options) (chat.CompletionClient, error) {
	factory, ok := r.factories[kind]
	if !ok {
		return nil, fmt.Errorf("unknown provider kind: %q", kind)
	}
	return factory(apiKey, apiBase, model, options)
}

func DefaultRegistry() *Registry {
	r := NewRegistry()

	r.Register("openai", func(apiKey, apiBase, model string, options Options) (chat.CompletionClient, error) {
		return NewOpenAIProvider(apiKey, apiBase, model)
	})
	r.Register("openai-responses", func(apiKey, apiBase, model string, options Options) (chat.CompletionClient, error) {
		return NewOpenAIResponsesProvider(apiKey, apiBase, model, options)
	})
	r.Register("qwen", func(apiKey, apiBase, model string, options Options) (chat.CompletionClient, error) {
		return NewQwenProvider(apiKey, apiBase, model, options.EnableThinking)
	})
	r.Register("ollama", func(apiKey, apiBase, model string, options Options) (chat.CompletionClient, error) {
		return NewOllamaProvider(apiKey, apiBase, model)
	})
	r.Register("deepseek", func(apiKey, apiBase, model string, options Options) (chat.CompletionClient, error) {
		return NewDeepseekProvider(apiKey, apiBase, model, options.EnableThinking)
	})
	return r
}

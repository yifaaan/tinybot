package provider

import (
	"fmt"
	"tinybot/internal/service/chat"
)

// ProviderFactory 是创建 CompletionClient 的工厂函数类型。
//
// 为什么用函数类型而不是接口？
// - 每个 provider 的"创建"只是一段构造代码，不需要方法集
// - 函数类型是 Go 里最轻量的"策略模式"实现方式
// - 避免为每个 provider 额外定义 Builder/Factory 接口
type ProviderFactory func(apiKey, apiBase, model string) (chat.CompletionClient, error)

type Registry struct {
	factories map[string]ProviderFactory
}

// Registry 按 kind 查找 provider 工厂。
//
// 生命周期：
// - 在 app 启动时，通过 DefaultRegistry() 预注册所有内置 provider
// - 运行时只读，不存在并发写入问题
// - 如果未来需要动态注册（比如插件），再加锁
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ProviderFactory),
	}
}

func (r *Registry) Register(kind string, factory ProviderFactory) {
	r.factories[kind] = factory
}

// Create 根据 kind 查找工厂函数，创建 chat.CompletionClient
func (r *Registry) Create(kind, apiKey, apiBase, model string) (chat.CompletionClient, error) {
	factory, ok := r.factories[kind]
	if !ok {
		return nil, fmt.Errorf("unknown provider kind: %q", kind)
	}
	return factory(apiKey, apiBase, model)
}

func DefaultRegistry() *Registry {
	r := NewRegistry()

	r.Register("openai", func(apiKey, apiBase, model string) (chat.CompletionClient, error) {
		return NewOpenAIProvider(apiKey, apiBase, model)
	})
	r.Register("qwen", func(apiKey, apiBase, model string) (chat.CompletionClient, error) {
		return NewQwenProvider(apiKey, apiBase, model)
	})
	r.Register("ollama", func(apiKey, apiBase, model string) (chat.CompletionClient, error) {
		return NewOllamaProvider(apiKey, apiBase, model)
	})
	r.Register("deepseek", func(apiKey, apiBase, model string) (chat.CompletionClient, error) {
		return NewDeepseekProvider(apiKey, apiBase, model)
	})
	return r
}

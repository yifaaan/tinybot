package tool

import (
	"context"
	"fmt"
	"tinybot/internal/domain/model"
	"tinybot/internal/utils/logger"
)

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register a tool in the registry.
func (r *Registry) Register(tool Tool) {
	if _, exists := r.tools[tool.Spec().Name]; exists {
		return
	}
	r.tools[tool.Spec().Name] = tool
}

// Unregister a tool from the registry.
func (r *Registry) Unregister(name string) {
	delete(r.tools, name)
}

// Get a tool from the registry.
func (r *Registry) Get(name string) (Tool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// Check if a tool is registered in the registry.
func (r *Registry) Has(name string) bool {
	_, exists := r.tools[name]
	return exists
}

// Get all tool definitions in OpenAI function schema format.
func (r *Registry) GetDefinitions() []map[string]any {
	definations := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		definations = append(definations, tool.Spec().ToSchema())
	}
	return definations
}

// Execute a tool by name with given parameters.
func (r *Registry) Execute(ctx context.Context, name string, params map[string]any) (string, error) {
	logger.Debug("tool execute start", "name", name, "params", params)

	tool, ok := r.Get(name)
	if !ok {
		logger.Warn("tool not found", "name", name)
		return "", fmt.Errorf("tool %s not found", name)
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		logger.Warn("tool execute failed", "name", name, "error", err)
	} else {
		logger.Debug("tool execute success", "name", name, "result_len", len(result))
	}
	return result, err
}

func (r *Registry) ToolNames() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// SetMessageContext 把当前会话上下文传给 message tool
func (r *Registry) SetMessageContext(channel model.Channel, chatID string) {
	tool, ok := r.messageTool()
	if !ok {
		return
	}
	tool.SetContext(channel, chatID)
}

func (r *Registry) SetMessageCallback(callback SendMessageCallback) {
	tool, ok := r.messageTool()
	if !ok {
		return
	}
	tool.SetSendMessageCallback(callback)
}

func (r *Registry) messageTool() (*MessageTool, bool) {
	raw, ok := r.Get("message")
	if !ok {
		return nil, false
	}
	mt, ok := raw.(*MessageTool)
	if !ok {
		return nil, false
	}
	return mt, true
}

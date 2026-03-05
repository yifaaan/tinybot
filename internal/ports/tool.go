package ports

import (
	"context"
	"encoding/json"
)

type ToolSpec struct {
	Name        string          // Tool name used in function calls
	Description string          // Description of what the tool does
	Parameters  json.RawMessage // JSON Schema for tool parameters
}

// ToSchema converts the tool spec to an OpenAI function schema format.
func (t ToolSpec) ToSchema() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Parameters,
		},
	}
}

type Tool interface {
	Spec() ToolSpec
	Execute(ctx context.Context, params map[string]any) (string, error)
}

// ToolRegistry is a registry of agent tools.
// It allows dynamic registration and execution of tools.
type ToolRegistry interface {
	// Register a tool in the registry.
	Register(tool Tool)
	// Unregister a tool from the registry.
	Unregister(name string)
	// Get a tool from the registry.
	Get(name string) (Tool, bool)
	// Check if a tool is registered in the registry.
	Has(name string) bool
	// Get all tool definitions in OpenAI function schema format.
	GetDefinitions() []map[string]any
	// Execute a tool by name with given parameters.
	Execute(ctx context.Context, name string, params map[string]any) (string, error)
}

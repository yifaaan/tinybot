package tool

import (
	"context"
	"encoding/json"
)

// ToolSpec describes one concrete tool and exposes the schema expected by the model.
//
// Responsibilities:
//   - name the tool as it appears in model tool calls
//   - describe the tool behavior for prompt/schema generation
//   - hold the JSON schema for tool parameters
//
// Inputs: name, description, and a JSON-schema parameter document.
// Outputs: OpenAI-style function schema maps via ToSchema.
// State changes: none.
// Side effects: none.
// Compatibility: mirrors the previous ports.ToolSpec shape so existing tool behavior and tests remain stable while removing the old shared ports dependency.
type ToolSpec struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// ToSchema converts the tool spec to the function-call schema format expected by the model client.
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

// Tool is the concrete adapter-side contract implemented by built-in tools.
//
// Responsibilities: describe the tool and execute it with decoded parameters.
// Inputs: execution context and JSON-like parameter maps.
// Outputs: string results or execution errors.
// State changes: implementation-specific, such as file edits or shell execution.
// Side effects: implementation-specific I/O, subprocess, or network work.
// Compatibility: keeps the current built-in tools self-contained inside the adapter layer.
type Tool interface {
	Spec() ToolSpec
	Execute(ctx context.Context, params map[string]any) (string, error)
}

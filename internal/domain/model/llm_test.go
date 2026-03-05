package model

import "testing"

func TestLLMResponse_HasToolCalls(t *testing.T) {
	tests := []struct {
		name string
		resp LLMResponse
		want bool
	}{
		{
			name: "no tool calls",
			resp: LLMResponse{Content: "hello"},
			want: false,
		},
		{
			name: "with tool calls",
			resp: LLMResponse{
				ToolCalls: []ToolCall{{ID: "1", Name: "search"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resp.HasToolCalls(); got != tt.want {
				t.Errorf("HasToolCalls() = %v, want %v", got, tt.want)
			}
		})
	}
}

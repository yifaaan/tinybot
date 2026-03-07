package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"tinybot/internal/adapters/workspace"
	"tinybot/internal/domain/model"
)

var defaultBootstrapFiles = []string{
	"AGENTS.md",
	"SOUL.md",
	"USER.md",
	"TOOLS.md",
}

// ContextBuilder assembles the messages sent to the LLM.
//
// This is intentionally small for now. It gives you a stable place to grow
// into the fuller nanobot-style prompt builder later.
type ContextBuilder struct {
	workspacePath string
	memory        *workspace.MemoryStore  // for future use when we add memory context(Long-term and Today) to the system prompt
	skills        *workspace.SkillsLoader // for future use when we add skills context to the system prompt and messages
}

func NewContextBuilder(wp string) *ContextBuilder {
	return &ContextBuilder{
		workspacePath: strings.TrimSpace(wp),
		memory:        workspace.NewMemoryStore(wp),
		skills:        workspace.NewSkillsLoader(wp, ""),
	}
}

// BuildMessages build the complete message list for an LLM call.
// Args:
//
//	history: Previous conversation messages.
//	currentMessage: The new user message.
//	skillNames: Optional skills to include.
//
// Returns:
//
//	List of messages including system prompt.
func (b *ContextBuilder) BuildMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any {
	messages := make([]map[string]any, 0, len(history)+2)

	// System prompt
	systemPrompt := b.BuildSystemPrompt(skillNames)
	messages = append(messages, map[string]any{
		"role":    "system",
		"content": systemPrompt,
	})

	// History
	his := toLLMMessages(history)
	messages = append(messages, his...)

	// Current message
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": currentMessage,
	})
	return messages
}

// AddToolResult adds a tool result to the message list.
// Args:
//
//	messages: Current message list.
//	toolCallID: ID of the tool call.
//	toolName: Name of the tool.
//	result: Tool execution result.
//
// Returns:
//
//	Updated message list.
func (b *ContextBuilder) AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any {
	return append(messages, map[string]any{
		"role":         "tool",
		"tool_call_id": toolCallID,
		"name":         toolName,
		"content":      result,
	})
}

// AddAssistantMessage adds an assistant message to the message list.
// Args:
//
//	messages: Current message list.
//	content: Message content.
//	toolCalls: Optional tool calls.
//
// Returns:
//
//	Updated message list.
func (b *ContextBuilder) AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any {
	msg := map[string]any{
		"role": "assistant",
	}
	if content != "" {
		msg["content"] = content
	}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	messages = append(messages, msg)
	return messages
}

// BuildSystemPrompt returns the initial system prompt.
//
// TODO:
// - refine the identity prompt
// - add memory and skills sections
// - align wording with the original nanobot ContextBuilder
func (b *ContextBuilder) BuildSystemPrompt(skillNames []string) string {
	// Core identity
	parts := []string{
		b.renderIdentity(),
	}

	// if b.workspace != "" {
	// 	parts = append(parts, fmt.Sprintf("Workspace: %s", b.workspace))
	// }

	// Bootstrap files
	if docs := b.collectBootstrapDocs(); docs != "" {
		parts = append(parts, docs)
	}

	// Add memory context
	memoryContext := b.memory.BuildContext()
	if memoryContext != "" {
		parts = append(parts, fmt.Sprintf("## Memory\n%s", memoryContext))
	}
	// Skills - prograssive loading
	// TODO:1.Always-loaded skills: include full content

	// 2.Available skills: only show summary (agent uses read_file to load)
	skillsSummary, err := b.skills.BuildSummary()
	if err == nil && skillsSummary != "" {
		parts = append(parts, fmt.Sprintf(
			`# Skills

The following skills extend your capabilities. To use a skill, read its SKILL.md file using the read_file tool.
Skills with available="false" need dependencies installed first - you can try installing them with apt/brew.

%s`, skillsSummary))
	}

	return strings.Join(parts, "\n\n--\n\n")
}

// collectBootstrapDocs collects all bootstrap files in the workspace.
func (b *ContextBuilder) collectBootstrapDocs() string {
	if b.workspacePath == "" {
		return ""
	}

	parts := make([]string, 0, len(defaultBootstrapFiles))
	for _, name := range defaultBootstrapFiles {
		content := strings.TrimSpace(b.readWorkspaceFile(name))
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("## %s\n\n%s", name, content))
	}

	return strings.Join(parts, "\n\n")
}

// readWorkspaceFile reads a file from the workspace.
func (b *ContextBuilder) readWorkspaceFile(name string) string {
	if b.workspacePath == "" {
		return ""
	}

	path := filepath.Join(b.workspacePath, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func (b *ContextBuilder) getIdentity() string {
	return `# tinybot 🐈

You are tinybot, a helpful AI assistant. You have access to tools that allow you to:
- Read, write, and edit files
- Execute shell commands
- Search the web and fetch web pages
- Send messages to users on chat channels

## Current Time
{now}

## Workspace
Your workspace is at: {workspace_path}
- Memory files: {workspace_path}/memory/MEMORY.md
- Daily notes: {workspace_path}/memory/YYYY-MM-DD.md
- Custom skills: {workspace_path}/skills/{{skill-name}}/SKILL.md

IMPORTANT: When responding to direct questions or conversations, reply directly with your text response.
Only use the 'message' tool when you need to send a message to a specific chat channel (like WhatsApp).
For normal conversation, just respond with text - do not call the message tool.

Always be helpful, accurate, and concise. When using tools, explain what you're doing.
When remembering something, write to {workspace_path}/memory/MEMORY.md`
}

func (b *ContextBuilder) renderIdentity() string {
	path := b.workspacePath
	if path == "" {
		path = "."
	}
	return strings.NewReplacer(
		"{now}", time.Now().Format("2006-01-02 15:04:05"),
		"{workspace_path}", path,
	).Replace(b.getIdentity())
}

package chat

import (
	"fmt"
	"strings"
	"time"

	"tinybot/internal/domain/model"
)

var defaultBootstrapFiles = []string{
	"AGENTS.md",
	"SOUL.md",
	"USER.md",
	"TOOLS.md",
	"IDENTITY.md",
}

// BootstrapReader loads workspace bootstrap documents by filename.
//
// Responsibilities:
//   - read optional bootstrap docs such as AGENTS.md and SOUL.md
//   - hide filesystem details from the chat service layer
//
// Inputs: a stable ordered list of filenames chosen by the prompt builder.
// Outputs: a map of filename -> content for successfully loaded documents.
// State changes: none in the interface itself.
// Side effects: concrete implementations usually read workspace files.
// Compatibility: keeps prompt assembly aligned with nanobot while shrinking the service boundary.
type BootstrapReader interface {
	Load(names []string) (map[string]string, error)
}

// MemoryReader exposes formatted memory context to the prompt builder.
//
// Responsibilities:
//   - assemble long-term memory and today's notes into one prompt section
//   - hide filesystem details from the chat service layer
//
// Inputs: none beyond the reader's configured workspace.
// Outputs: a preformatted memory section string, or an empty string when nothing is available.
// State changes: none.
// Side effects: concrete implementations usually read workspace memory files.
// Compatibility: matches the current Go memory prompt behavior without exposing workspace internals.
type MemoryReader interface {
	BuildContext() string
}

// SkillCatalog exposes active skills and progressive-loading summaries.
//
// Responsibilities:
//   - identify always-loaded skills
//   - load full skill bodies for those active skills
//   - build a summary for non-active skills
//
// Inputs: selected skill names or none for discovery.
// Outputs: skill names, full context bodies, and summary XML fragments.
// State changes: none in the interface itself.
// Side effects: concrete implementations usually scan skill directories and read SKILL.md files.
// Compatibility: preserves the current progressive skill loading design.
type SkillCatalog interface {
	GetAlwaysSkills() ([]string, error)
	LoadSkillsForContext(names []string) (string, error)
	BuildSummary() (string, error)
}

// Builder assembles the prompt for direct-chat turns.
//
// Responsibilities:
//   - render the stable identity section
//   - read optional workspace bootstrap files
//   - append memory context and skill context in a fixed order
//   - convert domain history plus the current user message into LLM-ready messages
//
// Inputs: domain message history, current user text, workspace bootstrap files, memory notes, and skill metadata.
// Outputs: a system prompt and helper message maps for assistant/tool trace items.
// State changes: none in memory; it only reads workspace state.
// Side effects: reads files from the workspace, memory directory, and skills directory when available.
// Compatibility: this mirrors nanobot's context-builder flow while keeping missing optional files as soft failures.
type Builder struct {
	workspacePath string
	bootstrap     BootstrapReader
	memory        MemoryReader
	skills        SkillCatalog
}

// NewPromptBuilder creates a prompt builder from explicit service-layer dependencies.
//
// Inputs: the workspace path plus readers for bootstrap docs, memory, and skills.
// Outputs: a Builder that can assemble LLM messages.
// State changes: none.
// Side effects: delegated to the injected adapters when prompt data is loaded.
// Compatibility: this keeps the same prompt behavior while moving concrete workspace dependencies out of the service layer.
func NewPromptBuilder(workspacePath string, bootstrap BootstrapReader, memory MemoryReader, skills SkillCatalog) *Builder {
	workspacePath = strings.TrimSpace(workspacePath)

	return &Builder{
		workspacePath: workspacePath,
		bootstrap:     bootstrap,
		memory:        memory,
		skills:        skills,
	}
}

// BuildMessages returns the message list for a single LLM call.
func (b *Builder) BuildMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any {
	messages := make([]map[string]any, 0, len(history)+2)
	messages = append(messages, map[string]any{
		"role":    model.RoleSystem,
		"content": b.BuildSystemPrompt(skillNames),
	})
	messages = append(messages, toLLMMessages(history)...)
	messages = append(messages, map[string]any{
		"role":    model.RoleUser,
		"content": currentMessage,
	})
	return messages
}

// AddToolResult appends a tool result message to the LLM transcript.
func (b *Builder) AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any {
	return append(messages, map[string]any{
		"role":         model.RoleTool,
		"tool_call_id": toolCallID,
		"name":         toolName,
		"content":      result,
	})
}

// AddAssistantMessage appends an assistant message, with optional tool calls, to the LLM transcript.
func (b *Builder) AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any {
	msg := map[string]any{"role": model.RoleAssistant}
	if content != "" {
		msg["content"] = content
	}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	return append(messages, msg)
}

// BuildSystemPrompt assembles the system prompt from identity, bootstrap docs, memory, and skills.
//
// Order is fixed on purpose to keep prompt diffs predictable for tests and future ports:
// identity -> bootstrap docs -> memory -> always skills -> skills summary.
// Missing optional files degrade gracefully instead of aborting the chat turn.
func (b *Builder) BuildSystemPrompt(skillNames []string) string {
	_ = skillNames // TODO: load explicitly selected skills when progressive skill activation is implemented.

	parts := []string{b.renderIdentity()}

	if docs := b.collectBootstrapDocs(); docs != "" {
		parts = append(parts, docs)
	}

	if b.memory != nil {
		if memoryContext := b.memory.BuildContext(); memoryContext != "" {
			parts = append(parts, fmt.Sprintf("## Memory\n%s", memoryContext))
		}
	}

	if b.skills != nil {
		alwaysSkills, err := b.skills.GetAlwaysSkills()
		if err == nil && len(alwaysSkills) > 0 {
			if activeSkills, loadErr := b.skills.LoadSkillsForContext(alwaysSkills); loadErr == nil && activeSkills != "" {
				parts = append(parts, fmt.Sprintf("# Active Skills\n\n%s", activeSkills))
			}
		}

		if summary, err := b.skills.BuildSummary(); err == nil && summary != "" {
			parts = append(parts, fmt.Sprintf(`# Skills

The following skills extend your capabilities. To use a skill, read its SKILL.md file using the read_file tool.
Skills with available="false" need dependencies installed first - you can try installing them with apt/brew.

%s`, summary))
		}
	}

	return strings.Join(parts, "\n\n--\n\n")
}

func (b *Builder) collectBootstrapDocs() string {
	if b.workspacePath == "" || b.bootstrap == nil {
		return ""
	}

	loaded, err := b.bootstrap.Load(defaultBootstrapFiles)
	if err != nil && len(loaded) == 0 {
		return ""
	}

	parts := make([]string, 0, len(defaultBootstrapFiles))
	for _, name := range defaultBootstrapFiles {
		content := strings.TrimSpace(loaded[name])
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("## %s\n\n%s", name, content))
	}
	return strings.Join(parts, "\n\n")
}

func (b *Builder) identityTemplate() string {
	return `# tinybot

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
- Custom skills: {workspace_path}/skills/{skill-name}/SKILL.md

IMPORTANT: When responding to direct questions or conversations, reply directly with your text response.
Only use the 'message' tool when you need to send a message to a specific chat channel (like WhatsApp).
For normal conversation, just respond with text - do not call the message tool.

Always be helpful, accurate, and concise. When using tools, explain what you are doing.
When remembering something, write to {workspace_path}/memory/MEMORY.md`
}

func (b *Builder) renderIdentity() string {
	path := b.workspacePath
	if path == "" {
		path = "."
	}
	return strings.NewReplacer(
		"{now}", time.Now().Format("2006-01-02 15:04:05"),
		"{workspace_path}", path,
	).Replace(b.identityTemplate())
}

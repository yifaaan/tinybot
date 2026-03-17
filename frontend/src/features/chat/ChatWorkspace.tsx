import React, { KeyboardEvent, useEffect, useRef, useState } from "react";

import type { ProviderInfo, SessionDetail, SessionMessage } from "../../app/types";
import { MarkdownContent } from "./MarkdownContent";

type Props = {
  session: SessionDetail | null;
  provider: ProviderInfo | null;
  draft: string;
  busy: boolean;
  notice: string;
  thinkingText: string;
  streamPhase: "idle" | "thinking" | "replying";
  assistantsOpen: boolean;
  topicsOpen: boolean;
  onDraftChange: (value: string) => void;
  onSend: () => void;
  onOpenSettings: () => void;
  onRename: () => void;
  onDelete: () => void;
  onToggleAssistants: () => void;
  onToggleTopics: () => void;
};

function IconGlyph({ children }: { children: React.ReactNode }) {
  return (
    <svg aria-hidden="true" className="ui-icon" viewBox="0 0 20 20">
      {children}
    </svg>
  );
}

function formatMessageTime(value: string | undefined): string {
  if (!value) {
    return "";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  return date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatHeaderTime(value: string | undefined): string {
  if (!value) {
    return "";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  return date.toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function summarizeNotice(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }

  if (/^Workspace:\s*/i.test(trimmed)) {
    const cleaned = trimmed.replace(/^Workspace:\s*/i, "");
    const segments = cleaned.split(/[\\/]/).filter(Boolean);
    const tail = segments[segments.length - 1] || cleaned;
    return `Workspace / ${tail}`;
  }

  return trimmed;
}

function shortMark(value: string): string {
  return value
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part.slice(0, 1).toUpperCase())
    .join("") || "AI";
}

function messageChrome(role: string, createdAt?: string, name?: string, assistantLabel?: string) {
  if (role === "user") {
    return { avatar: "ME", label: "You", time: formatMessageTime(createdAt) };
  }
  if (role === "tool") {
    return { avatar: "TL", label: name || "Tool", time: formatMessageTime(createdAt) };
  }

  const label = assistantLabel || "Assistant";
  return { avatar: shortMark(label), label, time: formatMessageTime(createdAt) };
}

function stackClassName(role: string, previousRole?: string, nextRole?: string, extra?: string) {
  const classes = ["message-stack", role === "assistant" ? "assistant" : role === "user" ? "user" : role];

  if (previousRole === role) {
    classes.push("continued-from-previous");
  }
  if (nextRole === role) {
    classes.push("continued-to-next");
  }
  if (role === "tool" && previousRole === "assistant") {
    classes.push("after-assistant");
  }
  if (role === "assistant" && nextRole === "tool") {
    classes.push("before-tool");
  }
  if (extra) {
    classes.push(extra);
  }

  return classes.join(" ");
}

function actionIcon(name: string) {
  switch (name) {
    case "copy":
      return (
        <IconGlyph>
          <rect x="7" y="5" width="8" height="10" rx="2" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path d="M5 13V7a2 2 0 0 1 2-2" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "retry":
      return (
        <IconGlyph>
          <path d="M14.5 7.5V4.8h-2.7" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M14.2 5.1A5.5 5.5 0 1 0 15.5 10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "assistants":
      return (
        <IconGlyph>
          <circle cx="7" cy="8" r="2" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <circle cx="13" cy="8" r="2" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path d="M4.8 14c.5-1.5 1.8-2.4 3.2-2.4h4c1.4 0 2.7.9 3.2 2.4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "topics":
      return (
        <IconGlyph>
          <path d="M5 5.5h10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <path d="M5 10h10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <path d="M5 14.5h6.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "rename":
      return (
        <IconGlyph>
          <path d="m5 13.8 1.2-3.6 6.8-6.8 2.6 2.6-6.8 6.8L5 13.8Z" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
          <path d="M11.8 4.5 14.5 7.2" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "delete":
      return (
        <IconGlyph>
          <path d="M6.5 6.5h7l-.6 8.2a1.5 1.5 0 0 1-1.5 1.3H8.6a1.5 1.5 0 0 1-1.5-1.3L6.5 6.5Z" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
          <path d="M5.5 5h9" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <path d="M8 5V4a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v1" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "tools":
      return (
        <IconGlyph>
          <circle cx="10" cy="10" r="2.2" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path
            d="M10 3.8v1.6M10 14.6v1.6M16.2 10h-1.6M5.4 10H3.8M14.4 5.6l-1.1 1.1M6.7 13.3l-1.1 1.1M14.4 14.4l-1.1-1.1M6.7 6.7 5.6 5.6"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
          />
        </IconGlyph>
      );
    case "plus":
      return (
        <IconGlyph>
          <path d="M10 4.5v11M4.5 10h11" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "slash":
      return (
        <IconGlyph>
          <path d="M13.5 4.5 6.5 15.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "mention":
      return (
        <IconGlyph>
          <circle cx="10" cy="10" r="5.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path d="M12.8 12.5V8.7a2.4 2.4 0 1 0-4.8 0v2.6a1.8 1.8 0 1 0 3.6 0V9.7" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    default:
      return null;
  }
}

export function ChatWorkspace({
  session,
  provider,
  draft,
  busy,
  notice,
  thinkingText,
  streamPhase,
  assistantsOpen,
  topicsOpen,
  onDraftChange,
  onSend,
  onOpenSettings,
  onRename,
  onDelete,
  onToggleAssistants,
  onToggleTopics,
}: Props) {
  const thinkingViewportRef = useRef<HTMLDivElement | null>(null);
  const [copiedMessageKey, setCopiedMessageKey] = useState("");
  const assistantLabel = provider?.name || "Assistant";
  const assistantModel = provider?.model || "No model";
  const messageCount = session?.summary.messageCount ?? session?.messages.length ?? 0;
  const providerStatus = provider ? (provider.configured ? "Configured" : "Setup required") : "No provider";
  const providerTone = provider?.configured ? "ok" : "warning";
  const hasMessages = Boolean(session && session.messages.length > 0);
  const topicTitle = session?.summary.title ?? "No topic selected";
  const topicTimestamp = session?.summary.updatedAt ? formatHeaderTime(session.summary.updatedAt) : "";
  const noticeLabel = summarizeNotice(notice);

  const renderEmptyState = (title: string, detail: string) => (
    <div className="empty-state chat-empty-state">
      <span className="eyebrow">Workspace</span>
      <strong>{title}</strong>
      <p>{detail}</p>
      {provider && <span className="empty-state-pill">{`${provider.name} / ${provider.model}`}</span>}
    </div>
  );

  const handleComposerKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      onSend();
    }
  };

  const handleCopyMessage = async (key: string, value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopiedMessageKey(key);
      window.setTimeout(() => {
        setCopiedMessageKey((current) => (current === key ? "" : current));
      }, 1200);
    } catch {
      setCopiedMessageKey("");
    }
  };

  const renderMessageActions = (key: string, role: string, content: string) => (
    <div className={`message-actions ${role === "user" ? "user" : "assistant"}`}>
      {role === "assistant" && <span className="message-action-pill subtle">{assistantModel}</span>}
      {role === "tool" && <span className="message-action-pill subtle">tool</span>}
      {role === "user" && <span className="message-action-pill subtle">prompt</span>}
      <button
        aria-label={copiedMessageKey === key ? "Copied" : "Copy"}
        className="message-action-button icon-only"
        onClick={() => void handleCopyMessage(key, content)}
        title={copiedMessageKey === key ? "Copied" : "Copy"}
        type="button">
        {actionIcon("copy")}
      </button>
      {role === "assistant" && (
        <button
          aria-label="Retry"
          className="message-action-button icon-only ghosted"
          title="Retry"
          type="button">
          {actionIcon("retry")}
        </button>
      )}
    </div>
  );

  useEffect(() => {
    const viewport = thinkingViewportRef.current;
    if (!viewport) {
      return;
    }

    viewport.scrollTo({
      top: viewport.scrollHeight,
      behavior: thinkingText.length > 12 ? "smooth" : "auto",
    });
  }, [thinkingText, streamPhase]);

  const streamingAssistantIndex =
    busy && session
      ? session.messages.reduce((lastIndex, message, index) => (message.role === "assistant" ? index : lastIndex), -1)
      : -1;

  const renderStreamingAssistant = (message: SessionMessage, index: number) => {
    const messageKey = `${message.createdAt}-${index}`;
    const hasThinking = thinkingText.trim() !== "";
    const hasReply = message.content.trim() !== "";
    const previousRole = index > 0 ? session?.messages[index - 1]?.role : undefined;
    const statusLabel = streamPhase === "replying" ? "Responding" : "Thinking";
    const statusNote =
      streamPhase === "replying"
        ? "Composing the final answer"
        : hasThinking
          ? "Reasoning in progress"
          : "Preparing a response";
    const chrome = messageChrome("assistant", message.createdAt, undefined, assistantLabel);

    return (
      <article
        key={messageKey}
        className={stackClassName("assistant", previousRole, undefined, "streaming-stack")}>
        <div className="message-meta">
          <div className="message-author">
            <span className="message-avatar">{chrome.avatar}</span>
            <div className="message-heading">
              <strong>{chrome.label}</strong>
              <span>{chrome.time}</span>
            </div>
          </div>
        </div>

        <div className="bubble assistant assistant-streaming">
          <div className="streaming-head">
            <div className="streaming-badge">
              <span className="status-dot" aria-hidden="true" />
              <strong>{statusLabel}</strong>
            </div>
            <span className="streaming-note">{statusNote}</span>
          </div>

          <section className={`thinking-panel ${hasReply ? "compact" : ""}`}>
            <div className="message-block-head">
              <span className="bubble-role">thinking</span>
              <span className="message-block-note">{hasThinking ? "Live reasoning trace" : "Preparing reasoning"}</span>
            </div>
            {hasThinking ? (
              <div className="thinking-viewport" ref={thinkingViewportRef}>
                <p className="thinking-content">
                  {thinkingText}
                  {streamPhase === "thinking" && <span className="stream-caret" aria-hidden="true" />}
                </p>
              </div>
            ) : (
              <div className="thinking-skeleton" aria-hidden="true">
                <span className="skeleton-line short" />
                <span className="skeleton-line medium" />
                <span className="skeleton-line long" />
              </div>
            )}
          </section>

          {hasReply && (
            <section className="stream-answer">
              <div className="message-block-head">
                <span className="bubble-role">assistant</span>
                <span className="message-block-note">Draft answer</span>
              </div>
              <div className="stream-answer-body">
                <MarkdownContent content={message.content} />
                <span className="stream-caret markdown-caret" aria-hidden="true" />
              </div>
            </section>
          )}
        </div>
        {renderMessageActions(messageKey, "assistant", [thinkingText, message.content].filter(Boolean).join("\n\n"))}
      </article>
    );
  };

  const renderMessage = (message: SessionMessage, index: number) => {
    if (index === streamingAssistantIndex) {
      return renderStreamingAssistant(message, index);
    }

    const previousRole = index > 0 ? session?.messages[index - 1]?.role : undefined;
    const nextRole = index < (session?.messages.length ?? 0) - 1 ? session?.messages[index + 1]?.role : undefined;

    if (message.role === "tool") {
      const messageKey = `${message.createdAt}-${index}`;
      const chrome = messageChrome("tool", message.createdAt, message.name, assistantLabel);
      return (
        <article className={stackClassName("tool", previousRole, nextRole)} key={messageKey}>
          <div className="message-meta">
            <div className="message-author">
              <span className="message-avatar">{chrome.avatar}</span>
              <div className="message-heading">
                <strong>{chrome.label}</strong>
                <span>{chrome.time}</span>
              </div>
            </div>
          </div>
          <details className="tool-result">
            <summary className="tool-result-summary">
              <div className="tool-result-title">
                <span className="bubble-role">Tool Result</span>
                <strong>{message.name || "Tool execution"}</strong>
              </div>
              <div className="tool-result-summary-side">
                <span className="tool-result-hint">Click to expand</span>
                <span className="tool-result-summary-glyph" aria-hidden="true">
                  ...
                </span>
              </div>
            </summary>
            <pre className="tool-result-body">{message.content}</pre>
          </details>
          {renderMessageActions(messageKey, "tool", message.content)}
        </article>
      );
    }

    const messageKey = `${message.createdAt}-${index}`;
    const chrome = messageChrome(message.role, message.createdAt, undefined, assistantLabel);
    return (
      <article
        key={messageKey}
        className={stackClassName(message.role, previousRole, nextRole)}>
        <div className="message-meta">
          <div className="message-author">
            <span className="message-avatar">{chrome.avatar}</span>
            <div className="message-heading">
              <strong>{chrome.label}</strong>
              <span>{chrome.time}</span>
            </div>
          </div>
        </div>
        <div className={`bubble ${message.role === "assistant" ? "assistant plain" : "user"}`}>
          <MarkdownContent content={message.content} />
        </div>
        {renderMessageActions(messageKey, message.role, message.content)}
      </article>
    );
  };

  return (
    <main className="chat-pane">
      <header className="pane-header chat-header">
        <div className="chat-header-main cherry-navbar">
          <div className="chat-navbar-main">
            <div className="chat-provider-chip">
              <span className="message-avatar chat-provider-avatar">{shortMark(assistantLabel)}</span>
              <div className="chat-provider-copy">
                <span className="eyebrow">Assistant</span>
                <div className="chat-provider-line">
                  <strong>{assistantLabel}</strong>
                  {provider && <span className="chat-provider-separator" aria-hidden="true" />}
                  {provider && <span className="context-pill strong model-pill">{assistantModel}</span>}
                </div>
              </div>
            </div>
            <div className="chat-topic-summary">
              <strong>{topicTitle}</strong>
              <span>
                {session ? `${messageCount} messages in this topic` : "Choose an assistant and create a topic to start chatting."}
              </span>
            </div>
          </div>
          <div className="chat-context-rail">
            <span className={`chat-context-chip state ${providerTone}`}>{providerStatus}</span>
            {topicTimestamp && <span className="chat-context-chip">{topicTimestamp}</span>}
          </div>
        </div>

        <div className="header-actions cherry-navbar-tools">
          <div className="header-action-cluster">
            <button
              className="ghost compact nav-button icon-nav-button"
              onClick={onToggleAssistants}
              title="Toggle assistants"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("assistants")}</span>
              <span className="nav-button-label">{assistantsOpen ? "Assistants" : "Show Assistants"}</span>
            </button>
            <button
              className="ghost compact nav-button icon-nav-button"
              onClick={onToggleTopics}
              title="Toggle topics"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("topics")}</span>
              <span className="nav-button-label">{topicsOpen ? "Topics" : "Show Topics"}</span>
            </button>
          </div>

          <div className="header-action-cluster">
            <button
              className="ghost compact nav-button icon-nav-button"
              onClick={onRename}
              title="Rename topic"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("rename")}</span>
              <span className="nav-button-label">Rename</span>
            </button>
            <button
              className="ghost compact danger nav-button icon-nav-button"
              onClick={onDelete}
              title="Delete topic"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("delete")}</span>
              <span className="nav-button-label">Delete</span>
            </button>
            <button
              className="action compact nav-button icon-nav-button"
              onClick={onOpenSettings}
              title="Open settings"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("tools")}</span>
              <span className="nav-button-label">Tools</span>
            </button>
          </div>
        </div>
      </header>

      <div className="chat-stage">
        <section className={`chat-scroll ${hasMessages ? "populated" : "empty"}`}>
          {!session &&
            renderEmptyState("No topic selected", "Choose an assistant and create a topic to start chatting.")}
          {session?.messages.map(renderMessage)}
          {session &&
            session.messages.length === 0 &&
            renderEmptyState("No messages yet", "Send a prompt to start this topic.")}
        </section>

        <footer className="composer-shell">
          <div className="composer-surface">
            <div className="composer-toolbar">
              <div className="composer-toolbar-group">
                <button className="ghost toolbar-chip active" onClick={onOpenSettings} type="button">
                  {assistantLabel}
                </button>
                <button aria-label="Attach" className="ghost toolbar-chip icon-chip" title="Attach" type="button">
                  {actionIcon("plus")}
                </button>
                <button aria-label="Commands" className="ghost toolbar-chip icon-chip" title="Commands" type="button">
                  {actionIcon("slash")}
                </button>
                <button aria-label="Mention" className="ghost toolbar-chip icon-chip" title="Mention" type="button">
                  {actionIcon("mention")}
                </button>
              </div>
              <div className="composer-toolbar-meta">
                {provider && <span className="composer-meta-pill">{assistantModel}</span>}
                <span className={`composer-meta-pill state ${providerTone}`}>{busy ? "Responding" : providerStatus}</span>
              </div>
            </div>

            <div className={`chat-notice ${busy ? "active" : ""}`} title={notice}>
              {noticeLabel}
            </div>

            <textarea
              id="composer"
              onChange={(event) => onDraftChange(event.target.value)}
              onKeyDown={handleComposerKeyDown}
              placeholder="Message this topic..."
              rows={4}
              value={draft}
            />

            <div className="composer-footer">
              <div className="composer-state">
                <span className={`composer-state-dot ${busy ? "busy" : "idle"}`} aria-hidden="true" />
                <span>{busy ? "Streaming response" : "Enter to send, Shift+Enter for newline"}</span>
              </div>
              <button className="action primary composer-send" disabled={busy} onClick={onSend} type="button">
                <span>Send</span>
                <span className="composer-send-arrow" aria-hidden="true">
                  -&gt;
                </span>
              </button>
            </div>
          </div>
        </footer>
      </div>
    </main>
  );
}

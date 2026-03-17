import React, { KeyboardEvent, useEffect, useRef } from "react";

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

  const handleComposerKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      onSend();
    }
  };

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
    const hasThinking = thinkingText.trim() !== "";
    const hasReply = message.content.trim() !== "";
    const statusLabel = streamPhase === "replying" ? "Responding" : "Thinking";
    const statusNote =
      streamPhase === "replying"
        ? "Composing the final answer"
        : hasThinking
          ? "Reasoning in progress"
          : "Preparing a response";

    return (
      <article key={`${message.createdAt}-${index}`} className="bubble assistant assistant-streaming">
        <div className="streaming-head">
          <div className="streaming-badge">
            <span className="status-dot" aria-hidden="true" />
            <strong>{statusLabel}</strong>
          </div>
          <span className="streaming-note">{statusNote}</span>
        </div>

        <section className={`thinking-panel ${hasReply ? "compact" : ""}`}>
          <span className="bubble-role">thinking</span>
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
            <span className="bubble-role">assistant</span>
            <div className="stream-answer-body">
              <MarkdownContent content={message.content} />
              <span className="stream-caret markdown-caret" aria-hidden="true" />
            </div>
          </section>
        )}
      </article>
    );
  };

  const renderMessage = (message: SessionMessage, index: number) => {
    if (index === streamingAssistantIndex) {
      return renderStreamingAssistant(message, index);
    }

    if (message.role === "tool") {
      return (
        <details className="tool-result" key={`${message.createdAt}-${index}`}>
          <summary className="tool-result-summary">
            <div>
              <span className="bubble-role">Tool Result</span>
              <strong>{message.name || "Tool execution"}</strong>
            </div>
            <span className="tool-result-hint">Click to expand</span>
          </summary>
          <pre className="tool-result-body">{message.content}</pre>
        </details>
      );
    }

    return (
      <article
        key={`${message.createdAt}-${index}`}
        className={`bubble ${message.role === "assistant" ? "assistant" : "user"}`}>
        <span className="bubble-role">{message.role}</span>
        <MarkdownContent content={message.content} />
      </article>
    );
  };

  return (
    <main className="chat-pane">
      <header className="pane-header chat-header">
        <div className="chat-header-main">
          <div className="chat-context">
            <span className="eyebrow">Conversation</span>
            <div className="chat-title-row">
              <h2>{session?.summary.title ?? "No topic selected"}</h2>
              {provider && <span className="context-pill">{provider.name}</span>}
            </div>
            <div className="chat-meta-row">
              <p className="chat-subtitle">
                {provider ? `${provider.model} / ${session?.summary.messageCount ?? 0} messages` : "Select an assistant"}
              </p>
              {session?.summary.updatedAt && (
                <span className="chat-timestamp">{formatHeaderTime(session.summary.updatedAt)}</span>
              )}
            </div>
          </div>
          <div className="chat-context-rail">
            <span className="chat-context-chip">desktop</span>
            <span className="chat-context-chip">{busy ? "live" : "idle"}</span>
          </div>
        </div>
        <div className="header-actions">
          <div className="header-action-cluster">
            <button className="ghost compact nav-button icon-nav-button" onClick={onToggleAssistants} title="Toggle assistants" type="button">
              <span className="nav-button-glyph">AS</span>
              <span className="nav-button-label">{assistantsOpen ? "Assistants" : "Show Assistants"}</span>
            </button>
            <button className="ghost compact nav-button icon-nav-button" onClick={onToggleTopics} title="Toggle topics" type="button">
              <span className="nav-button-glyph">TP</span>
              <span className="nav-button-label">{topicsOpen ? "Topics" : "Show Topics"}</span>
            </button>
          </div>
          <div className="header-action-cluster">
            <button className="ghost compact nav-button icon-nav-button" onClick={onRename} title="Rename topic" type="button">
              <span className="nav-button-glyph">RN</span>
              <span className="nav-button-label">Rename</span>
            </button>
            <button className="ghost compact danger nav-button icon-nav-button" onClick={onDelete} title="Delete topic" type="button">
              <span className="nav-button-glyph">DL</span>
              <span className="nav-button-label">Delete</span>
            </button>
            <button className="action compact nav-button icon-nav-button" onClick={onOpenSettings} title="Open settings" type="button">
              <span className="nav-button-glyph">ST</span>
              <span className="nav-button-label">Settings</span>
            </button>
          </div>
        </div>
      </header>

      <section className="chat-scroll">
        {!session && <div className="empty-state">Choose an assistant and create a topic to start chatting.</div>}
        {session?.messages.map(renderMessage)}
        {session && session.messages.length === 0 && (
          <div className="empty-state">This topic has no messages yet.</div>
        )}
      </section>

      <footer className="composer-shell">
        <div className="composer-surface">
          <div className="composer-toolbar">
            <div className="composer-toolbar-group">
              <button className="ghost toolbar-chip active" onClick={onOpenSettings} type="button">
                {provider?.name ?? "Assistant"}
              </button>
              <button className="ghost toolbar-chip" type="button">
                Attach
              </button>
              <button className="ghost toolbar-chip" type="button">
                Tools
              </button>
              <button className="ghost toolbar-chip" type="button">
                Skills
              </button>
            </div>
            <span className="composer-hint">Enter to send, Shift+Enter for newline</span>
          </div>
          <div className="chat-notice">{notice}</div>
          <textarea
            id="composer"
            onChange={(event) => onDraftChange(event.target.value)}
            onKeyDown={handleComposerKeyDown}
            placeholder="Message this topic..."
            rows={4}
            value={draft}
          />
          <div className="composer-footer">
            <span>{busy ? "Streaming..." : "Ready"}</span>
            <button className="action primary" disabled={busy} onClick={onSend} type="button">
              Send
            </button>
          </div>
        </div>
      </footer>
    </main>
  );
}

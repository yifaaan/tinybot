import React, { useEffect, useRef } from "react";

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
            <p className="chat-subtitle">
              {provider ? `${provider.model} · ${session?.summary.messageCount ?? 0} messages` : "Select an assistant"}
            </p>
          </div>
        </div>
        <div className="header-actions">
          <button className="ghost" onClick={onToggleAssistants} type="button">
            {assistantsOpen ? "Hide Assistants" : "Show Assistants"}
          </button>
          <button className="ghost" onClick={onToggleTopics} type="button">
            {topicsOpen ? "Hide Topics" : "Show Topics"}
          </button>
          <button className="ghost" onClick={onRename} type="button">
            Rename
          </button>
          <button className="ghost danger" onClick={onDelete} type="button">
            Delete
          </button>
          <button className="action" onClick={onOpenSettings} type="button">
            Settings
          </button>
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
        <div className="composer-toolbar">
          <button className="ghost toolbar-chip" onClick={onOpenSettings} type="button">
            {provider?.name ?? "Assistant"}
          </button>
          <button className="ghost toolbar-chip" type="button">
            Files
          </button>
          <button className="ghost toolbar-chip" type="button">
            Tools
          </button>
          <button className="ghost toolbar-chip" type="button">
            Skills
          </button>
        </div>
        <div className="chat-notice">{notice}</div>
        <textarea
          id="composer"
          placeholder="Message this topic..."
          rows={4}
          value={draft}
          onChange={(event) => onDraftChange(event.target.value)}
        />
        <div className="composer-footer">
          <span>{busy ? "Streaming..." : "Ready"}</span>
          <button className="action primary" disabled={busy} onClick={onSend} type="button">
            Send
          </button>
        </div>
      </footer>
    </main>
  );
}

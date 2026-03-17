import React from "react";

import type { ProviderInfo, SessionSummary } from "../../app/types";

type Props = {
  provider: ProviderInfo | null;
  sessions: SessionSummary[];
  selectedSessionKey: string;
  onCreateSession: () => void;
  onSelectSession: (session: SessionSummary) => void;
};

function formatTopicTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  const now = Date.now();
  const delta = now - date.getTime();
  if (delta < 1000 * 60 * 60 * 24) {
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  }
  return date.toLocaleDateString([], { month: "short", day: "numeric" });
}

export function TopicsPane({ provider, sessions, selectedSessionKey, onCreateSession, onSelectSession }: Props) {
  return (
    <section className="topics-pane">
      <header className="pane-header">
        <div>
          <span className="eyebrow">Topics</span>
          <h1>{provider?.name ?? "No assistant"}</h1>
        </div>
        <button className="action" onClick={onCreateSession} type="button">
          New Topic
        </button>
      </header>

      <div className="topic-list">
        {sessions.length === 0 && (
          <div className="empty-state compact-empty">No topics for this assistant yet.</div>
        )}
        {sessions.map((session) => (
          <button
            key={session.key}
            className={`topic-card ${session.key === selectedSessionKey ? "active" : ""}`}
            onClick={() => onSelectSession(session)}
            type="button">
            <div className="topic-card-head">
              <strong>{session.title}</strong>
              <span>{formatTopicTime(session.updatedAt)}</span>
            </div>
            <p>{session.preview || "No preview yet"}</p>
            <div className="topic-meta">
              <span>{session.messageCount} messages</span>
              <span>{session.channel}</span>
            </div>
          </button>
        ))}
      </div>
    </section>
  );
}

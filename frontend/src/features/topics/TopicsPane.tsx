import React, { useEffect, useMemo, useState } from "react";

import type { ProviderInfo, SessionSummary } from "../../app/types";

type Props = {
  provider: ProviderInfo | null;
  sessions: SessionSummary[];
  selectedSessionKey: string;
  onCreateSession: () => void;
  onSelectSession: (session: SessionSummary) => void;
  onRenameSession: (session: SessionSummary) => void;
  onDeleteSession: (session: SessionSummary) => void;
};

const PINNED_TOPICS_STORAGE_KEY = "tinybot:pinned-topics";

type PinnedTopicMap = Record<string, string[]>;
type TopicGroup = {
  label: string;
  sessions: SessionSummary[];
};

function topicPreview(value: string): string {
  const preview = value.replace(/\s+/g, " ").trim();
  return preview || "No preview yet";
}

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

function parseTime(value: string): number {
  const time = new Date(value).getTime();
  return Number.isNaN(time) ? 0 : time;
}

function startOfDay(value: number): number {
  const date = new Date(value);
  date.setHours(0, 0, 0, 0);
  return date.getTime();
}

function topicGroupLabel(updatedAt: string): string {
  const value = parseTime(updatedAt);
  if (!value) {
    return "Earlier";
  }

  const todayStart = startOfDay(Date.now());
  const yesterdayStart = todayStart - 1000 * 60 * 60 * 24;
  const weekStart = todayStart - 1000 * 60 * 60 * 24 * 7;

  if (value >= todayStart) {
    return "Today";
  }
  if (value >= yesterdayStart) {
    return "Yesterday";
  }
  if (value >= weekStart) {
    return "This Week";
  }
  return "Earlier";
}

function loadPinnedTopics(): PinnedTopicMap {
  if (typeof window === "undefined") {
    return {};
  }

  try {
    const raw = window.localStorage.getItem(PINNED_TOPICS_STORAGE_KEY);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as PinnedTopicMap;
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

export function TopicsPane({
  provider,
  sessions,
  selectedSessionKey,
  onCreateSession,
  onSelectSession,
  onRenameSession,
  onDeleteSession,
}: Props) {
  const [query, setQuery] = useState("");
  const [pinnedByProvider, setPinnedByProvider] = useState<PinnedTopicMap>(() => loadPinnedTopics());

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.localStorage.setItem(PINNED_TOPICS_STORAGE_KEY, JSON.stringify(pinnedByProvider));
  }, [pinnedByProvider]);

  const providerName = provider?.name ?? "";
  const pinnedKeys = pinnedByProvider[providerName] ?? [];

  const groupedSessions = useMemo<TopicGroup[]>(() => {
    const normalized = query.trim().toLowerCase();
    const matches = !normalized
      ? sessions
      : sessions.filter((session) => {
          const haystack = `${session.title} ${session.preview}`.toLowerCase();
          return haystack.includes(normalized);
        });

    const timeSorted = matches.slice().sort((left, right) => parseTime(right.updatedAt) - parseTime(left.updatedAt));
    const pinnedSet = new Set(pinnedKeys);
    const pinned = pinnedKeys
      .map((key) => timeSorted.find((session) => session.key === key))
      .filter((session): session is SessionSummary => Boolean(session));
    const regular = timeSorted.filter((session) => !pinnedSet.has(session.key));

    const groups: TopicGroup[] = [];
    if (pinned.length > 0) {
      groups.push({ label: "Pinned", sessions: pinned });
    }

    const grouped = new Map<string, SessionSummary[]>();
    regular.forEach((session) => {
      const label = topicGroupLabel(session.updatedAt);
      grouped.set(label, [...(grouped.get(label) ?? []), session]);
    });

    ["Today", "Yesterday", "This Week", "Earlier"].forEach((label) => {
      const bucket = grouped.get(label);
      if (bucket && bucket.length > 0) {
        groups.push({ label, sessions: bucket });
      }
    });

    return groups;
  }, [pinnedKeys, query, sessions]);

  const togglePinned = (sessionKey: string) => {
    if (!providerName) {
      return;
    }
    setPinnedByProvider((previous) => {
      const current = previous[providerName] ?? [];
      const next = current.includes(sessionKey)
        ? current.filter((key) => key !== sessionKey)
        : [sessionKey, ...current];
      return { ...previous, [providerName]: next };
    });
  };

  return (
    <section className="topics-pane">
      <header className="pane-header">
        <div>
          <span className="eyebrow">Conversations</span>
          <h1>{provider?.name ?? "No assistant"}</h1>
          <p className="pane-subtitle">{sessions.length} topics</p>
        </div>
        <button className="action" onClick={onCreateSession} type="button">
          New Topic
        </button>
      </header>

      <div className="topics-toolbar">
        <input
          aria-label="Search topics"
          className="topic-search"
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search topics"
          type="search"
          value={query}
        />
      </div>

      <div className="topic-list">
        {sessions.length === 0 && (
          <div className="empty-state compact-empty">No topics for this assistant yet.</div>
        )}
        {sessions.length > 0 && groupedSessions.length === 0 && (
          <div className="empty-state compact-empty">No topics match this search.</div>
        )}
        {groupedSessions.map((group) => (
          <section className="topic-group" key={group.label}>
            <div className="topic-group-label">
              <span>{group.label}</span>
              <span>{group.sessions.length}</span>
            </div>
            <div className="topic-group-list">
              {group.sessions.map((session) => {
                const pinned = pinnedKeys.includes(session.key);
                return (
                  <article
                    className={`topic-card ${session.key === selectedSessionKey ? "active" : ""}`}
                    key={session.key}
                    onClick={() => onSelectSession(session)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        onSelectSession(session);
                      }
                    }}
                    role="button"
                    tabIndex={0}>
                    <div className="topic-card-head">
                      <div className="topic-card-title">
                        <span className={`topic-status-dot ${session.key === selectedSessionKey ? "active" : ""}`} aria-hidden="true" />
                        <strong>{session.title}</strong>
                        {pinned && <span className="topic-badge">Pinned</span>}
                      </div>
                      <div className="topic-card-tail">
                        <span>{formatTopicTime(session.updatedAt)}</span>
                        <span className="topic-menu-glyph" aria-hidden="true">
                          ...
                        </span>
                      </div>
                    </div>
                    <p>{topicPreview(session.preview)}</p>
                    <div className="topic-meta">
                      <span>{session.messageCount} messages</span>
                      <span>{session.channel}</span>
                    </div>
                    <div className="topic-quick-actions">
                      <button
                        className={`icon-ghost ${pinned ? "active" : ""}`}
                        onClick={(event) => {
                          event.stopPropagation();
                          togglePinned(session.key);
                        }}
                        type="button">
                        {pinned ? "Unpin" : "Pin"}
                      </button>
                      <button
                        className="icon-ghost"
                        onClick={(event) => {
                          event.stopPropagation();
                          onRenameSession(session);
                        }}
                        type="button">
                        Rename
                      </button>
                      <button
                        className="icon-ghost danger"
                        onClick={(event) => {
                          event.stopPropagation();
                          onDeleteSession(session);
                        }}
                        type="button">
                        Delete
                      </button>
                    </div>
                  </article>
                );
              })}
            </div>
          </section>
        ))}
      </div>
    </section>
  );
}

import React from "react";

import type { ProviderInfo, SessionSummary } from "../../app/types";
import { ProviderStrip } from "../models/ProviderStrip";

type Props = {
  workspace: string;
  sessions: SessionSummary[];
  selectedSessionKey: string;
  providers: ProviderInfo[];
  onCreateSession: () => void;
  onSelectSession: (key: string) => void;
  onSelectProvider: (provider: ProviderInfo) => void;
};

export function SessionsPane({
  workspace,
  sessions,
  selectedSessionKey,
  providers,
  onCreateSession,
  onSelectSession,
  onSelectProvider,
}: Props) {
  return (
    <section className="sessions-pane">
      <header className="pane-header">
        <div>
          <span className="eyebrow">Workspace</span>
          <h1>{workspace}</h1>
        </div>
        <button className="action" onClick={onCreateSession} type="button">
          New
        </button>
      </header>
      <ProviderStrip onSelect={onSelectProvider} providers={providers} />
      <div className="session-list">
        {sessions.map((session) => (
          <button
            key={session.key}
            className={`session-card ${session.key === selectedSessionKey ? "active" : ""}`}
            onClick={() => onSelectSession(session.key)}
            type="button">
            <strong>{session.title}</strong>
            <span>{session.preview || "No preview yet"}</span>
          </button>
        ))}
      </div>
    </section>
  );
}

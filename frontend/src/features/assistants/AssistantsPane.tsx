import React from "react";

import type { ProviderInfo } from "../../app/types";

type Props = {
  providers: ProviderInfo[];
  selectedProviderName: string;
  sessionCounts: Record<string, number>;
  onSelectProvider: (provider: ProviderInfo) => void;
};

export function AssistantsPane({ providers, selectedProviderName, sessionCounts, onSelectProvider }: Props) {
  return (
    <section className="assistants-pane">
      <header className="pane-header pane-header-tight">
        <div>
          <span className="eyebrow">Assistants</span>
          <h1>Providers</h1>
        </div>
      </header>

      <div className="assistant-list">
        {providers.map((provider) => {
          const isActive = provider.name === selectedProviderName;
          return (
            <button
              key={provider.name}
              className={`assistant-card ${isActive ? "active" : ""}`}
              onClick={() => onSelectProvider(provider)}
              type="button">
              <div className="assistant-card-head">
                <strong>{provider.name}</strong>
                <span className={`assistant-dot ${provider.configured ? "configured" : "missing"}`} aria-hidden="true" />
              </div>
              <span className="assistant-model">{provider.model}</span>
              <div className="assistant-meta">
                <span>{sessionCounts[provider.name] ?? 0} topics</span>
                <span>{provider.configured ? "Configured" : "Missing key"}</span>
              </div>
            </button>
          );
        })}
      </div>
    </section>
  );
}

import React, { useMemo, useState } from "react";

import type { ProviderInfo } from "../../app/types";

type Props = {
  providers: ProviderInfo[];
  selectedProviderName: string;
  sessionCounts: Record<string, number>;
  onSelectProvider: (provider: ProviderInfo) => void;
};

function providerInitials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("") || "AI";
}

export function AssistantsPane({ providers, selectedProviderName, sessionCounts, onSelectProvider }: Props) {
  const [query, setQuery] = useState("");

  const visibleProviders = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) {
      return providers;
    }

    return providers.filter((provider) => {
      const haystack = `${provider.name} ${provider.model} ${provider.kind} ${provider.apiBase}`.toLowerCase();
      return haystack.includes(normalized);
    });
  }, [providers, query]);

  const configuredCount = providers.filter((provider) => provider.configured).length;

  return (
    <section className="assistants-pane">
      <header className="pane-header pane-header-tight">
        <div>
          <span className="eyebrow">Assistants</span>
          <h1>Providers</h1>
          <p className="pane-subtitle">
            {configuredCount}/{providers.length} configured
          </p>
        </div>
      </header>

      <div className="assistants-toolbar">
        <input
          aria-label="Search assistants"
          className="assistant-search"
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search assistants"
          type="search"
          value={query}
        />
      </div>

      <div className="assistant-list">
        {visibleProviders.length === 0 && (
          <div className="empty-state compact-empty">No assistants match this search.</div>
        )}
        {visibleProviders.map((provider) => {
          const isActive = provider.name === selectedProviderName;
          return (
            <article
              key={provider.name}
              className={`assistant-card ${isActive ? "active" : ""}`}
              onClick={() => onSelectProvider(provider)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  onSelectProvider(provider);
                }
              }}
              role="button"
              tabIndex={0}>
              <div className="assistant-card-top">
                <span className="assistant-avatar" aria-hidden="true">
                  {providerInitials(provider.name)}
                </span>
                <div className="assistant-card-head">
                  <div className="assistant-name-stack">
                    <strong>{provider.name}</strong>
                    <span className="assistant-kind">{provider.kind}</span>
                  </div>
                  <span className={`assistant-dot ${provider.configured ? "configured" : "missing"}`} aria-hidden="true" />
                </div>
              </div>
              <span className="assistant-model">{provider.model}</span>
              <div className="assistant-meta">
                <span>{sessionCounts[provider.name] ?? 0} topics</span>
                <span>{provider.configured ? "Configured" : "Missing key"}</span>
              </div>
              <div className="assistant-footer">
                <span className="assistant-api">{provider.apiBase || "Local provider"}</span>
                {isActive && <span className="assistant-badge">Active</span>}
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}

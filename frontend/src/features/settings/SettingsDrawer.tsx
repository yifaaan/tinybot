import React, { useEffect, useMemo, useState } from "react";

import type { Bootstrap, ThemeMode } from "../../app/types";

export type ProviderDraft = {
  name: string;
  kind: string;
  model: string;
  apiBase: string;
  apiKey: string;
  configured: boolean;
  isCustom: boolean;
};

type Props = {
  open: boolean;
  bootstrap: Bootstrap;
  theme: ThemeMode;
  onThemeChange: (theme: ThemeMode) => void;
  onSave: (form: HTMLFormElement, providers: ProviderDraft[]) => void;
  onClose: () => void;
};

export function SettingsDrawer({ open, bootstrap, theme, onThemeChange, onSave, onClose }: Props) {
  const [providerDrafts, setProviderDrafts] = useState<ProviderDraft[]>([]);

  useEffect(() => {
    setProviderDrafts(
      bootstrap.providers.map((provider) => ({
        name: provider.name,
        kind: provider.kind,
        model: provider.model,
        apiBase: provider.apiBase,
        apiKey: "",
        configured: provider.configured,
        isCustom: false,
      })),
    );
  }, [bootstrap.providers]);

  const providerOptions = useMemo(() => {
    const seen = new Set<string>();
    return providerDrafts.filter((provider) => {
      const key = provider.name.trim();
      if (!key || seen.has(key)) {
        return false;
      }
      seen.add(key);
      return true;
    });
  }, [providerDrafts]);

  const updateProviderDraft = (index: number, patch: Partial<ProviderDraft>) => {
    setProviderDrafts((current) =>
      current.map((provider, providerIndex) => (providerIndex === index ? { ...provider, ...patch } : provider)),
    );
  };

  const handleAddProvider = () => {
    setProviderDrafts((current) => [
      ...current,
      {
        name: "",
        kind: "openai",
        model: "",
        apiBase: "",
        apiKey: "",
        configured: false,
        isCustom: true,
      },
    ]);
  };

  return (
    <>
      <div className={`settings-backdrop ${open ? "open" : ""}`} onClick={onClose} aria-hidden="true" />
      <aside className={`details-pane ${open ? "open" : ""}`}>
        <header className="pane-header">
          <div>
            <span className="eyebrow">Settings</span>
            <h2>Client and runtime</h2>
          </div>
          <button className="ghost" onClick={onClose} type="button">
            Close
          </button>
        </header>
        <form
          className="settings-form"
          onSubmit={(event) => {
            event.preventDefault();
            onSave(event.currentTarget, providerDrafts);
          }}>
          <label className="field">
            <span>Theme</span>
            <select
              defaultValue={theme}
              name="theme"
              onChange={(event) => onThemeChange(event.target.value as ThemeMode)}>
              <option value="black">Black</option>
              <option value="white">White</option>
            </select>
          </label>

          <label className="field">
            <span>Active provider</span>
            <select defaultValue={bootstrap.config.providers.active} name="activeProvider">
              {providerOptions.map((provider) => (
                <option key={provider.name} value={provider.name}>
                  {provider.name} / {provider.model}
                </option>
              ))}
            </select>
          </label>

          <label className="field">
            <span>Temperature</span>
            <input
              defaultValue={bootstrap.config.agents.temperature}
              min="0"
              max="2"
              name="temperature"
              step="0.05"
              type="number"
            />
          </label>

          <label className="field">
            <span>Max tokens</span>
            <input
              defaultValue={bootstrap.config.agents.max_tokens}
              min="256"
              name="maxTokens"
              step="256"
              type="number"
            />
          </label>

          <label className="toggle">
            <input defaultChecked={bootstrap.config.agents.enable_thinking} name="enableThinking" type="checkbox" />
            <span>Enable thinking stream</span>
          </label>

          <label className="toggle">
            <input defaultChecked={bootstrap.config.channels.console?.enabled} name="consoleEnabled" type="checkbox" />
            <span>Keep console channel enabled</span>
          </label>

          <section className="settings-provider-section">
            <div className="settings-provider-header">
              <div>
                <span className="eyebrow">Models</span>
                <strong>Provider and model settings</strong>
                <p>Edit built-in providers or add a custom model entry.</p>
              </div>
              <button className="ghost compact" onClick={handleAddProvider} type="button">
                Add model
              </button>
            </div>

            <div className="provider-config-list">
              {providerDrafts.map((provider, index) => (
                <article className="provider-config-card" key={`${provider.name || "new"}-${index}`}>
                  <div className="provider-config-card-head">
                    <div>
                      <strong>{provider.name.trim() || `Custom model ${index + 1}`}</strong>
                      <span>{provider.isCustom ? "Custom provider entry" : provider.configured ? "Configured" : "Built-in provider"}</span>
                    </div>
                    <span className={`assistant-badge provider-config-badge ${provider.isCustom ? "custom" : ""}`}>
                      {provider.isCustom ? "Custom" : "Built-in"}
                    </span>
                  </div>

                  <div className="provider-config-grid">
                    <label className="field compact-field">
                      <span>Name</span>
                      <input
                        onChange={(event) => updateProviderDraft(index, { name: event.target.value })}
                        placeholder="provider-name"
                        type="text"
                        value={provider.name}
                      />
                    </label>

                    <label className="field compact-field">
                      <span>Kind</span>
                      <input
                        onChange={(event) => updateProviderDraft(index, { kind: event.target.value })}
                        placeholder="openai"
                        type="text"
                        value={provider.kind}
                      />
                    </label>

                    <label className="field compact-field">
                      <span>Model</span>
                      <input
                        onChange={(event) => updateProviderDraft(index, { model: event.target.value })}
                        placeholder="gpt-4o-mini"
                        type="text"
                        value={provider.model}
                      />
                    </label>

                    <label className="field compact-field provider-config-wide">
                      <span>API base</span>
                      <input
                        onChange={(event) => updateProviderDraft(index, { apiBase: event.target.value })}
                        placeholder="https://api.example.com/v1"
                        type="text"
                        value={provider.apiBase}
                      />
                    </label>

                    <label className="field compact-field provider-config-wide">
                      <span>API key</span>
                      <input
                        onChange={(event) => updateProviderDraft(index, { apiKey: event.target.value })}
                        placeholder={provider.isCustom ? "Optional unless your provider requires it" : "Leave blank to keep current key"}
                        type="password"
                        value={provider.apiKey}
                      />
                    </label>
                  </div>
                </article>
              ))}
            </div>
          </section>

          <div className="settings-actions">
            <button className="ghost" onClick={onClose} type="button">
              Cancel
            </button>
            <button className="action primary" type="submit">
              Save settings
            </button>
          </div>
        </form>
      </aside>
    </>
  );
}

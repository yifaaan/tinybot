import React, { useEffect, useMemo, useState } from "react";

import type { Bootstrap, ThemeMode } from "../../app/types";

export type ProviderDraft = {
  id: string;
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
  const providerKinds = ["openai", "openai-responses", "qwen", "deepseek", "ollama"];
  const reasoningEfforts = ["low", "medium", "high"];
  const reasoningSummaries = ["off", "auto", "concise", "detailed"];
  const textVerbosityLevels = ["low", "medium", "high"];
  const [providerDrafts, setProviderDrafts] = useState<ProviderDraft[]>([]);

  useEffect(() => {
    setProviderDrafts(
      bootstrap.providers.map((provider) => ({
        id: `provider-${provider.name}`,
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
        id: `custom-${Date.now()}-${current.length}`,
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

  const handleAddResponsesProvider = () => {
    setProviderDrafts((current) => [
      ...current,
      {
        id: `responses-${Date.now()}-${current.length}`,
        name: "",
        kind: "openai-responses",
        model: "",
        apiBase: "https://codex-api.packycode.com/v1",
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
                <span className="eyebrow">Reasoning</span>
                <strong>Model response controls</strong>
                <p>Adjust reasoning strength, summary detail, and response verbosity for compatible OpenAI Responses models.</p>
              </div>
            </div>

            <div className="provider-config-grid response-controls-grid">
              <label className="field compact-field">
                <span>Reasoning effort</span>
                <select defaultValue={bootstrap.config.agents.reasoning_effort ?? "high"} name="reasoningEffort">
                  {reasoningEfforts.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              </label>

              <label className="field compact-field">
                <span>Reasoning summary</span>
                <select defaultValue={bootstrap.config.agents.reasoning_summary ?? "detailed"} name="reasoningSummary">
                  {reasoningSummaries.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              </label>

              <label className="field compact-field provider-config-wide">
                <span>Text verbosity</span>
                <select defaultValue={bootstrap.config.agents.text_verbosity ?? "medium"} name="textVerbosity">
                  {textVerbosityLevels.map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              </label>
            </div>
          </section>

          <section className="settings-provider-section">
            <div className="settings-provider-header">
              <div>
                <span className="eyebrow">Models</span>
                <strong>Provider and model settings</strong>
                <p>Edit built-in providers, add a custom model entry, or add an OpenAI Responses-compatible endpoint.</p>
              </div>
              <div className="settings-provider-buttons">
              <button className="ghost compact" onClick={handleAddResponsesProvider} type="button">
                Add Responses API
                </button>
                <button className="ghost compact" onClick={handleAddProvider} type="button">
                  Add model
                </button>
              </div>
            </div>

            <div className="provider-config-list">
              {providerDrafts.map((provider, index) => (
                <article className="provider-config-card" key={provider.id}>
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
                      <select
                        onChange={(event) => updateProviderDraft(index, { kind: event.target.value })}
                        value={provider.kind}
                      >
                        {providerKinds.map((kind) => (
                          <option key={kind} value={kind}>
                            {kind}
                          </option>
                        ))}
                      </select>
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

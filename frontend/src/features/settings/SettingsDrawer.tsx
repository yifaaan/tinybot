import React from "react";

import type { Bootstrap, ThemeMode } from "../../app/types";

type Props = {
  open: boolean;
  bootstrap: Bootstrap;
  theme: ThemeMode;
  onThemeChange: (theme: ThemeMode) => void;
  onSave: (form: HTMLFormElement) => void;
  onClose: () => void;
};

export function SettingsDrawer({ open, bootstrap, theme, onThemeChange, onSave, onClose }: Props) {
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
            onSave(event.currentTarget);
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
              {bootstrap.providers.map((provider) => (
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

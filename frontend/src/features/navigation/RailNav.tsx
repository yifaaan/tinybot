import React from "react";

type Props = {
  notice: string;
  providerName: string;
  topicCount: number;
  onOpenSettings: () => void;
};

type RailItem = {
  key: string;
  label: string;
  active?: boolean;
  icon: React.ReactNode;
};

function RailGlyph({ children }: { children: React.ReactNode }) {
  return (
    <svg aria-hidden="true" className="rail-svg" viewBox="0 0 24 24">
      {children}
    </svg>
  );
}

function providerMark(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part.slice(0, 1).toUpperCase())
    .join("") || "TB";
}

function compactNotice(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return "Workspace ready";
  }

  const cleaned = trimmed.replace(/^Workspace:\s*/i, "");
  const segments = cleaned.split(/[\\/]/).filter(Boolean);
  const tail = segments[segments.length - 1];
  return tail || cleaned;
}

export function RailNav({ notice, providerName, topicCount, onOpenSettings }: Props) {
  const items: RailItem[] = [
    {
      key: "chat",
      label: "Chat",
      active: true,
      icon: (
        <RailGlyph>
          <rect x="4" y="5" width="16" height="11" rx="3" fill="none" stroke="currentColor" strokeWidth="1.8" />
          <path d="M8 19h5l2.5-3" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </RailGlyph>
      ),
    },
    {
      key: "agents",
      label: "Agents",
      icon: (
        <RailGlyph>
          <circle cx="8" cy="9" r="2.5" fill="none" stroke="currentColor" strokeWidth="1.8" />
          <circle cx="16" cy="9" r="2.5" fill="none" stroke="currentColor" strokeWidth="1.8" />
          <path d="M6 17c.6-1.9 2.2-3 4-3h4c1.8 0 3.4 1.1 4 3" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </RailGlyph>
      ),
    },
    {
      key: "library",
      label: "Library",
      icon: (
        <RailGlyph>
          <path d="M7 5.5h9.5A2.5 2.5 0 0 1 19 8v10H9A2 2 0 0 0 7 20V5.5Z" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" />
          <path d="M7 6v14" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </RailGlyph>
      ),
    },
    {
      key: "models",
      label: "Models",
      icon: (
        <RailGlyph>
          <path d="M12 4 19 8v8l-7 4-7-4V8l7-4Z" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" />
          <path d="m12 4 7 4-7 4-7-4 7-4Zm0 8v8" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" />
        </RailGlyph>
      ),
    },
  ];

  return (
    <aside className="rail">
      <div className="rail-brand">
        <button className="rail-avatar-button" title="Open settings" type="button" onClick={onOpenSettings}>
          <span className="brand-mark">{providerMark(providerName)}</span>
        </button>
      </div>

      <nav className="rail-nav" aria-label="Workspace navigation">
        {items.map((item) => (
          <button className={`rail-icon-button ${item.active ? "active" : ""}`} key={item.key} title={item.label} type="button">
            <span className="rail-icon-glyph">{item.icon}</span>
            <span className="rail-icon-label">{item.label}</span>
          </button>
        ))}
      </nav>

      <div className="rail-current">
        <span className="eyebrow">Active</span>
        <strong>{providerName || "No provider"}</strong>
        <span>{topicCount} topics</span>
      </div>

      <div className="rail-status">
        <span title={notice}>{compactNotice(notice)}</span>
      </div>

      <button className="rail-icon-button rail-settings-button" onClick={onOpenSettings} title="Settings" type="button">
        <span className="rail-icon-glyph">
          <RailGlyph>
            <circle cx="12" cy="12" r="3.5" fill="none" stroke="currentColor" strokeWidth="1.8" />
            <path
              d="M12 3.8v2.1M12 18.1v2.1M20.2 12h-2.1M5.9 12H3.8M17.9 6.1l-1.5 1.5M7.6 16.4l-1.5 1.5M17.9 17.9l-1.5-1.5M7.6 7.6 6.1 6.1"
              fill="none"
              stroke="currentColor"
              strokeLinecap="round"
              strokeWidth="1.8"
            />
          </RailGlyph>
        </span>
        <span className="rail-icon-label">Settings</span>
      </button>
    </aside>
  );
}

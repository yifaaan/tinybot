import React from "react";

type Props = {
  notice: string;
  providerName: string;
  topicCount: number;
};

export function RailNav({ notice, providerName, topicCount }: Props) {
  return (
    <aside className="rail">
      <div className="rail-brand">
        <span className="brand-mark">tb</span>
        <div>
          <strong>tinybot Desktop</strong>
          <span>Desktop workspace</span>
        </div>
      </div>
      <nav className="rail-nav">
        <button className="rail-chip active" type="button">
          <span className="rail-chip-kicker">01</span>
          <strong>Chat</strong>
        </button>
        <button className="rail-chip" type="button">
          <span className="rail-chip-kicker">02</span>
          <strong>Agents</strong>
        </button>
        <button className="rail-chip" type="button">
          <span className="rail-chip-kicker">03</span>
          <strong>Library</strong>
        </button>
        <button className="rail-chip" type="button">
          <span className="rail-chip-kicker">04</span>
          <strong>Models</strong>
        </button>
      </nav>
      <div className="rail-current">
        <span className="eyebrow">Current</span>
        <strong>{providerName || "No provider"}</strong>
        <span>{topicCount} topics</span>
      </div>
      <div className="rail-status">
        <span>{notice}</span>
      </div>
    </aside>
  );
}

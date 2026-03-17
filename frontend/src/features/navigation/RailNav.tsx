import React from "react";

export function RailNav({ notice }: { notice: string }) {
  return (
    <aside className="rail">
      <div className="rail-brand">
        <span className="brand-mark">tb</span>
        <div>
          <strong>tinybot Desktop</strong>
          <span>CherryStudio-style shell</span>
        </div>
      </div>
      <nav className="rail-nav">
        <button className="rail-chip active" type="button">
          Chat
        </button>
        <button className="rail-chip" type="button">
          Agents
        </button>
        <button className="rail-chip" type="button">
          Library
        </button>
        <button className="rail-chip" type="button">
          Models
        </button>
      </nav>
      <div className="rail-status">
        <span>{notice}</span>
      </div>
    </aside>
  );
}

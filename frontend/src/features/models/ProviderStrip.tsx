import React from "react";

import type { ProviderInfo } from "../../app/types";

type Props = {
  providers: ProviderInfo[];
  onSelect: (provider: ProviderInfo) => void;
};

export function ProviderStrip({ providers, onSelect }: Props) {
  return (
    <div className="provider-strip">
      {providers.map((provider) => (
        <button
          key={provider.name}
          className={`provider-pill ${provider.active ? "active" : ""}`}
          onClick={() => onSelect(provider)}
          type="button">
          <strong>{provider.name}</strong>
          <span>{provider.model}</span>
        </button>
      ))}
    </div>
  );
}

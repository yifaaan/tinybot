import type { Bootstrap, SessionDetail, SessionSummary } from "./types";

export const mockBootstrap: Bootstrap = {
  workspace: "./workspace",
  config: {
    agents: {
      max_tokens: 8192,
      temperature: 0.7,
      enable_thinking: true,
    },
    providers: {
      active: "qwen",
    },
    channels: {
      console: {
        enabled: true,
      },
    },
  },
  providers: [
    {
      name: "qwen",
      kind: "qwen",
      model: "qwen3.5-plus",
      apiBase: "https://dashscope.aliyuncs.com/compatible-mode/v1",
      active: true,
      configured: true,
    },
    {
      name: "openai",
      kind: "openai",
      model: "gpt-4.1-mini",
      apiBase: "https://api.openai.com/v1",
      active: false,
      configured: false,
    },
  ],
  sessions: [
    {
      key: "desktop:demo-a",
      title: "Design a gateway client",
      preview: "Map the CLI flow and stream the answer into the desktop shell.",
      providerName: "qwen",
      channel: "desktop",
      messageCount: 14,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    {
      key: "desktop:demo-b",
      title: "Audit provider config",
      preview: "Summarise active providers and surface missing keys in settings.",
      providerName: "openai",
      channel: "desktop",
      messageCount: 7,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
  ],
};

export function mockDetail(session: SessionSummary): SessionDetail {
  return {
    summary: session,
    metadata: { title: session.title, provider: session.providerName },
    messages: [
      {
        role: "user",
        content: "Build a CherryStudio-style shell for tinybot.",
        createdAt: new Date().toISOString(),
      },
      {
        role: "assistant",
        content: "The desktop shell is ready to bind against the new Go desktop service.",
        createdAt: new Date().toISOString(),
      },
    ],
  };
}

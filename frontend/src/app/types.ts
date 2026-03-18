export type ProviderInfo = {
  name: string;
  kind: string;
  model: string;
  apiBase: string;
  active: boolean;
  configured: boolean;
};

export type SessionSummary = {
  key: string;
  title: string;
  preview: string;
  providerName: string;
  channel: string;
  messageCount: number;
  createdAt: string;
  updatedAt: string;
};

export type SessionMessage = {
  role: string;
  content: string;
  createdAt: string;
  thinking?: string;
  name?: string;
  toolCallId?: string;
  attachments?: FileAttachment[];
};

export type FileAttachment = {
  id: string;
  name: string;
  type: string;
  size: number;
  preview?: string;
  content?: string; // For text files
  path?: string;
};

export type SessionDetail = {
  summary: SessionSummary;
  messages: SessionMessage[];
  metadata: Record<string, unknown>;
};

export type ConfigShape = {
  agents: {
    max_tokens: number;
    temperature: number;
    enable_thinking: boolean;
    reasoning_effort?: string;
    reasoning_summary?: string;
    text_verbosity?: string;
  };
  providers: {
    active: string;
  };
  channels: {
    console?: {
      enabled: boolean;
    };
  };
};

export type Bootstrap = {
  workspace: string;
  config: ConfigShape;
  providers: ProviderInfo[];
  sessions: SessionSummary[];
};

export type ChatReply = {
  sessionKey: string;
  content: string;
  createdAt: string;
};

export type StreamEvent = {
  sessionKey: string;
  kind: "thinking" | "delta" | "done" | "error";
  delta?: string;
  content?: string;
  error?: string;
};

export type SendMessagePayload = {
  sessionKey: string;
  content: string;
  attachments?: FileAttachment[];
};

export type CreateSessionPayload = {
  title: string;
  providerName: string;
};

export type DesktopApi = {
  Bootstrap: () => Promise<Bootstrap>;
  GetSession: (key: string) => Promise<SessionDetail>;
  CreateSession: (payload: CreateSessionPayload) => Promise<SessionSummary>;
  RenameSession: (key: string, title: string) => Promise<SessionSummary>;
  DeleteSession: (key: string) => Promise<void>;
  SaveConfig: (patch: Record<string, unknown>) => Promise<Bootstrap>;
  StreamMessage: (payload: SendMessagePayload) => Promise<ChatReply>;
  RetryMessage: (sessionKey: string) => Promise<ChatReply>;
};

export type ThemeMode = "black" | "white";

declare global {
  interface Window {
    go?: {
      main?: {
        DesktopApp?: DesktopApi;
      };
    };
    runtime?: {
      EventsOn?: (event: string, callback: (payload: StreamEvent) => void) => () => void;
    };
  }
}

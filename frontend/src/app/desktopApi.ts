import type { Bootstrap, ChatReply, CreateSessionPayload, DesktopApi, SessionDetail, SessionSummary } from "./types";

function getDesktopApi(): DesktopApi | undefined {
  return window.go?.main?.DesktopApp;
}

export function getErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim() !== "") {
    return error.message;
  }
  if (typeof error === "string" && error.trim() !== "") {
    return error;
  }
  if (error && typeof error === "object") {
    const record = error as Record<string, unknown>;
    const directMessage = record.message;
    if (typeof directMessage === "string" && directMessage.trim() !== "") {
      return directMessage;
    }
    const nestedError = record.error;
    if (typeof nestedError === "string" && nestedError.trim() !== "") {
      return nestedError;
    }
    const cause = record.cause;
    if (typeof cause === "string" && cause.trim() !== "") {
      return cause;
    }
    try {
      const serialized = JSON.stringify(error);
      if (serialized && serialized !== "{}") {
        return serialized;
      }
    } catch {
      // Ignore serialization failures and fall through to the generic message.
    }
  }
  return "Unknown desktop error";
}

export async function waitForDesktopApi(timeoutMs = 2500): Promise<DesktopApi | undefined> {
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    const api = getDesktopApi();
    if (api) {
      return api;
    }
    await new Promise((resolve) => window.setTimeout(resolve, 50));
  }
  return getDesktopApi();
}

async function call<T>(method: keyof DesktopApi, ...args: unknown[]): Promise<T> {
  const api = await waitForDesktopApi();
  if (!api || typeof api[method] !== "function") {
    throw new Error(`Desktop binding ${String(method)} unavailable`);
  }
  try {
    return await (api[method] as (...inner: unknown[]) => Promise<T>)(...args);
  } catch (error) {
    throw new Error(`${String(method)} failed: ${getErrorMessage(error)}`);
  }
}

export function subscribeStream(callback: (payload: any) => void): (() => void) | undefined {
  return window.runtime?.EventsOn?.("desktop:chat-stream", callback);
}

export function bootstrapDesktop(): Promise<Bootstrap> {
  return call<Bootstrap>("Bootstrap");
}

export function getSessionDetail(key: string): Promise<SessionDetail> {
  return call<SessionDetail>("GetSession", key);
}

export function createDesktopSession(payload: CreateSessionPayload): Promise<SessionSummary> {
  return call<SessionSummary>("CreateSession", payload);
}

export function renameDesktopSession(key: string, title: string): Promise<SessionSummary> {
  return call<SessionSummary>("RenameSession", key, title);
}

export function deleteDesktopSession(key: string): Promise<void> {
  return call<void>("DeleteSession", key);
}

export function saveDesktopConfig(patch: Record<string, unknown>): Promise<Bootstrap> {
  return call<Bootstrap>("SaveConfig", patch);
}

export function streamDesktopMessage(payload: { sessionKey: string; content: string }): Promise<ChatReply> {
  return call<ChatReply>("StreamMessage", payload);
}

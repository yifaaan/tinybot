import React, { useCallback, useEffect, useMemo, useState } from "react";

import {
  bootstrapDesktop,
  createDesktopSession,
  deleteDesktopSession,
  getErrorMessage,
  getSessionDetail,
  renameDesktopSession,
  retryDesktopMessage,
  saveDesktopConfig,
  streamDesktopMessage,
  subscribeStream,
  waitForDesktopApi,
} from "./desktopApi";
import { mockBootstrap, mockDetail } from "./mockData";
import { useTheme } from "./providers/ThemeProvider";
import type { Bootstrap, FileAttachment, ProviderInfo, SessionDetail, SessionMessage, SessionSummary, StreamEvent } from "./types";
import { AssistantsPane } from "../features/assistants/AssistantsPane";
import { ChatWorkspace } from "../features/chat/ChatWorkspace";
import { RailNav } from "../features/navigation/RailNav";
import { SettingsDrawer } from "../features/settings/SettingsDrawer";
import type { ProviderDraft } from "../features/settings/SettingsDrawer";
import { RenameDialog } from "../features/topics/RenameDialog";
import { TopicsPane } from "../features/topics/TopicsPane";
import { ResizablePanel } from "../features/layout/ResizablePanel";

const NEW_SESSION_TITLE = "Untitled desktop chat";
type StreamPhase = "idle" | "thinking" | "replying";
const ASSISTANTS_PANEL_KEY = "tinybot:assistants-width";
const TOPICS_PANEL_KEY = "tinybot:topics-width";

function summaryProviderName(summary: SessionSummary | undefined, fallbackProviderName: string): string {
  return summary?.providerName?.trim() || fallbackProviderName;
}

function metadataProviderName(metadata: Record<string, unknown> | undefined): string {
  const provider = metadata?.provider;
  return typeof provider === "string" ? provider.trim() : "";
}

function resolveSessionKeyForProvider(
  sessions: SessionSummary[],
  providerName: string,
  preferredSessionKey?: string,
): string {
  const preferred = sessions.find((session) => session.key === preferredSessionKey);
  if (preferred && summaryProviderName(preferred, providerName) === providerName) {
    return preferred.key;
  }
  return sessions.find((session) => summaryProviderName(session, providerName) === providerName)?.key ?? "";
}

function ensureSession(
  summary: SessionSummary | undefined,
  selectedSession: SessionDetail | null,
  sessionKey: string,
  providerName: string,
): SessionDetail {
  if (selectedSession) {
    return selectedSession;
  }
  return {
    summary: summary ?? {
      key: sessionKey,
      title: sessionKey || "Untitled",
      preview: "",
      providerName,
      channel: "desktop",
      messageCount: 0,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    metadata: { provider: providerName },
    messages: [],
  };
}

function replaceLastAssistant(messages: SessionMessage[], content: string, thinking?: string): SessionMessage[] {
  const next = messages.slice();
  const last = next[next.length - 1];
  if (last && last.role === "assistant") {
    next[next.length - 1] = { ...last, content, ...(thinking !== undefined ? { thinking } : {}) };
    return next;
  }
  next.push({
    role: "assistant",
    content,
    ...(thinking !== undefined ? { thinking } : {}),
    createdAt: new Date().toISOString(),
  });
  return next;
}

export function App() {
  const { theme, setTheme } = useTheme();
  const [bootstrap, setBootstrap] = useState<Bootstrap>(mockBootstrap);
  const [selectedProviderName, setSelectedProviderName] = useState<string>(
    mockBootstrap.config.providers.active || mockBootstrap.providers[0]?.name || "",
  );
  const [selectedSessionKey, setSelectedSessionKey] = useState<string>(mockBootstrap.sessions[0]?.key ?? "");
  const [selectedSession, setSelectedSession] = useState<SessionDetail | null>(
    mockBootstrap.sessions[0] ? mockDetail(mockBootstrap.sessions[0]) : null,
  );
  const [assistantsOpen, setAssistantsOpen] = useState(true);
  const [topicsOpen, setTopicsOpen] = useState(true);
  const [assistantsWidth, setAssistantsWidth] = useState(248);
  const [topicsWidth, setTopicsWidth] = useState(286);
  const [draft, setDraft] = useState("");
  const [attachments, setAttachments] = useState<FileAttachment[]>([]);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [renameTarget, setRenameTarget] = useState<SessionSummary | null>(null);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("Desktop preview mode");
  const [thinkingText, setThinkingText] = useState("");
  const [thinkingTarget, setThinkingTarget] = useState("");
  const [streamPhase, setStreamPhase] = useState<StreamPhase>("idle");
  const [replyBuffer, setReplyBuffer] = useState("");
  const [thinkingDone, setThinkingDone] = useState(false);

  const currentProvider = useMemo(
    () => bootstrap.providers.find((provider) => provider.name === selectedProviderName) ?? bootstrap.providers[0] ?? null,
    [bootstrap.providers, selectedProviderName],
  );

  const currentSummary = useMemo(
    () => bootstrap.sessions.find((session) => session.key === selectedSessionKey),
    [bootstrap.sessions, selectedSessionKey],
  );

  const providerSessionCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    bootstrap.sessions.forEach((session) => {
      const providerName = summaryProviderName(session, bootstrap.config.providers.active);
      counts[providerName] = (counts[providerName] ?? 0) + 1;
    });
    return counts;
  }, [bootstrap.config.providers.active, bootstrap.sessions]);

  const filteredSessions = useMemo(
    () =>
      bootstrap.sessions.filter(
        (session) => summaryProviderName(session, bootstrap.config.providers.active) === selectedProviderName,
      ),
    [bootstrap.config.providers.active, bootstrap.sessions, selectedProviderName],
  );

  const refreshBootstrap = useCallback(
    async (preferredSessionKey?: string, preferredProviderName?: string) => {
      const api = await waitForDesktopApi();
      if (!api) {
        setNotice("Running in mock mode");
        return;
      }
      const data = await bootstrapDesktop();
      const nextProviderName =
        preferredProviderName ||
        data.sessions.find((session) => session.key === preferredSessionKey)?.providerName ||
        selectedProviderName ||
        data.config.providers.active ||
        data.providers[0]?.name ||
        "";
      const nextSessionKey = resolveSessionKeyForProvider(
        data.sessions,
        nextProviderName,
        preferredSessionKey || selectedSessionKey,
      );

      setBootstrap(data);
      setSelectedProviderName(nextProviderName);
      setSelectedSessionKey(nextSessionKey);
      if (!nextSessionKey) {
        setSelectedSession(null);
      }
      setNotice(`Workspace: ${data.workspace}`);
    },
    [selectedProviderName, selectedSessionKey],
  );

  const refreshSession = useCallback(
    async (sessionKey: string) => {
      if (!sessionKey) {
        return;
      }

      const api = await waitForDesktopApi(250);
      if (!api) {
        const mock = bootstrap.sessions.find((session) => session.key === sessionKey);
        if (mock) {
          setSelectedSession(mockDetail(mock));
          setSelectedProviderName(summaryProviderName(mock, selectedProviderName));
        }
        return;
      }

      const detail = await getSessionDetail(sessionKey);
      const nextProviderName =
        detail.summary.providerName || metadataProviderName(detail.metadata) || selectedProviderName;
      setSelectedSession(detail);
      setSelectedProviderName(nextProviderName);
      setBusy(false);
    },
    [bootstrap.sessions, selectedProviderName],
  );

  const ensureActiveSession = useCallback(async (): Promise<string> => {
    if (selectedSessionKey) {
      return selectedSessionKey;
    }

    const providerName = selectedProviderName || bootstrap.config.providers.active || bootstrap.providers[0]?.name || "";
    const fallbackSummary: SessionSummary = {
      key: `desktop:${Date.now()}`,
      title: NEW_SESSION_TITLE,
      preview: "",
      providerName,
      channel: "desktop",
      messageCount: 0,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    const api = await waitForDesktopApi(250);
    const summary = api
      ? await createDesktopSession({ title: NEW_SESSION_TITLE, providerName })
      : fallbackSummary;

    setBootstrap((previous) => ({ ...previous, sessions: [summary, ...previous.sessions] }));
    setSelectedProviderName(summary.providerName || providerName);
    setSelectedSessionKey(summary.key);
    setSelectedSession({
      summary,
      metadata: { title: summary.title, provider: summary.providerName || providerName },
      messages: [],
    });
    return summary.key;
  }, [bootstrap.config.providers.active, bootstrap.providers, selectedProviderName, selectedSessionKey]);

  useEffect(() => {
    void refreshBootstrap();
  }, [refreshBootstrap]);

  useEffect(() => {
    if (!selectedSessionKey) {
      return;
    }
    void refreshSession(selectedSessionKey);
  }, [refreshSession, selectedSessionKey]);

  useEffect(() => {
    if (thinkingText === thinkingTarget) {
      return;
    }

    const nextLength = Math.min(
      thinkingTarget.length,
      thinkingText.length + Math.max(4, Math.ceil((thinkingTarget.length - thinkingText.length) / 18)),
    );
    const timer = window.setTimeout(() => {
      setThinkingText(thinkingTarget.slice(0, nextLength));
    }, 28);

    return () => {
      window.clearTimeout(timer);
    };
  }, [thinkingTarget, thinkingText]);

  useEffect(() => {
    const activeSessionKey = selectedSession?.summary.key || selectedSessionKey;
    const unsubscribe = subscribeStream((payload: StreamEvent) => {
      if (payload.sessionKey !== activeSessionKey) {
        return;
      }

      const activeProviderName =
        selectedSession?.summary.providerName || selectedProviderName || bootstrap.config.providers.active;

      if (payload.kind === "thinking") {
        setThinkingTarget((previous) => `${previous}${payload.delta ?? ""}`);
        setStreamPhase("thinking");
        setThinkingDone(false);
        setNotice("Thinking...");
        return;
      }
      if (payload.kind === "error") {
        const message = payload.error ?? "Streaming error";
        setStreamPhase("idle");
        setThinkingTarget("");
        setThinkingText("");
        setReplyBuffer("");
        setThinkingDone(false);
        setNotice(message);
        setSelectedSession((previous) => {
          const ensured = ensureSession(currentSummary, previous, activeSessionKey, activeProviderName);
          return {
            ...ensured,
            messages: replaceLastAssistant(ensured.messages, `Error: ${message}`),
          };
        });
        setBusy(false);
        return;
      }
      if (payload.kind === "delta" || payload.kind === "done") {
        const text = payload.kind === "done" ? payload.content ?? "" : payload.delta ?? "";

        // If we're still in thinking phase, buffer the reply content
        if (payload.kind === "delta" && !thinkingDone && streamPhase === "thinking") {
          setReplyBuffer((previous) => `${previous}${text}`);
          return;
        }

        // Thinking is done, switch to replying phase
        if (payload.kind === "delta") {
          setStreamPhase("replying");
          setThinkingDone(true);
          setNotice("Responding...");
        }

        // Combine buffered content with current text
        setSelectedSession((previous) => {
          const ensured = ensureSession(currentSummary, previous, activeSessionKey, activeProviderName);
          const currentText = ensured.messages[ensured.messages.length - 1]?.content ?? "";
          const bufferedText = replyBuffer + text;
          const merged = payload.kind === "done" ? text : `${currentText}${bufferedText}`;
          return {
            ...ensured,
            messages: replaceLastAssistant(ensured.messages, merged),
          };
        });
        // Clear the buffer after using it
        if (replyBuffer) {
          setReplyBuffer("");
        }
      }
      if (payload.kind === "done") {
        setStreamPhase("idle");
        setThinkingDone(false);
        setReplyBuffer("");
        const finalThinking = thinkingTarget.trim() || thinkingText.trim();
        if (finalThinking !== "") {
          setSelectedSession((previous) => {
            const ensured = ensureSession(currentSummary, previous, activeSessionKey, activeProviderName);
            const currentText = ensured.messages[ensured.messages.length - 1]?.content ?? "";
            return {
              ...ensured,
              messages: replaceLastAssistant(ensured.messages, currentText, finalThinking),
            };
          });
        }
        setThinkingTarget("");
        setThinkingText("");
        setNotice("Stream complete");
        setBusy(false);
      }
    });
    return () => {
      if (typeof unsubscribe === "function") {
        unsubscribe();
      }
    };
  }, [
    bootstrap.config.providers.active,
    currentSummary,
    selectedProviderName,
    selectedSession?.summary.key,
    selectedSession?.summary.providerName,
    selectedSessionKey,
    streamPhase,
    thinkingDone,
    replyBuffer,
    thinkingTarget,
    thinkingText,
  ]);

  const handleCreateSession = useCallback(async () => {
    const providerName = selectedProviderName || bootstrap.config.providers.active || bootstrap.providers[0]?.name || "";
    const api = await waitForDesktopApi(250);
    if (!api) {
      const mock: SessionSummary = {
        key: `desktop:${Date.now()}`,
        title: NEW_SESSION_TITLE,
        preview: "",
        providerName,
        channel: "desktop",
        messageCount: 0,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };
      setBootstrap((previous) => ({ ...previous, sessions: [mock, ...previous.sessions] }));
      setSelectedProviderName(providerName);
      setSelectedSessionKey(mock.key);
      setSelectedSession({ summary: mock, metadata: { provider: providerName }, messages: [] });
      return;
    }

    const summary = await createDesktopSession({ title: NEW_SESSION_TITLE, providerName });
    setBootstrap((previous) => ({ ...previous, sessions: [summary, ...previous.sessions] }));
    setSelectedProviderName(summary.providerName || providerName);
    setSelectedSessionKey(summary.key);
    setSelectedSession({ summary, metadata: { provider: summary.providerName || providerName }, messages: [] });
  }, [bootstrap.config.providers.active, bootstrap.providers, selectedProviderName]);

  const handleRename = useCallback(async () => {
    if (!currentSummary) {
      return;
    }
    setRenameTarget(currentSummary);
  }, [currentSummary]);

  const handleConfirmRename = useCallback(async (title: string) => {
    const target = renameTarget;
    if (!target) {
      return;
    }

    setRenameTarget(null);
    const api = await waitForDesktopApi(250);
    if (!api) {
      setBootstrap((previous) => ({
        ...previous,
        sessions: previous.sessions.map((session) => (session.key === target.key ? { ...session, title } : session)),
      }));
      setSelectedSession((previous) =>
        previous && previous.summary.key === target.key ? { ...previous, summary: { ...previous.summary, title } } : previous,
      );
      return;
    }
    const summary = await renameDesktopSession(target.key, title);
    setBootstrap((previous) => ({
      ...previous,
      sessions: previous.sessions.map((session) => (session.key === summary.key ? summary : session)),
    }));
    setSelectedSession((previous) =>
      previous && previous.summary.key === summary.key ? { ...previous, summary } : previous,
    );
  }, [renameTarget]);

  const handleRenameSession = useCallback(async (session: SessionSummary) => {
    setRenameTarget(session);
  }, []);

  const handleDelete = useCallback(async () => {
    if (!currentSummary) {
      return;
    }
    if (!window.confirm(`Delete "${currentSummary.title}"?`)) {
      return;
    }
    const api = await waitForDesktopApi(250);
    if (api) {
      await deleteDesktopSession(currentSummary.key);
    }
    const nextSessions = bootstrap.sessions.filter((session) => session.key !== currentSummary.key);
    const nextSessionKey = resolveSessionKeyForProvider(nextSessions, selectedProviderName, undefined);
    setBootstrap((previous) => ({
      ...previous,
      sessions: previous.sessions.filter((session) => session.key !== currentSummary.key),
    }));
    setSelectedSessionKey(nextSessionKey);
    setSelectedSession(null);
  }, [bootstrap.sessions, currentSummary, selectedProviderName]);

  const handleDeleteSession = useCallback(
    async (session: SessionSummary) => {
      if (!window.confirm(`Delete "${session.title}"?`)) {
        return;
      }

      const api = await waitForDesktopApi(250);
      if (api) {
        await deleteDesktopSession(session.key);
      }

      const nextSessions = bootstrap.sessions.filter((item) => item.key !== session.key);
      const nextSessionKey =
        session.key === selectedSessionKey
          ? resolveSessionKeyForProvider(nextSessions, selectedProviderName, undefined)
          : selectedSessionKey;

      setBootstrap((previous) => ({
        ...previous,
        sessions: previous.sessions.filter((item) => item.key !== session.key),
      }));

      if (session.key === selectedSessionKey) {
        setSelectedSessionKey(nextSessionKey);
        setSelectedSession(null);
      }
    },
    [bootstrap.sessions, selectedProviderName, selectedSessionKey],
  );

  const handleProviderSelect = useCallback(
    async (provider: ProviderInfo) => {
      const nextSessionKey = resolveSessionKeyForProvider(bootstrap.sessions, provider.name, selectedSessionKey);
      setSelectedProviderName(provider.name);
      setSelectedSessionKey(nextSessionKey);
      if (!nextSessionKey) {
        setSelectedSession(null);
      }
      setNotice(`Switching to ${provider.name}`);

      const api = await waitForDesktopApi(250);
      if (!api) {
        setBootstrap((previous) => ({
          ...previous,
          config: {
            ...previous.config,
            providers: { active: provider.name },
          },
          providers: previous.providers.map((item) => ({ ...item, active: item.name === provider.name })),
        }));
        return;
      }

      const updated = await saveDesktopConfig({ activeProvider: provider.name });
      setBootstrap(updated);
      setSelectedProviderName(provider.name);
      setSelectedSessionKey(resolveSessionKeyForProvider(updated.sessions, provider.name, nextSessionKey));
      setNotice(`Provider set to ${provider.name}`);
    },
    [bootstrap.sessions, selectedSessionKey],
  );

  const handleSelectSession = useCallback(
    async (session: SessionSummary) => {
      const providerName = summaryProviderName(session, selectedProviderName);
      setSelectedSessionKey(session.key);
      setSelectedProviderName(providerName);

      const api = await waitForDesktopApi(250);
      if (!api || providerName === bootstrap.config.providers.active) {
        return;
      }

      const updated = await saveDesktopConfig({ activeProvider: providerName });
      setBootstrap(updated);
    },
    [bootstrap.config.providers.active, selectedProviderName],
  );

  const handleSaveSettings = useCallback(
    async (
      form: HTMLFormElement,
      providers: ProviderDraft[],
    ) => {
      const data = new FormData(form);
      const nextTheme = String(data.get("theme") ?? "black") as "black" | "white";
      setTheme(nextTheme);

      const providerPatches = providers
        .map((provider) => {
          const name = provider.name.trim();
          const kind = provider.kind.trim();
          const model = provider.model.trim();
          const apiBase = provider.apiBase.trim();
          const apiKey = provider.apiKey.trim();
          if (name === "" || model === "") {
            return null;
          }

          return {
            name,
            kind: kind || name,
            model,
            ...(apiBase !== "" ? { apiBase } : {}),
            ...(apiKey !== "" ? { apiKey } : {}),
          };
        })
        .filter((provider): provider is { name: string; kind: string; model: string; apiBase?: string; apiKey?: string } => Boolean(provider));

      const patch = {
        activeProvider: String(data.get("activeProvider") ?? bootstrap.config.providers.active),
        temperature: Number(data.get("temperature") ?? bootstrap.config.agents.temperature),
        maxTokens: Number(data.get("maxTokens") ?? bootstrap.config.agents.max_tokens),
        enableThinking: data.get("enableThinking") === "on",
        reasoningEffort: String(data.get("reasoningEffort") ?? bootstrap.config.agents.reasoning_effort ?? "high"),
        reasoningSummary: String(data.get("reasoningSummary") ?? bootstrap.config.agents.reasoning_summary ?? "detailed"),
        textVerbosity: String(data.get("textVerbosity") ?? bootstrap.config.agents.text_verbosity ?? "medium"),
        consoleEnabled: data.get("consoleEnabled") === "on",
        providers: providerPatches,
      };

      const api = await waitForDesktopApi(250);
      if (!api) {
        setBootstrap((previous) => ({
          ...previous,
          config: {
            ...previous.config,
            agents: {
              ...previous.config.agents,
              temperature: patch.temperature,
              max_tokens: patch.maxTokens,
              enable_thinking: patch.enableThinking,
              reasoning_effort: patch.reasoningEffort,
              reasoning_summary: patch.reasoningSummary,
              text_verbosity: patch.textVerbosity,
            },
            providers: {
              active: patch.activeProvider,
            },
            channels: {
              ...previous.config.channels,
              console: {
                enabled: patch.consoleEnabled,
              },
            },
          },
          providers:
            providerPatches.length > 0
              ? providerPatches.map((provider) => ({
                  name: provider.name,
                  kind: provider.kind,
                  model: provider.model,
                  apiBase: provider.apiBase ?? "",
                  active: provider.name === patch.activeProvider,
                  configured:
                    typeof provider.apiKey === "string"
                      ? provider.apiKey.trim() !== "" || (provider.apiBase ?? "").startsWith("http://localhost")
                      : previous.providers.some((item) => item.name === provider.name && item.configured),
                }))
              : previous.providers,
        }));
        setSelectedProviderName(patch.activeProvider);
        setSelectedSessionKey(resolveSessionKeyForProvider(bootstrap.sessions, patch.activeProvider, selectedSessionKey));
        setSettingsOpen(false);
        setNotice("Settings saved");
        return;
      }

      const updated = await saveDesktopConfig(patch);
      const nextSessionKey = resolveSessionKeyForProvider(updated.sessions, patch.activeProvider, selectedSessionKey);
      setBootstrap(updated);
      setSelectedProviderName(patch.activeProvider);
      setSelectedSessionKey(nextSessionKey);
      if (!nextSessionKey) {
        setSelectedSession(null);
      }
      setSettingsOpen(false);
      setNotice("Settings saved");
    },
    [bootstrap.config, bootstrap.sessions, selectedSessionKey, setTheme],
  );

  const handleUpdateModelControls = useCallback(
    async (patch: { reasoningEffort?: string; reasoningSummary?: string; textVerbosity?: string }) => {
      const nextPatch = {
        ...(patch.reasoningEffort ? { reasoningEffort: patch.reasoningEffort } : {}),
        ...(patch.reasoningSummary ? { reasoningSummary: patch.reasoningSummary } : {}),
        ...(patch.textVerbosity ? { textVerbosity: patch.textVerbosity } : {}),
      };

      if (Object.keys(nextPatch).length === 0) {
        return;
      }

      const api = await waitForDesktopApi(250);
      if (!api) {
        setBootstrap((previous) => ({
          ...previous,
          config: {
            ...previous.config,
            agents: {
              ...previous.config.agents,
              ...(patch.reasoningEffort ? { reasoning_effort: patch.reasoningEffort } : {}),
              ...(patch.reasoningSummary ? { reasoning_summary: patch.reasoningSummary } : {}),
              ...(patch.textVerbosity ? { text_verbosity: patch.textVerbosity } : {}),
            },
          },
        }));
        setNotice("Response controls updated");
        return;
      }

      const updated = await saveDesktopConfig(nextPatch);
      setBootstrap(updated);
      setNotice("Response controls updated");
    },
    [],
  );

  const handleSend = useCallback(async () => {
    const content = draft.trim();
    const hasAttachments = attachments.length > 0;

    // Allow sending if there's content OR attachments
    if ((!content && !hasAttachments) || busy) {
      return;
    }

    // If only attachments without text, add placeholder
    const messageContent = content || "[Attachment]";

    const providerName = selectedProviderName || bootstrap.config.providers.active || bootstrap.providers[0]?.name || "";
    const sessionKey = await ensureActiveSession();
    const userMessage: SessionMessage = {
      role: "user",
      content: messageContent,
      createdAt: new Date().toISOString(),
    };
    const assistantPlaceholder: SessionMessage = {
      role: "assistant",
      content: "",
      createdAt: new Date().toISOString(),
    };

    setBusy(true);
    setDraft("");
    setNotice("Streaming reply");
    setThinkingText("");
    setThinkingTarget("");
    setReplyBuffer("");
    setThinkingDone(false);
    setStreamPhase("thinking");
    setSelectedSession((previous) => {
      const ensured = ensureSession(currentSummary, previous, sessionKey, providerName);
      return {
        ...ensured,
        messages: [...ensured.messages, userMessage, assistantPlaceholder],
      };
    });

    const api = await waitForDesktopApi(250);
    if (!api) {
      setSelectedSession((previous) => {
        const ensured = ensureSession(currentSummary, previous, sessionKey, providerName);
        return {
          ...ensured,
          messages: replaceLastAssistant(
            ensured.messages,
            "Mock desktop response flowing through the CherryStudio-style shell.",
          ),
        };
      });
      setNotice("Mock stream complete");
      setStreamPhase("idle");
      setBusy(false);
      return;
    }

    try {
      const reply = await streamDesktopMessage({ sessionKey, content: messageContent, attachments });
      if (!reply || typeof reply.content !== "string") {
        throw new Error("StreamMessage returned no reply payload");
      }
      setAttachments([]);
      setSelectedSessionKey(sessionKey);
      setSelectedSession((previous) => {
        const ensured = ensureSession(currentSummary, previous, sessionKey, providerName);
        return {
          ...ensured,
          messages: replaceLastAssistant(ensured.messages, reply.content),
        };
      });
      await refreshBootstrap(sessionKey, providerName);
      await refreshSession(sessionKey);
      setNotice("Reply received");
      setStreamPhase("idle");
      setThinkingTarget("");
      setThinkingText("");
      setReplyBuffer("");
      setThinkingDone(false);
      setBusy(false);
    } catch (error) {
      const message = `Send failed: ${getErrorMessage(error)}`;
      setStreamPhase("idle");
      setThinkingTarget("");
      setThinkingText("");
      setReplyBuffer("");
      setThinkingDone(false);
      setNotice(message);
      setSelectedSession((previous) => {
        const ensured = ensureSession(currentSummary, previous, sessionKey, providerName);
        return {
          ...ensured,
          messages: replaceLastAssistant(ensured.messages, message),
        };
      });
      setBusy(false);
    }
  }, [
    bootstrap.config.providers.active,
    bootstrap.providers,
    busy,
    currentSummary,
    draft,
    ensureActiveSession,
    refreshBootstrap,
    refreshSession,
    selectedProviderName,
    attachments,
  ]);

  const handleRetry = useCallback(async () => {
    if (busy || !selectedSessionKey) {
      return;
    }

    const providerName = selectedProviderName || bootstrap.config.providers.active || bootstrap.providers[0]?.name || "";
    const sessionKey = selectedSessionKey;

    setBusy(true);
    setNotice("Retrying...");
    setThinkingText("");
    setThinkingTarget("");
    setReplyBuffer("");
    setThinkingDone(false);
    setStreamPhase("thinking");

    const api = await waitForDesktopApi(250);
    if (!api) {
      setNotice("Mock mode - retry not available");
      setStreamPhase("idle");
      setBusy(false);
      return;
    }

    try {
      const reply = await retryDesktopMessage(sessionKey);
      if (!reply || typeof reply.content !== "string") {
        throw new Error("RetryMessage returned no reply payload");
      }
      await refreshSession(sessionKey);
      setNotice("Retry complete");
      setStreamPhase("idle");
      setThinkingTarget("");
      setThinkingText("");
      setReplyBuffer("");
      setThinkingDone(false);
      setBusy(false);
    } catch (error) {
      const message = `Retry failed: ${getErrorMessage(error)}`;
      setStreamPhase("idle");
      setThinkingTarget("");
      setThinkingText("");
      setReplyBuffer("");
      setThinkingDone(false);
      setNotice(message);
      setBusy(false);
    }
  }, [
    bootstrap.config.providers.active,
    bootstrap.providers,
    busy,
    refreshSession,
    selectedProviderName,
    selectedSessionKey,
  ]);

  const handleClear = useCallback(() => {
    if (!selectedSessionKey) {
      return;
    }
    setSelectedSession((previous) => {
      if (!previous || previous.summary.key !== selectedSessionKey) {
        return previous;
      }
      return {
        ...previous,
        messages: [],
        summary: {
          ...previous.summary,
          messageCount: 0,
          updatedAt: new Date().toISOString(),
        },
      };
    });
    setNotice("Conversation cleared");
  }, [selectedSessionKey]);

  const handleExport = useCallback(() => {
    if (!selectedSession) {
      return;
    }
    const exportData = {
      title: selectedSession.summary.title,
      exportedAt: new Date().toISOString(),
      messages: selectedSession.messages.map((msg) => ({
        role: msg.role,
        content: msg.content,
        createdAt: msg.createdAt,
      })),
    };
    const blob = new Blob([JSON.stringify(exportData, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `${selectedSession.summary.title.replace(/[^a-zA-Z0-9]/g, "_")}-${Date.now()}.json`;
    link.click();
    URL.revokeObjectURL(url);
    setNotice("Conversation exported");
  }, [selectedSession]);

  const shellColumns = useMemo(() => {
    const columns = ["64px"];
    if (assistantsOpen) {
      columns.push(`${assistantsWidth}px`);
    }
    if (topicsOpen) {
      columns.push(`${topicsWidth}px`);
    }
    columns.push("minmax(0, 1fr)");
    return columns.join(" ");
  }, [assistantsOpen, assistantsWidth, topicsOpen, topicsWidth]);

  return (
    <div className="app-shell">
      <div className="shell" style={{ gridTemplateColumns: shellColumns }}>
        <RailNav
          notice={notice}
          onOpenSettings={() => setSettingsOpen(true)}
          providerName={currentProvider?.name ?? ""}
          topicCount={filteredSessions.length}
        />
        {assistantsOpen && (
          <ResizablePanel
            defaultWidth={248}
            minWidth={180}
            maxWidth={400}
            storageKey="tinybot-assistants-width"
            side="left"
            onResize={setAssistantsWidth}>
            <AssistantsPane
              onSelectProvider={handleProviderSelect}
              providers={bootstrap.providers}
              selectedProviderName={selectedProviderName}
              sessionCounts={providerSessionCounts}
            />
          </ResizablePanel>
        )}
        {topicsOpen && (
          <ResizablePanel
            defaultWidth={286}
            minWidth={200}
            maxWidth={450}
            storageKey="tinybot-topics-width"
            side="left"
            onResize={setTopicsWidth}>
            <TopicsPane
              onCreateSession={handleCreateSession}
              onDeleteSession={handleDeleteSession}
              onRenameSession={handleRenameSession}
              onSelectSession={handleSelectSession}
              provider={currentProvider}
              selectedSessionKey={selectedSessionKey}
              sessions={filteredSessions}
            />
          </ResizablePanel>
        )}
        <ChatWorkspace
          assistantsOpen={assistantsOpen}
          attachments={attachments}
          busy={busy}
          draft={draft}
          notice={notice}
          reasoningEffort={bootstrap.config.agents.reasoning_effort ?? "high"}
          reasoningSummary={bootstrap.config.agents.reasoning_summary ?? "detailed"}
          textVerbosity={bootstrap.config.agents.text_verbosity ?? "medium"}
          onClear={handleClear}
          onDelete={handleDelete}
          onDraftChange={setDraft}
          onAttachmentsChange={setAttachments}
          onExport={handleExport}
          onOpenSettings={() => setSettingsOpen(true)}
          onRename={handleRename}
          onRetry={handleRetry}
          onSend={handleSend}
          onToggleAssistants={() => setAssistantsOpen((current) => !current)}
          onToggleTopics={() => setTopicsOpen((current) => !current)}
          onUpdateModelControls={handleUpdateModelControls}
          provider={currentProvider}
          session={selectedSession}
          streamPhase={streamPhase}
          thinkingText={thinkingText}
          topicsOpen={topicsOpen}
        />
      </div>
      <SettingsDrawer
        bootstrap={bootstrap}
        onClose={() => setSettingsOpen(false)}
        onSave={handleSaveSettings}
        onThemeChange={setTheme}
        open={settingsOpen}
        theme={theme}
      />
      <RenameDialog
        initialValue={renameTarget?.title ?? ""}
        onClose={() => setRenameTarget(null)}
        onSubmit={handleConfirmRename}
        open={Boolean(renameTarget)}
      />
    </div>
  );
}

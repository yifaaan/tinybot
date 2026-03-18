import React, { KeyboardEvent, useEffect, useRef, useState } from "react";

import type { FileAttachment, ProviderInfo, SessionDetail, SessionMessage } from "../../app/types";
import { AttachmentList } from "./FileAttachment";
import { MarkdownContent } from "./MarkdownContent";
import { CommandPalette, useSlashCommands, type CommandContext } from "./SlashCommands";

type Props = {
  session: SessionDetail | null;
  provider: ProviderInfo | null;
  draft: string;
  busy: boolean;
  notice: string;
  reasoningEffort: string;
  reasoningSummary: string;
  textVerbosity: string;
  thinkingText: string;
  streamPhase: "idle" | "thinking" | "replying";
  assistantsOpen: boolean;
  topicsOpen: boolean;
  attachments: FileAttachment[];
  onDraftChange: (value: string) => void;
  onAttachmentsChange: (attachments: FileAttachment[]) => void;
  onClear: () => void;
  onExport: () => void;
  onUpdateModelControls: (patch: {
    reasoningEffort?: string;
    reasoningSummary?: string;
    textVerbosity?: string;
  }) => void;
  onSend: () => void;
  onRetry: () => void;
  onOpenSettings: () => void;
  onRename: () => void;
  onDelete: () => void;
  onToggleAssistants: () => void;
  onToggleTopics: () => void;
};

function IconGlyph({ children }: { children: React.ReactNode }) {
  return (
    <svg aria-hidden="true" className="ui-icon" viewBox="0 0 20 20">
      {children}
    </svg>
  );
}

function formatMessageTime(value: string | undefined): string {
  if (!value) {
    return "";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  return date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatHeaderTime(value: string | undefined): string {
  if (!value) {
    return "";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  return date.toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function summarizeNotice(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }

  if (/^Workspace:\s*/i.test(trimmed)) {
    const cleaned = trimmed.replace(/^Workspace:\s*/i, "");
    const segments = cleaned.split(/[\\/]/).filter(Boolean);
    const tail = segments[segments.length - 1] || cleaned;
    return `Workspace / ${tail}`;
  }

  return trimmed;
}

function shortMark(value: string): string {
  return value
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part.slice(0, 1).toUpperCase())
    .join("") || "AI";
}

type MessageGroup = {
  key: string;
  label: string;
  messages: { message: SessionMessage; index: number }[];
};

function getMessageGroupLabel(date: Date): string {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 24 * 60 * 60 * 1000);
  const weekAgo = new Date(today.getTime() - 7 * 24 * 60 * 60 * 1000);

  const messageDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());

  if (messageDate.getTime() === today.getTime()) {
    return "Today";
  }
  if (messageDate.getTime() === yesterday.getTime()) {
    return "Yesterday";
  }
  if (messageDate.getTime() > weekAgo.getTime()) {
    return date.toLocaleDateString([], { weekday: "long" });
  }
  return date.toLocaleDateString([], { month: "short", day: "numeric", year: "numeric" });
}

function groupMessagesByDate(messages: SessionMessage[]): MessageGroup[] {
  if (messages.length === 0) {
    return [];
  }

  const groups: MessageGroup[] = [];
  let currentGroup: MessageGroup | null = null;

  messages.forEach((message, index) => {
    const date = new Date(message.createdAt || "");
    if (Number.isNaN(date.getTime())) {
      if (currentGroup) {
        currentGroup.messages.push({ message, index });
      }
      return;
    }

    const groupLabel = getMessageGroupLabel(date);
    const groupKey = groupLabel.toLowerCase().replace(/\s+/g, "-");

    if (!currentGroup || currentGroup.key !== groupKey) {
      currentGroup = { key: groupKey, label: groupLabel, messages: [] };
      groups.push(currentGroup);
    }
    currentGroup.messages.push({ message, index });
  });

  return groups;
}

function messageChrome(role: string, createdAt?: string, name?: string, assistantLabel?: string) {
  if (role === "user") {
    return { avatar: "ME", label: "You", time: formatMessageTime(createdAt) };
  }
  if (role === "tool") {
    return { avatar: "TL", label: name || "Tool", time: formatMessageTime(createdAt) };
  }

  const label = assistantLabel || "Assistant";
  return { avatar: shortMark(label), label, time: formatMessageTime(createdAt) };
}

function stackClassName(role: string, previousRole?: string, nextRole?: string, extra?: string) {
  const classes = ["message-stack", role === "assistant" ? "assistant" : role === "user" ? "user" : role];

  if (previousRole === role) {
    classes.push("continued-from-previous");
  }
  if (nextRole === role) {
    classes.push("continued-to-next");
  }
  if (role === "tool" && previousRole === "assistant") {
    classes.push("after-assistant");
  }
  if (role === "assistant" && nextRole === "tool") {
    classes.push("before-tool");
  }
  if (extra) {
    classes.push(extra);
  }

  return classes.join(" ");
}

function actionIcon(name: string) {
  switch (name) {
    case "copy":
      return (
        <IconGlyph>
          <rect x="7" y="5" width="8" height="10" rx="2" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path d="M5 13V7a2 2 0 0 1 2-2" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "retry":
      return (
        <IconGlyph>
          <path d="M14.5 7.5V4.8h-2.7" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M14.2 5.1A5.5 5.5 0 1 0 15.5 10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "assistants":
      return (
        <IconGlyph>
          <circle cx="7" cy="8" r="2" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <circle cx="13" cy="8" r="2" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path d="M4.8 14c.5-1.5 1.8-2.4 3.2-2.4h4c1.4 0 2.7.9 3.2 2.4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "topics":
      return (
        <IconGlyph>
          <path d="M5 5.5h10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <path d="M5 10h10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <path d="M5 14.5h6.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "rename":
      return (
        <IconGlyph>
          <path d="m5 13.8 1.2-3.6 6.8-6.8 2.6 2.6-6.8 6.8L5 13.8Z" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
          <path d="M11.8 4.5 14.5 7.2" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "delete":
      return (
        <IconGlyph>
          <path d="M6.5 6.5h7l-.6 8.2a1.5 1.5 0 0 1-1.5 1.3H8.6a1.5 1.5 0 0 1-1.5-1.3L6.5 6.5Z" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
          <path d="M5.5 5h9" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <path d="M8 5V4a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v1" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "tools":
      return (
        <IconGlyph>
          <circle cx="10" cy="10" r="2.2" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path
            d="M10 3.8v1.6M10 14.6v1.6M16.2 10h-1.6M5.4 10H3.8M14.4 5.6l-1.1 1.1M6.7 13.3l-1.1 1.1M14.4 14.4l-1.1-1.1M6.7 6.7 5.6 5.6"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
          />
        </IconGlyph>
      );
    case "plus":
      return (
        <IconGlyph>
          <path d="M10 4.5v11M4.5 10h11" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "slash":
      return (
        <IconGlyph>
          <path d="M13.5 4.5 6.5 15.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "mention":
      return (
        <IconGlyph>
          <circle cx="10" cy="10" r="5.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path d="M12.8 12.5V8.7a2.4 2.4 0 1 0-4.8 0v2.6a1.8 1.8 0 1 0 3.6 0V9.7" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "sliders":
      return (
        <IconGlyph>
          <path d="M5 6.5h10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <circle cx="8" cy="6.5" r="1.6" fill="currentColor" />
          <path d="M5 10h10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <circle cx="12" cy="10" r="1.6" fill="currentColor" />
          <path d="M5 13.5h10" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
          <circle cx="10" cy="13.5" r="1.6" fill="currentColor" />
        </IconGlyph>
      );
    case "search":
      return (
        <IconGlyph>
          <circle cx="9" cy="9" r="4.5" fill="none" stroke="currentColor" strokeWidth="1.5" />
          <path d="M12.5 12.5L16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    case "close":
      return (
        <IconGlyph>
          <path d="M5 5L15 15M15 5L5 15" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </IconGlyph>
      );
    default:
      return null;
  }
}

function thinkingStatus(text: string): { label: string; detail?: string } | null {
  const match = text.match(/Provider used reasoning internally \((\d+) reasoning tokens\)/i);
  if (!match) {
    return null;
  }

  return {
    label: `Reasoning used ${match[1]} tokens`,
    detail: "Provider did not expose a readable summary.",
  };
}

export function ChatWorkspace({
  session,
  provider,
  draft,
  busy,
  notice,
  reasoningEffort,
  reasoningSummary,
  textVerbosity,
  thinkingText,
  streamPhase,
  assistantsOpen,
  topicsOpen,
  attachments,
  onDraftChange,
  onAttachmentsChange,
  onClear,
  onExport,
  onUpdateModelControls,
  onSend,
  onRetry,
  onOpenSettings,
  onRename,
  onDelete,
  onToggleAssistants,
  onToggleTopics,
}: Props) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const thinkingViewportRef = useRef<HTMLDivElement | null>(null);
  const responseControlsRef = useRef<HTMLDivElement | null>(null);
  const [copiedMessageKey, setCopiedMessageKey] = useState("");
  const [responseControlsOpen, setResponseControlsOpen] = useState(false);
  const [isDragOver, setIsDragOver] = useState(false);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  const commandContext: CommandContext = {
    sessionKey: session?.summary.key ?? "",
    draft,
    onClear,
    onExport,
    onSettings: onOpenSettings,
  };

  const {
    commands,
    paletteOpen,
    commandQuery,
    handleCommandSelect,
    handleCommandClose,
    handleSlashInput,
    closePalette,
  } = useSlashCommands({
    context: commandContext,
    onCommandExecuted: () => onDraftChange(""),
  });

  const assistantLabel = provider?.name || "Assistant";
  const assistantModel = provider?.model || "No model";
  const messageCount = session?.summary.messageCount ?? session?.messages.length ?? 0;
  const providerStatus = provider ? (provider.configured ? "Configured" : "Setup required") : "No provider";
  const providerTone = provider?.configured ? "ok" : "warning";
  const hasMessages = Boolean(session && session.messages.length > 0);
  const topicTitle = session?.summary.title ?? "No topic selected";
  const topicTimestamp = session?.summary.updatedAt ? formatHeaderTime(session.summary.updatedAt) : "";
  const noticeLabel = summarizeNotice(notice);

  // Search functionality
  const searchLower = searchQuery.toLowerCase().trim();
  const filteredMessages = React.useMemo(() => {
    if (!session || !searchLower) {
      return session?.messages ?? [];
    }
    return session.messages.filter(
      (message) =>
        message.content.toLowerCase().includes(searchLower) ||
        (message.thinking && message.thinking.toLowerCase().includes(searchLower)),
    );
  }, [session, searchLower]);

  const searchMatchCount = searchLower ? filteredMessages.length : 0;

  const highlightText = (text: string): React.ReactNode => {
    if (!searchLower || !text.toLowerCase().includes(searchLower)) {
      return text;
    }
    const parts: React.ReactNode[] = [];
    let remaining = text;
    let keyIndex = 0;
    while (remaining.length > 0) {
      const index = remaining.toLowerCase().indexOf(searchLower);
      if (index === -1) {
        parts.push(remaining);
        break;
      }
      if (index > 0) {
        parts.push(remaining.slice(0, index));
      }
      parts.push(
        <mark key={`highlight-${keyIndex++}`} className="search-highlight">
          {remaining.slice(index, index + searchLower.length)}
        </mark>,
      );
      remaining = remaining.slice(index + searchLower.length);
    }
    return parts;
  };
  const supportsResponseControls = provider?.kind === "openai-responses" || provider?.kind === "openai";
  const responseControlsSummary = [reasoningEffort, reasoningSummary, textVerbosity].join(" / ");
  const responseControlRows = [
    {
      label: "Reasoning",
      value: reasoningEffort,
      options: ["low", "medium", "high"],
      onSelect: (value: string) => {
        onUpdateModelControls({ reasoningEffort: value });
        setResponseControlsOpen(false);
      },
    },
    {
      label: "Summary",
      value: reasoningSummary,
      options: ["off", "auto", "concise", "detailed"],
      onSelect: (value: string) => {
        onUpdateModelControls({ reasoningSummary: value });
        setResponseControlsOpen(false);
      },
    },
    {
      label: "Verbosity",
      value: textVerbosity,
      options: ["low", "medium", "high"],
      onSelect: (value: string) => {
        onUpdateModelControls({ textVerbosity: value });
        setResponseControlsOpen(false);
      },
    },
  ] as const;

  const renderEmptyState = (title: string, detail: string) => (
    <div className="empty-state chat-empty-state">
      <span className="eyebrow">Workspace</span>
      <strong>{title}</strong>
      <p>{detail}</p>
      {provider && <span className="empty-state-pill">{`${provider.name} / ${provider.model}`}</span>}
    </div>
  );

  const handleComposerKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      onSend();
    }
  };

  const handleCopyMessage = async (key: string, value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopiedMessageKey(key);
      window.setTimeout(() => {
        setCopiedMessageKey((current) => (current === key ? "" : current));
      }, 1200);
    } catch {
      setCopiedMessageKey("");
    }
  };

  const handleFileSelect = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = event.target.files;
    if (!files || files.length === 0) {
      return;
    }

    const isTextFile = (file: File): boolean => {
      if (file.type.startsWith("text/")) return true;
      const textExtensions = [".md", ".txt", ".json", ".yaml", ".yml", ".xml", ".csv", ".log", ".ini", ".cfg", ".conf", ".sh", ".bash", ".zsh", ".ps1", ".bat", ".cmd"];
      const name = file.name.toLowerCase();
      return textExtensions.some((ext) => name.endsWith(ext));
    };

    const processFile = (file: File): Promise<FileAttachment> => {
      return new Promise((resolve) => {
        const reader = new FileReader();
        reader.onerror = () => {
          resolve({
            id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
            name: file.name,
            type: file.type || "application/octet-stream",
            size: file.size,
          });
        };

        if (file.type.startsWith("image/")) {
          reader.onload = () => {
            resolve({
              id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
              name: file.name,
              type: file.type,
              size: file.size,
              preview: reader.result as string,
            });
          };
          reader.readAsDataURL(file);
        } else if (isTextFile(file)) {
          reader.onload = () => {
            resolve({
              id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
              name: file.name,
              type: file.type || "text/plain",
              size: file.size,
              content: reader.result as string,
            });
          };
          reader.readAsText(file);
        } else {
          resolve({
            id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
            name: file.name,
            type: file.type || "application/octet-stream",
            size: file.size,
          });
        }
      });
    };

    const newAttachments = await Promise.all(Array.from(files).map(processFile));
    onAttachmentsChange([...attachments, ...newAttachments]);

    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleRemoveAttachment = (id: string) => {
    onAttachmentsChange(attachments.filter((a) => a.id !== id));
  };

  const handlePaste = async (event: React.ClipboardEvent) => {
    const items = event.clipboardData.items;
    const imageItems = Array.from(items).filter((item) => item.type.startsWith("image/"));

    if (imageItems.length === 0) {
      return;
    }

    event.preventDefault();
    const newAttachments: FileAttachment[] = [];

    for (const item of imageItems) {
      const file = item.getAsFile();
      if (!file) {
        continue;
      }

      const reader = new FileReader();
      reader.onload = () => {
        const attachment: FileAttachment = {
          id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
          name: file.name || `image-${Date.now()}.png`,
          type: file.type,
          size: file.size,
          preview: reader.result as string,
        };
        newAttachments.push(attachment);
        if (newAttachments.length === imageItems.length) {
          onAttachmentsChange([...attachments, ...newAttachments]);
        }
      };
      reader.readAsDataURL(file);
    }
  };

  const handleDragOver = (event: React.DragEvent) => {
    event.preventDefault();
    event.stopPropagation();
    if (event.dataTransfer.types.includes("Files")) {
      setIsDragOver(true);
    }
  };

  const handleDragLeave = (event: React.DragEvent) => {
    event.preventDefault();
    event.stopPropagation();
    if (event.currentTarget.contains(event.relatedTarget as Node)) {
      return;
    }
    setIsDragOver(false);
  };

  const handleDrop = async (event: React.DragEvent) => {
    event.preventDefault();
    event.stopPropagation();
    setIsDragOver(false);

    const files = Array.from(event.dataTransfer.files);
    console.log("Dropped files:", files.map((f) => ({ name: f.name, type: f.type, size: f.size })));

    if (files.length === 0) {
      return;
    }

    // Accept all files, filter out empty ones
    const validFiles = files.filter((file) => file.size > 0);

    if (validFiles.length === 0) {
      return;
    }

    // Process all files and wait for completion
    const processFile = (file: File): Promise<FileAttachment> => {
      return new Promise((resolve) => {
        if (file.type.startsWith("image/")) {
          const reader = new FileReader();
          reader.onload = () => {
            resolve({
              id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
              name: file.name,
              type: file.type,
              size: file.size,
              preview: reader.result as string,
            });
          };
          reader.onerror = () => {
            resolve({
              id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
              name: file.name,
              type: file.type,
              size: file.size,
              preview: undefined,
            });
          };
          reader.readAsDataURL(file);
        } else if (
          file.type.startsWith("text/") ||
          file.type === "application/json" ||
          file.name.toLowerCase().endsWith(".md") ||
          file.name.toLowerCase().endsWith(".txt") ||
          file.name.toLowerCase().endsWith(".json") ||
          file.name.toLowerCase().endsWith(".yaml") ||
          file.name.toLowerCase().endsWith(".yml") ||
          file.name.toLowerCase().endsWith(".xml") ||
          file.name.toLowerCase().endsWith(".csv") ||
          file.name.toLowerCase().endsWith(".log") ||
          file.name.toLowerCase().endsWith(".ini") ||
          file.name.toLowerCase().endsWith(".cfg") ||
          file.name.toLowerCase().endsWith(".sh") ||
          file.name.toLowerCase().endsWith(".bash")
        ) {
          // Read text file content
          const reader = new FileReader();
          reader.onload = () => {
            resolve({
              id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
              name: file.name,
              type: file.type || "text/plain",
              size: file.size,
              preview: undefined,
              content: reader.result as string,
            });
          };
          reader.onerror = () => {
            resolve({
              id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
              name: file.name,
              type: file.type || "application/octet-stream",
              size: file.size,
              preview: undefined,
            });
          };
          reader.readAsText(file);
        } else {
          resolve({
            id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
            name: file.name,
            type: file.type || "application/octet-stream",
            size: file.size,
            preview: undefined,
          });
        }
      });
    };

    const newAttachments = await Promise.all(validFiles.map(processFile));
    console.log("New attachments:", newAttachments);
    onAttachmentsChange([...attachments, ...newAttachments]);
  };

  const toggleGroupCollapse = (groupKey: string) => {
    setCollapsedGroups((previous) => {
      const next = new Set(previous);
      if (next.has(groupKey)) {
        next.delete(groupKey);
      } else {
        next.add(groupKey);
      }
      return next;
    });
  };

  const renderGroupHeader = (group: MessageGroup) => {
    const isCollapsed = collapsedGroups.has(group.key);
    return (
      <button
        className={`message-group-header ${isCollapsed ? "collapsed" : ""}`}
        key={`group-${group.key}`}
        onClick={() => toggleGroupCollapse(group.key)}
        type="button">
        <span className="message-group-toggle" aria-hidden="true">
          {isCollapsed ? "▶" : "▼"}
        </span>
        <span className="message-group-label">{group.label}</span>
        <span className="message-group-count">{group.messages.length} messages</span>
      </button>
    );
  };

  const renderMessageActions = (key: string, role: string, content: string, contextLabel?: string) => (
    <div className={`message-actions ${role === "tool" ? "tool" : role === "user" ? "user" : "assistant"}`}>
      {contextLabel && <span className="message-action-context">{contextLabel}</span>}
      <button
        aria-label={copiedMessageKey === key ? "Copied" : "Copy"}
        className="message-action-button icon-only"
        onClick={() => void handleCopyMessage(key, content)}
        title={copiedMessageKey === key ? "Copied" : "Copy"}
        type="button">
        {actionIcon("copy")}
      </button>
      {role === "assistant" && !busy && (
        <button
          aria-label="Retry"
          className="message-action-button icon-only ghosted"
          onClick={onRetry}
          title="Retry"
          type="button">
          {actionIcon("retry")}
        </button>
      )}
    </div>
  );

  useEffect(() => {
    const viewport = thinkingViewportRef.current;
    if (!viewport) {
      return;
    }

    viewport.scrollTo({
      top: viewport.scrollHeight,
      behavior: thinkingText.length > 12 ? "smooth" : "auto",
    });
  }, [thinkingText, streamPhase]);

  useEffect(() => {
    if (!supportsResponseControls) {
      setResponseControlsOpen(false);
    }
  }, [supportsResponseControls]);

  useEffect(() => {
    if (!responseControlsOpen) {
      return;
    }

    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node | null;
      if (!responseControlsRef.current || !target) {
        return;
      }
      if (!responseControlsRef.current.contains(target)) {
        setResponseControlsOpen(false);
      }
    };

    document.addEventListener("mousedown", handlePointerDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
    };
  }, [responseControlsOpen]);

  const streamingAssistantIndex =
    busy && session
      ? session.messages.reduce((lastIndex, message, index) => (message.role === "assistant" ? index : lastIndex), -1)
      : -1;

  const renderStreamingAssistant = (message: SessionMessage, index: number) => {
    const messageKey = `${message.createdAt}-${index}`;
    const hasThinking = thinkingText.trim() !== "";
    const hasReply = message.content.trim() !== "";
    const previousRole = index > 0 ? session?.messages[index - 1]?.role : undefined;
    const statusLabel = streamPhase === "replying" ? "Responding" : "Thinking";
    const statusNote =
      streamPhase === "replying"
        ? "Composing the final answer"
        : hasThinking
          ? "Reasoning in progress"
          : "Preparing a response";
    const chrome = messageChrome("assistant", message.createdAt, undefined, assistantLabel);

    return (
      <article
        key={messageKey}
        className={stackClassName("assistant", previousRole, undefined, "streaming-stack")}>
        <div className="message-meta">
          <div className="message-author">
            <span className="message-avatar">{chrome.avatar}</span>
            <div className="message-heading">
              <strong>{chrome.label}</strong>
              <span>{chrome.time}</span>
            </div>
          </div>
        </div>

        <div className="message-content-shell assistant">
          <div className="bubble assistant assistant-streaming">
            <div className="streaming-head">
              <div className="streaming-badge">
                <span className="status-dot" aria-hidden="true" />
                <strong>{statusLabel}</strong>
              </div>
              <span className="streaming-note">{statusNote}</span>
            </div>

            <section className={`thinking-panel ${hasReply ? "compact" : ""}`}>
              <div className="message-block-head">
                <span className="bubble-role">thinking</span>
                <span className="message-block-note">{hasThinking ? "Live reasoning trace" : "Preparing reasoning"}</span>
              </div>
              {hasThinking ? (
                <div className="thinking-viewport" ref={thinkingViewportRef}>
                  <p className="thinking-content">
                    {thinkingText}
                    {streamPhase === "thinking" && <span className="stream-caret" aria-hidden="true" />}
                  </p>
                </div>
              ) : (
                <div className="thinking-skeleton" aria-hidden="true">
                  <span className="skeleton-line short" />
                  <span className="skeleton-line medium" />
                  <span className="skeleton-line long" />
                </div>
              )}
            </section>

            {hasReply && streamPhase === "replying" && (
              <section className="stream-answer">
                <div className="message-block-head">
                  <span className="bubble-role">assistant</span>
                  <span className="message-block-note">Draft answer</span>
                </div>
                <div className="stream-answer-body">
                  <MarkdownContent content={message.content} />
                  <span className="stream-caret markdown-caret" aria-hidden="true" />
                </div>
              </section>
            )}
          </div>
          {renderMessageActions(
            messageKey,
            "assistant",
            [thinkingText, message.content].filter(Boolean).join("\n\n"),
            assistantModel,
          )}
        </div>
      </article>
    );
  };

  const renderMessage = (message: SessionMessage, index: number) => {
    if (index === streamingAssistantIndex) {
      return renderStreamingAssistant(message, index);
    }

    const previousRole = index > 0 ? session?.messages[index - 1]?.role : undefined;
    const nextRole = index < (session?.messages.length ?? 0) - 1 ? session?.messages[index + 1]?.role : undefined;

    if (message.role === "tool") {
      const messageKey = `${message.createdAt}-${index}`;
      const chrome = messageChrome("tool", message.createdAt, message.name, assistantLabel);
      return (
        <article className={stackClassName("tool", previousRole, nextRole)} key={messageKey}>
          <div className="message-meta">
            <div className="message-author">
              <span className="message-avatar">{chrome.avatar}</span>
              <div className="message-heading">
                <strong>{chrome.label}</strong>
                <span>{chrome.time}</span>
              </div>
            </div>
          </div>
          <div className="message-content-shell tool">
            <details className="tool-result">
              <summary className="tool-result-summary">
                <div className="tool-result-title">
                  <span className="bubble-role">Tool Result</span>
                  <strong>{message.name || "Tool execution"}</strong>
                </div>
                <div className="tool-result-summary-side">
                  <span className="tool-result-hint">Click to expand</span>
                  <span className="tool-result-summary-glyph" aria-hidden="true">
                    ...
                  </span>
                </div>
              </summary>
              <pre className="tool-result-body">{message.content}</pre>
            </details>
            {renderMessageActions(messageKey, "tool", message.content, message.name || "Tool")}
          </div>
        </article>
      );
    }

    const messageKey = `${message.createdAt}-${index}`;
    const chrome = messageChrome(message.role, message.createdAt, undefined, assistantLabel);
    const hasPersistedThinking = message.role === "assistant" && (message.thinking ?? "").trim() !== "";
    const persistedThinkingStatus = hasPersistedThinking ? thinkingStatus(message.thinking ?? "") : null;
    return (
      <article
        key={messageKey}
        className={stackClassName(message.role, previousRole, nextRole)}>
        <div className="message-meta">
          <div className="message-author">
            <span className="message-avatar">{chrome.avatar}</span>
            <div className="message-heading">
              <strong>{chrome.label}</strong>
              <span>{chrome.time}</span>
            </div>
          </div>
        </div>
        <div className={`message-content-shell ${message.role}`}>
          {hasPersistedThinking ? (
            <div className="assistant-response-stack">
              {persistedThinkingStatus ? (
                <div
                  aria-label={`${persistedThinkingStatus.label}. ${persistedThinkingStatus.detail ?? ""}`.trim()}
                  className="thinking-status-badge persisted-thinking"
                  data-tooltip={persistedThinkingStatus.detail ?? "Summary unavailable"}
                  role="note"
                  tabIndex={0}>
                  <span className="thinking-status-dot" aria-hidden="true" />
                  <span>{persistedThinkingStatus.label}</span>
                </div>
              ) : (
                <section className="thinking-panel persisted-thinking">
                  <div className="message-block-head">
                    <span className="bubble-role">thinking</span>
                    <span className="message-block-note">Reasoning summary</span>
                  </div>
                  <div className="thinking-viewport static">
                    <p className="thinking-content">{message.thinking}</p>
                  </div>
                </section>
              )}
              <div className="bubble assistant plain">
                <MarkdownContent content={message.content} />
              </div>
            </div>
          ) : (
            <div className={`bubble ${message.role === "assistant" ? "assistant plain" : "user"}`}>
              <MarkdownContent content={message.content} />
            </div>
          )}
          {renderMessageActions(messageKey, message.role, message.content, message.role === "assistant" ? assistantModel : undefined)}
        </div>
      </article>
    );
  };

  return (
    <main
      className={`chat-pane ${isDragOver ? "drag-over" : ""}`}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}>
      {isDragOver && (
        <div className="drag-overlay">
          <div className="drag-overlay-content">
            <span className="drag-icon">+</span>
            <span className="drag-text">Drop files here</span>
          </div>
        </div>
      )}
      <header className="pane-header chat-header">
        <div className="chat-header-main cherry-navbar">
          <div className="chat-navbar-main">
            <div className="chat-provider-chip">
              <span className="message-avatar chat-provider-avatar">{shortMark(assistantLabel)}</span>
              <div className="chat-provider-copy">
                <span className="eyebrow">Assistant</span>
                <div className="chat-provider-line">
                  <strong>{assistantLabel}</strong>
                  {provider && <span className="chat-provider-separator" aria-hidden="true" />}
                  {provider && <span className="context-pill strong model-pill">{assistantModel}</span>}
                </div>
              </div>
            </div>
            <div className="chat-topic-summary">
              <strong>{topicTitle}</strong>
              <span>
                {session ? `${messageCount} messages in this topic` : "Choose an assistant and create a topic to start chatting."}
              </span>
            </div>
          </div>
          <div className="chat-context-rail">
            <span className={`chat-context-chip state ${providerTone}`}>{providerStatus}</span>
            {topicTimestamp && <span className="chat-context-chip">{topicTimestamp}</span>}
          </div>
        </div>

        <div className="header-actions cherry-navbar-tools">
          <div className="header-action-cluster">
            <button
              className="ghost compact nav-button icon-nav-button"
              onClick={onToggleAssistants}
              title="Toggle assistants"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("assistants")}</span>
              <span className="nav-button-label">{assistantsOpen ? "Assistants" : "Show Assistants"}</span>
            </button>
            <button
              className="ghost compact nav-button icon-nav-button"
              onClick={onToggleTopics}
              title="Toggle topics"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("topics")}</span>
              <span className="nav-button-label">{topicsOpen ? "Topics" : "Show Topics"}</span>
            </button>
          </div>

          <div className="header-action-cluster">
            <button
              className={`ghost compact nav-button icon-nav-button ${searchOpen ? "active" : ""}`}
              onClick={() => {
                setSearchOpen((current) => !current);
                if (searchOpen) {
                  setSearchQuery("");
                }
              }}
              title="Search in topic"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("search")}</span>
              <span className="nav-button-label">{searchOpen ? "Close" : "Search"}</span>
            </button>
            <button
              className="ghost compact nav-button icon-nav-button"
              onClick={onRename}
              title="Rename topic"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("rename")}</span>
              <span className="nav-button-label">Rename</span>
            </button>
            <button
              className="ghost compact danger nav-button icon-nav-button"
              onClick={onDelete}
              title="Delete topic"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("delete")}</span>
              <span className="nav-button-label">Delete</span>
            </button>
          </div>

          <div className="header-action-cluster header-model-tools" ref={responseControlsRef}>
            <button
              className="action compact nav-button icon-nav-button"
              onClick={onOpenSettings}
              title="Open settings"
              type="button">
              <span className="nav-button-glyph icon">{actionIcon("tools")}</span>
              <span className="nav-button-label">Tools</span>
            </button>
            {supportsResponseControls && (
              <button
                aria-expanded={responseControlsOpen}
                aria-haspopup="dialog"
                aria-label="Model controls"
                className={`ghost compact nav-button icon-nav-button model-tools-trigger ${responseControlsOpen ? "active" : ""}`}
                onClick={() => setResponseControlsOpen((current) => !current)}
                title={`Model controls: ${responseControlsSummary}`}
                type="button">
                <span className="nav-button-glyph icon">{actionIcon("sliders")}</span>
                <span className="nav-button-label">Model Tools</span>
              </button>
            )}

            {supportsResponseControls && responseControlsOpen && (
              <div className="model-tools-popover">
                <div className="model-tools-popover-head">
                  <div className="model-tools-heading">
                    <strong>Model Controls</strong>
                    <span>{assistantModel}</span>
                  </div>
                  <span className="model-tools-summary">{responseControlsSummary}</span>
                </div>
                {responseControlRows.map((row) => (
                  <div className="model-tools-row" key={row.label}>
                    <span>{row.label}</span>
                    <div className="model-tools-options">
                      {row.options.map((option) => (
                        <button
                          className={`model-tools-option ${row.value === option ? "active" : ""}`}
                          key={option}
                          onClick={() => row.onSelect(option)}
                          type="button">
                          {option}
                        </button>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {searchOpen && (
          <div className="search-bar">
            <div className="search-input-wrapper">
              <span className="search-icon">{actionIcon("search")}</span>
              <input
                className="search-input"
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="Search in conversation..."
                type="text"
                value={searchQuery}
              />
              {searchQuery && (
                <button
                  className="search-clear"
                  onClick={() => setSearchQuery("")}
                  type="button">
                  {actionIcon("close")}
                </button>
              )}
            </div>
            {searchQuery && (
              <span className="search-count">
                {searchMatchCount === 0 ? "No matches" : `${searchMatchCount} match${searchMatchCount !== 1 ? "es" : ""}`}
              </span>
            )}
          </div>
        )}
      </header>

      <div className="chat-stage">
        <section className={`chat-scroll ${hasMessages ? "populated" : "empty"}`}>
          {!session &&
            renderEmptyState("No topic selected", "Choose an assistant and create a topic to start chatting.")}
          {session &&
            session.messages.length > 0 &&
            (searchLower ? (
              // Search mode: show filtered messages without grouping
              filteredMessages.length > 0 ? (
                filteredMessages.map((message, index) => (
                  <div key={`search-${index}`} className="message-stack message-role-${message.role} search-result">
                    {renderMessage(message, session.messages.indexOf(message))}
                  </div>
                ))
              ) : (
                <div className="search-no-results">
                  <p>No messages match your search.</p>
                </div>
              )
            ) : (
              // Normal mode: show grouped messages
              groupMessagesByDate(session.messages).map((group) => (
                <div key={group.key} className="message-group">
                  {renderGroupHeader(group)}
                  {!collapsedGroups.has(group.key) && group.messages.map(({ message, index }) => renderMessage(message, index))}
                </div>
              ))
            ))}
          {session &&
            session.messages.length === 0 &&
            renderEmptyState("No messages yet", "Send a prompt to start this topic.")}
        </section>

        <footer className="composer-shell">
          <div className="composer-surface">
            <div className="composer-toolbar">
              <div className="composer-toolbar-group">
                <button className="ghost toolbar-chip active" onClick={onOpenSettings} type="button">
                  {assistantLabel}
                </button>
                <button
                  aria-label="Attach file"
                  className="ghost toolbar-chip icon-chip"
                  onClick={handleFileSelect}
                  title="Attach file"
                  type="button">
                  {actionIcon("plus")}
                </button>
                <button aria-label="Commands" className="ghost toolbar-chip icon-chip" title="Commands" type="button">
                  {actionIcon("slash")}
                </button>
                <button aria-label="Mention" className="ghost toolbar-chip icon-chip" title="Mention" type="button">
                  {actionIcon("mention")}
                </button>
                <input
                  accept="image/*,.pdf,.txt,.md,.json,.csv"
                  multiple
                  onChange={handleFileChange}
                  ref={fileInputRef}
                  style={{ display: "none" }}
                  type="file"
                />
              </div>
              <div className="composer-toolbar-meta">
                {provider && <span className="composer-meta-pill">{assistantModel}</span>}
                <span className={`composer-meta-pill state ${providerTone}`}>{busy ? "Responding" : providerStatus}</span>
              </div>
            </div>

            {attachments.length > 0 && (
              <AttachmentList attachments={attachments} onRemove={handleRemoveAttachment} />
            )}

            <div className={`chat-notice ${busy ? "active" : ""}`} title={notice}>
              {noticeLabel}
            </div>

            <textarea
              id="composer"
              onChange={(event) => onDraftChange(event.target.value)}
              onKeyDown={handleComposerKeyDown}
              onPaste={handlePaste}
              placeholder="Message this topic..."
              rows={4}
              value={draft}
            />

            <div className="composer-footer">
              <div className="composer-state">
                <span className={`composer-state-dot ${busy ? "busy" : "idle"}`} aria-hidden="true" />
                <span>{busy ? "Streaming response" : "Enter to send, Shift+Enter for newline"}</span>
              </div>
              <button className="action primary composer-send" disabled={busy} onClick={onSend} type="button">
                <span>Send</span>
                <span className="composer-send-arrow" aria-hidden="true">
                  -&gt;
                </span>
              </button>
            </div>
          </div>
        </footer>
      </div>
    </main>
  );
}

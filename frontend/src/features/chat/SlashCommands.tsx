import React, { useEffect, useMemo, useRef, useState } from "react";

export type CommandContext = {
  sessionKey: string;
  draft: string;
  onClear: () => void;
  onExport: () => void;
  onSettings: () => void;
};

export type SlashCommand = {
  name: string;
  description: string;
  handler: (args: string, context: CommandContext) => void | Promise<void>;
};

type Props = {
  commands: SlashCommand[];
  context: CommandContext;
  query: string;
  onSelect: (command: SlashCommand, args: string) => void;
  onClose: () => void;
  visible: boolean;
};

export function CommandPalette({ commands, context, query, onSelect, onClose, visible }: Props) {
  const listRef = useRef<HTMLDivElement | null>(null);
  const [selectedIndex, setSelectedIndex] = useState(0);

  const filteredCommands = useMemo(() => {
    if (!query) {
      return commands;
    }
    const normalized = query.toLowerCase();
    return commands.filter(
      (cmd) =>
        cmd.name.toLowerCase().startsWith(normalized) ||
        cmd.description.toLowerCase().includes(normalized),
    );
  }, [commands, query]);

  useEffect(() => {
    setSelectedIndex(0);
  }, [query]);

  useEffect(() => {
    if (!visible) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        setSelectedIndex((current) => Math.min(current + 1, filteredCommands.length - 1));
      } else if (event.key === "ArrowUp") {
        event.preventDefault();
        setSelectedIndex((current) => Math.max(current - 1, 0));
      } else if (event.key === "Enter" && filteredCommands[selectedIndex]) {
        event.preventDefault();
        onSelect(filteredCommands[selectedIndex], "");
      } else if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [filteredCommands, selectedIndex, onSelect, onClose, visible]);

  useEffect(() => {
    if (!listRef.current || !visible) {
      return;
    }

    const selectedElement = listRef.current.querySelector(`[data-index="${selectedIndex}"]`);
    if (selectedElement) {
      selectedElement.scrollIntoView({ block: "nearest" });
    }
  }, [selectedIndex, visible]);

  if (!visible || filteredCommands.length === 0) {
    return null;
  }

  return (
    <div className="command-palette">
      <div className="command-palette-head">
        <span className="command-palette-title">Commands</span>
        <span className="command-palette-hint">Enter to select, Esc to close</span>
      </div>
      <div className="command-list" ref={listRef}>
        {filteredCommands.map((command, index) => (
          <button
            className={`command-item ${index === selectedIndex ? "selected" : ""}`}
            data-index={index}
            key={command.name}
            onClick={() => onSelect(command, "")}
            type="button">
            <span className="command-name">{command.name}</span>
            <span className="command-description">{command.description}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

type UseSlashCommandsOptions = {
  context: CommandContext;
  onCommandExecuted?: () => void;
};

export function useSlashCommands({ context, onCommandExecuted }: UseSlashCommandsOptions) {
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [commandQuery, setCommandQuery] = useState("");

  const commands: SlashCommand[] = useMemo(
    () => [
      {
        name: "/clear",
        description: "Clear the current conversation",
        handler: (_args: string, ctx: CommandContext) => {
          ctx.onClear();
          onCommandExecuted?.();
        },
      },
      {
        name: "/export",
        description: "Export conversation to file",
        handler: async (_args: string, ctx: CommandContext) => {
          ctx.onExport();
          onCommandExecuted?.();
        },
      },
      {
        name: "/settings",
        description: "Open settings panel",
        handler: (_args: string, ctx: CommandContext) => {
          ctx.onSettings();
          onCommandExecuted?.();
        },
      },
    ],
    [onCommandExecuted],
  );

  const handleCommandSelect = (command: SlashCommand, args: string) => {
    command.handler(args, context);
    setPaletteOpen(false);
    setCommandQuery("");
  };

  const handleCommandClose = () => {
    setPaletteOpen(false);
    setCommandQuery("");
  };

  const handleSlashInput = (value: string) => {
    if (value.startsWith("/")) {
      const match = value.match(/^\/(\w*)/);
      if (match) {
        setCommandQuery(match[1]);
        setPaletteOpen(true);
      }
    } else {
      setPaletteOpen(false);
    }
  };

  return {
    commands,
    paletteOpen,
    commandQuery,
    handleCommandSelect,
    handleCommandClose,
    handleSlashInput,
    closePalette: () => {
      setPaletteOpen(false);
      setCommandQuery("");
    },
  };
}

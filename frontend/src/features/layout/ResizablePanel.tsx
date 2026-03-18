import React, { useCallback, useEffect, useRef, useState } from "react";

type Props = {
  children: React.ReactNode;
  defaultWidth: number;
  minWidth?: number;
  maxWidth?: number;
  storageKey: string;
  side?: "left" | "right";
  onResize?: (width: number) => void;
};

export function ResizablePanel({
  children,
  defaultWidth,
  minWidth = 180,
  maxWidth = 500,
  storageKey,
  side = "left",
  onResize,
}: Props) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  const [isResizing, setIsResizing] = useState(false);
  const [width, setWidth] = useState(() => {
    if (typeof window === "undefined") {
      return defaultWidth;
    }
    const stored = localStorage.getItem(storageKey);
    if (stored) {
      const parsed = parseInt(stored, 10);
      if (!Number.isNaN(parsed) && parsed >= minWidth && parsed <= maxWidth) {
        return parsed;
      }
    }
    return defaultWidth;
  });

  const handleMouseDown = useCallback((event: React.MouseEvent) => {
    event.preventDefault();
    setIsResizing(true);
  }, []);

  useEffect(() => {
    if (!isResizing) {
      return;
    }

    const handleMouseMove = (event: MouseEvent) => {
      if (!panelRef.current) {
        return;
      }

      const panel = panelRef.current;
      const rect = panel.getBoundingClientRect();
      let newWidth: number;

      if (side === "left") {
        newWidth = event.clientX - rect.left;
      } else {
        newWidth = rect.right - event.clientX;
      }

      const clampedWidth = Math.max(minWidth, Math.min(maxWidth, newWidth));
      setWidth(clampedWidth);
      localStorage.setItem(storageKey, String(clampedWidth));
      onResize?.(clampedWidth);
    };

    const handleMouseUp = () => {
      setIsResizing(false);
    };

    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
    document.body.style.cursor = isResizing ? "col-resize" : "";
    document.body.style.userSelect = isResizing ? "none" : "";

    return () => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
  }, [isResizing, minWidth, maxWidth, storageKey, side, onResize]);

  return (
    <div
      className={`resizable-panel ${isResizing ? "resizing" : ""}`}
      ref={panelRef}
      style={{ width: `${width}px` }}>
      <div className="resizable-panel-content">{children}</div>
      <div
        className={`resize-handle ${side}`}
        onMouseDown={handleMouseDown}
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize panel"
        tabIndex={0}
      />
    </div>
  );
}

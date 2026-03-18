import React from "react";

import type { FileAttachment as FileAttachmentType } from "../../app/types";

type Props = {
  attachment: FileAttachmentType;
  onRemove?: (id: string) => void;
  readonly?: boolean;
};

function formatFileSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function getFileIcon(type: string): string {
  if (type.startsWith("image/")) {
    return "🖼";
  }
  if (type.startsWith("text/")) {
    return "📄";
  }
  if (type.includes("pdf")) {
    return "📕";
  }
  if (type.includes("spreadsheet") || type.includes("excel") || type.includes("csv")) {
    return "📊";
  }
  if (type.includes("zip") || type.includes("archive") || type.includes("compressed")) {
    return "📦";
  }
  if (type.includes("audio")) {
    return "🎵";
  }
  if (type.includes("video")) {
    return "🎬";
  }
  return "📎";
}

export function AttachmentPreview({ attachment, onRemove, readonly = false }: Props) {
  const isImage = attachment.type.startsWith("image/");
  const hasPreview = isImage && attachment.preview;

  return (
    <div className="attachment-preview">
      {hasPreview ? (
        <div className="attachment-image-wrapper">
          <img alt={attachment.name} className="attachment-image" src={attachment.preview} />
          {!readonly && onRemove && (
            <button
              aria-label={`Remove ${attachment.name}`}
              className="attachment-remove"
              onClick={() => onRemove(attachment.id)}
              type="button">
              ×
            </button>
          )}
        </div>
      ) : (
        <div className="attachment-file">
          <span className="attachment-icon">{getFileIcon(attachment.type)}</span>
          <div className="attachment-info">
            <span className="attachment-name">{attachment.name}</span>
            <span className="attachment-size">{formatFileSize(attachment.size)}</span>
          </div>
          {!readonly && onRemove && (
            <button
              aria-label={`Remove ${attachment.name}`}
              className="attachment-remove-file"
              onClick={() => onRemove(attachment.id)}
              type="button">
              ×
            </button>
          )}
        </div>
      )}
    </div>
  );
}

type ListProps = {
  attachments: FileAttachmentType[];
  onRemove?: (id: string) => void;
  readonly?: boolean;
};

export function AttachmentList({ attachments, onRemove, readonly = false }: ListProps) {
  if (attachments.length === 0) {
    return null;
  }

  return (
    <div className="attachment-list">
      {attachments.map((attachment) => (
        <AttachmentPreview
          attachment={attachment}
          key={attachment.id}
          onRemove={onRemove}
          readonly={readonly}
        />
      ))}
    </div>
  );
}

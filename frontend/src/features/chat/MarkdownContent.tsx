import React, { Fragment, useEffect, useState } from "react";

type Props = {
  content: string;
};

type InlineToken =
  | { type: "text"; value: string }
  | { type: "code"; value: string }
  | { type: "strong"; children: InlineToken[] }
  | { type: "em"; children: InlineToken[] }
  | { type: "link"; href: string; children: InlineToken[] }
  | { type: "image"; src: string; alt: string };

type ListItem = {
  text: string;
  checked?: boolean;
};

type Block =
  | { type: "heading"; depth: number; content: string }
  | { type: "paragraph"; lines: string[] }
  | { type: "code"; language: string; content: string }
  | { type: "hr" }
  | { type: "blockquote"; lines: string[] }
  | { type: "ul"; items: ListItem[] }
  | { type: "ol"; items: ListItem[] }
  | { type: "table"; headers: string[]; rows: string[][] };

function splitTableRow(line: string): string[] {
  return line
    .trim()
    .replace(/^\|/, "")
    .replace(/\|$/, "")
    .split("|")
    .map((cell) => cell.trim());
}

function isTableSeparator(line: string): boolean {
  const cells = splitTableRow(line);
  return cells.length > 0 && cells.every((cell) => /^:?-{3,}:?$/.test(cell));
}

function isTableStart(lines: string[], index: number): boolean {
  if (index + 1 >= lines.length) {
    return false;
  }

  const header = lines[index].trim();
  const separator = lines[index + 1].trim();
  return header.includes("|") && separator.includes("|") && isTableSeparator(separator);
}

function parseBlocks(markdown: string): Block[] {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  const blocks: Block[] = [];
  let index = 0;

  while (index < lines.length) {
    const line = lines[index];
    const trimmed = line.trim();

    if (trimmed === "") {
      index += 1;
      continue;
    }

    const fence = trimmed.match(/^```([\w-]*)\s*$/);
    if (fence) {
      const language = fence[1] ?? "";
      const codeLines: string[] = [];
      index += 1;
      while (index < lines.length && !lines[index].trim().startsWith("```")) {
        codeLines.push(lines[index]);
        index += 1;
      }
      if (index < lines.length) {
        index += 1;
      }
      blocks.push({ type: "code", language, content: codeLines.join("\n") });
      continue;
    }

    const heading = line.match(/^(#{1,6})\s+(.*)$/);
    if (heading) {
      blocks.push({ type: "heading", depth: heading[1].length, content: heading[2].trim() });
      index += 1;
      continue;
    }

    if (/^(-{3,}|\*{3,}|_{3,})$/.test(trimmed)) {
      blocks.push({ type: "hr" });
      index += 1;
      continue;
    }

    if (trimmed.startsWith(">")) {
      const quoteLines: string[] = [];
      while (index < lines.length) {
        const candidate = lines[index].trim();
        if (!candidate.startsWith(">")) {
          break;
        }
        quoteLines.push(candidate.replace(/^>\s?/, ""));
        index += 1;
      }
      blocks.push({ type: "blockquote", lines: quoteLines });
      continue;
    }

    if (isTableStart(lines, index)) {
      const headers = splitTableRow(lines[index]);
      const rows: string[][] = [];
      index += 2;
      while (index < lines.length) {
        const candidate = lines[index].trim();
        if (candidate === "" || !candidate.includes("|")) {
          break;
        }
        rows.push(splitTableRow(lines[index]));
        index += 1;
      }
      blocks.push({ type: "table", headers, rows });
      continue;
    }

    if (/^[-*]\s+/.test(trimmed)) {
      const items: ListItem[] = [];
      while (index < lines.length) {
        const candidate = lines[index].trim();
        const taskMatch = candidate.match(/^[-*]\s+\[( |x|X)\]\s+(.*)$/);
        const bulletMatch = candidate.match(/^[-*]\s+(.*)$/);
        if (taskMatch) {
          items.push({ text: taskMatch[2], checked: taskMatch[1].toLowerCase() === "x" });
          index += 1;
          continue;
        }
        if (bulletMatch) {
          items.push({ text: bulletMatch[1] });
          index += 1;
          continue;
        }
        break;
      }
      blocks.push({ type: "ul", items });
      continue;
    }

    if (/^\d+\.\s+/.test(trimmed)) {
      const items: ListItem[] = [];
      while (index < lines.length) {
        const candidate = lines[index].trim();
        const match = candidate.match(/^\d+\.\s+(.*)$/);
        if (!match) {
          break;
        }
        items.push({ text: match[1] });
        index += 1;
      }
      blocks.push({ type: "ol", items });
      continue;
    }

    const paragraphLines: string[] = [];
    while (index < lines.length) {
      const candidate = lines[index];
      const candidateTrimmed = candidate.trim();
      if (
        candidateTrimmed === "" ||
        candidateTrimmed.startsWith(">") ||
        candidateTrimmed.startsWith("```") ||
        /^#{1,6}\s+/.test(candidateTrimmed) ||
        /^[-*]\s+/.test(candidateTrimmed) ||
        /^\d+\.\s+/.test(candidateTrimmed) ||
        /^(-{3,}|\*{3,}|_{3,})$/.test(candidateTrimmed) ||
        isTableStart(lines, index)
      ) {
        break;
      }
      paragraphLines.push(candidate);
      index += 1;
    }
    blocks.push({ type: "paragraph", lines: paragraphLines });
  }

  return blocks;
}

function parseInline(text: string): InlineToken[] {
  const tokens: InlineToken[] = [];
  let remaining = text;
  const pattern =
    /(!\[([^\]]*)\]\(([^)\s]+)\)|`[^`]+`|\[([^\]]+)\]\(([^)\s]+)\)|\*\*([^*]+)\*\*|\*([^*]+)\*|_([^_]+)_)/;

  while (remaining.length > 0) {
    const match = remaining.match(pattern);
    if (!match || match.index === undefined) {
      tokens.push({ type: "text", value: remaining });
      break;
    }

    if (match.index > 0) {
      tokens.push({ type: "text", value: remaining.slice(0, match.index) });
    }

    const raw = match[0];
    if (raw.startsWith("![")) {
      tokens.push({
        type: "image",
        src: match[3],
        alt: match[2] ?? "",
      });
    } else if (raw.startsWith("`")) {
      tokens.push({ type: "code", value: raw.slice(1, -1) });
    } else if (raw.startsWith("[")) {
      tokens.push({
        type: "link",
        href: match[5],
        children: parseInline(match[4]),
      });
    } else if (raw.startsWith("**")) {
      tokens.push({ type: "strong", children: parseInline(match[6]) });
    } else {
      const emphasis = match[7] ?? match[8] ?? "";
      tokens.push({ type: "em", children: parseInline(emphasis) });
    }

    remaining = remaining.slice(match.index + raw.length);
  }

  return tokens;
}

function renderInline(tokens: InlineToken[]): React.ReactNode[] {
  return tokens.map((token, index) => {
    const key = `${token.type}-${index}`;
    switch (token.type) {
      case "text":
        return <Fragment key={key}>{token.value}</Fragment>;
      case "code":
        return (
          <code key={key} className="md-inline-code">
            {token.value}
          </code>
        );
      case "strong":
        return <strong key={key}>{renderInline(token.children)}</strong>;
      case "em":
        return <em key={key}>{renderInline(token.children)}</em>;
      case "link":
        return (
          <a key={key} href={token.href} target="_blank" rel="noreferrer">
            {renderInline(token.children)}
          </a>
        );
      case "image":
        return <img key={key} className="md-inline-image" src={token.src} alt={token.alt} />;
      default:
        return null;
    }
  });
}

function renderLines(lines: string[]): React.ReactNode {
  return lines.map((line, index) => (
    <Fragment key={`line-${index}`}>
      {renderInline(parseInline(line))}
      {index < lines.length - 1 && <br />}
    </Fragment>
  ));
}

function TableBlock({ headers, rows }: { headers: string[]; rows: string[][] }) {
  return (
    <div className="md-table-wrap">
      <table className="md-table">
        <thead>
          <tr>
            {headers.map((header, index) => (
              <th key={`header-${index}`}>{renderInline(parseInline(header))}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr key={`row-${rowIndex}`}>
              {headers.map((_, cellIndex) => (
                <td key={`row-${rowIndex}-cell-${cellIndex}`}>
                  {renderInline(parseInline(row[cellIndex] ?? ""))}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function CodeBlockView({ language, content }: { language: string; content: string }) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) {
      return;
    }

    const timer = window.setTimeout(() => setCopied(false), 1200);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const handleCopy = async () => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(content);
        setCopied(true);
      }
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="md-code-shell">
      <div className="md-code-toolbar">
        <span className="md-code-window" aria-hidden="true">
          <span />
          <span />
          <span />
        </span>
        <span className="md-code-lang">{language || "text"}</span>
        <button className="md-code-copy" onClick={handleCopy} type="button">
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
      <pre className="md-code-block">
        <code>{content}</code>
      </pre>
    </div>
  );
}

export function MarkdownContent({ content }: Props) {
  const blocks = parseBlocks(content);

  return (
    <div className="markdown-content">
      {blocks.map((block, index) => {
        const key = `${block.type}-${index}`;
        switch (block.type) {
          case "heading": {
            const Tag = `h${Math.min(block.depth, 4)}` as const;
            return <Tag key={key}>{renderInline(parseInline(block.content))}</Tag>;
          }
          case "paragraph":
            return <p key={key}>{renderLines(block.lines)}</p>;
          case "code":
            return <CodeBlockView key={key} language={block.language} content={block.content} />;
          case "hr":
            return <hr key={key} className="md-rule" />;
          case "blockquote":
            return <blockquote key={key}>{renderLines(block.lines)}</blockquote>;
          case "ul":
            return (
              <ul key={key} className={block.items.some((item) => typeof item.checked === "boolean") ? "md-task-list" : undefined}>
                {block.items.map((item, itemIndex) => (
                  <li
                    key={`${key}-${itemIndex}`}
                    className={typeof item.checked === "boolean" ? "md-task-item" : undefined}>
                    {typeof item.checked === "boolean" && (
                      <input checked={item.checked} disabled readOnly type="checkbox" />
                    )}
                    <span>{renderInline(parseInline(item.text))}</span>
                  </li>
                ))}
              </ul>
            );
          case "ol":
            return (
              <ol key={key}>
                {block.items.map((item, itemIndex) => (
                  <li key={`${key}-${itemIndex}`}>{renderInline(parseInline(item.text))}</li>
                ))}
              </ol>
            );
          case "table":
            return <TableBlock key={key} headers={block.headers} rows={block.rows} />;
          default:
            return null;
        }
      })}
    </div>
  );
}

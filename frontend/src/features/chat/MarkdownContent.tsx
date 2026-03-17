import React, { Fragment } from "react";

type Props = {
  content: string;
};

type InlineToken =
  | { type: "text"; value: string }
  | { type: "code"; value: string }
  | { type: "strong"; children: InlineToken[] }
  | { type: "em"; children: InlineToken[] }
  | { type: "link"; href: string; children: InlineToken[] };

type Block =
  | { type: "heading"; depth: number; content: string }
  | { type: "paragraph"; lines: string[] }
  | { type: "code"; language: string; content: string }
  | { type: "blockquote"; lines: string[] }
  | { type: "ul"; items: string[] }
  | { type: "ol"; items: string[] };

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

    if (/^[-*]\s+/.test(trimmed)) {
      const items: string[] = [];
      while (index < lines.length) {
        const candidate = lines[index].trim();
        const match = candidate.match(/^[-*]\s+(.*)$/);
        if (!match) {
          break;
        }
        items.push(match[1]);
        index += 1;
      }
      blocks.push({ type: "ul", items });
      continue;
    }

    if (/^\d+\.\s+/.test(trimmed)) {
      const items: string[] = [];
      while (index < lines.length) {
        const candidate = lines[index].trim();
        const match = candidate.match(/^\d+\.\s+(.*)$/);
        if (!match) {
          break;
        }
        items.push(match[1]);
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
        /^\d+\.\s+/.test(candidateTrimmed)
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
  const pattern = /(`[^`]+`|\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)|\*\*([^*]+)\*\*|\*([^*]+)\*|_([^_]+)_)/;

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
    if (raw.startsWith("`")) {
      tokens.push({ type: "code", value: raw.slice(1, -1) });
    } else if (raw.startsWith("[")) {
      tokens.push({
        type: "link",
        href: match[3],
        children: parseInline(match[2]),
      });
    } else if (raw.startsWith("**")) {
      tokens.push({ type: "strong", children: parseInline(match[4]) });
    } else {
      const emphasis = match[5] ?? match[6] ?? "";
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
            return (
              <pre key={key} className="md-code-block">
                {block.language && <span className="md-code-lang">{block.language}</span>}
                <code>{block.content}</code>
              </pre>
            );
          case "blockquote":
            return <blockquote key={key}>{renderLines(block.lines)}</blockquote>;
          case "ul":
            return (
              <ul key={key}>
                {block.items.map((item, itemIndex) => (
                  <li key={`${key}-${itemIndex}`}>{renderInline(parseInline(item))}</li>
                ))}
              </ul>
            );
          case "ol":
            return (
              <ol key={key}>
                {block.items.map((item, itemIndex) => (
                  <li key={`${key}-${itemIndex}`}>{renderInline(parseInline(item))}</li>
                ))}
              </ol>
            );
          default:
            return null;
        }
      })}
    </div>
  );
}

/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * A deliberately small Markdown parser for API documents.
 *
 * Why not a library: a Markdown-to-HTML converter passes raw inline HTML
 * through unchanged, so its output would need sanitizing and then rendering via
 * `dangerouslySetInnerHTML` (`.claude/rules/js-output-encoding-xss.md`). This
 * parser instead produces a plain tree that `MarkdownView` renders as React
 * elements, so every piece of document text reaches the DOM as an escaped text
 * node and raw HTML in a document is shown as literal text, never executed.
 *
 * Supported: ATX headings, paragraphs, fenced and indented code blocks,
 * bulleted and numbered lists, block quotes, horizontal rules, and inline code,
 * bold, italic, links and hard line breaks. Anything else renders as text.
 */

export type MarkdownInline =
  | { kind: 'text'; text: string }
  | { kind: 'code'; text: string }
  | { kind: 'strong'; children: MarkdownInline[] }
  | { kind: 'em'; children: MarkdownInline[] }
  | { kind: 'link'; href: string; children: MarkdownInline[] }
  | { kind: 'break' };

export type MarkdownBlock =
  | { kind: 'heading'; level: 1 | 2 | 3 | 4 | 5 | 6; children: MarkdownInline[] }
  | { kind: 'paragraph'; children: MarkdownInline[] }
  | { kind: 'code'; language?: string; text: string }
  | { kind: 'list'; ordered: boolean; start: number; items: MarkdownInline[][] }
  | { kind: 'quote'; children: MarkdownBlock[] }
  | { kind: 'rule' };

const FENCE = /^ {0,3}(`{3,}|~{3,})\s*([\w+-]*)/;
const HEADING = /^ {0,3}(#{1,6})\s+(.*?)\s*#*\s*$/;
const RULE = /^ {0,3}([-*_])(\s*\1){2,}\s*$/;
const BULLET = /^ {0,3}[-*+]\s+(.*)$/;
const ORDERED = /^ {0,3}(\d{1,9})[.)]\s+(.*)$/;
const QUOTE = /^ {0,3}>\s?(.*)$/;
const INDENTED_CODE = /^( {4}|\t)(.*)$/;

/**
 * Link targets allowed through. Anything else — `javascript:`, `data:`,
 * `vbscript:` and friends — is dropped and the link text rendered on its own.
 * Relative targets are allowed: they resolve against the console's own origin.
 */
export const safeHref = (raw: string): string | undefined => {
  // Browsers ignore control characters and whitespace inside a scheme
  // ("java\tscript:"), so they are stripped before the scheme is judged.
  // eslint-disable-next-line no-control-regex
  const href = raw.trim().replace(/[\u0000-\u001F\u007F\s]+/g, '');
  if (!href) return undefined;
  if (/^(https?:|mailto:)/i.test(href)) return href;
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return undefined;
  if (href.startsWith('//')) return undefined;
  return href;
};

/** Parses inline spans: code, links, bold, italic, line breaks. */
export function parseInline(source: string): MarkdownInline[] {
  const out: MarkdownInline[] = [];
  let buffer = '';
  const flush = () => {
    if (buffer) out.push({ kind: 'text', text: buffer });
    buffer = '';
  };

  let i = 0;
  while (i < source.length) {
    const char = source[i];
    const rest = source.slice(i);

    // Backslash escape: the next punctuation character is literal.
    if (char === '\\' && i + 1 < source.length && /[\\`*_{}[\]()#+\-.!>~|]/.test(source[i + 1])) {
      buffer += source[i + 1];
      i += 2;
      continue;
    }

    if (char === '\n') {
      flush();
      out.push({ kind: 'break' });
      i += 1;
      continue;
    }

    if (char === '`') {
      const ticks = /^`+/.exec(rest)![0];
      const end = source.indexOf(ticks, i + ticks.length);
      if (end !== -1) {
        flush();
        out.push({ kind: 'code', text: source.slice(i + ticks.length, end).trim() });
        i = end + ticks.length;
        continue;
      }
    }

    if (char === '[') {
      const link = /^\[([^\]]*)\]\(\s*<?([^\s)>]*)>?(?:\s+"[^"]*")?\s*\)/.exec(rest);
      if (link) {
        flush();
        const children = parseInline(link[1]);
        const href = safeHref(link[2]);
        if (href) out.push({ kind: 'link', href, children });
        else out.push(...children);
        i += link[0].length;
        continue;
      }
    }

    if (char === '<') {
      const auto = /^<((?:https?:\/\/|mailto:)[^\s<>]+)>/i.exec(rest);
      if (auto) {
        flush();
        const href = safeHref(auto[1]);
        out.push(
          href
            ? { kind: 'link', href, children: [{ kind: 'text', text: auto[1] }] }
            : { kind: 'text', text: auto[0] }
        );
        i += auto[0].length;
        continue;
      }
    }

    if (char === '*' || char === '_') {
      const strong = new RegExp(`^\\${char}{2}(?=\\S)([\\s\\S]*?\\S)\\${char}{2}`).exec(rest);
      if (strong) {
        flush();
        out.push({ kind: 'strong', children: parseInline(strong[1]) });
        i += strong[0].length;
        continue;
      }
      // `_` only opens emphasis at a word boundary, so snake_case stays intact.
      const boundary = char === '*' || i === 0 || /[\s([{]/.test(source[i - 1]);
      const em = new RegExp(`^\\${char}(?=\\S)([\\s\\S]*?\\S)\\${char}(?!\\${char})`).exec(rest);
      if (boundary && em && (char === '*' || !/\w/.test(source[i + em[0].length] ?? ''))) {
        flush();
        out.push({ kind: 'em', children: parseInline(em[1]) });
        i += em[0].length;
        continue;
      }
    }

    buffer += char;
    i += 1;
  }
  flush();
  return out;
}

/** Joins a paragraph's lines: a trailing double space or backslash is a hard break, else a space. */
const joinParagraph = (lines: string[]): string =>
  lines
    .map((line, index) => {
      if (index === lines.length - 1) return line.trim();
      if (/( {2,}|\\)$/.test(line)) return `${line.replace(/( {2,}|\\)$/, '').trim()}\n`;
      return `${line.trim()} `;
    })
    .join('');

const startsBlock = (line: string): boolean =>
  FENCE.test(line) ||
  HEADING.test(line) ||
  RULE.test(line) ||
  BULLET.test(line) ||
  ORDERED.test(line) ||
  QUOTE.test(line);

/** Parses a Markdown document into blocks. Never throws: unknown syntax becomes text. */
export function parseMarkdown(source: string): MarkdownBlock[] {
  const lines = source.replace(/\r\n?/g, '\n').split('\n');
  const blocks: MarkdownBlock[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (line.trim() === '') {
      i += 1;
      continue;
    }

    const fence = FENCE.exec(line);
    if (fence) {
      const marker = fence[1];
      const closing = new RegExp(
        `^ {0,3}${marker[0] === '`' ? '`' : '~'}{${marker.length},}\\s*$`,
      );
      const body: string[] = [];
      i += 1;
      while (i < lines.length && !closing.test(lines[i])) {
        body.push(lines[i]);
        i += 1;
      }
      i += 1; // closing fence (or end of document)
      blocks.push({ kind: 'code', language: fence[2] || undefined, text: body.join('\n') });
      continue;
    }

    const heading = HEADING.exec(line);
    if (heading) {
      blocks.push({
        kind: 'heading',
        level: heading[1].length as 1 | 2 | 3 | 4 | 5 | 6,
        children: parseInline(heading[2]),
      });
      i += 1;
      continue;
    }

    if (RULE.test(line)) {
      blocks.push({ kind: 'rule' });
      i += 1;
      continue;
    }

    if (QUOTE.test(line)) {
      const body: string[] = [];
      while (i < lines.length && QUOTE.test(lines[i])) {
        body.push(QUOTE.exec(lines[i])![1]);
        i += 1;
      }
      blocks.push({ kind: 'quote', children: parseMarkdown(body.join('\n')) });
      continue;
    }

    const bullet = BULLET.test(line);
    const ordered = ORDERED.exec(line);
    if (bullet || ordered) {
      const pattern = bullet ? BULLET : ORDERED;
      const items: string[][] = [];
      while (i < lines.length) {
        const match = pattern.exec(lines[i]);
        if (match) {
          items.push([bullet ? match[1] : match[2]]);
        } else if (lines[i].trim() !== '' && /^\s+\S/.test(lines[i]) && items.length > 0) {
          // An indented continuation line belongs to the current item.
          items[items.length - 1].push(lines[i]);
        } else {
          break;
        }
        i += 1;
      }
      blocks.push({
        kind: 'list',
        ordered: Boolean(ordered),
        start: ordered ? Number(ordered[1]) : 1,
        items: items.map((item) => parseInline(joinParagraph(item))),
      });
      continue;
    }

    if (INDENTED_CODE.test(line)) {
      const body: string[] = [];
      while (i < lines.length && (INDENTED_CODE.test(lines[i]) || lines[i].trim() === '')) {
        body.push(INDENTED_CODE.exec(lines[i])?.[2] ?? '');
        i += 1;
      }
      while (body.length && body[body.length - 1] === '') body.pop();
      blocks.push({ kind: 'code', text: body.join('\n') });
      continue;
    }

    const paragraph: string[] = [];
    while (i < lines.length && lines[i].trim() !== '' && (paragraph.length === 0 || !startsBlock(lines[i]))) {
      paragraph.push(lines[i]);
      i += 1;
    }
    blocks.push({ kind: 'paragraph', children: parseInline(joinParagraph(paragraph)) });
  }

  return blocks;
}


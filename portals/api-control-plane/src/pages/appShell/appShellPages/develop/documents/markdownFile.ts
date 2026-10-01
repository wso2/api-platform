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
 * Reading a Markdown file the user picks into the editor.
 *
 * The file never goes to the server as-is: its text is loaded into the editor
 * so the user can review it, and is then saved as inline content like anything
 * typed by hand. These checks are a courtesy that fails fast in the browser —
 * platform-api still sniffs the content and enforces its own size limit.
 */

/** Matches platform-api's default document size ceiling (5 MiB). */
export const MAX_MARKDOWN_FILE_BYTES = 5 * 1024 * 1024;

const MARKDOWN_EXTENSION = /\.(md|markdown)$/i;

export type MarkdownFileError = 'type' | 'size' | 'encoding';

export type MarkdownFile = { fileName: string; content: string };

/** The file's bytes, via `FileReader` — supported everywhere `Blob.arrayBuffer` is not. */
const readBytes = (file: File): Promise<ArrayBuffer> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as ArrayBuffer);
    reader.onerror = () => reject(reader.error ?? new Error('read failed'));
    reader.readAsArrayBuffer(file);
  });

export async function readMarkdownFile(file: File): Promise<MarkdownFile | { error: MarkdownFileError }> {
  if (!MARKDOWN_EXTENSION.test(file.name)) return { error: 'type' };
  if (file.size > MAX_MARKDOWN_FILE_BYTES) return { error: 'size' };

  let content: string;
  try {
    // `fatal` makes invalid UTF-8 throw instead of being silently replaced.
    content = new TextDecoder('utf-8', { fatal: true }).decode(await readBytes(file));
  } catch {
    return { error: 'encoding' };
  }
  // A NUL byte means binary content wearing a .md extension.
  if (content.includes('\u0000')) return { error: 'encoding' };

  // Only the base name is kept; a browser never exposes a path, but be explicit.
  const fileName = file.name.split(/[\\/]/).pop() ?? file.name;
  return { content: content.replace(/^\uFEFF/, ''), fileName };
}

/**
 * A name for a document created from a file: its first `# Heading`, else the
 * file name without its extension and with separators turned into spaces.
 */
export function suggestDocumentName(content: string, fileName: string): string {
  const heading = /^ {0,3}#\s+(.+?)\s*#*\s*$/m.exec(content);
  if (heading) return heading[1].trim();
  return fileName.replace(MARKDOWN_EXTENSION, '').replace(/[-_]+/g, ' ').trim();
}

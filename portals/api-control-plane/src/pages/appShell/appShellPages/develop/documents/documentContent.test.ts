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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it } from 'vitest';

import { isTextContent } from './documentContent';

/**
 * `isTextContent` guards both the Markdown viewer and the Markdown-only editor.
 * A false positive on a binary type would hand raw bytes to the editor; a false
 * negative on a legitimate text type would hide a document the user can
 * genuinely edit. These tests pin the allowlist.
 */

describe('isTextContent', () => {
  it('accepts an empty content type (what a 204 or an unknown-type doc carries)', () => {
    expect(isTextContent('')).toBe(true);
  });

  it('accepts the Markdown types the server stores inline content as', () => {
    expect(isTextContent('text/markdown')).toBe(true);
    expect(isTextContent('text/markdown; charset=utf-8')).toBe(true);
    expect(isTextContent('text/x-markdown')).toBe(true);
  });

  it('accepts plain text', () => {
    expect(isTextContent('text/plain')).toBe(true);
    expect(isTextContent('text/plain; charset=utf-8')).toBe(true);
  });

  it('ignores case and whitespace around the media type', () => {
    // Operators can set any Content-Type on upload; the sniff output is
    // case-insensitive in RFC 7231. A new DB write of `TEXT/MARKDOWN` must
    // still render in the viewer.
    expect(isTextContent('TEXT/MARKDOWN')).toBe(true);
    expect(isTextContent('  text/markdown  ; charset=utf-8')).toBe(true);
  });

  it('rejects binary and rich-document types that would corrupt in the Markdown viewer', () => {
    expect(isTextContent('application/pdf')).toBe(false);
    expect(isTextContent('application/octet-stream')).toBe(false);
    expect(isTextContent('image/png')).toBe(false);
    expect(isTextContent('application/vnd.openxmlformats-officedocument.wordprocessingml.document'))
      .toBe(false);
  });

  it('rejects HTML — rendered through MarkdownView it would show as source, not markup', () => {
    expect(isTextContent('text/html')).toBe(false);
    expect(isTextContent('text/html; charset=utf-8')).toBe(false);
  });

  it('rejects JSON — valid text, but the viewer’s heuristics would format it as prose', () => {
    expect(isTextContent('application/json')).toBe(false);
  });
});

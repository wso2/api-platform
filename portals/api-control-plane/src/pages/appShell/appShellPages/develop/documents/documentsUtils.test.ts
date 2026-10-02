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

import { describe, expect, it } from 'vitest';

import { createIntl } from 'react-intl';

import { documentTypeName, validateCustomType } from './documentTypes';
import { documentsSearch, readDocumentsView } from './documentsSearch';
import { readMarkdownFile, suggestDocumentName } from './markdownFile';

describe('documents view in the URL', () => {
  it.each([
    ['', { mode: 'browse', docId: undefined }],
    ['doc=faq', { mode: 'browse', docId: 'faq' }],
    ['mode=create', { mode: 'create' }],
    ['doc=faq&mode=edit', { mode: 'edit', docId: 'faq' }],
    // Edit without a document has nothing to edit.
    ['mode=edit', { mode: 'browse', docId: undefined }],
  ])('reads "%s"', (query, view) => {
    expect(readDocumentsView(new URLSearchParams(query))).toEqual(view);
  });

  it('round-trips through documentsSearch', () => {
    expect(documentsSearch({ docId: 'a b', mode: 'edit' })).toBe('?doc=a+b&mode=edit');
    expect(documentsSearch({ mode: 'browse' })).toBe('');
  });
});

describe('readMarkdownFile', () => {
  it('reads a Markdown file and strips a byte-order mark', async () => {
    const file = new File(['\uFEFF# Hello'], 'hello.md');
    await expect(readMarkdownFile(file)).resolves.toEqual({ content: '# Hello', fileName: 'hello.md' });
  });

  it('rejects other extensions', async () => {
    await expect(readMarkdownFile(new File(['x'], 'notes.txt'))).resolves.toEqual({ error: 'type' });
  });

  it('rejects binary content', async () => {
    const file = new File([new Uint8Array([0x23, 0x00, 0x41])], 'binary.md');
    await expect(readMarkdownFile(file)).resolves.toEqual({ error: 'encoding' });
  });
});

describe('suggestDocumentName', () => {
  it('prefers the first top-level heading', () => {
    expect(suggestDocumentName('intro\n# Error handling\n## More', 'x.md')).toBe('Error handling');
  });

  it('falls back to a readable file name', () => {
    expect(suggestDocumentName('no heading', 'getting-started_guide.md')).toBe('getting started guide');
  });
});

describe('custom "Other" document types', () => {
  const intl = createIntl({ locale: 'en', messages: {} });

  it('shows the custom name, or the fixed label, as the type', () => {
    // Custom types are shown exactly as stored, case included.
    expect(documentTypeName(intl, 'faq')).toBe('faq');
    expect(documentTypeName(intl, 'FAQ')).toBe('FAQ');
    expect(documentTypeName(intl, 'Release notes')).toBe('Release notes');
    // Fixed types
    expect(documentTypeName(intl, 'HOW_TO')).toBe('How To');
    expect(documentTypeName(intl, 'OTHER')).toBe('Other');
  });

  it.each([
    ['', 'required'],
    ['   ', 'required'],
    ['A very long type name', 'tooLong'],
    ['Notes!', 'invalid'],
    ['Changelog', undefined],
    ['Release notes', undefined],
  ])('validates %j', (name, error) => {
    expect(validateCustomType(name)).toBe(error);
  });
});

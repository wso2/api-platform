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

import {
  documentsSearch,
  documentsSearchParams,
  readDocumentsView,
  type DocumentsView,
} from './documentsSearch';

/**
 * The Documents page carries its view in the URL (`?doc=`, `?mode=`), so the
 * browser's Back button steps out of the editor and the overview page can
 * deep-link to one document. If the parser and the writer disagree, Back
 * either stops working or lands on a view the user did not have open.
 *
 * These tests pin the three branches (`browse`, `create`, `edit`) and the
 * round-trip between them.
 */

describe('readDocumentsView', () => {
  it('is browse with no params', () => {
    expect(readDocumentsView(new URLSearchParams())).toEqual({ mode: 'browse' });
  });

  it('is browse with just a doc selected', () => {
    expect(readDocumentsView(new URLSearchParams('doc=getting-started'))).toEqual({
      mode: 'browse',
      docId: 'getting-started',
    });
  });

  it('is create regardless of doc=', () => {
    // `?mode=create&doc=…` is a legal URL — Back from the editor can land on
    // it. Create wins, since a half-written doc has no id yet.
    expect(readDocumentsView(new URLSearchParams('mode=create'))).toEqual({ mode: 'create' });
    expect(readDocumentsView(new URLSearchParams('mode=create&doc=stale'))).toEqual({
      mode: 'create',
    });
  });

  it('is edit only when a docId is also present', () => {
    // `?mode=edit` with no `doc=` would be ambiguous — fall back to browse so
    // the user sees the list rather than a blank editor.
    expect(readDocumentsView(new URLSearchParams('mode=edit&doc=guide'))).toEqual({
      mode: 'edit',
      docId: 'guide',
    });
    expect(readDocumentsView(new URLSearchParams('mode=edit'))).toEqual({ mode: 'browse' });
  });

  it('treats an unknown mode as browse', () => {
    // A hand-edited `?mode=export` from a bookmark should not blow up the
    // page; the browse fallback keeps the UI usable.
    expect(readDocumentsView(new URLSearchParams('mode=export&doc=guide'))).toEqual({
      mode: 'browse',
      docId: 'guide',
    });
  });

  it('an empty doc= is treated as no selection', () => {
    expect(readDocumentsView(new URLSearchParams('doc='))).toEqual({ mode: 'browse' });
  });
});

describe('documentsSearchParams', () => {
  it('builds nothing for an unselected browse view', () => {
    expect(documentsSearchParams({ mode: 'browse' }).toString()).toBe('');
  });

  it('builds `doc=` only for a selected browse view', () => {
    expect(documentsSearchParams({ mode: 'browse', docId: 'guide' }).toString()).toBe('doc=guide');
  });

  it('builds `mode=create` with no doc for the create form', () => {
    expect(documentsSearchParams({ mode: 'create' }).toString()).toBe('mode=create');
  });

  it('builds both `doc` and `mode=edit` for the edit form', () => {
    const params = documentsSearchParams({ mode: 'edit', docId: 'guide' });
    expect(params.get('doc')).toBe('guide');
    expect(params.get('mode')).toBe('edit');
  });
});

describe('documentsSearch', () => {
  it('is empty when nothing is selected (so a link does not grow a trailing ?)', () => {
    expect(documentsSearch({ mode: 'browse' })).toBe('');
  });

  it('prefixes `?` otherwise', () => {
    expect(documentsSearch({ mode: 'browse', docId: 'guide' })).toBe('?doc=guide');
    expect(documentsSearch({ mode: 'create' })).toBe('?mode=create');
  });
});

describe('round trip', () => {
  // The Back button only works if what the writer serialises the parser can
  // deserialise. Encode-then-decode each variant and expect the same view.
  const views: DocumentsView[] = [
    { mode: 'browse' },
    { mode: 'browse', docId: 'guide' },
    { mode: 'create' },
    { mode: 'edit', docId: 'guide' },
  ];

  for (const view of views) {
    it(`is stable for ${JSON.stringify(view)}`, () => {
      const encoded = documentsSearchParams(view);
      expect(readDocumentsView(encoded)).toEqual(view);
    });
  }
});

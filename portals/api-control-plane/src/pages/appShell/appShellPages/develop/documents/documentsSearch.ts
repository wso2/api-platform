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
 * Which part of the Documents page is showing, carried in the URL's query
 * string so a document can be deep-linked (the API overview links straight to
 * one), and so the browser's Back button steps out of the editor.
 *
 *   ?doc=<id>              view one document
 *   ?mode=create           the create form
 *   ?doc=<id>&mode=edit    the edit form for one document
 *
 * Query parameters rather than child routes: every state is the same sidebar
 * entry and the same page, and adding routes would mean registering each one
 * for every scope-less alias too.
 */

export type DocumentsView =
  | { mode: 'browse'; docId?: string }
  | { mode: 'create' }
  | { mode: 'edit'; docId: string };

const DOC_PARAM = 'doc';
const MODE_PARAM = 'mode';

export const readDocumentsView = (params: URLSearchParams): DocumentsView => {
  const docId = params.get(DOC_PARAM) || undefined;
  const mode = params.get(MODE_PARAM);
  if (mode === 'create') return { mode: 'create' };
  if (mode === 'edit' && docId) return { mode: 'edit', docId };
  return { mode: 'browse', docId };
};

export const documentsSearchParams = (view: DocumentsView): URLSearchParams => {
  const params = new URLSearchParams();
  if (view.mode !== 'create' && view.docId) params.set(DOC_PARAM, view.docId);
  if (view.mode !== 'browse') params.set(MODE_PARAM, view.mode);
  return params;
};

/** `?doc=…` suffix for a link into the Documents page; empty when there is nothing to select. */
export const documentsSearch = (view: DocumentsView): string => {
  const query = documentsSearchParams(view).toString();
  return query ? `?${query}` : '';
};

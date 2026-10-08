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

import { useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';

import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { DocumentEditor } from './DocumentEditor';
import { DocumentsBrowser } from './DocumentsBrowser';
import { documentsSearchParams, readDocumentsView, type DocumentsView } from './documentsSearch';

/**
 * Develop › Documents for the API in scope.
 *
 * Switches between the browser (list + viewer) and the create/edit form from
 * the URL's query string — see `documentsSearch.ts` — so every state can be
 * linked to and Back leaves the editor.
 */
export function DocumentsPanel() {
  const { params } = useConsoleScope();
  const apiHandle = params.apiHandler;
  const [searchParams, setSearchParams] = useSearchParams();
  const view = readDocumentsView(searchParams);

  const show = useCallback(
    (next: DocumentsView, options: { replace?: boolean } = {}) =>
      setSearchParams(documentsSearchParams(next), options),
    [setSearchParams],
  );

  const selectDocument = useCallback(
    (docId: string, options?: { replace?: boolean }) => show({ docId, mode: 'browse' }, options),
    [show],
  );

  // `ScopeGate` only renders this once an API is in scope.
  if (!apiHandle) return null;

  if (view.mode === 'create' || view.mode === 'edit') {
    return (
      <DocumentEditor
        apiHandle={apiHandle}
        docId={view.mode === 'edit' ? view.docId : undefined}
        // Back to the document it came from; a fresh create lands on the new one.
        onCancel={() =>
          show({ docId: view.mode === 'edit' ? view.docId : undefined, mode: 'browse' })
        }
        // `replace`, so Back from the saved document does not reopen the form.
        onSaved={(docId) => show({ docId, mode: 'browse' }, { replace: true })}
      />
    );
  }

  return (
    <DocumentsBrowser
      apiHandle={apiHandle}
      onCreate={() => show({ mode: 'create' })}
      onDeleted={() => show({ mode: 'browse' }, { replace: true })}
      onEdit={(docId) => show({ docId, mode: 'edit' })}
      onSelect={selectDocument}
      selectedId={view.docId}
    />
  );
}

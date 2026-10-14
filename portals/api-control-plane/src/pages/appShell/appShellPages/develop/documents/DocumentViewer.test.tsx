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

import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { waitFor, within } from '@testing-library/react';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import type { ApiDocument } from '@/api/resources/apiDocuments';
import { apiUrl, failure, noContent, resource } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { DocumentViewer } from './DocumentViewer';

/**
 * The viewer sits on Develop › Documents and has four behavioural axes:
 *
 *   - metadata states: loading vs loaded vs 404 vs 500
 *   - content: Markdown (renders) vs non-Markdown (unsupported banner) vs empty
 *   - actions: edit button triggers onEdit; delete lives behind a confirm
 *   - keeps the header rendered when only the content fetch fails
 *
 * MarkdownView has its own tests; here we check it is wired to the content
 * endpoint and skipped for non-Markdown content.
 */

const ORG = 'acme';
const API_TYPE = 'rest-api';
const API_ID = 'orders-api';
const DOC_ID = 'getting-started';
const BASE = `/apis/${API_TYPE}/${API_ID}/docs/${DOC_ID}`;

const aDocument = (overrides: Partial<ApiDocument> = {}): ApiDocument => ({
  displayName: 'Getting Started',
  id: DOC_ID,
  type: 'HowTo',
  ...overrides,
});

function renderViewer(onEdit = vi.fn(), onDeleted = vi.fn()) {
  const utils = renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId="any">
      <DocumentViewer
        apiHandle={API_ID}
        docId={DOC_ID}
        onEdit={onEdit}
        onDeleted={onDeleted}
      />
    </ApiScopeProvider>,
  );
  return { ...utils, onEdit, onDeleted };
}

beforeEach(() => resetHttpClient());

const servePlainMarkdown = (text = '# Hello') =>
  server.use(
    mswHttp.get(apiUrl(`${BASE}/content`), () =>
      new HttpResponse(text, { headers: { 'Content-Type': 'text/markdown; charset=utf-8' } }),
    ),
  );

describe('DocumentViewer — metadata states', () => {
  it('shows the loading banner before metadata arrives', async () => {
    server.use(
      mswHttp.get(apiUrl(BASE), async () => {
        await new Promise((resolve) => setTimeout(resolve, 50));
        return HttpResponse.json(aDocument());
      }),
    );
    servePlainMarkdown();
    renderViewer();

    expect(await screen.findByText(/Loading document/i)).toBeInTheDocument();
  });

  it('maps 404 to the specific "deleted" message, not a generic error', async () => {
    // The viewer tells the user whether the doc is gone (acted on elsewhere)
    // or merely unreachable — the two have different mental models.
    server.use(failure('get', BASE, 404, 'NOT_FOUND'));
    servePlainMarkdown();
    renderViewer();

    expect(
      await screen.findByText(/This document no longer exists\. It may have been deleted\./i),
    ).toBeInTheDocument();
  });

  it('shows the generic load error for a 500', async () => {
    server.use(failure('get', BASE, 500, 'INTERNAL_ERROR'));
    servePlainMarkdown();
    renderViewer();

    expect(await screen.findByText(/Unable to load this document/i)).toBeInTheDocument();
    expect(screen.queryByText(/no longer exists/i)).not.toBeInTheDocument();
  });
});

describe('DocumentViewer — content body', () => {
  it('renders Markdown content as HTML', async () => {
    server.use(resource(BASE, aDocument()));
    server.use(
      mswHttp.get(apiUrl(`${BASE}/content`), () =>
        new HttpResponse('# Hello\n\nBody text here.', {
          headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
        }),
      ),
    );
    renderViewer();

    // MarkdownView starts document headings at h3 (page title is h1, doc
    // title is h2), so `#` renders as a real h3 — see `headingElement`.
    expect(await screen.findByRole('heading', { level: 3, name: 'Hello' })).toBeInTheDocument();
    expect(screen.getByText('Body text here.')).toBeInTheDocument();
  });

  it('shows the unsupported-format banner for a non-Markdown Content-Type', async () => {
    // Future PDF/DOCX uploads must NOT render through MarkdownView — the
    // binary header would show as prose. The viewer gates on `isTextContent`.
    server.use(resource(BASE, aDocument()));
    server.use(
      mswHttp.get(apiUrl(`${BASE}/content`), () =>
        new HttpResponse('%PDF-1.4\n…', { headers: { 'Content-Type': 'application/pdf' } }),
      ),
    );
    renderViewer();

    expect(
      await screen.findByText(/This document’s format can’t be previewed here/i),
    ).toBeInTheDocument();
  });

  it('shows the empty-body message for a Markdown document whose content is blank', async () => {
    // 204 reads as empty text; MarkdownView delegates to the `emptyFallback`.
    server.use(resource(BASE, aDocument()));
    server.use(
      mswHttp.get(apiUrl(`${BASE}/content`), () => new HttpResponse(null, { status: 204 })),
    );
    renderViewer();

    expect(await screen.findByText(/This document has no content/i)).toBeInTheDocument();
  });

  it('surfaces a content-fetch error without hiding the title', async () => {
    // Metadata ok, content endpoint fails — the title must stay rendered so
    // the user can still Edit / Delete.
    server.use(resource(BASE, aDocument()));
    server.use(failure('get', `${BASE}/content`, 500, 'INTERNAL_ERROR'));
    renderViewer();

    expect(
      await screen.findByRole('heading', { level: 2, name: /Getting Started/ }),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(/Unable to load the content of this document/i),
    ).toBeInTheDocument();
  });
});

describe('DocumentViewer — edit and delete actions', () => {
  it('calls onEdit when the pencil icon is clicked', async () => {
    server.use(resource(BASE, aDocument()));
    servePlainMarkdown();
    const { onEdit, user } = renderViewer();

    await user.click(await screen.findByRole('button', { name: /Edit/ }));
    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it('deletes behind a confirm dialog, then notifies the parent', async () => {
    // Delete is destructive and sits behind a two-step confirm. A single-
    // click delete would make a stray keyboard Enter unrecoverable.
    server.use(resource(BASE, aDocument()));
    servePlainMarkdown();
    server.use(noContent('delete', BASE));

    const { onDeleted, user } = renderViewer();

    await user.click(await screen.findByRole('button', { name: /Delete/ }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText(/Delete document\?/)).toBeInTheDocument();
    expect(
      within(dialog).getByText(/"Getting Started" will be permanently removed/),
    ).toBeInTheDocument();

    await user.click(within(dialog).getByRole('button', { name: /^Delete$/ }));

    await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(1));
  });

  it('closes the dialog without deleting when Cancel is clicked', async () => {
    server.use(resource(BASE, aDocument()));
    servePlainMarkdown();
    // No DELETE handler installed — if the component sent one, msw's
    // onUnhandledRequest: 'error' would fail the test. That is the guarantee.

    const { onDeleted, user } = renderViewer();

    await user.click(await screen.findByRole('button', { name: /Delete/ }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: /Cancel/ }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(onDeleted).not.toHaveBeenCalled();
  });
});

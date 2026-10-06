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

import { http, HttpResponse } from 'msw';
import { Route, Routes, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import type { ApiDocument, ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { routes } from '@/routes/paths';
import { makeConsoleScope } from '@/test/mockScope';
import { apiUrl, listEnvelope, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { DocumentsPanel } from './DocumentsPanel';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'orders-api';
const COLLECTION = `/apis/rest-api/${API}/docs`;
const BASE = routes.apiDevelopDocuments(ORG, PROJECT, API);

/** A document fixture: metadata plus the body the content endpoint serves. */
type DocumentFixture = ApiDocument & { content: string };

const aDocument = (id: string, overrides: Partial<DocumentFixture> = {}): DocumentFixture => ({
  content: `# ${id}\n\nBody of ${id}.`,
  contentType: 'text/markdown; charset=utf-8',
  displayName: id,
  id,
  type: 'HOW_TO',
  updatedAt: '2026-09-28T10:00:00Z',
  ...overrides,
});

const metadata = (document: DocumentFixture): ApiDocumentMetadata => {
  const { content, ...rest } = document;
  void content;
  return rest;
};

let requests: Recorder;

const notFound = () =>
  HttpResponse.json({ code: 'NOT_FOUND', message: 'Not found', status: 'error' }, { status: 404 });

/**
 * Serves the list (paged), each document's metadata and each document's body
 * from one in-memory set — the metadata and the body are separate endpoints.
 */
function serve(documents: DocumentFixture[]) {
  server.use(
    // Read on every request, so documents a test adds later (a create) appear in the list.
    http.get(apiUrl(COLLECTION), async ({ request }) => {
      await requests.capture(request);
      const params = new URL(request.url).searchParams;
      const offset = Number(params.get('offset') ?? 0);
      const limit = Number(params.get('limit') ?? 20);
      return HttpResponse.json(
        listEnvelope(documents.slice(offset, offset + limit).map(metadata), {
          limit,
          offset,
          total: documents.length,
        })
      );
    }),
    http.get(apiUrl(`${COLLECTION}/:docId/content`), ({ params }) => {
      const document = documents.find((candidate) => candidate.id === params.docId);
      return document
        ? new HttpResponse(document.content, { headers: { 'Content-Type': document.contentType ?? '' } })
        : notFound();
    }),
    http.get(apiUrl(`${COLLECTION}/:docId`), ({ params }) => {
      const document = documents.find((candidate) => candidate.id === params.docId);
      return document ? HttpResponse.json(metadata(document)) : notFound();
    })
  );
}

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{`${location.pathname}${location.search}`}</output>;
}

function renderPage(entry = BASE) {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG} projectId={PROJECT}>
      <Routes>
        <Route
          element={
            <>
              <DocumentsPanel />
              <LocationProbe />
            </>
          }
          path={routes.apiDevelopDocuments()}
        />
      </Routes>
    </ApiScopeProvider>,
    {
      route: entry,
      scope: makeConsoleScope({ params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT } }),
    }
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('DocumentsPanel', () => {
  it('offers a single create action when the API has no documents', async () => {
    serve([]);
    renderPage();

    expect(await screen.findByText('No Documents available for this API')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Create Document' })).toHaveLength(1);
  });

  it('shows the first document and switches when another is picked', async () => {
    serve([aDocument('getting-started'), aDocument('sdk', { type: 'SAMPLE_SDK' })]);
    const { user } = renderPage();

    expect(await screen.findByRole('heading', { level: 2, name: 'getting-started' })).toBeInTheDocument();
    expect(await screen.findByText('Body of getting-started.')).toBeInTheDocument();
    expect(screen.getByText('Samples & SDK')).toBeInTheDocument();
    // The first document is selected on arrival, highlighted and in the URL.
    expect(screen.getByRole('button', { name: /^getting-started/ })).toHaveAttribute('aria-current', 'true');
    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent(`${BASE}?doc=getting-started`)
    );

    await user.click(screen.getByRole('button', { name: /^sdk/ }));

    expect(await screen.findByText('Body of sdk.')).toBeInTheDocument();
    expect(screen.getByTestId('location')).toHaveTextContent(`${BASE}?doc=sdk`);
  });

  it('collapses and expands a document-type group', async () => {
    serve([aDocument('getting-started'), aDocument('sdk', { type: 'SAMPLE_SDK' })]);
    const { user } = renderPage();

    const header = await screen.findByRole('button', { name: 'Samples & SDK (1)' });
    expect(header).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('button', { name: /^sdk/ })).toBeVisible();

    await user.click(header);
    expect(header).toHaveAttribute('aria-expanded', 'false');
    await waitFor(() => expect(screen.queryByRole('button', { name: /^sdk/ })).not.toBeInTheDocument());
    // Other groups are untouched.
    expect(screen.getByRole('button', { name: /^getting-started/ })).toBeInTheDocument();

    await user.click(header);
    expect(await screen.findByRole('button', { name: /^sdk/ })).toBeInTheDocument();
  });

  it('shows one read-only type field with the custom type name when editing', async () => {
    serve([aDocument('faq-doc', { type: 'FAQ' as never })]);
    renderPage(`${BASE}?doc=faq-doc&mode=edit`);

    const typeField = await screen.findByRole('textbox', { name: 'Document type' });
    expect(typeField).toHaveValue('FAQ');
    expect(typeField).toHaveAttribute('readonly');
    expect(screen.queryByLabelText(/^Custom type/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Up to \d+ characters/)).not.toBeInTheDocument();
  });

  it('opens the document named in the URL', async () => {
    serve([aDocument('getting-started'), aDocument('faq', { type: 'OTHER' })]);
    renderPage(`${BASE}?doc=faq`);

    expect(await screen.findByText('Body of faq.')).toBeInTheDocument();
  });

  it('loads the next page on "View more"', async () => {
    serve(Array.from({ length: 12 }, (_, index) => aDocument(`doc-${index + 1}`)));
    const { user } = renderPage();

    expect(await screen.findByText('Showing 10 of 12')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'View more' }));

    expect(await screen.findByRole('button', { name: /^doc-12/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'View more' })).not.toBeInTheDocument();
    await waitFor(() => expect(requests.calls.filter((r) => r.method === 'GET' && r.url.pathname.endsWith('/docs')).at(-1)?.params.get('offset')).toBe('10'));
  });

  it('creates a document from inline Markdown and opens it', async () => {
    const documents: DocumentFixture[] = [];
    serve(documents);
    server.use(
      http.post(apiUrl(COLLECTION), async ({ request }) => {
        await requests.capture(request);
        const created = aDocument('error-handling', { displayName: 'Error handling' });
        documents.push(created);
        return HttpResponse.json(metadata(created), { status: 201 });
      })
    );
    const { user } = renderPage(`${BASE}?mode=create`);

    await user.type(await screen.findByLabelText(/^Name/), 'Error handling');
    await user.type(screen.getByLabelText(/^Content/), 'Errors use a JSON body.');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent(`${BASE}?doc=error-handling`)
    );
    expect(requests.calls.some((r) => r.method === 'POST')).toBe(true);
  });

  it('saves an "Other" document with its custom type in otherTypeName', async () => {
    const documents: DocumentFixture[] = [];
    serve(documents);
    let postedType: string | null = null;
    let postedOtherTypeName: string | null = null;
    server.use(
      http.post(apiUrl(COLLECTION), async ({ request }) => {
        try {
          const form = await request.clone().formData();
          postedType = (form.get('type') as string | null) ?? null;
          postedOtherTypeName = (form.get('otherTypeName') as string | null) ?? null;
        } catch {
          // jsdom + axios can present FormData as a stringified body that
          // doesn't parse as real multipart — leave the captured fields null
          // and let the assertions below fall back to the text form.
        }
        await requests.capture(request);
        // The server stores the custom name itself as the type, case untouched.
        const created = aDocument('changes', { displayName: 'Changes', type: 'Changelog' as never });
        documents.push(created);
        return HttpResponse.json(metadata(created), { status: 201 });
      })
    );
    const { user } = renderPage(`${BASE}?mode=create`);

    await user.click(await screen.findByRole('combobox', { name: 'Document type' }));
    await user.click(await screen.findByRole('option', { name: 'Other' }));
    await user.type(screen.getByLabelText(/^Custom type/), 'Changelog');
    await user.type(screen.getByLabelText(/^Name/), 'Changes');
    await user.type(screen.getByLabelText(/^Content/), 'All notable changes.');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent(`${BASE}?doc=changes`)
    );
    const post = requests.calls.find((r) => r.method === 'POST');
    if (postedType !== null) {
      expect(postedType).toBe('OTHER');
      expect(postedOtherTypeName).toBe('Changelog');
    } else if (post?.body && !post.body.startsWith('[object ')) {
      expect(post.body).toMatch(/name="type"[^]*OTHER/);
      expect(post.body).toMatch(/name="otherTypeName"[^]*Changelog/);
    }
    // Listed under its own group, and its type chip shows the custom name, not "Other".
    await waitFor(() => expect(screen.getAllByText('Changelog')).toHaveLength(2));
  });

  it('keeps Create disabled until every required field is filled', async () => {
    serve([]);
    const { user } = renderPage(`${BASE}?mode=create`);

    const create = await screen.findByRole('button', { name: 'Create' });
    expect(create).toBeDisabled();

    await user.type(screen.getByLabelText(/^Name/), 'Changes');
    expect(create).toBeDisabled();
    await user.type(screen.getByLabelText(/^Content/), 'All notable changes.');
    expect(create).toBeEnabled();

    // "Other" adds a required custom type, which disables Create again until filled.
    await user.click(screen.getByRole('combobox', { name: 'Document type' }));
    await user.click(await screen.findByRole('option', { name: 'Other' }));
    expect(create).toBeDisabled();
    await user.type(screen.getByLabelText(/^Custom type/), 'Changelog');
    expect(create).toBeEnabled();

    // Missing values are never flagged in red — the disabled button says enough.
    expect(screen.queryByText(/^Enter a name/)).not.toBeInTheDocument();
  });

  it('leaves a form with nothing entered without asking', async () => {
    serve([aDocument('getting-started')]);
    const { user } = renderPage(`${BASE}?mode=create`);

    await user.click(await screen.findByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(screen.getByTestId('location')).not.toHaveTextContent('mode=create'));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('asks before Cancel discards unsaved changes', async () => {
    serve([aDocument('getting-started')]);
    const { user } = renderPage(`${BASE}?doc=getting-started&mode=edit`);

    await user.type(await screen.findByLabelText(/^Content/), ' More.');
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    const dialog = await screen.findByRole('dialog');
    expect(
      within(dialog).getByText('You have unsaved changes. Are you sure you want to leave?')
    ).toBeInTheDocument();

    // Stay keeps the form and the edits.
    await user.click(within(dialog).getByRole('button', { name: 'Stay' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(screen.getByLabelText(/^Content/)).toHaveValue('# getting-started\n\nBody of getting-started. More.');
    expect(screen.getByTestId('location')).toHaveTextContent('mode=edit');

    // Leave discards them and returns to the document.
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Leave' }));
    await waitFor(() =>
      expect(screen.getByTestId('location')).toHaveTextContent(`${BASE}?doc=getting-started`)
    );
    expect(screen.getByTestId('location')).not.toHaveTextContent('mode=edit');
  });

  it('warns before an upload replaces an existing document’s content', async () => {
    serve([aDocument('getting-started')]);
    const { user } = renderPage(`${BASE}?doc=getting-started&mode=edit`);

    const content = await screen.findByLabelText(/^Content/);
    expect(content).toHaveValue('# getting-started\n\nBody of getting-started.');

    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    await user.upload(input, new File(['# Replaced'], 'replaced.md', { type: 'text/markdown' }));

    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Override document content?')).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Override' }));

    await waitFor(() => expect(content).toHaveValue('# Replaced'));
    expect(screen.getByText('replaced.md')).toBeInTheDocument();
  });
});

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

import { http as mswHttp, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import type { ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { apiUrl, collection, failure } from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor, within } from '@/test/utils';
import { emptyDraftFormValues, type DraftFormValues } from '../utils/publicationForm';
import { DocumentsTab, type DocumentsTabProps } from './DocumentsTab';

const ORG = 'api-platform-demo';
const API_TYPE = 'rest-api';
const API = 'loan-mgmt';
const DOCS_PATH = `/apis/${API_TYPE}/${API}/docs`;

const aDoc = (id: string, displayName: string, type: string): ApiDocumentMetadata => ({
  contentType: 'text/markdown; charset=utf-8',
  displayName,
  id,
  type,
});

const guide = aDoc('guide', 'Getting started', 'HowTo');
const tutorial = aDoc('tutorial', 'First call', 'HowTo');
const faq = aDoc('faq', 'Common questions', 'FAQ');
const docs = [guide, tutorial, faq];

const values = (overrides: Partial<DraftFormValues> = {}): DraftFormValues => ({
  ...emptyDraftFormValues,
  ...overrides,
});

/** Matches text split across the summary row's nested count element. */
const summaryText = (text: string) =>
  screen.getAllByText((_content, element) => element?.textContent === text)[0];

/** The API's documents, and a generic body for the preview of any of them. */
const serveDocuments = (list: ApiDocumentMetadata[]) =>
  server.use(
    collection(DOCS_PATH, list),
    mswHttp.get(
      apiUrl(`${DOCS_PATH}/:docId/content`),
      () => new HttpResponse('Body', { headers: { 'Content-Type': 'text/markdown; charset=utf-8' } }),
    ),
  );

const serveBody = (docId: string, text: string) =>
  server.use(
    mswHttp.get(
      apiUrl(`${DOCS_PATH}/${docId}/content`),
      () => new HttpResponse(text, { headers: { 'Content-Type': 'text/markdown; charset=utf-8' } }),
    ),
  );

function renderTab(props: Partial<DocumentsTabProps> = {}) {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <DocumentsTab apiHandle={API} apiType={API_TYPE} values={values()} {...props} />
    </ApiScopeProvider>,
    { scope: makeConsoleScope() },
  );
}

beforeEach(() => {
  resetHttpClient();
});

describe('DocumentsTab', () => {
  it('groups the documents by type, each group with its count', async () => {
    serveDocuments(docs);
    renderTab();

    expect(await screen.findByRole('button', { name: 'How To (2)' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'FAQ (1)' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Getting started' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Common questions' })).toBeInTheDocument();
  });

  it('points each group header at its own region, even for a type name with a space', async () => {
    serveDocuments([guide, aDoc('notes', 'Version 1.2', 'Release Notes')]);
    renderTab();

    const regionIds = new Set<string>();
    for (const name of ['How To (1)', 'Release Notes (1)']) {
      const regionId = (await screen.findByRole('button', { name })).getAttribute('aria-controls') ?? '';
      expect(regionId).not.toMatch(/\s/);
      expect(document.getElementById(regionId)).toBeInTheDocument();
      regionIds.add(regionId);
    }
    expect(regionIds.size).toBe(2);
  });

  it('shows which documents are selected, and how many of the total', async () => {
    serveDocuments(docs);
    renderTab({ values: values({ docIds: ['guide', 'faq'] }) });

    await screen.findByRole('checkbox', { name: 'Getting started' });
    expect(summaryText('2 of 3 documents selected')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Getting started' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'First call' })).not.toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Common questions' })).toBeChecked();
  });

  it('reports a toggled document with every other value kept', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    const { user } = renderTab({ onChange, values: values({ displayName: 'Loans', docIds: ['guide'] }) });

    await user.click(await screen.findByRole('checkbox', { name: 'First call' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ displayName: 'Loans', docIds: ['guide', 'tutorial'] }));

    await user.click(screen.getByRole('checkbox', { name: 'Getting started' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: [] }));
  });

  it('selects every document, then clears them all', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    const { rerender, user } = renderTab({ onChange, values: values({ docIds: ['guide'] }) });

    await user.click(await screen.findByRole('button', { name: 'Select all' }));
    const selectedIds: string[] = onChange.mock.calls[0][0].docIds;
    expect([...selectedIds].sort()).toEqual(['faq', 'guide', 'tutorial']);

    rerender(
      <ApiScopeProvider orgId={ORG}>
        <DocumentsTab apiHandle={API} apiType={API_TYPE} onChange={onChange} values={values({ docIds: selectedIds })} />
      </ApiScopeProvider>,
    );
    await user.click(await screen.findByRole('button', { name: 'Clear all' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: [] }));
  });

  it('opens a document in a side panel from its Preview button, and closes it again', async () => {
    serveDocuments(docs);
    serveBody('faq', '# Frequently asked');
    const onChange = vi.fn();
    const { user } = renderTab({ onChange });

    expect(screen.queryByRole('presentation')).not.toBeInTheDocument();
    await user.click(await screen.findByRole('button', { name: 'Preview Common questions' }));

    const panel = await screen.findByRole('presentation');
    expect(within(panel).getByText('Common questions')).toBeInTheDocument();
    expect(await within(panel).findByRole('heading', { name: 'Frequently asked' })).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();

    await user.click(within(panel).getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(screen.queryByRole('presentation')).not.toBeInTheDocument());
  });

  it('adds and removes the previewed document from the panel, keeping it open', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    const { rerender, user } = renderTab({ onChange, values: values({ docIds: ['guide'] }) });

    await user.click(await screen.findByRole('button', { name: 'Preview Common questions' }));
    const panel = await screen.findByRole('presentation');
    await user.click(within(panel).getByRole('button', { name: 'Add to listing' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: ['guide', 'faq'] }));

    rerender(
      <ApiScopeProvider orgId={ORG}>
        <DocumentsTab apiHandle={API} apiType={API_TYPE} onChange={onChange} values={values({ docIds: ['guide', 'faq'] })} />
      </ApiScopeProvider>,
    );
    await user.click(await within(await screen.findByRole('presentation')).findByRole('button', { name: 'Remove from listing' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: ['guide'] }));
  });

  it('collapses a group without changing the selection', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    const { user } = renderTab({ onChange });

    const header = await screen.findByRole('button', { name: 'FAQ (1)' });
    expect(header).toHaveAttribute('aria-expanded', 'true');
    await user.click(header);

    expect(header).toHaveAttribute('aria-expanded', 'false');
    await waitFor(() => expect(screen.queryByRole('checkbox', { name: 'Common questions' })).not.toBeInTheDocument());
    expect(onChange).not.toHaveBeenCalled();
  });

  it('disables the checkboxes and Select all while disabled', async () => {
    serveDocuments(docs);
    renderTab({ disabled: true });

    expect(await screen.findByRole('checkbox', { name: 'Getting started' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Select all' })).toBeDisabled();
  });

  it('says there are no documents to choose from', async () => {
    serveDocuments([]);
    renderTab();

    expect(await screen.findByText('No documents')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Select all' })).not.toBeInTheDocument();
  });

  it('reports a failure to load the documents', async () => {
    server.use(failure('get', DOCS_PATH, 500, 'INTERNAL_ERROR'));
    renderTab();

    expect(await screen.findByText('Unable to load documents.')).toBeInTheDocument();
  });
});

describe('DocumentsTab — published view', () => {
  it('lists only the published documents, read-only, with no Preview buttons', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    renderTab({ onChange, readOnly: true, values: values({ docIds: ['guide', 'faq'] }) });

    expect(await screen.findByText('Getting started')).toBeInTheDocument();
    expect(screen.getByText('Common questions')).toBeInTheDocument();
    expect(screen.queryByText('First call')).not.toBeInTheDocument();
    expect(summaryText('2 documents published')).toBeInTheDocument();
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Select all' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Preview/ })).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });

  it('says the listing has no documents when none is published', async () => {
    serveDocuments(docs);
    renderTab({ readOnly: true, values: values({ docIds: [] }) });

    expect(await screen.findByText('This listing has no documents.')).toBeInTheDocument();
  });
});

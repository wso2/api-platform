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
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import type { ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { mockDataGridLayout } from '@/test/dataGridLayout';
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

beforeAll(mockDataGridLayout);

/** The grid's row for a document, found by the name shown in it. */
const rowOf = (name: string) => screen.getByText(name).closest('[role="row"]') as HTMLElement;
const boxOf = (name: string) => within(rowOf(name)).getByRole('checkbox');
/** The ids of the rows shown, top to bottom. */
const rowIds = () => Array.from(document.querySelectorAll('[role="row"][data-id]')).map((row) => row.getAttribute('data-id'));
/** The Preview side panel; the grid has presentation elements of its own, so it can't be found by role. */
const previewPanel = () => document.querySelector('.MuiDrawer-root') as HTMLElement | null;

describe('DocumentsTab', () => {
  it('lists each document with its name and type, ordered by type', async () => {
    serveDocuments(docs);
    renderTab();

    expect(await screen.findByRole('columnheader', { name: 'Name' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Type' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Actions' })).toBeInTheDocument();

    expect(within(rowOf('Getting started')).getByText('How To')).toBeInTheDocument();
    expect(within(rowOf('First call')).getByText('How To')).toBeInTheDocument();
    expect(within(rowOf('Common questions')).getByText('FAQ')).toBeInTheDocument();
    expect(rowIds()).toEqual(['guide', 'tutorial', 'faq']);
  });

  it('shows which documents are selected, and how many of the total', async () => {
    serveDocuments(docs);
    renderTab({ values: values({ docIds: ['guide', 'faq'] }) });

    await screen.findByText('Getting started');
    expect(summaryText('2 of 3 documents selected')).toBeInTheDocument();
    expect(boxOf('Getting started')).toBeChecked();
    expect(boxOf('First call')).not.toBeChecked();
    expect(boxOf('Common questions')).toBeChecked();
  });

  it('reports a toggled document with every other value kept', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    const { user } = renderTab({ onChange, values: values({ displayName: 'Loans', docIds: ['guide'] }) });

    await screen.findByText('First call');
    await user.click(boxOf('First call'));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ displayName: 'Loans', docIds: ['guide', 'tutorial'] }));

    await user.click(boxOf('Getting started'));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: [] }));
  });

  it('selects every document, then clears them all', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    const { rerender, user } = renderTab({ onChange, values: values({ docIds: ['guide'] }) });

    await user.click(await screen.findByRole('checkbox', { name: 'Select all' }));
    const selectedIds: string[] = onChange.mock.calls[0][0].docIds;
    expect([...selectedIds].sort()).toEqual(['faq', 'guide', 'tutorial']);

    rerender(
      <ApiScopeProvider orgId={ORG}>
        <DocumentsTab apiHandle={API} apiType={API_TYPE} onChange={onChange} values={values({ docIds: selectedIds })} />
      </ApiScopeProvider>,
    );
    await user.click(await screen.findByRole('checkbox', { name: 'Clear all' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: [] }));
  });

  it('keeps a selected id the list did not load when another row is toggled', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    const { user } = renderTab({ onChange, values: values({ docIds: ['guide', 'beyond'] }) });

    await screen.findByText('First call');
    await user.click(boxOf('First call'));
    expect([...onChange.mock.calls[0][0].docIds].sort()).toEqual(['beyond', 'guide', 'tutorial']);
  });

  it('opens a document in a side panel from its Preview button, and closes it again', async () => {
    serveDocuments(docs);
    serveBody('faq', '# Frequently asked');
    const onChange = vi.fn();
    const { user } = renderTab({ onChange });

    expect(previewPanel()).toBeNull();
    await user.click(await screen.findByRole('button', { name: 'Preview Common questions' }));

    const panel = (await waitFor(() => {
      const found = previewPanel();
      expect(found).not.toBeNull();
      return found;
    })) as HTMLElement;
    expect(within(panel).getByText('Common questions')).toBeInTheDocument();
    expect(await within(panel).findByRole('heading', { name: 'Frequently asked' })).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();

    await user.click(within(panel).getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(previewPanel()).toBeNull());
  });

  it('adds and removes the previewed document from the panel, keeping it open', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    const { rerender, user } = renderTab({ onChange, values: values({ docIds: ['guide'] }) });

    await user.click(await screen.findByRole('button', { name: 'Preview Common questions' }));
    await user.click(await screen.findByRole('button', { name: 'Add to listing' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: ['guide', 'faq'] }));

    rerender(
      <ApiScopeProvider orgId={ORG}>
        <DocumentsTab apiHandle={API} apiType={API_TYPE} onChange={onChange} values={values({ docIds: ['guide', 'faq'] })} />
      </ApiScopeProvider>,
    );
    await user.click(await screen.findByRole('button', { name: 'Remove from listing' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: ['guide'] }));
  });

  it('marks the select-all checkbox indeterminate for a partial selection', async () => {
    serveDocuments(docs);
    renderTab({ values: values({ docIds: ['guide'] }) });

    const selectAll = await screen.findByRole('checkbox', { name: 'Select all' });
    expect(selectAll).toHaveAttribute('data-indeterminate', 'true');
    expect(selectAll).not.toBeChecked();
  });

  it('disables the checkboxes while disabled', async () => {
    serveDocuments(docs);
    renderTab({ disabled: true });

    await screen.findByText('Getting started');
    expect(boxOf('Getting started')).toBeDisabled();
    expect(boxOf('Common questions')).toBeDisabled();
  });

  it('says there are no documents to choose from', async () => {
    serveDocuments([]);
    renderTab();

    expect(await screen.findByText('No documents')).toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: 'Select all' })).not.toBeInTheDocument();
  });

  it('reports a failure to load the documents', async () => {
    server.use(failure('get', DOCS_PATH, 500, 'INTERNAL_ERROR'));
    renderTab();

    expect(await screen.findByText('Unable to load documents.')).toBeInTheDocument();
  });
});

describe('DocumentsTab — published view', () => {
  it('lists only the published documents, every one ticked and unchangeable, with no Preview buttons', async () => {
    serveDocuments(docs);
    const onChange = vi.fn();
    const { user } = renderTab({ onChange, readOnly: true, values: values({ docIds: ['guide', 'faq'] }) });

    expect(await screen.findByText('Getting started')).toBeInTheDocument();
    expect(screen.getByText('Common questions')).toBeInTheDocument();
    expect(screen.queryByText('First call')).not.toBeInTheDocument();
    expect(summaryText('2 documents published')).toBeInTheDocument();
    const boxes = screen.getAllByRole('checkbox');
    expect(boxes).toHaveLength(3);
    for (const box of boxes) {
      expect(box).toBeChecked();
      expect(box).toBeEnabled();
    }
    expect(screen.queryByRole('columnheader', { name: 'Actions' })).not.toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Type' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Preview/ })).not.toBeInTheDocument();

    // Not disabled, which would grey the ticks: a click changes nothing.
    await user.click(boxOf('Getting started'));
    await user.click(screen.getByRole('checkbox', { name: 'Clear all' }));
    expect(onChange).not.toHaveBeenCalled();
    for (const box of screen.getAllByRole('checkbox')) expect(box).toBeChecked();
  });

  it('has the same columns in both views, except that the published one has no Actions column', async () => {
    serveDocuments(docs);
    const headerNames = () => screen.getAllByRole('columnheader').map((header) => header.textContent);

    const draft = renderTab({ values: values({ docIds: ['guide'] }) });
    await screen.findByText('Getting started');
    expect(headerNames()).toEqual(['', 'Name', 'Type', 'Actions']);
    draft.unmount();

    renderTab({ readOnly: true, values: values({ docIds: ['guide'] }) });
    await screen.findByText('Getting started');
    expect(headerNames()).toEqual(['', 'Name', 'Type']);
    expect(within(rowOf('Getting started')).getAllByRole('gridcell')).toHaveLength(3);
  });

  it('is one grid with its own header row, which stays put while the rows scroll, and the summary outside it', async () => {
    serveDocuments(docs);
    renderTab({ values: values({ docIds: ['guide'] }) });

    const grid = await screen.findByRole('grid');
    const summary = summaryText('1 of 3 documents selected');
    const root = grid.closest('.MuiDataGrid-root') as HTMLElement;
    expect(root).not.toBeNull();
    expect(root.querySelector('.MuiDataGrid-columnHeaders')).not.toBeNull();
    expect(root.contains(summary)).toBe(false);
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('says the listing has no documents when none is published', async () => {
    serveDocuments(docs);
    renderTab({ readOnly: true, values: values({ docIds: [] }) });

    expect(await screen.findByText('This listing has no documents.')).toBeInTheDocument();
  });

  describe('deleted and edited documents', () => {
    const gone = [{ id: 'gone', displayName: 'Old guide' }];

    it('drops a deleted document from the draft, in the list and the count', async () => {
      serveDocuments(docs);
      renderTab({ deletedDocuments: gone, values: values({ docIds: ['guide'] }) });

      await screen.findByText('Getting started');
      expect(screen.queryByText('Old guide')).not.toBeInTheDocument();
      expect(rowIds()).toEqual(['guide', 'tutorial', 'faq']);
      expect(summaryText('1 of 3 documents selected')).toBeInTheDocument();
    });

    it('Select all and Clear all touch only existing documents', async () => {
      serveDocuments(docs);
      const onChange = vi.fn();
      const { user } = renderTab({ deletedDocuments: gone, onChange, values: values() });

      await user.click(await screen.findByRole('checkbox', { name: 'Select all' }));
      expect(onChange.mock.calls[0][0].docIds.sort()).toEqual(['faq', 'guide', 'tutorial']);
    });

    it('read-only: lists a deleted document first, struck through under its stored name, with a tooltip and no chip or notice', async () => {
      serveDocuments(docs);
      const { user } = renderTab({ deletedDocuments: gone, readOnly: true, values: values({ docIds: ['guide'] }) });

      const name = await screen.findByText('Old guide');
      const row = name.closest('[role="row"]') as HTMLElement;
      expect(name.tagName).toBe('DEL');
      expect(rowIds()[0]).toBe('gone');
      expect(within(row).getByText('—')).toBeInTheDocument();
      expect(within(row).queryByRole('button')).not.toBeInTheDocument();
      expect(row).not.toHaveTextContent('Deleted');
      expect(row).not.toHaveTextContent('after last publish');
      expect(screen.getByText('Getting started').tagName).not.toBe('DEL');
      // Greyed (disabled) but still ticked, since the listing holds it; the others keep their read-only ticks.
      expect(boxOf('Old guide')).toBeDisabled();
      expect(boxOf('Old guide')).toBeChecked();
      expect(boxOf('Getting started')).not.toBeDisabled();
      await user.hover(name);
      expect(await screen.findByRole('tooltip')).toHaveTextContent(
        'This document was deleted. Republish to remove it from the listing.',
      );
    });

    it('read-only: counts a deleted document as published, shows stored names, and tags an edited one', async () => {
      serveDocuments(docs);
      renderTab({
        deletedDocuments: gone,
        editedDocIds: ['faq'],
        readOnly: true,
        storedNames: { faq: 'Questions (published name)' },
        values: values({ docIds: ['guide', 'faq'] }),
      });

      expect(await screen.findByText('Old guide')).toBeInTheDocument();
      expect(summaryText('3 documents published')).toBeInTheDocument();
      expect(rowOf('Questions (published name)')).toHaveTextContent('Edited');
      expect(rowOf('Getting started')).not.toHaveTextContent('Edited');
      for (const box of screen.getAllByRole('checkbox')) expect(box).toBeChecked();
    });

    it('read-only: puts an outlined Edited chip, with no icon, right after the name, with a tooltip and no notice', async () => {
      serveDocuments(docs);
      const { user } = renderTab({ editedDocIds: ['faq'], readOnly: true, values: values({ docIds: ['guide', 'faq'] }) });

      const name = await screen.findByText('Common questions');
      const row = rowOf('Common questions');
      const chip = within(row).getByText('Edited').closest('.MuiChip-root') as HTMLElement;
      expect(chip.className).toContain('MuiChip-outlined');
      expect(chip.querySelector('svg')).toBeNull();
      expect(name.nextElementSibling).toBe(chip);
      expect(row).not.toHaveTextContent('after last publish');

      await user.hover(chip);
      expect(await screen.findByRole('tooltip')).toHaveTextContent(
        'Document was edited after publishing. Republish to apply changes.',
      );
    });

    it('read-only: orders deleted rows first, then edited ones, then the rest, each in type order', async () => {
      serveDocuments([...docs, aDoc('sample', 'Sample call', 'Samples'), aDoc('notes', 'Notes', 'FAQ')]);
      renderTab({
        deletedDocuments: gone,
        editedDocIds: ['faq', 'sample'],
        readOnly: true,
        values: values({ docIds: ['guide', 'tutorial', 'faq', 'sample', 'notes'] }),
      });

      await screen.findByText('Old guide');
      expect(rowIds()).toEqual(['gone', 'sample', 'faq', 'guide', 'tutorial', 'notes']);
    });

    it('never shows Edited in the draft', async () => {
      serveDocuments(docs);
      renderTab({ editedDocIds: ['guide'], values: values({ docIds: ['guide'] }) });

      await screen.findByText('Getting started');
      expect(rowOf('Getting started')).not.toHaveTextContent('Edited');
    });

    it('read-only: a listing holding only a deleted document is not shown as empty', async () => {
      serveDocuments(docs);
      renderTab({ deletedDocuments: gone, readOnly: true, values: values() });

      expect(await screen.findByText('Old guide')).toBeInTheDocument();
      expect(screen.queryByText('This listing has no documents.')).not.toBeInTheDocument();
    });
  });
});

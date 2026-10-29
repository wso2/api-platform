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

import { describe, expect, it, vi } from 'vitest';

import { emptyDraftFormValues } from './mcpPublishForm';
import { renderWithProviders, screen, waitFor, within } from '../../../../test/utils';
import { DocumentsTab } from './DocumentsTab';

vi.mock('./DocumentPreview', () => ({
  DocumentPreview: ({ docId, mcpProxyId }: { docId: string; mcpProxyId: string }) => (
    <div data-testid="preview">{`${mcpProxyId}/${docId}`}</div>
  ),
}));

const documents = [
  { id: 'setup', type: 'HowTo', displayName: 'Setup guide' },
  { id: 'faq', type: 'FAQ', displayName: 'FAQ list' },
  { id: 'sdk', type: 'Samples', displayName: 'Client SDK' },
];
const values = (docIds: string[]) => ({ ...emptyDraftFormValues, docIds });

describe('DocumentsTab', () => {
  it('shows a loader while the documents are on their way, and says so when they cannot be read', () => {
    const { rerender } = renderWithProviders(<DocumentsTab mcpProxyId="travel" values={values([])} />);
    expect(screen.getByText('Loading documents')).toBeInTheDocument();

    rerender(<DocumentsTab mcpProxyId="travel" failed values={values([])} />);
    expect(screen.getByText('Unable to load documents.')).toBeInTheDocument();
  });

  it('says there is nothing to choose from when the proxy has no documents', () => {
    renderWithProviders(<DocumentsTab mcpProxyId="travel" documents={[]} values={values([])} />);

    expect(screen.getByText('No documents')).toBeInTheDocument();
  });

  it('lists each document with its name and type, ordered by type', () => {
    renderWithProviders(<DocumentsTab mcpProxyId="travel" documents={documents} values={values([])} />);

    expect(screen.getByRole('columnheader', { name: 'Name' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Type' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Actions' })).toBeInTheDocument();

    const rowOf = (name: string) => screen.getByRole('checkbox', { name }).closest('tr') as HTMLElement;
    expect(within(rowOf('Setup guide')).getByText('How To')).toBeInTheDocument();
    expect(within(rowOf('FAQ list')).getByText('FAQ')).toBeInTheDocument();
    expect(within(rowOf('Client SDK')).getByText('Samples & SDK')).toBeInTheDocument();

    const order = screen.getAllByRole('row').slice(1).map((row) => within(row).getByRole('checkbox').getAttribute('aria-label'));
    expect(order).toEqual(['Setup guide', 'Client SDK', 'FAQ list']);
    // Rows highlight on hover, as the control plane's document table does.
    screen.getAllByRole('row').slice(1).forEach((row) => expect(row).toHaveClass('MuiTableRow-hover'));
  });

  it('shows which documents are selected, and how many of the total', () => {
    renderWithProviders(<DocumentsTab mcpProxyId="travel" documents={documents} values={values(['setup'])} />);

    expect(screen.getByRole('checkbox', { name: 'Setup guide' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'FAQ list' })).not.toBeChecked();
    expect(screen.getByText(/of 3 documents selected/)).toBeInTheDocument();
  });

  it('adds and removes a document from the selection, keeping everything else', async () => {
    const onChange = vi.fn();
    const { user } = renderWithProviders(
      <DocumentsTab mcpProxyId="travel" documents={documents} onChange={onChange} values={{ ...values(['setup']), displayName: 'Travel' }} />
    );

    await user.click(screen.getByRole('checkbox', { name: 'FAQ list' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ displayName: 'Travel', docIds: ['setup', 'faq'] }));

    await user.click(screen.getByRole('checkbox', { name: 'Setup guide' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: [] }));
  });

  it('selects every document, then clears them', async () => {
    const onChange = vi.fn();
    const { user, rerender } = renderWithProviders(
      <DocumentsTab mcpProxyId="travel" documents={documents} onChange={onChange} values={values([])} />
    );

    await user.click(screen.getByRole('checkbox', { name: 'Select all' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: ['setup', 'faq', 'sdk'] }));

    rerender(<DocumentsTab mcpProxyId="travel" documents={documents} onChange={onChange} values={values(['setup', 'faq', 'sdk'])} />);
    await user.click(screen.getByRole('checkbox', { name: 'Clear all' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: [] }));
  });

  it('keeps a selected id the list did not load when another row is toggled', async () => {
    const onChange = vi.fn();
    const { user } = renderWithProviders(
      <DocumentsTab mcpProxyId="travel" documents={documents} onChange={onChange} values={values(['setup', 'beyond'])} />
    );

    await user.click(screen.getByRole('checkbox', { name: 'FAQ list' }));

    expect([...onChange.mock.calls[0][0].docIds].sort()).toEqual(['beyond', 'faq', 'setup']);
  });

  it('keeps a selected id the list did not load when every document is selected or cleared', async () => {
    const onChange = vi.fn();
    const { user, rerender } = renderWithProviders(
      <DocumentsTab mcpProxyId="travel" documents={documents} onChange={onChange} values={values(['beyond'])} />
    );

    await user.click(screen.getByRole('checkbox', { name: 'Select all' }));
    expect([...onChange.mock.calls[0][0].docIds].sort()).toEqual(['beyond', 'faq', 'sdk', 'setup']);

    rerender(
      <DocumentsTab mcpProxyId="travel" documents={documents} onChange={onChange} values={values(['setup', 'faq', 'sdk', 'beyond'])} />
    );
    await user.click(screen.getByRole('checkbox', { name: 'Clear all' }));
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: ['beyond'] }));
  });

  it('marks the select-all checkbox indeterminate for a partial selection', () => {
    renderWithProviders(<DocumentsTab mcpProxyId="travel" documents={documents} values={values(['setup'])} />);

    const selectAll = screen.getByRole('checkbox', { name: 'Select all' });
    expect(selectAll).toHaveAttribute('data-indeterminate', 'true');
    expect(selectAll).not.toBeChecked();
  });

  it('locks the checkboxes while an action is running', () => {
    renderWithProviders(<DocumentsTab mcpProxyId="travel" disabled documents={documents} values={values([])} />);

    expect(screen.getByRole('checkbox', { name: 'Setup guide' })).toBeDisabled();
    expect(screen.getByRole('checkbox', { name: 'Select all' })).toBeDisabled();
  });

  describe('previewing', () => {
    it('opens a document’s content beside the list, for the proxy it belongs to', async () => {
      const { user } = renderWithProviders(<DocumentsTab mcpProxyId="travel" documents={documents} values={values([])} />);

      await user.click(screen.getByRole('button', { name: 'Preview FAQ list' }));

      const drawer = await screen.findByRole('presentation');
      expect(within(drawer).getByRole('heading', { name: 'FAQ list' })).toBeInTheDocument();
      expect(within(drawer).getByTestId('preview')).toHaveTextContent('travel/faq');
    });

    it('adds the document being read to the listing from the preview, keeping the rest of the form', async () => {
      const onChange = vi.fn();
      const { user } = renderWithProviders(
        <DocumentsTab mcpProxyId="travel" documents={documents} onChange={onChange} values={{ ...values(['setup']), displayName: 'Travel' }} />
      );
      await user.click(screen.getByRole('button', { name: 'Preview FAQ list' }));

      await user.click(await screen.findByRole('button', { name: 'Add to listing' }));

      expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ displayName: 'Travel', docIds: ['setup', 'faq'] }));
    });

    it('offers to remove a document that is already in the listing', async () => {
      const onChange = vi.fn();
      const { user } = renderWithProviders(
        <DocumentsTab mcpProxyId="travel" documents={documents} onChange={onChange} values={values(['faq'])} />
      );
      await user.click(screen.getByRole('button', { name: 'Preview FAQ list' }));

      await user.click(await screen.findByRole('button', { name: 'Remove from listing' }));

      expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ docIds: [] }));
    });

    it('locks the add and remove button while an action is running', async () => {
      const { user } = renderWithProviders(
        <DocumentsTab mcpProxyId="travel" disabled documents={documents} values={values([])} />
      );
      await user.click(screen.getByRole('button', { name: 'Preview FAQ list' }));

      expect(await screen.findByRole('button', { name: 'Add to listing' })).toBeDisabled();
    });

    it('closes again, leaving the selection as it was', async () => {
      const onChange = vi.fn();
      const { user } = renderWithProviders(
        <DocumentsTab mcpProxyId="travel" documents={documents} onChange={onChange} values={values([])} />
      );
      await user.click(screen.getByRole('button', { name: 'Preview FAQ list' }));

      await user.click(await screen.findByRole('button', { name: 'Close' }));

      await waitFor(() => expect(screen.queryByTestId('preview')).not.toBeInTheDocument());
      expect(onChange).not.toHaveBeenCalled();
    });

    it('is not offered in the published view', () => {
      renderWithProviders(<DocumentsTab mcpProxyId="travel" documents={documents} readOnly values={values(['faq'])} />);

      expect(screen.queryByRole('button', { name: /^Preview/ })).not.toBeInTheDocument();
    });
  });

  describe('as the published view', () => {
    it('lists only the published documents, each ticked and with no way to change it', () => {
      const { container } = renderWithProviders(
        <DocumentsTab mcpProxyId="travel" documents={documents} readOnly values={values(['faq', 'sdk'])} />
      );

      expect(screen.getByText('FAQ list')).toBeInTheDocument();
      expect(screen.getByText('Client SDK')).toBeInTheDocument();
      expect(screen.queryByText('Setup guide')).not.toBeInTheDocument();
      const boxes = screen.getAllByRole('checkbox');
      expect(boxes).toHaveLength(2);
      boxes.forEach((box) => expect(box).toBeChecked());
      expect(screen.queryByRole('checkbox', { name: 'Select all' })).not.toBeInTheDocument();
      // The header keeps the draft's box, ticked and hidden from assistive technology.
      expect(container.querySelector('thead input[type="checkbox"]')).toBeChecked();
      // Every listed row is shown selected, as in the control plane's published view.
      screen.getAllByRole('row').slice(1).forEach((row) => expect(row).toHaveClass('Mui-selected'));
      expect(screen.queryByRole('columnheader', { name: 'Actions' })).not.toBeInTheDocument();
      expect(screen.getByRole('columnheader', { name: 'Type' })).toBeInTheDocument();
      expect(screen.getByText(/2/)).toBeInTheDocument();
    });

    it('shows the names stored at the last publish, not the current ones', () => {
      renderWithProviders(
        <DocumentsTab
          documents={documents}
          mcpProxyId="travel"
          readOnly
          storedNames={{ faq: 'FAQ (old name)' }}
          values={values(['faq'])}
        />
      );

      expect(screen.getByText('FAQ (old name)')).toBeInTheDocument();
      expect(screen.queryByText('FAQ list')).not.toBeInTheDocument();
    });

    it('shows a deleted document under its stored name, struck through, first, with its checkbox greyed and no known type', () => {
      renderWithProviders(
        <DocumentsTab
          deletedDocuments={[{ id: 'gone', displayName: 'Old guide' }]}
          documents={documents}
          mcpProxyId="travel"
          readOnly
          values={values(['faq'])}
        />
      );

      const name = screen.getByText('Old guide');
      expect(name.tagName).toBe('DEL');
      const rows = screen.getAllByRole('row');
      expect(rows[1]).toHaveTextContent('Old guide');
      expect(rows[1]).toHaveTextContent('—');
      expect(screen.getByRole('checkbox', { name: 'Old guide' })).toBeDisabled();
      expect(screen.getByRole('checkbox', { name: 'Old guide' })).toBeChecked();
      expect(screen.getByText(/2/)).toBeInTheDocument();
    });

    it('is not shown as empty when the listing holds only a deleted document', () => {
      renderWithProviders(
        <DocumentsTab
          deletedDocuments={[{ id: 'gone', displayName: 'Old guide' }]}
          documents={documents}
          mcpProxyId="travel"
          readOnly
          values={values([])}
        />
      );

      expect(screen.queryByText('This listing has no documents.')).not.toBeInTheDocument();
      expect(screen.getByText('Old guide')).toBeInTheDocument();
    });

    it('tags an edited document with an outlined chip and lists it ahead of the rest', () => {
      renderWithProviders(
        <DocumentsTab documents={documents} editedDocIds={['sdk']} mcpProxyId="travel" readOnly values={values(['faq', 'sdk'])} />
      );

      const rows = screen.getAllByRole('row');
      expect(rows[1]).toHaveTextContent('Client SDK');
      expect(rows[1]).toHaveTextContent('Edited');
      expect(rows[2]).not.toHaveTextContent('Edited');
    });

    it('draft: shows no deleted document and no chip, whatever it is given', () => {
      renderWithProviders(
        <DocumentsTab
          deletedDocuments={[{ id: 'gone', displayName: 'Old guide' }]}
          documents={documents}
          editedDocIds={['faq']}
          mcpProxyId="travel"
          values={values(['faq'])}
        />
      );

      expect(screen.queryByText('Old guide')).not.toBeInTheDocument();
      expect(screen.queryByText('Edited')).not.toBeInTheDocument();
    });

    it('says the listing has no documents when none are published', () => {
      renderWithProviders(<DocumentsTab mcpProxyId="travel" documents={documents} readOnly values={values([])} />);

      expect(screen.getByText('This listing has no documents.')).toBeInTheDocument();
    });
  });
});

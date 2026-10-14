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
import { waitFor } from '@testing-library/react';

import type { ApiDocumentMetadata } from '@/api/resources/apiDocuments';
import { renderWithProviders, screen } from '@/test/utils';
import { DocumentList } from './DocumentList';

/**
 * DocumentList is pure (no network, no routing). These tests cover the
 * behaviours end-to-end panel tests cannot see cleanly:
 *
 *   1. Grouping — fixed types in canonical order, custom types alphabetical,
 *      plain Other last; empty buckets are skipped.
 *   2. Collapse — the subheader is a real button; clicking it hides the group.
 *   3. Selection — the matching row carries `aria-current="true"`.
 *   4. The load-more sentinel and loading banner appear only when the caller
 *      signals it; a fully loaded list has neither.
 *
 * The IntersectionObserver → onLoadMore wiring itself is covered by the
 * develop-tab panel test end-to-end.
 */

const HEIGHT = { md: '600px', xs: '400px' } as const;

const aDoc = (id: string, overrides: Partial<ApiDocumentMetadata> = {}): ApiDocumentMetadata => ({
  displayName: id,
  id,
  type: 'HowTo',
  updatedAt: '2026-09-28T10:00:00Z',
  updatedBy: 'admin',
  ...overrides,
});

function renderList(props: Partial<React.ComponentProps<typeof DocumentList>> = {}) {
  const onSelect = vi.fn();
  const onLoadMore = vi.fn();
  const utils = renderWithProviders(
    <DocumentList
      documents={props.documents ?? []}
      height={HEIGHT}
      total={props.total ?? props.documents?.length ?? 0}
      hasMore={props.hasMore ?? false}
      loadingMore={props.loadingMore ?? false}
      onLoadMore={onLoadMore}
      onSelect={onSelect}
      selectedId={props.selectedId}
    />,
  );
  return { ...utils, onSelect, onLoadMore };
}

describe('DocumentList — grouping and order', () => {
  it('fixed groups in canonical order, custom types alphabetically, plain Other last', async () => {
    // One doc per type, mixed insertion order — the output order depends on
    // the component's logic, not the input array.
    const docs: ApiDocumentMetadata[] = [
      aDoc('o-plain', { type: 'Other' }),
      aDoc('zebra-faq', { type: 'Zebra' }), // custom
      aDoc('alpha-faq', { type: 'Alpha' }), // custom
      aDoc('sf-1', { type: 'SupportForum' }),
      aDoc('samples-1', { type: 'Samples' }),
      aDoc('howto-1', { type: 'HowTo' }),
      aDoc('pf-1', { type: 'PublicForum' }),
    ];
    renderList({ documents: docs });

    const headers = screen
      .getAllByRole('button', { name: /\(1\)$/ })
      .map((btn) => btn.getAttribute('aria-label'));

    expect(headers).toEqual([
      'How To (1)',
      'Samples & SDK (1)',
      'Public Forum (1)',
      'Support Forum (1)',
      'Alpha (1)',
      'Zebra (1)',
      'Other (1)',
    ]);
  });

  it('does not render an empty group header', async () => {
    renderList({ documents: [aDoc('one', { type: 'HowTo' })] });

    expect(screen.getByRole('button', { name: /How To \(1\)/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Samples &/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Public Forum/ })).not.toBeInTheDocument();
  });
});

describe('DocumentList — group collapse', () => {
  it('clicking a group header toggles aria-expanded and hides its items', async () => {
    const docs = [aDoc('a', { type: 'HowTo' }), aDoc('b', { type: 'HowTo' })];
    const { user } = renderList({ documents: docs });

    const header = await screen.findByRole('button', { name: /How To \(2\)/ });
    expect(header).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('button', { name: /^a Updated/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^b Updated/ })).toBeInTheDocument();

    await user.click(header);
    expect(header).toHaveAttribute('aria-expanded', 'false');
    // Collapse plays an exit transition; wait for the items to leave the DOM.
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /^a Updated/ })).not.toBeInTheDocument(),
    );
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /^b Updated/ })).not.toBeInTheDocument(),
    );
  });

  it('each group collapses independently', async () => {
    const docs = [aDoc('h1', { type: 'HowTo' }), aDoc('s1', { type: 'Samples' })];
    const { user } = renderList({ documents: docs });

    await user.click(await screen.findByRole('button', { name: /How To \(1\)/ }));

    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /^h1 Updated/ })).not.toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: /^s1 Updated/ })).toBeInTheDocument();
  });
});

describe('DocumentList — selection', () => {
  it('marks the selected row with aria-current="true" and leaves the rest unmarked', async () => {
    renderList({ documents: [aDoc('one'), aDoc('two')], selectedId: 'two' });

    const one = await screen.findByRole('button', { name: /^one Updated/ });
    const two = await screen.findByRole('button', { name: /^two Updated/ });
    expect(one).not.toHaveAttribute('aria-current');
    expect(two).toHaveAttribute('aria-current', 'true');
  });

  it('calls onSelect with the clicked document id', async () => {
    const { user, onSelect } = renderList({ documents: [aDoc('target')] });

    await user.click(await screen.findByRole('button', { name: /target/ }));
    expect(onSelect).toHaveBeenCalledWith('target');
  });
});

describe('DocumentList — load-more state', () => {
  it('renders neither sentinel nor banner when the list is fully loaded', async () => {
    renderList({ documents: [aDoc('one')], hasMore: false, loadingMore: false });

    expect(screen.queryByText(/Loading documents…/)).not.toBeInTheDocument();
    expect(document.querySelector('li[aria-hidden="true"]')).toBeNull();
  });

  it('renders the loading banner while the next page is in flight', async () => {
    renderList({ documents: [aDoc('one')], hasMore: true, loadingMore: true });

    expect(await screen.findByText(/Loading documents…/)).toBeInTheDocument();
  });

  it('renders the IntersectionObserver sentinel when there is more to load but no fetch is pending', async () => {
    renderList({ documents: [aDoc('one')], hasMore: true, loadingMore: false, total: 50 });

    // The sentinel is a 1px-tall list item with aria-hidden; used only as the
    // observer target, kept off the a11y tree.
    const sentinel = document.querySelector('li[aria-hidden="true"]');
    expect(sentinel).not.toBeNull();
    expect(screen.queryByText(/Loading documents…/)).not.toBeInTheDocument();
  });
});

describe('DocumentList — total line', () => {
  it('reports the server-side total, not the loaded length', async () => {
    renderList({ documents: [aDoc('one'), aDoc('two')], total: 50, hasMore: true });
    expect(screen.getByText(/50 documents/)).toBeInTheDocument();
  });

  it('reports the singular form for exactly one document', async () => {
    renderList({ documents: [aDoc('one')], total: 1 });
    expect(screen.getByText(/^1 document$/)).toBeInTheDocument();
  });
});

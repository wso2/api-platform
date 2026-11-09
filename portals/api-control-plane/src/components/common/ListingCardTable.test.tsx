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

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen, within } from '@/test/utils';
import { ListingCardTable, type ListingCardColumn } from './ListingCardTable';

type Row = { id: string; name: string; kind: string };

const rows: Row[] = [
  { id: 'a', name: 'Orders', kind: 'REST' },
  { id: 'b', name: 'Inventory', kind: 'GraphQL' },
];

const columns: ListingCardColumn<Row>[] = [
  { id: 'name', header: 'Name', render: (row) => row.name },
  { id: 'kind', header: 'Kind', render: (row) => row.kind, hideOnMobile: true },
];

function renderTable(onRowOpen?: (row: Row) => void) {
  return renderWithProviders(
    <ListingCardTable
      columns={columns}
      data-testid="table"
      getRowId={(row) => row.id}
      getRowLabel={(row) => row.name}
      onRowOpen={onRowOpen}
      rows={rows}
    />,
  );
}

describe('ListingCardTable', () => {
  it('renders a header cell per column and a row per record', () => {
    renderTable();

    const table = screen.getByTestId('table');
    expect(within(table).getByText('Name')).toBeInTheDocument();
    expect(within(table).getByText('Kind')).toBeInTheDocument();
    // One header row plus one row per record.
    expect(within(table).getAllByRole('row')).toHaveLength(rows.length + 1);
    expect(within(table).getByText('Orders')).toBeInTheDocument();
    expect(within(table).getByText('GraphQL')).toBeInTheDocument();
  });

  it('renders each cell with its column renderer', () => {
    renderTable();

    const inventory = screen.getByText('Inventory').closest('tr') as HTMLElement;
    expect(within(inventory).getByText('GraphQL')).toBeInTheDocument();
    expect(within(inventory).queryByText('REST')).not.toBeInTheDocument();
  });

  it('opens a row on click', async () => {
    const onRowOpen = vi.fn();
    const { user } = renderTable(onRowOpen);

    await user.click(screen.getByRole('button', { name: 'Open Orders' }));

    expect(onRowOpen).toHaveBeenCalledWith(rows[0]);
  });

  it.each([['{Enter}'], [' ']])('opens a focused row with %j', async (key) => {
    const onRowOpen = vi.fn();
    const { user } = renderTable(onRowOpen);

    screen.getByRole('button', { name: 'Open Inventory' }).focus();
    await user.keyboard(key);

    expect(onRowOpen).toHaveBeenCalledOnce();
    expect(onRowOpen).toHaveBeenCalledWith(rows[1]);
  });

  it('does not make rows buttons when nothing opens them', () => {
    renderTable();

    expect(screen.queryByRole('button', { name: /^Open / })).not.toBeInTheDocument();
  });

  it('lets a control inside a cell take its own click without opening the row', async () => {
    const onRowOpen = vi.fn();
    const onDelete = vi.fn();
    const { user } = renderWithProviders(
      <ListingCardTable
        columns={[
          {
            id: 'name',
            header: 'Name',
            render: (row: Row) => (
              <button
                onClick={(event) => {
                  event.stopPropagation();
                  onDelete(row);
                }}
                type="button"
              >
                Delete {row.name}
              </button>
            ),
          },
        ]}
        getRowId={(row) => row.id}
        getRowLabel={(row) => row.name}
        onRowOpen={onRowOpen}
        rows={rows}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Delete Orders' }));

    expect(onDelete).toHaveBeenCalledWith(rows[0]);
    expect(onRowOpen).not.toHaveBeenCalled();
  });

  it('ignores a keypress that bubbles up from a control inside the row', async () => {
    const onRowOpen = vi.fn();
    const { user } = renderWithProviders(
      <ListingCardTable
        columns={[
          {
            id: 'name',
            header: 'Name',
            // A native button would turn Enter into a click; a bare focusable shows the key path.
            render: (row: Row) => <span tabIndex={0}>Menu {row.name}</span>,
          },
        ]}
        getRowId={(row) => row.id}
        getRowLabel={(row) => row.name}
        onRowOpen={onRowOpen}
        rows={rows}
      />,
    );

    screen.getByText('Menu Orders').focus();
    await user.keyboard('{Enter}');

    expect(onRowOpen).not.toHaveBeenCalled();
  });
});

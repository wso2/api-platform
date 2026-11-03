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

import type { ReactNode } from 'react';
import { ListingTable, type Theme } from '@wso2/oxygen-ui';
import { useIntl } from 'react-intl';

import { openableProps } from '@/components/openable';
import { focusRingSx, hairline } from '@/theme';

export type ListingCardColumn<T> = {
  /** Stable key for the column. */
  id: string;
  /** Header cell content (already translated). */
  header: ReactNode;
  /** Cell content for one row. */
  render: (row: T) => ReactNode;
  align?: 'left' | 'center' | 'right';
  /** CSS width of the column; omit to share the remaining space. */
  width?: number | string;
  /** Hide the column below the `md` breakpoint. */
  hideOnMobile?: boolean;
};

export type ListingCardTableProps<T> = {
  columns: ListingCardColumn<T>[];
  rows: T[];
  getRowId: (row: T) => string;
  /** Accessible name of a row, used for its "Open {name}" label. */
  getRowLabel: (row: T) => string;
  onRowOpen?: (row: T) => void;
  /** Class of a cell control that stays hidden until its row is hovered or focused. */
  revealOnHoverClassName?: string;
  'data-testid'?: string;
};

const edge = (theme: Theme) => `${hairline(theme)} ${(theme.vars ?? theme).palette.divider}`;

const hideOnMobileSx = { display: { md: 'table-cell', xs: 'none' } } as const;

/**
 * Card-style listing: a header row over one rounded card per record, built on
 * Oxygen's `ListingTable` with `variant="card"`.
 *
 * Rows are keyboard-openable via `openableProps`. A control inside a cell (a
 * delete button, say) must stop its own click from bubbling to the row.
 */
export function ListingCardTable<T>({
  columns,
  rows,
  getRowId,
  getRowLabel,
  onRowOpen,
  revealOnHoverClassName = 'listing-row-action',
  'data-testid': testId,
}: ListingCardTableProps<T>) {
  const intl = useIntl();

  return (
    <ListingTable.Container data-testid={testId} disablePaper>
      <ListingTable variant="card">
        <ListingTable.Head>
          <ListingTable.Row>
            {columns.map((column) => (
              <ListingTable.Cell
                align={column.align}
                key={column.id}
                sx={{
                  width: column.width,
                  ...(column.hideOnMobile ? hideOnMobileSx : {}),
                }}
              >
                {column.header}
              </ListingTable.Cell>
            ))}
          </ListingTable.Row>
        </ListingTable.Head>
        <ListingTable.Body>
          {rows.map((row) => (
            <ListingTable.Row
              {...(onRowOpen
                ? openableProps(intl, getRowLabel(row), () => onRowOpen(row))
                : {})}
              clickable={Boolean(onRowOpen)}
              key={getRowId(row)}
              sx={(theme) => ({
                ...focusRingSx(theme),
                // Tighter than Oxygen's 16px "standard" density, which is a little tall for these rows.
                '& > .MuiTableCell-root': { py: 1.25 },
                // Oxygen's own card border is a faint white wash that is invisible on
                // the dark Acrylic background, so dark mode sets its own edge and fill.
                // Light mode keeps Oxygen's default.
                ...theme.applyStyles('dark', {
                  bgcolor: 'action.hover',
                  // Colour is part of every shorthand: a bare `1px solid` would
                  // reset the side to currentColor (the bright text colour).
                  '& .MuiTableCell-root': {
                    borderBottom: edge(theme),
                    borderTop: edge(theme),
                    '&:first-of-type': { borderLeft: edge(theme) },
                    '&:last-of-type': { borderRight: edge(theme) },
                  },
                  '&:hover': { bgcolor: 'action.selected' },
                }),
                [`&:focus-within .${revealOnHoverClassName}, &:hover .${revealOnHoverClassName}`]: {
                  opacity: 1,
                },
              })}
              variant="card"
            >
              {columns.map((column) => (
                <ListingTable.Cell
                  align={column.align}
                  key={column.id}
                  sx={column.hideOnMobile ? hideOnMobileSx : undefined}
                >
                  {column.render(row)}
                </ListingTable.Cell>
              ))}
            </ListingTable.Row>
          ))}
        </ListingTable.Body>
      </ListingTable>
    </ListingTable.Container>
  );
}

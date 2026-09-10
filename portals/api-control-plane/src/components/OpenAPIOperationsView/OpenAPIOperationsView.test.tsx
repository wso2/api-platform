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

import { renderWithProviders, screen } from '@/test/utils';
import { OpenAPIOperationsView } from './OpenAPIOperationsView';

describe('OpenAPIOperationsView', () => {
  it('renders method, path, and available descriptions from API operations', () => {
    renderWithProviders(
      <OpenAPIOperationsView
        operations={[
          {
            name: 'listBooks',
            description: 'List all the reading list books',
            request: { method: 'GET', path: '/books' },
          },
          { name: 'addBook', request: { method: 'POST', path: '/books' } },
        ]}
      />,
    );

    expect(screen.getByText('GET')).toBeInTheDocument();
    expect(screen.getByText('POST')).toBeInTheDocument();
    expect(screen.getAllByText('/books')).toHaveLength(2);
    expect(screen.getByText('List all the reading list books')).toBeInTheDocument();
  });

  it('renders an empty state when the API has no operations', () => {
    renderWithProviders(<OpenAPIOperationsView operations={[]} />);
    expect(screen.getByText('No operations available.')).toBeInTheDocument();
  });

  it('allows an editable view to delete an operation', async () => {
    const onDelete = vi.fn();
    const { user } = renderWithProviders(
      <OpenAPIOperationsView
        onDelete={onDelete}
        operations={[{ name: 'getBook', request: { method: 'GET', path: '/books/{id}' } }]}
        showDelete
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Delete GET /books/{id}' }));
    expect(onDelete).toHaveBeenCalledWith(0);
  });

  it('only shows delete controls when requested and disables staged deletions', () => {
    const operation = { name: 'getBook', request: { method: 'GET' as const, path: '/books' } };
    const { rerender } = renderWithProviders(<OpenAPIOperationsView operations={[operation]} />);

    expect(screen.queryByRole('button', { name: 'Delete GET /books' })).not.toBeInTheDocument();

    rerender(
      <OpenAPIOperationsView
        isOperationDisabled={() => true}
        onDelete={vi.fn()}
        operations={[operation]}
        showDelete
      />,
    );
    expect(screen.getByRole('button', { name: 'Delete GET /books' })).toBeDisabled();
  });
});

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

import type { UseQueryResult } from '@tanstack/react-query';
import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { components as mockComponents } from '../../../../api/mocks/data';
import { renderWithProviders, screen } from '../../../../test/utils';
import type { Api } from '../../../../types/domain';

vi.mock('../../api/hooks/useMvpQueries', async (importActual) => ({
  ...(await importActual<typeof import('../../../../api/hooks/useMvpQueries')>()),
  useApis: vi.fn(),
  useDeleteApi: vi.fn(),
}));

import { useApis, useDeleteApi } from '../../../../api/hooks/useMvpQueries';
import { ApiListPage } from './ApiListPage';

const ROUTE = '/organizations/acme/projects/retail/apis';

const queryResult = (overrides: Partial<UseQueryResult<Api[], Error>>) =>
  overrides as UseQueryResult<Api[], Error>;

function renderPage() {
  return renderWithProviders(
    <Routes>
      <Route
        path="/organizations/:orgHandle/projects/:projectHandler/apis"
        element={<ApiListPage />}
      />
    </Routes>,
    { route: ROUTE }
  );
}

describe('ApiListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Most tests don't exercise delete; provide a default mutation.
    vi.mocked(useDeleteApi).mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useDeleteApi>);
  });

  it('shows the loading state', () => {
    vi.mocked(useApis).mockReturnValue(queryResult({ isLoading: true }));
    renderPage();
    expect(screen.getByText('Loading APIs')).toBeInTheDocument();
  });

  it('shows the error state', () => {
    vi.mocked(useApis).mockReturnValue(
      queryResult({ isLoading: false, error: new Error('x') })
    );
    renderPage();
    expect(screen.getByText('Unable to load APIs')).toBeInTheDocument();
  });

  it('shows the empty state', () => {
    vi.mocked(useApis).mockReturnValue(
      queryResult({ isLoading: false, data: [] })
    );
    renderPage();
    expect(screen.getByText('No APIs yet')).toBeInTheDocument();
  });

  it('renders the API Proxies section and filters by search', async () => {
    vi.mocked(useApis).mockReturnValue(
      queryResult({ isLoading: false, data: mockComponents })
    );
    const { user } = renderPage();

    expect(screen.getByText('API Proxies')).toBeInTheDocument();
    expect(screen.getByText('Orders API')).toBeInTheDocument();

    await user.type(screen.getByPlaceholderText('Search APIs'), 'orders');
    expect(screen.getByText('Orders API')).toBeInTheDocument();

    await user.clear(screen.getByPlaceholderText('Search APIs'));
    await user.type(screen.getByPlaceholderText('Search APIs'), 'nomatch');
    expect(screen.queryByText('Orders API')).not.toBeInTheDocument();
    expect(screen.getByText('No matching APIs')).toBeInTheDocument();
  });

  it('switches between grid and list views', async () => {
    vi.mocked(useApis).mockReturnValue(
      queryResult({ isLoading: false, data: mockComponents })
    );
    const { user } = renderPage();

    expect(screen.queryByTestId('api-list-view')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'List view' }));
    expect(screen.getAllByTestId('api-list-view').length).toBeGreaterThan(0);
    // Rows still open the API and show status.
    expect(screen.getByText('Orders API')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Grid view' }));
    expect(screen.queryByTestId('api-list-view')).not.toBeInTheDocument();
  });
});

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

import type { ApiError } from '../../../../api/core/errors';
import type {
  RestApi,
  RestApiListResponse,
} from '../../../../api/resources/restApis';
import { makeConsoleScope } from '../../../../test/mockScope';
import { renderWithProviders, screen } from '../../../../test/utils';

vi.mock('../../../../api/resources/restApis', async (importActual) => ({
  ...(await importActual<typeof import('../../../../api/resources/restApis')>()),
  useRestApis: vi.fn(),
  useDeleteRestApi: vi.fn(),
}));

import {
  useDeleteRestApi,
  useRestApis,
} from '../../../../api/resources/restApis';
import { ApiListPage } from './ApiListPage';

const ROUTE = '/organizations/acme/projects/retail/apis';

const restApi = (overrides: Partial<RestApi>): RestApi => ({
  context: '/orders',
  displayName: 'Orders API',
  id: 'orders-api',
  kind: 'RestApi',
  lifeCycleStatus: 'PUBLISHED',
  projectId: 'retail',
  transport: ['http', 'https'],
  upstream: { main: { url: 'https://backend.example.com' } },
  version: '1.0.0',
  ...overrides,
});

const API_LIST: RestApi[] = [
  restApi({}),
  restApi({
    context: '/payments',
    displayName: 'Payments API',
    id: 'payments-api',
  }),
];

const listResponse = (list: RestApi[]): RestApiListResponse =>
  ({
    count: list.length,
    list,
    pagination: { limit: 25, offset: 0, total: list.length },
  }) as RestApiListResponse;

const queryResult = (
  overrides: Partial<UseQueryResult<RestApiListResponse, ApiError>>
) => overrides as UseQueryResult<RestApiListResponse, ApiError>;

function renderPage() {
  return renderWithProviders(
    <Routes>
      <Route
        path="/organizations/:orgHandle/projects/:projectHandler/apis"
        element={<ApiListPage />}
      />
    </Routes>,
    {
      route: ROUTE,
      // The page's `ScopeGate` reads console scope to decide between the API
      // list and a project picker; the default mock scope is inside a project,
      // which is the case these tests exercise.
      scope: makeConsoleScope(),
    }
  );
}

describe('ApiListPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Most tests don't exercise delete; provide a default mutation.
    vi.mocked(useDeleteRestApi).mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useDeleteRestApi>);
  });

  it('shows the loading state', () => {
    vi.mocked(useRestApis).mockReturnValue(queryResult({ isPending: true }));
    renderPage();
    expect(screen.getByText('Loading APIs')).toBeInTheDocument();
  });

  it('shows the error state', () => {
    vi.mocked(useRestApis).mockReturnValue(
      queryResult({ isPending: false, error: { message: 'x' } as ApiError })
    );
    renderPage();
    expect(screen.getByText('Unable to load APIs')).toBeInTheDocument();
  });

  it('shows the empty state', () => {
    vi.mocked(useRestApis).mockReturnValue(
      queryResult({ isPending: false, data: listResponse([]) })
    );
    renderPage();
    expect(screen.getByText('No APIs yet')).toBeInTheDocument();
  });

  it('renders the API cards and filters by search', async () => {
    vi.mocked(useRestApis).mockReturnValue(
      queryResult({ isPending: false, data: listResponse(API_LIST) })
    );
    const { user } = renderPage();

    expect(screen.getByText('Orders API')).toBeInTheDocument();
    expect(screen.getByText('Payments API')).toBeInTheDocument();
    // Card details come straight from the spec shape.
    expect(screen.getAllByText('v1.0.0').length).toBe(2);
    // Transports render as one Chip each, labelled in upper case.
    expect(screen.getAllByText('HTTP').length).toBe(2);
    expect(screen.getAllByText('HTTPS').length).toBe(2);

    await user.type(screen.getByPlaceholderText('Search APIs'), 'orders');
    expect(screen.getByText('Orders API')).toBeInTheDocument();
    expect(screen.queryByText('Payments API')).not.toBeInTheDocument();

    await user.clear(screen.getByPlaceholderText('Search APIs'));
    await user.type(screen.getByPlaceholderText('Search APIs'), 'nomatch');
    expect(screen.queryByText('Orders API')).not.toBeInTheDocument();
    expect(screen.getByText('No matching APIs')).toBeInTheDocument();
  });

  it('deletes from the card action instead of an overflow menu', async () => {
    vi.mocked(useRestApis).mockReturnValue(
      queryResult({ isPending: false, data: listResponse(API_LIST) })
    );
    const { user } = renderPage();

    expect(
      screen.queryByRole('button', { name: 'API actions' })
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Delete Orders API' }));

    // Confirmation opens, and the click never reached the card underneath —
    // which would have navigated away from the list.
    expect(
      screen.getByText(/permanently deletes the API "Orders API"/)
    ).toBeInTheDocument();
    expect(screen.getByText('Payments API')).toBeInTheDocument();
  });

  it('switches between grid and list views', async () => {
    vi.mocked(useRestApis).mockReturnValue(
      queryResult({ isPending: false, data: listResponse(API_LIST) })
    );
    const { user } = renderPage();

    expect(screen.queryByTestId('api-list-view')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'List view' }));
    expect(screen.getAllByTestId('api-list-view').length).toBeGreaterThan(0);
    // Rows still open the API and show context plus lifecycle.
    expect(screen.getByText('Orders API')).toBeInTheDocument();
    expect(screen.getByText(/\/orders/)).toBeInTheDocument();
    expect(screen.getAllByText('Published').length).toBe(2);

    await user.click(screen.getByRole('button', { name: 'Grid view' }));
    expect(screen.queryByTestId('api-list-view')).not.toBeInTheDocument();
  });
});

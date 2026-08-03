import type { UseQueryResult } from '@tanstack/react-query';
import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { useApiDetail } from '../../api/hooks/useMvpQueries';
import { renderWithProviders, screen } from '../../test/utils';
import type { ApiDetail } from '../../types/domain';
import { ApiDetailPage } from './ApiDetailPage';

vi.mock('../../api/hooks/useMvpQueries', async (importActual) => ({
  ...(await importActual<typeof import('../../api/hooks/useMvpQueries')>()),
  useApiDetail: vi.fn(),
}));

const ROUTE = '/organizations/acme/projects/retail/apis/orders-api';

const detail: ApiDetail = {
  id: 'api-1',
  projectId: 'proj-1',
  name: 'orders-api',
  displayName: 'Orders API',
  handler: 'orders-api',
  kind: 'API_PROXY',
  status: 'ACTIVE',
  operations: [],
  policies: [],
  endpoints: {},
};

const queryResult = (overrides: Partial<UseQueryResult<ApiDetail, Error>>) =>
  overrides as UseQueryResult<ApiDetail, Error>;

function renderPage() {
  return renderWithProviders(
    <Routes>
      <Route
        path="/organizations/:orgHandle/projects/:projectHandler/apis"
        element={<div>API list marker</div>}
      />
      <Route
        path="/organizations/:orgHandle/projects/:projectHandler/apis/:apiHandler"
        element={<ApiDetailPage />}
      />
    </Routes>,
    { route: ROUTE }
  );
}

describe('ApiDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the loading state', () => {
    vi.mocked(useApiDetail).mockReturnValue(queryResult({ isLoading: true }));
    renderPage();
    expect(screen.getByText('Loading API')).toBeInTheDocument();
  });

  it('shows the error state', () => {
    vi.mocked(useApiDetail).mockReturnValue(
      queryResult({ isLoading: false, error: new Error('x') })
    );
    renderPage();
    expect(screen.getByText('API not found')).toBeInTheDocument();
  });

  it('renders the API title without the header action buttons', () => {
    vi.mocked(useApiDetail).mockReturnValue(
      queryResult({ isLoading: false, data: detail })
    );
    renderPage();

    expect(screen.getByText('Orders API')).toBeInTheDocument();
    // The Delete/Deploy/Test/Manage header pallet has been removed; progress
    // navigation now lives in the Overview tab's progress banner. Delete has no
    // banner equivalent, so its absence proves the header pallet is gone.
    expect(
      screen.queryByRole('button', { name: 'Delete' })
    ).not.toBeInTheDocument();
    expect(screen.getByText('Track your progress here')).toBeInTheDocument();
  });
});

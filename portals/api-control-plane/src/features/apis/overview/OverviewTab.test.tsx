import type { UseQueryResult } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  useApiKeys,
  useCreateApiKey,
  useGatewayDeployments,
  useGateways,
  useRevokeApiKey,
} from '../../../api/hooks/useMvpQueries';
import { renderWithProviders, screen } from '../../../test/utils';
import type {
  ApiDetail,
  ApiKeySummary,
  Gateway,
  GatewayDeployment,
} from '../../../types/domain';
import { OverviewTab } from './OverviewTab';

vi.mock('../../../api/hooks/useMvpQueries', async (importActual) => ({
  ...(await importActual<typeof import('../../../api/hooks/useMvpQueries')>()),
  useGateways: vi.fn(),
  useGatewayDeployments: vi.fn(),
  useApiKeys: vi.fn(),
  useCreateApiKey: vi.fn(),
  useRevokeApiKey: vi.fn(),
}));

const detail: ApiDetail = {
  id: 'api-1',
  projectId: 'proj-1',
  name: 'orders-api',
  displayName: 'Orders API',
  handler: 'orders-api',
  kind: 'API_PROXY',
  status: 'ACTIVE',
  context: '/orders',
  operations: [
    { method: 'GET', path: '/items', description: 'List order items' },
    { method: 'POST', path: '/items' },
  ],
  policies: [],
  endpoints: {},
};

const gateway: Gateway = {
  id: 'gw-1',
  name: 'prod-gw',
  displayName: 'Production Gateway',
  vhost: 'mg.acme.dev',
  functionalityType: 'regular',
  mode: 'self-hosted',
  isActive: true,
};

const deployed: GatewayDeployment = {
  id: 'dep-1',
  name: 'v1.0-prod',
  gatewayId: 'gw-1',
  status: 'DEPLOYED',
  createdAt: '2026-07-01T08:00:00Z',
};

const query = <T,>(overrides: Partial<UseQueryResult<T, Error>>) =>
  overrides as UseQueryResult<T, Error>;

const mutation = <H extends (...args: never[]) => unknown>(mutate = vi.fn()) =>
  ({ mutate, isPending: false }) as unknown as ReturnType<H>;

describe('OverviewTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useGateways).mockReturnValue(
      query({ isLoading: false, data: [gateway] })
    );
    vi.mocked(useGatewayDeployments).mockReturnValue(
      query({ isLoading: false, data: [deployed] })
    );
    vi.mocked(useApiKeys).mockReturnValue(
      query<ApiKeySummary[]>({
        isLoading: false,
        data: [
          {
            name: 'prod-key',
            maskedApiKey: 'abcd****xy',
            expiresAt: '2026-10-01T00:00:00Z',
          },
        ],
      })
    );
    vi.mocked(useCreateApiKey).mockReturnValue(
      mutation<typeof useCreateApiKey>()
    );
    vi.mocked(useRevokeApiKey).mockReturnValue(
      mutation<typeof useRevokeApiKey>()
    );
  });

  it('renders the resources list with method chips', () => {
    renderWithProviders(<OverviewTab detail={detail} />);
    expect(screen.getByText('Resources')).toBeInTheDocument();
    expect(screen.getByText('GET')).toBeInTheDocument();
    expect(screen.getByText('List order items')).toBeInTheDocument();
    expect(screen.getAllByText('/items')).toHaveLength(2);
  });

  it('renders the progress banner with the life-cycle steps', () => {
    renderWithProviders(<OverviewTab detail={detail} />);
    expect(screen.getByText('Track your progress here')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Deploy' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Test' })).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Publish to Devportal' })
    ).toBeInTheDocument();
  });

  it('advances the progress chart as the API is deployed', () => {
    // Deployed on a gateway but still a draft → Create + Deploy done: 2/4 (50%).
    renderWithProviders(
      <OverviewTab detail={{ ...detail, status: 'DRAFT' }} />
    );
    expect(screen.getByText('2 of 4 completed')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute(
      'aria-valuenow',
      '50'
    );
  });

  it('completes every step once the API is published', () => {
    // status ACTIVE (PUBLISHED) + deployed → all four steps done: 4/4 (100%).
    renderWithProviders(<OverviewTab detail={detail} />);
    expect(screen.getByText('4 of 4 completed')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute(
      'aria-valuenow',
      '100'
    );
  });

  it('shows only Create complete when the API is not deployed', () => {
    vi.mocked(useGatewayDeployments).mockReturnValue(
      query({ isLoading: false, data: [] })
    );
    renderWithProviders(
      <OverviewTab detail={{ ...detail, status: 'DRAFT' }} />
    );
    expect(screen.getByText('1 of 4 completed')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute(
      'aria-valuenow',
      '25'
    );
  });

  it('builds the invoke URL from the deployed gateway vhost and context', () => {
    renderWithProviders(<OverviewTab detail={detail} />);
    expect(screen.getByText('Invoke URL')).toBeInTheDocument();
    expect(
      screen.getByDisplayValue('https://mg.acme.dev/orders')
    ).toBeInTheDocument();
  });

  it('hides the right column when the API is not deployed anywhere', () => {
    vi.mocked(useGatewayDeployments).mockReturnValue(
      query({ isLoading: false, data: [] })
    );
    renderWithProviders(<OverviewTab detail={detail} />);
    expect(screen.queryByText('Invoke URL')).not.toBeInTheDocument();
    expect(screen.queryByText('API Keys')).not.toBeInTheDocument();
  });

  it('lists API keys and adds a new one through the dialog', async () => {
    const mutate = vi.fn(
      (_variables: unknown, options?: { onSuccess?: () => void }) =>
        options?.onSuccess?.()
    );
    vi.mocked(useCreateApiKey).mockReturnValue(
      mutation<typeof useCreateApiKey>(mutate)
    );
    const { user } = renderWithProviders(<OverviewTab detail={detail} />);

    expect(screen.getByText('prod-key')).toBeInTheDocument();
    expect(screen.getByText('abcd****xy')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Add API Key' }));
    await user.type(
      screen.getByPlaceholderText('Ex: Production Key'),
      'Staging Key'
    );
    const dialogFields = screen.getAllByRole('textbox');
    await user.type(dialogFields[dialogFields.length - 1], 'secret-value');
    await user.click(screen.getByRole('button', { name: 'Add' }));

    expect(mutate).toHaveBeenCalledWith(
      {
        api: detail,
        input: { displayName: 'Staging Key', apiKey: 'secret-value' },
      },
      expect.any(Object)
    );
  });
});

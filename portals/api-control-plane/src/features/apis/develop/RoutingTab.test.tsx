import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { server } from '../../../test/server';
import { renderWithProviders, screen } from '../../../test/utils';
import type { ApiDetail } from '../../../types/domain';
import { RoutingTab } from './RoutingTab';

const detail: ApiDetail = {
  id: 'orders-api',
  projectId: 'proj',
  name: 'orders',
  displayName: 'Orders',
  handler: 'orders-api',
  kind: 'API_PROXY',
  status: 'ACTIVE',
  operations: [
    { method: 'GET', path: '/orders' },
    { method: 'GET', path: '/new' },
  ],
  policies: [],
  endpoints: { prodUrl: 'http://api.test' },
};

describe('RoutingTab backend discovery', () => {
  it('auto-maps proxy resources found in the backend contract and leaves the rest dry', async () => {
    // The backend contract only declares /orders — not /new.
    server.use(
      http.get('http://api.test/openapi.json', () =>
        HttpResponse.json({
          openapi: '3.0.0',
          paths: { '/orders': { get: {} } },
        })
      )
    );

    const { user } = renderWithProviders(<RoutingTab detail={detail} />);

    // Seeded from the operations, both resources start mapped.
    expect(screen.queryByTitle(/Not connected/)).not.toBeInTheDocument();

    await user.click(screen.getByLabelText('Discover backend resources'));

    // After discovery, /orders stays mapped while /new hangs dry.
    const dry = await screen.findByTitle(/Not connected/);
    expect(dry).toHaveTextContent('/new');
    expect(screen.getAllByTitle(/Not connected/)).toHaveLength(1);
  });

  it('lets the backend URL be edited inline on the canvas, which triggers discovery', async () => {
    server.use(
      http.get('http://api.test/openapi.json', () =>
        HttpResponse.json({
          openapi: '3.0.0',
          paths: { '/orders': { get: {} } },
        })
      )
    );

    const noUrl: ApiDetail = { ...detail, endpoints: {} };
    const { user } = renderWithProviders(<RoutingTab detail={noUrl} />);

    // The canvas holder is a real, editable input (not a static label).
    const field = screen.getByLabelText('Backend URL') as HTMLInputElement;
    expect(field).toBeEnabled();
    expect(field.readOnly).toBe(false);

    await user.type(field, 'http://api.test');
    expect(field).toHaveValue('http://api.test');

    // Typing a new URL auto-runs discovery (debounced): /new has no backend
    // counterpart and is left dry.
    const dry = await screen.findByTitle(/Not connected/, undefined, {
      timeout: 3000,
    });
    expect(dry).toHaveTextContent('/new');
  });
});

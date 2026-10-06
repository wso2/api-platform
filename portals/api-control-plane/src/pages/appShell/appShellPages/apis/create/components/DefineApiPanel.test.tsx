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

import { renderWithProviders, screen } from '@/test/utils';
import { DefineApiPanel } from './DefineApiPanel';

vi.mock('swagger-ui-react', () => ({ default: () => null }));

describe('DefineApiPanel — start from scratch', () => {
  it('does not produce a draft until a backend endpoint is entered', () => {
    const onDraftChange = vi.fn();

    renderWithProviders(<DefineApiPanel onDraftChange={onDraftChange} />);

    expect(onDraftChange).toHaveBeenLastCalledWith(null);
  });

  it('shows only the endpoint form after From an endpoint is selected', async () => {
    const { user } = renderWithProviders(<DefineApiPanel onDraftChange={vi.fn()} />);

    await user.click(screen.getByRole('button', { name: /From an endpoint/ }));

    expect(screen.getByRole('heading', { name: 'Backend endpoint' })).toBeInTheDocument();
    expect(screen.getByLabelText(/Backend URL/)).toHaveValue('');
    expect(screen.queryByText('How do you want to start?')).not.toBeInTheDocument();
    expect(screen.queryByText('API resources')).not.toBeInTheDocument();
  });

  it('fills the reading-list service when the sample URL action is used', async () => {
    const { user } = renderWithProviders(<DefineApiPanel onDraftChange={vi.fn()} />);

    await user.click(screen.getByRole('button', { name: 'Try a sample' }));

    expect(screen.getByLabelText(/Backend URL/)).toHaveValue(
      'https://apis.bijira.dev/samples/reading-list-api-service/v1.0/books',
    );
  });

  it('carries an edited endpoint into the scratch draft', async () => {
    const onDraftChange = vi.fn();
    const { user } = renderWithProviders(<DefineApiPanel onDraftChange={onDraftChange} />);
    await user.click(screen.getByRole('button', { name: /From an endpoint/ }));

    const endpoint = screen.getByLabelText(/Backend URL/);
    await user.clear(endpoint);
    await user.type(endpoint, 'https://api.acme.com/v1');

    expect(onDraftChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ upstream: { main: { url: 'https://api.acme.com/v1' } } }),
    );
  });

  it('names the API from the endpoint, and holds Continue until the URL parses', async () => {
    const onDraftChange = vi.fn();
    const { user } = renderWithProviders(<DefineApiPanel onDraftChange={onDraftChange} />);
    await user.click(screen.getByRole('button', { name: /From an endpoint/ }));

    const endpoint = screen.getByLabelText(/Backend URL/);
    await user.type(endpoint, 'orders');
    expect(onDraftChange).toHaveBeenLastCalledWith(null);

    // The error waits for the user to leave the field.
    expect(screen.queryByText(/doesn’t look like a URL yet/)).not.toBeInTheDocument();
    await user.tab();
    expect(screen.getByText(/doesn’t look like a URL yet/)).toBeInTheDocument();

    await user.clear(endpoint);
    await user.type(endpoint, 'https://api.example.com/orders/v2');
    expect(screen.queryByText(/doesn’t look like a URL yet/)).not.toBeInTheDocument();
    expect(onDraftChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ displayName: 'Orders', version: '2.0.0' }),
    );
  });
});

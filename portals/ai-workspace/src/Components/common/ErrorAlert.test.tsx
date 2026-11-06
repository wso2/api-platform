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

import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { renderWithProviders, screen } from '../../test/utils';
import ErrorAlert from './ErrorAlert';

const forceLogoutAndRedirect = vi.fn();
vi.mock('../../auth/logout', () => ({
  forceLogoutAndRedirect: (...args: unknown[]) => forceLogoutAndRedirect(...args),
}));

const withProps = (message: string, props: Record<string, unknown>) =>
  Object.assign(new Error(message), props);

const onRetry = vi.fn();

beforeEach(() => {
  forceLogoutAndRedirect.mockReset();
  onRetry.mockReset();
});

describe('ErrorAlert', () => {
  it('offers Logout (not Retry) on a 401 and calls forceLogoutAndRedirect', async () => {
    const { user } = renderWithProviders(
      <ErrorAlert error={withProps('nope', { status: 401 })} onRetry={onRetry} />
    );

    expect(screen.getByText('Your session has expired. Please log in again.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Retry' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Logout' }));
    expect(forceLogoutAndRedirect).toHaveBeenCalledTimes(1);
    expect(onRetry).not.toHaveBeenCalled();
  });

  it('hides the backend message on 5xx, shows the reference id, and retries', async () => {
    const { user } = renderWithProviders(
      <ErrorAlert
        error={withProps('secret db detail', { status: 500, trackingId: 'trk-123' })}
        onRetry={onRetry}
      />
    );

    expect(screen.getByText(/Something went wrong\./)).toBeInTheDocument();
    expect(screen.queryByText(/secret db detail/)).not.toBeInTheDocument();
    expect(screen.getByText('(Reference ID: trk-123)')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('omits the reference id on 5xx when there is no tracking id', () => {
    renderWithProviders(<ErrorAlert error={withProps('x', { status: 503 })} onRetry={onRetry} />);

    expect(screen.getByText('Something went wrong.')).toBeInTheDocument();
    expect(screen.queryByText(/Reference ID/)).not.toBeInTheDocument();
  });

  it('shows the backend message on 4xx with a Retry button', async () => {
    const { user } = renderWithProviders(
      <ErrorAlert error={withProps('Provider not found', { status: 404 })} onRetry={onRetry} />
    );

    expect(screen.getByText('Provider not found')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('reads the status from response.status', () => {
    renderWithProviders(
      <ErrorAlert error={withProps('boom', { response: { status: 502 } })} onRetry={onRetry} />
    );

    expect(screen.getByText('Something went wrong.')).toBeInTheDocument();
    expect(screen.queryByText('boom')).not.toBeInTheDocument();
  });

  it('reads a 401 from response.status', () => {
    renderWithProviders(
      <ErrorAlert error={withProps('boom', { response: { status: 401 } })} onRetry={onRetry} />
    );

    expect(screen.getByRole('button', { name: 'Logout' })).toBeInTheDocument();
  });

  it('parses the status out of the message ("status: 503")', () => {
    renderWithProviders(
      <ErrorAlert error={new Error('Request failed, status: 503')} onRetry={onRetry} />
    );

    expect(screen.getByText('Something went wrong.')).toBeInTheDocument();
    expect(screen.queryByText(/Request failed/)).not.toBeInTheDocument();
  });

  it('shows the message with Retry when there is no status, and retries on click', async () => {
    const { user } = renderWithProviders(
      <ErrorAlert error={new Error('Network Error')} onRetry={onRetry} />
    );

    expect(screen.getByText('Network Error')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('falls back to a generic message and Retry when no error object is given', async () => {
    const { user } = renderWithProviders(<ErrorAlert error={null} onRetry={onRetry} />);

    expect(screen.getByText('Something went wrong.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});

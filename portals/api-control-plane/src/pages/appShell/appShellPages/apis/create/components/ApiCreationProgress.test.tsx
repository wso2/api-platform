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

import { act, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders } from '@/test/utils';
import { ApiCreationProgress } from './ApiCreationProgress';

const noop = () => {};

describe('ApiCreationProgress', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('climbs toward — but never reaches — 100% while the request is in flight', () => {
    renderWithProviders(
      <ApiCreationProgress
        displayName="Orders API"
        onBack={noop}
        onComplete={noop}
        onRetry={noop}
        status="creating"
      />,
    );

    expect(screen.getByText('Orders API', { exact: false })).toBeInTheDocument();

    // Far longer than the flow could plausibly take: the bar must still be
    // short of 100, because only the server's answer may claim completion.
    act(() => {
      vi.advanceTimersByTime(60_000);
    });

    const progress = screen.getByRole('progressbar', {
      name: 'API proxy creation progress',
    });
    expect(Number(progress.getAttribute('aria-valuenow'))).toBeLessThan(100);
  });

  it('hands over to the caller once creation succeeds, after showing 100%', () => {
    const onComplete = vi.fn();
    const { rerender } = renderWithProviders(
      <ApiCreationProgress
        onBack={noop}
        onComplete={onComplete}
        onRetry={noop}
        status="creating"
      />,
    );

    rerender(
      <ApiCreationProgress onBack={noop} onComplete={onComplete} onRetry={noop} status="created" />,
    );

    expect(
      screen.getByRole('progressbar', { name: 'API proxy creation progress' }),
    ).toHaveAttribute('aria-valuenow', '100');
    expect(onComplete).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1_000);
    });
    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it('offers both ways out when creation fails, and navigates nowhere', () => {
    const onComplete = vi.fn();
    renderWithProviders(
      <ApiCreationProgress onBack={noop} onComplete={onComplete} onRetry={noop} status="failed" />,
    );

    expect(screen.getByText('We could not create this API proxy')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Back to configuration' })).toBeInTheDocument();
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(onComplete).not.toHaveBeenCalled();
  });
});

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

import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { EntitlementSummary } from './entitlementTypes';
import type { TrialStatusHostPort } from './hostPort';
import TrialStatusFeature from './TrialStatusFeature';
import type { BillingOrganization } from './types';

const getEntitlementSummary = vi.fn<() => Promise<EntitlementSummary | null>>();
vi.mock('./entitlementStore', () => ({
  getEntitlementSummary: () => getEntitlementSummary(),
}));

const DAY = 24 * 60 * 60 * 1000;
const CONSOLE = 'https://console.example.com';
const SUBSCRIPTIONS = `${CONSOLE}/billing?tab=subscriptions`;

const port = (billing: BillingOrganization | null): TrialStatusHostPort => ({
  orgHandle: 'acme',
  billing: { organization: () => Promise.resolve(billing) },
});

const renderBadge = (billing: BillingOrganization | null) =>
  render(<TrialStatusFeature consoleUrl={CONSOLE} port={port(billing)} />);

const quota = (name: string, limit: number, used: number) => ({
  name,
  type: 'quota',
  used,
  enforcement: { mode: 'hard', source: 'plan' },
  quota: { hard_limit: limit, used, remaining: Math.max(0, limit - used) },
});

afterEach(() => {
  getEntitlementSummary.mockReset();
});

describe('TrialStatusFeature', () => {
  it('counts down a running trial and opens the subscriptions', async () => {
    getEntitlementSummary.mockResolvedValue(null);
    renderBadge({
      subscription: {
        status: 'trial',
        trial: {
          days_remaining: 18,
          trial_end: new Date(Date.now() + 18 * DAY - 60_000).toISOString(),
          trial_days: 30,
        },
      },
    });

    const badge = await screen.findByRole('link', { name: '18 days left of free trial' });
    expect(badge.getAttribute('href')).toBe(SUBSCRIPTIONS);
    expect(badge.textContent).toContain('18 days');
    expect(screen.getByRole('separator')).toBeTruthy();
  });

  it('keeps the usage panel and its upgrade on the same page', async () => {
    getEntitlementSummary.mockResolvedValue({
      products: [{ product: 'api-platform', entitlements: [quota('apis', 5, 3)] }],
    } as EntitlementSummary);
    renderBadge({
      subscription: {
        status: 'trial',
        trial: { days_remaining: 5, trial_end: new Date(Date.now() + 5 * DAY).toISOString() },
      },
    });

    const badge = await screen.findByRole('link', { name: /days left of free trial/ });
    badge.focus();
    const upgrade = await screen.findByRole('link', { name: /^Upgrade/ });
    expect(upgrade.getAttribute('href')).toBe(SUBSCRIPTIONS);
  });

  it('shows the trial expired, with an upgrade, once the trial is suspended', async () => {
    getEntitlementSummary.mockResolvedValue(null);
    renderBadge({
      subscription: {
        status: 'suspended',
        current_period_start: '2026-10-01T00:00:00.000Z',
        trial: { days_remaining: 0, trial_end: '2026-10-31T00:00:00.000Z', trial_days: 30 },
      },
    });

    const badge = await screen.findByRole('link', { name: 'Free trial expired' });
    expect(badge.getAttribute('href')).toBe(SUBSCRIPTIONS);
    expect(badge.textContent).toContain('Trial expired');
    expect(badge.textContent).toContain('Upgrade now');
  });

  it('renders nothing, divider included, for a paid plan', async () => {
    getEntitlementSummary.mockResolvedValue(null);
    const { container } = renderBadge({
      subscription: { status: 'active', trial: { days_remaining: 0, trial_end: '2026-10-31' } },
    });

    await Promise.resolve();
    expect(screen.queryByRole('link')).toBeNull();
    expect(screen.queryByRole('separator')).toBeNull();
    expect(container.textContent).toBe('');
  });
});

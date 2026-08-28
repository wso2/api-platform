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

import { AppShell } from '@wso2/oxygen-ui';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { SidebarErrorFallback } from '../../components/errors/ErrorFallback';
import { renderWithProviders, screen } from '../../test/utils';

// The switchers own every scope-dependent hook in the header, so making the
// whole module throw is exactly the failure the boundary around it exists for.
vi.mock('./HeaderScopeSwitchers', () => ({
  HeaderScopeSwitchers: () => {
    throw new Error('scope lookup returned an unexpected shape');
  },
}));

import { AppHeader } from './AppHeader';

/** `useAppShell()` throws outside a provider, so the header needs its real slot. */
const renderHeader = () =>
  renderWithProviders(
    <AppShell>
      <AppShell.Navbar>
        <AppHeader />
      </AppShell.Navbar>
    </AppShell>
  );

beforeEach(() => {
  vi.spyOn(console, 'error').mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('AppHeader', () => {
  it('keeps the rest of the header usable when the switchers throw', async () => {
    const { user } = renderHeader();

    // Brand and actions are unaffected — they read nothing from scope.
    expect(screen.getByText('API Platform')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Notifications' })
    ).toBeInTheDocument();

    // The one that matters: losing a switcher must never cost the user their
    // way out of the session.
    await user.click(screen.getByRole('button', { name: 'Account' }));
    expect(await screen.findByText('Test User')).toBeInTheDocument();
    expect(
      await screen.findByText(/log ?out|sign out/i)
    ).toBeInTheDocument();
  });

  it('leaves a visible marker rather than silently dropping the switchers', () => {
    renderHeader();

    expect(
      screen.getByRole('status', {
        name: /switchers are unavailable/i,
      })
    ).toBeInTheDocument();
    expect(console.error).toHaveBeenCalled();
  });
});

describe('SidebarErrorFallback', () => {
  it('renders an empty rail carrying a marker', () => {
    renderWithProviders(<SidebarErrorFallback />);

    expect(
      screen.getByRole('status', { name: /navigation is unavailable/i })
    ).toBeInTheDocument();
  });
});

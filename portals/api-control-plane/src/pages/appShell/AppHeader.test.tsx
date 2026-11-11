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
import { Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  BrandLogoProvider,
  defaultBrandLogo,
  type BrandLogo,
} from '../../branding/BrandLogoProvider';
import { SidebarErrorFallback } from '../../components/errors/ErrorFallback';
import { renderWithProviders, screen, within } from '../../test/utils';

// The switchers own every scope-dependent hook in the header, so making the
// whole module throw is exactly the failure the boundary around it exists for.
vi.mock('./HeaderScopeSwitchers', () => ({
  HeaderScopeSwitchers: () => {
    throw new Error('scope lookup returned an unexpected shape');
  },
}));

import { AppHeader } from './AppHeader';

/**
 * `useAppShell()` throws outside a provider, so the header needs its real slot.
 * `BrandLogoProvider` is mounted as `App` mounts it: with no `brandLogo` it
 * paints the on-prem default, and a cloud host passes its own.
 */
const renderHeader = ({ route, brandLogo }: { route?: string; brandLogo?: BrandLogo } = {}) =>
  renderWithProviders(
    <BrandLogoProvider brandLogo={brandLogo}>
      <AppShell>
        <AppShell.Navbar>
          <AppHeader />
        </AppShell.Navbar>
        <Routes>
          <Route path="*" element={<CurrentPath />} />
        </Routes>
      </AppShell>
    </BrandLogoProvider>,
    route ? { route } : undefined,
  );

/**
 * The header's brand, found by the name the header gives its control. Not by
 * the product name or the logo: those belong to the deployment (the lockup's
 * name is a translatable message, and the logo is whatever the host supplies),
 * so a cloud or rebranded build would fail a test that hard-codes them.
 */
const brandControl = () => screen.getByRole('button', { name: 'Go to organization overview' });

/** The brand's logo image; decorative (`alt=""`), as the lockup's name is set beside it in text. */
const brandLogoSrc = () => within(brandControl()).getByRole('presentation').getAttribute('src');

/** Surfaces the router's location so a brand click can be asserted on. */
function CurrentPath() {
  return <div data-testid="pathname">{useLocation().pathname}</div>;
}

beforeEach(() => {
  vi.spyOn(console, 'error').mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('AppHeader', () => {
  it('keeps the rest of the header usable when the switchers throw', async () => {
    const { user } = renderHeader();

    // Brand and actions are unaffected — they read nothing from scope. The brand
    // still paints its logo: the on-prem default here, as no host supplies one.
    expect(Object.values(defaultBrandLogo)).toContain(brandLogoSrc());
    // expect(screen.getByRole('button', { name: 'Notifications' })).toBeInTheDocument();

    // The one that matters: losing a switcher must never cost the user their
    // way out of the session.
    await user.click(screen.getByRole('button', { name: 'Account' }));
    expect(await screen.findByText('Test User')).toBeInTheDocument();
    expect(await screen.findByText(/log ?out|sign out/i)).toBeInTheDocument();
  });

  it('paints the logo a host supplies, so a cloud deployment shows its own brand', () => {
    const hostLogo: BrandLogo = { dark: '/host/logo-on-dark.svg', light: '/host/logo-on-light.svg' };
    renderHeader({ brandLogo: hostLogo });

    expect(Object.values(hostLogo)).toContain(brandLogoSrc());
  });

  it('sends the brand logo to the organization overview', async () => {
    const { user } = renderHeader({ route: '/organizations/acme/projects/orders/apis/a-1/deploy' });

    await user.click(brandControl());

    expect(screen.getByTestId('pathname')).toHaveTextContent('/organizations/acme/home');
  });

  it('sends the brand logo to organization selection when no organization is in scope', async () => {
    const { user } = renderHeader({ route: '/' });

    await user.click(brandControl());

    expect(screen.getByTestId('pathname')).toHaveTextContent('/organizations');
  });

  it('leaves a visible marker rather than silently dropping the switchers', () => {
    renderHeader();

    expect(
      screen.getByRole('status', {
        name: /switchers are unavailable/i,
      }),
    ).toBeInTheDocument();
    expect(console.error).toHaveBeenCalled();
  });
});

describe('SidebarErrorFallback', () => {
  it('renders an empty rail carrying a marker', () => {
    renderWithProviders(<SidebarErrorFallback />);

    expect(screen.getByRole('status', { name: /navigation is unavailable/i })).toBeInTheDocument();
  });
});

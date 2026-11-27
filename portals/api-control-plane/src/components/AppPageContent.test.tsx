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

import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { runtimeConfig } from '@/config/runtime';

import { renderWithProviders, screen } from '@/test/utils';

import { AppPageContent } from './AppPageContent';

describe('AppPageContent', () => {
  it('puts the console footer after the page content', () => {
    renderWithProviders(
      <AppPageContent>
        <p>Page body</p>
      </AppPageContent>,
    );

    const page = screen.getByText('Page body');
    const footer = screen.getByText(/Choreo/);
    expect(page.compareDocumentPosition(footer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('offers the four footer links, each opening in a new tab', () => {
    renderWithProviders(<AppPageContent>Page body</AppPageContent>);

    const expected: Record<string, string> = {
      Documentation: 'https://choreolabs.com/documentation/api-management/cloud/',
      Support: 'https://choreolabs.com/subscription',
      'Terms of Use': 'https://choreolabs.com/cloud/terms-of-use/',
      'Privacy Policy': 'https://choreolabs.com/privacy-policy',
    };
    Object.entries(expected).forEach(([name, href]) => {
      const link = screen.getByRole('link', { name });
      expect(link).toHaveAttribute('href', href);
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'));
    });
  });

  describe('Cookie Settings link', () => {
    afterEach(() => {
      runtimeConfig.cookieProDomainScript = '';
      runtimeConfig.gtmContainerId = '';
      delete (window as Window & { OneTrust?: unknown }).OneTrust;
    });

    it('is hidden unless the cookie banner is configured', () => {
      renderWithProviders(<AppPageContent>Page body</AppPageContent>);
      expect(screen.queryByRole('link', { name: 'Cookie Settings' })).not.toBeInTheDocument();
    });

    it('is hidden when an id is not in the format the tracking plugin accepts', () => {
      runtimeConfig.cookieProDomainScript = '0198a8a4-0000-7000-8000-000000000000';
      runtimeConfig.gtmContainerId = 'not-a-gtm-id';
      renderWithProviders(<AppPageContent>Page body</AppPageContent>);
      expect(screen.queryByRole('link', { name: 'Cookie Settings' })).not.toBeInTheDocument();
    });

    it('opens the cookie preference center when clicked', async () => {
      const toggle = vi.fn();
      (window as Window & { OneTrust?: unknown }).OneTrust = { ToggleInfoDisplay: toggle };
      runtimeConfig.cookieProDomainScript = '0198a8a4-0000-7000-8000-000000000000';
      runtimeConfig.gtmContainerId = 'GTM-ABCD123';
      renderWithProviders(<AppPageContent>Page body</AppPageContent>);
      await userEvent.click(screen.getByRole('link', { name: 'Cookie Settings' }));
      expect(toggle).toHaveBeenCalledTimes(1);
    });
  });

  describe('Cookie Policy link', () => {
    afterEach(() => {
      runtimeConfig.cookiePolicyLink = '';
    });

    it('is hidden unless a deployment sets one', () => {
      renderWithProviders(<AppPageContent>Page body</AppPageContent>);
      expect(screen.queryByRole('link', { name: 'Cookie Policy' })).not.toBeInTheDocument();
    });

    it('shows the https link in a new tab', () => {
      runtimeConfig.cookiePolicyLink = 'https://choreolabs.com/cookie-policy/';
      renderWithProviders(<AppPageContent>Page body</AppPageContent>);
      const link = screen.getByRole('link', { name: 'Cookie Policy' });
      expect(link).toHaveAttribute('href', 'https://choreolabs.com/cookie-policy/');
      expect(link).toHaveAttribute('target', '_blank');
    });

    it.each(['javascript:alert(1)', 'http://choreolabs.com/cookie-policy/', 'https://', 'https://exa mple.com'])(
      'leaves out %s, which is not a usable https URL',
      (value) => {
        runtimeConfig.cookiePolicyLink = value;
        renderWithProviders(<AppPageContent>Page body</AppPageContent>);
        expect(screen.queryByRole('link', { name: 'Cookie Policy' })).not.toBeInTheDocument();
      }
    );
  });

  it('replaces or removes the footer through the footer prop', () => {
    const { unmount } = renderWithProviders(
      <AppPageContent footer={<span>Custom footer</span>}>Page body</AppPageContent>,
    );
    expect(screen.getByText('Custom footer')).toBeInTheDocument();
    expect(screen.queryByText(/Choreo/)).not.toBeInTheDocument();
    unmount();

    renderWithProviders(<AppPageContent footer={null}>Page body</AppPageContent>);
    expect(screen.queryByText(/Choreo/)).not.toBeInTheDocument();
  });

  it('shows one footer when page containers are nested', () => {
    renderWithProviders(
      <AppPageContent noPadding>
        <AppPageContent>Inner page</AppPageContent>
      </AppPageContent>,
    );

    expect(screen.getByText('Inner page')).toBeInTheDocument();
    expect(screen.getAllByText(/Choreo/)).toHaveLength(1);
  });
});

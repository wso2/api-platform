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

import { describe, expect, it } from 'vitest';

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
    const footer = screen.getByText(/WSO2 LLC/);
    expect(page.compareDocumentPosition(footer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('offers the four footer links, each opening in a new tab', () => {
    renderWithProviders(<AppPageContent>Page body</AppPageContent>);

    const expected: Record<string, string> = {
      Documentation: 'https://wso2.com/api-platform/docs/',
      Support: 'https://discord.com/invite/wso2',
      'Terms of Use': 'https://wso2.com/api-platform/terms-of-use/',
      'Privacy Policy': 'https://wso2.com/privacy-policy',
    };
    Object.entries(expected).forEach(([name, href]) => {
      const link = screen.getByRole('link', { name });
      expect(link).toHaveAttribute('href', href);
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', expect.stringContaining('noopener'));
    });
  });

  it('replaces or removes the footer through the footer prop', () => {
    const { unmount } = renderWithProviders(
      <AppPageContent footer={<span>Custom footer</span>}>Page body</AppPageContent>,
    );
    expect(screen.getByText('Custom footer')).toBeInTheDocument();
    expect(screen.queryByText(/WSO2 LLC/)).not.toBeInTheDocument();
    unmount();

    renderWithProviders(<AppPageContent footer={null}>Page body</AppPageContent>);
    expect(screen.queryByText(/WSO2 LLC/)).not.toBeInTheDocument();
  });

  it('shows one footer when page containers are nested', () => {
    renderWithProviders(
      <AppPageContent noPadding>
        <AppPageContent>Inner page</AppPageContent>
      </AppPageContent>,
    );

    expect(screen.getByText('Inner page')).toBeInTheDocument();
    expect(screen.getAllByText(/WSO2 LLC/)).toHaveLength(1);
  });
});

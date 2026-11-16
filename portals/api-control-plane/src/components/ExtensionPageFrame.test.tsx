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

import { AppPage } from './AppPage';
import { ExtensionPageFrame } from './ExtensionPageFrame';

describe('ExtensionPageFrame', () => {
  it('shows the extension page with the console footer after it', () => {
    renderWithProviders(
      <ExtensionPageFrame>
        <p>Plugin page</p>
      </ExtensionPageFrame>,
    );

    const page = screen.getByText('Plugin page');
    const footer = screen.getByText(/WSO2 LLC/);
    expect(page.compareDocumentPosition(footer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Terms of Use' })).toBeInTheDocument();
  });

  it('shows one footer when a built-in page renders inside the frame', () => {
    renderWithProviders(
      <ExtensionPageFrame>
        <AppPage>Built-in page</AppPage>
      </ExtensionPageFrame>,
    );

    expect(screen.getByText('Built-in page')).toBeInTheDocument();
    expect(screen.getAllByText(/WSO2 LLC/)).toHaveLength(1);
  });

  it('shows one footer when frames are nested', () => {
    renderWithProviders(
      <ExtensionPageFrame>
        <ExtensionPageFrame>
          <p>Plugin page</p>
        </ExtensionPageFrame>
      </ExtensionPageFrame>,
    );

    expect(screen.getAllByText(/WSO2 LLC/)).toHaveLength(1);
  });
});

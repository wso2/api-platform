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
import { runtimeConfig } from '@/config/runtime';
import { DesignWithAiPanel } from './DesignWithAiPanel';

describe('DesignWithAiPanel', () => {
  it('hands off to the API Designer extension in a new tab', () => {
    renderWithProviders(<DesignWithAiPanel />);

    const action = screen.getByRole('link', { name: /Open API Designer/ });
    expect(action).toHaveAttribute('href', runtimeConfig.apiDesignerVsCodeUrl);
    expect(action).toHaveAttribute('target', '_blank');
    // Keeps the opened tab from reaching back through `window.opener`.
    expect(action).toHaveAttribute('rel', expect.stringContaining('noopener'));
  });

  it('links out to the extension documentation', () => {
    renderWithProviders(<DesignWithAiPanel />);

    expect(screen.getByRole('link', { name: /How to get started/ })).toHaveAttribute(
      'href',
      runtimeConfig.apiDesignerDocsUrl,
    );
  });

  it('keeps the editable skeleton as an alternative to leaving the wizard', () => {
    renderWithProviders(<DesignWithAiPanel />);

    expect(screen.getByText(/skeleton on the right/)).toBeInTheDocument();
  });
});

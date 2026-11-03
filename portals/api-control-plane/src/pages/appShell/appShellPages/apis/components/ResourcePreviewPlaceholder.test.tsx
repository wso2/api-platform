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
import { ResourcePreviewPlaceholder } from './ResourcePreviewPlaceholder';

describe('ResourcePreviewPlaceholder', () => {
  it('shows the mock REST rows with the default explanation', () => {
    renderWithProviders(<ResourcePreviewPlaceholder />);

    expect(screen.getByTestId('resource-preview-placeholder')).toBeInTheDocument();
    for (const verb of ['GET', 'POST', 'PUT', 'DELETE']) {
      expect(screen.getByText(verb)).toBeInTheDocument();
    }
    expect(screen.getByText('Resources will show here')).toBeInTheDocument();
    expect(screen.getByText('Import a contract to explore its endpoints')).toBeInTheDocument();
  });

  it('hides the mock rows from assistive technology but keeps the copy readable', () => {
    renderWithProviders(<ResourcePreviewPlaceholder />);

    expect(screen.getByText('GET').closest('[aria-hidden="true"]')).not.toBeNull();
    expect(screen.getByText('Resources will show here').closest('[aria-hidden="true"]')).toBeNull();
  });

  it('accepts a custom title, description, rows and test id', () => {
    renderWithProviders(
      <ResourcePreviewPlaceholder
        description="Add a schema"
        rows={[{ label: 'QUERY', tone: () => ({ badge: 'red', bg: 'blue', border: 'green' }) }]}
        testId="custom"
        title="Operations will show here"
      />,
    );

    expect(screen.getByTestId('custom')).toBeInTheDocument();
    expect(screen.getByText('QUERY')).toBeInTheDocument();
    expect(screen.queryByText('GET')).not.toBeInTheDocument();
    expect(screen.getByText('Operations will show here')).toBeInTheDocument();
    expect(screen.getByText('Add a schema')).toBeInTheDocument();
  });
});

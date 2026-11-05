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

import { screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders } from '@/test/utils';
import { FeatureGate } from './FeatureGate';

const disabled = vi.hoisted(() => ({ names: [] as string[] }));

vi.mock('@/config/features', () => ({
  isFeatureDisabled: (name: string) => disabled.names.includes(name),
}));
vi.mock('@/pages/appShell/appShellPages/system/SystemPages', () => ({
  NotFoundPage: () => <p>Page not found</p>,
}));

const gated = () =>
  renderWithProviders(
    <FeatureGate features={['observability', 'metrics']}>
      <p>Metrics page</p>
    </FeatureGate>,
  );

describe('FeatureGate', () => {
  beforeEach(() => {
    disabled.names = [];
  });

  it('shows the page when nothing is disabled', () => {
    gated();

    expect(screen.getByText('Metrics page')).toBeInTheDocument();
  });

  it.each(['metrics', 'observability'])('answers not found when %s is disabled', (name) => {
    disabled.names = [name];
    gated();

    expect(screen.getByText('Page not found')).toBeInTheDocument();
    expect(screen.queryByText('Metrics page')).not.toBeInTheDocument();
  });

  it('keeps the page when a different feature is disabled', () => {
    disabled.names = ['logs'];
    gated();

    expect(screen.getByText('Metrics page')).toBeInTheDocument();
  });
});

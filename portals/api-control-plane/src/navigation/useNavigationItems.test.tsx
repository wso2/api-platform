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

import { ExtensionsProvider, type ApiControlPlaneExtension } from '../extensions';
import { makeConsoleScope } from '../test/mockScope';
import { renderWithProviders, screen } from '../test/utils';
import { useNavigationItems } from './useNavigationItems';

// A sidebar extension whose routePath ("environments") is also the tail
// segment of an unrelated settings-tab route — the exact collision
// CodeRabbit flagged: the old `pathname.indexOf(routeSegment)` matcher found
// "/environments" as a substring of ".../settings/environments" too, even
// though that route belongs to a completely different extension/feature.
const sidebarExtension: ApiControlPlaneExtension = {
  id: 'environments-sidebar',
  routePath: 'environments',
  render: () => <div>Sidebar Environments</div>,
  label: 'Environments',
  scope: 'project',
  slot: 'sidebar.project',
  order: 50,
};

function Probe() {
  const items = useNavigationItems();
  const item = items.find((entry) => entry.id === 'environments-sidebar');
  return (
    <div data-testid="result">
      {item ? (item.isActive ? 'active' : 'inactive') : 'missing'}
    </div>
  );
}

describe('useNavigationItems sidebar extension matching', () => {
  const scope = makeConsoleScope();

  it('is not active on an unrelated route that merely ends with the same segment name', () => {
    renderWithProviders(
      <ExtensionsProvider extensions={[sidebarExtension]}>
        <Probe />
      </ExtensionsProvider>,
      {
        scope,
        route:
          '/organizations/api-platform-demo/projects/retail-apis/settings/environments',
      }
    );

    expect(screen.getByTestId('result')).toHaveTextContent('inactive');
  });

  it('is active at its own real destination', () => {
    renderWithProviders(
      <ExtensionsProvider extensions={[sidebarExtension]}>
        <Probe />
      </ExtensionsProvider>,
      {
        scope,
        route: '/organizations/api-platform-demo/projects/retail-apis/environments',
      }
    );

    expect(screen.getByTestId('result')).toHaveTextContent('active');
  });
});

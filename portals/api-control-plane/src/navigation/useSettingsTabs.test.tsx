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

import { makeAuthState } from '../test/mockAuthState';
import { makeConsoleScope } from '../test/mockScope';
import { renderWithProviders, screen } from '../test/utils';
import type { NavigationLevel } from './navigationTypes';
import { useSettingsTabs } from './useSettingsTabs';

function TabLabels({ level }: { level: NavigationLevel }) {
  const tabs = useSettingsTabs(level);
  return (
    <ul>
      {tabs.map((tab) => (
        <li key={tab.id}>{tab.label}</li>
      ))}
    </ul>
  );
}

const renderTabs = (level: NavigationLevel, scopes: string[]) =>
  renderWithProviders(<TabLabels level={level} />, {
    authState: makeAuthState({ user: { email: 't@example.com', name: 'T', scopes } }),
    permissionMode: 'enforce',
    scope: makeConsoleScope(),
  });

describe('useSettingsTabs — service accounts', () => {
  it('offers the tab to someone who can manage service accounts', () => {
    renderTabs('organization', ['ap:service_account:manage']);
    expect(screen.getByText('Service accounts')).toBeInTheDocument();
  });

  it('hides it from someone who can only read them', () => {
    renderTabs('organization', ['ap:service_account:read']);
    expect(screen.queryByText('Service accounts')).not.toBeInTheDocument();
    expect(screen.getByText('General')).toBeInTheDocument();
  });

  it('never offers it in project settings', () => {
    renderTabs('project', ['ap:service_account:manage']);
    expect(screen.queryByText('Service accounts')).not.toBeInTheDocument();
  });
});

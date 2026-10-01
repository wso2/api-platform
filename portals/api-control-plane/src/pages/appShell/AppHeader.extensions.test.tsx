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

import {
  ExtensionsProvider,
  HEADER_ACTIONS_SLOT,
  type ApiControlPlaneHeaderAction,
} from '../../extensions';
import { PortProvider, type CloudHostPort } from '../../hostPort';
import { renderWithProviders, screen } from '../../test/utils';

import { AppHeader } from './AppHeader';

const port: CloudHostPort = {
  orgHandle: 'acme',
  navigate: () => {},
  notify: () => {},
  apiFetch: async () => undefined as never,
};

const headerEntry: ApiControlPlaneHeaderAction = {
  id: 'trial-status',
  slot: HEADER_ACTIONS_SLOT,
  order: 10,
  render: () => <span>Trial ends in 14 days</span>,
};

const renderHeader = (extensions: ApiControlPlaneHeaderAction[]) =>
  renderWithProviders(
    <ExtensionsProvider extensions={extensions}>
      <PortProvider value={port}>
        <AppShell>
          <AppShell.Navbar>
            <AppHeader />
          </AppShell.Navbar>
        </AppShell>
      </PortProvider>
    </ExtensionsProvider>
  );

beforeEach(() => {
  vi.spyOn(console, 'error').mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('AppHeader cloud extensions', () => {
  it('renders an entry registered against the header actions slot', async () => {
    renderHeader([headerEntry]);

    expect(await screen.findByText('Trial ends in 14 days')).toBeInTheDocument();
  });

  it('ignores an entry registered against another slot', () => {
    renderHeader([{ ...headerEntry, slot: 'sidebar.organization' }]);

    expect(screen.queryByText('Trial ends in 14 days')).not.toBeInTheDocument();
  });
});

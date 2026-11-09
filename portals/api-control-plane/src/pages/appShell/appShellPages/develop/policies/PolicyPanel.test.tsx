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

import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { aRestApi, collection } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { PolicyPanel } from './PolicyPanel';

const ORG = 'api-platform-demo';

beforeEach(() => {
  resetHttpClient();
  // The plan notes under the panels: an API published nowhere offers no plans.
  server.use(collection('/api-publications', []), collection('/subscription-plans', []));
});

const renderPanel = (readOnly: boolean) => {
  const api = aRestApi({
    policies: [{ name: 'cors', params: {}, version: 'v1' }],
    readOnly,
  });
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <PolicyPanel api={api} />
    </ApiScopeProvider>,
  );
};

describe('PolicyPanel', () => {
  // An API synced from a data-plane gateway is read-only in the control
  // plane, which rejects any change to its runtime artifact — so the panel
  // must not offer one.
  it('lists a gateway-managed API’s policies without any way to change them', async () => {
    const { user } = renderPanel(true);

    expect(screen.getByText('cors')).toBeInTheDocument();
    expect(screen.getByText(/This API was created from a gateway/)).toBeInTheDocument();
    expect(screen.queryByLabelText('Edit policy')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Remove policy')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save changes' })).not.toBeInTheDocument();
    // Nothing to interact with that could make the page dirty either.
    await user.keyboard('{Tab}');
    expect(screen.queryByText(/unsaved change/)).not.toBeInTheDocument();
  });

  it('keeps the editing controls for a control-plane API', () => {
    renderPanel(false);

    expect(screen.getByText('cors')).toBeInTheDocument();
    expect(screen.queryByText(/This API was created from a gateway/)).not.toBeInTheDocument();
    expect(screen.getAllByLabelText('Remove policy').length).toBeGreaterThan(0);
  });

  it('shows the global and resource sections with a method filter', () => {
    renderPanel(false);

    expect(screen.getByText('Global policies')).toBeInTheDocument();
    expect(screen.getByText('Applied to every resource in this API')).toBeInTheDocument();
    expect(screen.getByText('Resource policies')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Filter resources by method' })).toHaveTextContent(
      'All methods',
    );
    expect(screen.getByPlaceholderText('Search resources by path')).toBeInTheDocument();
  });

  it('numbers the global policies as pills that can be edited and removed', () => {
    const api = aRestApi({ policies: [{ name: 'cors', params: {}, version: 'v1' }] });
    renderWithProviders(
      <ApiScopeProvider orgId={ORG}>
        <PolicyPanel api={api} />
      </ApiScopeProvider>,
    );

    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getAllByLabelText('Edit policy').length).toBeGreaterThan(0);
    expect(screen.getAllByLabelText('Remove policy').length).toBeGreaterThan(0);
  });
});

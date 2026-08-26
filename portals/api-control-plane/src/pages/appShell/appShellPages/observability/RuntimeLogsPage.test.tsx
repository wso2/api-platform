/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '../../../../api/core/ApiScopeProvider';
import { runtimeConfig } from '../../../../config/runtime';
import {
  aProject,
  aRestApi,
  collection,
  recorder,
  resource,
  type Recorder,
} from '../../../../test/msw';
import { server } from '../../../../test/server';
import {
  renderWithProviders,
  screen,
  waitFor,
  within,
} from '../../../../test/utils';
import { RuntimeLogsPage } from './RuntimeLogsPage';

let requests: Recorder;

/** Org scope: the scope-less alias binds neither a project nor an API handle. */
const ORG_SCOPE_ROUTE = '/organizations/acme/select-scope/observability/logs';

const renderPage = () =>
  renderWithProviders(
    <ApiScopeProvider orgId="acme">
      <RuntimeLogsPage />
    </ApiScopeProvider>,
    { route: ORG_SCOPE_ROUTE }
  );

/** The filter selects, in render order. */
const filterBox = (
  name: 'timeRange' | 'project' | 'component' | 'environment' | 'level'
) => {
  const order = ['timeRange', 'project', 'component', 'environment', 'level'];
  return screen.getAllByRole('combobox')[order.indexOf(name)];
};

const applyFilters = async (user: ReturnType<typeof renderPage>['user']) => {
  const before = requests.count();
  await user.click(screen.getByRole('button', { name: 'Apply filters' }));
  await waitFor(() => expect(requests.count()).toBeGreaterThan(before));
};

beforeEach(() => {
  requests = recorder();
  vi.spyOn(
    runtimeConfig,
    'observabilityLogsEnabled',
    'get' as never
  ).mockReturnValue(true as never);

  server.use(
    collection('/projects', [
      aProject({ id: 'default-project', displayName: 'Default Project' }),
      aProject({ id: 'retail', displayName: 'Retail APIs' }),
    ]),
    collection('/rest-apis', [
      aRestApi({ id: 'orders', displayName: 'Orders API' }),
    ]),
    resource(
      '/observability/logs',
      {
        items: [
          { timestamp: '2026-08-26T10:00:00Z', level: 'INFO', log: 'hello' },
        ],
        pagination: { limit: 100, nextCursor: null },
      },
      { record: requests }
    )
  );
});

describe('RuntimeLogsPage filters', () => {
  it('shows what each unfiltered dropdown has selected', async () => {
    renderPage();
    await waitFor(() => expect(requests.count()).toBeGreaterThan(0));

    // MUI renders a zero-width space for a selected empty value unless
    // `displayEmpty` is set, which left every default reading as blank.
    expect(filterBox('project')).toHaveTextContent('All projects');
    expect(filterBox('component')).toHaveTextContent('All components');
    expect(filterBox('level')).toHaveTextContent('All levels');
    expect(filterBox('timeRange')).toHaveTextContent('Last hour');
    expect(filterBox('environment')).toHaveTextContent('Development');
  });

  it('omits every narrowing filter until one is chosen', async () => {
    renderPage();
    await waitFor(() => expect(requests.count()).toBeGreaterThan(0));

    const params = requests.last()!.params;
    expect(params.has('project')).toBe(false);
    expect(params.has('component')).toBe(false);
    expect(params.getAll('logLevel')).toEqual([]);
    expect(params.has('query')).toBe(false);
  });

  it('sends the selected project handle', async () => {
    const { user } = renderPage();
    await waitFor(() => expect(requests.count()).toBeGreaterThan(0));

    await user.click(filterBox('project'));
    await user.click(
      within(await screen.findByRole('listbox')).getByText('Default Project')
    );
    expect(filterBox('project')).toHaveTextContent('Default Project');

    await applyFilters(user);
    expect(requests.last()!.params.get('project')).toBe('default-project');
  });

  it('sends one logLevel parameter per selected level', async () => {
    const { user } = renderPage();
    await waitFor(() => expect(requests.count()).toBeGreaterThan(0));

    await user.click(filterBox('level'));
    const listbox = await screen.findByRole('listbox');
    await user.click(within(listbox).getByRole('option', { name: /ERROR/ }));
    await user.click(within(listbox).getByRole('option', { name: /WARN/ }));
    await user.keyboard('{Escape}');

    expect(filterBox('level')).toHaveTextContent('ERROR, WARN');

    await applyFilters(user);
    expect(requests.last()!.params.getAll('logLevel')).toEqual([
      'ERROR',
      'WARN',
    ]);
  });

  it('drops the level filter again when every level is deselected', async () => {
    const { user } = renderPage();
    await waitFor(() => expect(requests.count()).toBeGreaterThan(0));

    await user.click(filterBox('level'));
    await user.click(
      within(await screen.findByRole('listbox')).getByRole('option', {
        name: /ERROR/,
      })
    );
    await user.keyboard('{Escape}');
    await applyFilters(user);
    expect(requests.last()!.params.getAll('logLevel')).toEqual(['ERROR']);

    await user.click(filterBox('level'));
    await user.click(
      within(await screen.findByRole('listbox')).getByRole('option', {
        name: /ERROR/,
      })
    );
    await user.keyboard('{Escape}');
    expect(filterBox('level')).toHaveTextContent('All levels');

    await applyFilters(user);
    expect(requests.last()!.params.getAll('logLevel')).toEqual([]);
  });

  it('offers components only once a project scopes the API list', async () => {
    const { user } = renderPage();
    await waitFor(() => expect(requests.count()).toBeGreaterThan(0));

    // `useRestApis` is disabled without a project, so there is nothing to list.
    expect(filterBox('component')).toHaveAttribute('aria-disabled', 'true');

    await user.click(filterBox('project'));
    await user.click(
      within(await screen.findByRole('listbox')).getByText('Default Project')
    );
    await waitFor(() =>
      expect(filterBox('component')).not.toHaveAttribute(
        'aria-disabled',
        'true'
      )
    );

    await user.click(filterBox('component'));
    await user.click(
      within(await screen.findByRole('listbox')).getByText('Orders API')
    );

    await applyFilters(user);
    expect(requests.last()!.params.get('component')).toBe('orders');
    expect(requests.last()!.params.get('project')).toBe('default-project');
  });

  it('clears a chosen component when the project changes under it', async () => {
    const { user } = renderPage();
    await waitFor(() => expect(requests.count()).toBeGreaterThan(0));

    await user.click(filterBox('project'));
    await user.click(
      within(await screen.findByRole('listbox')).getByText('Default Project')
    );
    await waitFor(() =>
      expect(filterBox('component')).not.toHaveAttribute(
        'aria-disabled',
        'true'
      )
    );
    await user.click(filterBox('component'));
    await user.click(
      within(await screen.findByRole('listbox')).getByText('Orders API')
    );
    expect(filterBox('component')).toHaveTextContent('Orders API');

    await user.click(filterBox('project'));
    await user.click(
      within(await screen.findByRole('listbox')).getByText('Retail APIs')
    );

    // A component belongs to one project; keeping it would 404 the query.
    expect(filterBox('component')).toHaveTextContent('All components');
    await applyFilters(user);
    expect(requests.last()!.params.has('component')).toBe(false);
  });
});

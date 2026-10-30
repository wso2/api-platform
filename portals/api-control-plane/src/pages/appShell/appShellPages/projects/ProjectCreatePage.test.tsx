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

import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import { routes } from '@/routes/paths';
import { accepts, aProject, failure, recorder, type Recorder } from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { ProjectCreatePage } from './ProjectCreatePage';

const ORG = 'api-platform-demo';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

/** The create needs `ApiScopeContext` for its organization, so the scope provider is mounted. */
function setup() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <Routes>
        <Route path={routes.newProject()} element={<ProjectCreatePage />} />
        <Route path={routes.projectHome()} element={<div>Project home</div>} />
        <Route path={routes.projects()} element={<div>Project list</div>} />
      </Routes>
    </ApiScopeProvider>,
    { route: routes.newProject(ORG) },
  );
}

describe('ProjectCreatePage', () => {
  it('disables Create until a name is entered', () => {
    setup();
    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
  });

  it('creates a project with name + description, then opens its home', async () => {
    server.use(accepts('post', '/projects', aProject({ id: 'billing' }), { record: requests }));
    const { user } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Billing');
    await user.type(screen.getByLabelText(/Description/), 'Invoices');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'Billing',
      id: 'billing',
      description: 'Invoices',
    });
    await screen.findByText('Project home');
  });

  it('derives the project ID from the name until the edit icon is used', async () => {
    server.use(accepts('post', '/projects', aProject({ id: 'billing-v2' }), { record: requests }));
    const { user } = setup();

    expect(screen.getByRole('button', { name: 'Edit project ID' })).toBeDisabled();
    await user.type(screen.getByLabelText(/Name/), 'Retail APIs');
    expect(screen.getByLabelText(/Project ID/)).toHaveValue('retail-apis');
    expect(screen.getByLabelText(/Project ID/)).toBeDisabled();

    await user.click(screen.getByRole('button', { name: 'Edit project ID' }));
    const idField = screen.getByLabelText(/Project ID/);
    await user.clear(idField);
    await user.type(idField, 'billing-v2');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toMatchObject({ id: 'billing-v2' });
  });

  it('stays on the page and surfaces the error message on failure', async () => {
    server.use(
      failure('post', '/projects', 409, 'PROJECT_ALREADY_EXISTS', {
        message: 'Project already exists in organization',
      }),
    );
    const { user } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Retail APIs');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    await screen.findByText('Project already exists in organization');
    expect(screen.queryByText('Project home')).not.toBeInTheDocument();
  });
});

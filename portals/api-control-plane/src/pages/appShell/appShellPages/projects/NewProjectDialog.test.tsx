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

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '../../../../api/core/ApiScopeProvider';
import { resetHttpClient } from '../../../../api/core/http';
import {
  accepts,
  aProject,
  failure,
  recorder,
  type Recorder,
} from '../../../../test/msw';
import { server } from '../../../../test/server';
import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import { NewProjectDialog } from './NewProjectDialog';

const ORG = 'api-platform-demo';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

/**
 * The dialog takes its organization from `ApiScopeContext` rather than its
 * `orgHandle` prop (which only builds the post-create redirect), so the scope
 * provider has to be mounted for the create to be allowed out at all.
 */
function setup() {
  const onClose = vi.fn();
  const utils = renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <NewProjectDialog onClose={onClose} open orgHandle={ORG} />
    </ApiScopeProvider>
  );
  return { ...utils, onClose };
}

describe('NewProjectDialog', () => {
  it('disables Create until a name is entered', () => {
    setup();
    expect(screen.getByRole('button', { name: 'Create' })).toBeDisabled();
  });

  it('creates a project with name + description, then closes', async () => {
    server.use(
      accepts('post', '/projects', aProject({ id: 'billing' }), {
        record: requests,
      })
    );
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Billing');
    await user.type(screen.getByLabelText(/Description/), 'Invoices');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(JSON.parse(requests.last()!.body)).toEqual({
      displayName: 'Billing',
      description: 'Invoices',
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('keeps the dialog open and surfaces the error message on failure', async () => {
    server.use(
      failure('post', '/projects', 409, 'PROJECT_ALREADY_EXISTS', {
        message: 'Project already exists in organization',
      })
    );
    const { user, onClose } = setup();

    await user.type(screen.getByLabelText(/Name/), 'Retail APIs');
    await user.click(screen.getByRole('button', { name: 'Create' }));

    await screen.findByText('Project already exists in organization');
    expect(onClose).not.toHaveBeenCalled();
  });
});

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

import { waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';

import {
  aProject,
  accepts,
  noContent,
  recorder,
  type Recorder,
} from '../../../test/msw';
import { renderApiHook, settle } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { ClientErrorCode } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import {
  useCreateProject,
  useDeleteProject,
  useUpdateProject,
} from './projects.hooks';
import { projectKeys } from './projects.queries';

/**
 * Hook-layer tests for the mutation scope gate.
 *
 * Queries hold themselves back with `enabled` until the organization is known.
 * A mutation has no such gate — a button is clickable while the route is still
 * resolving — so each one has to refuse the call itself. What must not happen is
 * the request going out with no `X-Org-Id`: the server would then apply the
 * write to whatever scope it infers, which is the wrong organization by
 * definition.
 */

const PROJECT_ID = 'retail';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('project mutations without an organization scope', () => {
  it('useCreateProject refuses to send an unscoped create', async () => {
    server.use(accepts('post', '/projects', aProject(), { record: requests }));

    const { result } = renderApiHook(() => useCreateProject(), {
      orgId: undefined,
    });
    result.current.mutate({ displayName: 'Wholesale' });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.code).toBe(
      ClientErrorCode.CLIENT_MISSING_ORG_SCOPE
    );
    await settle();
    expect(requests.count()).toBe(0);
  });

  it('useUpdateProject refuses to send an unscoped update', async () => {
    server.use(
      accepts('put', `/projects/${PROJECT_ID}`, aProject(), { record: requests })
    );

    const { result } = renderApiHook(() => useUpdateProject(), {
      orgId: undefined,
    });
    result.current.mutate({
      projectId: PROJECT_ID,
      body: aProject({ displayName: 'Retail (renamed)' }),
    });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.code).toBe(
      ClientErrorCode.CLIENT_MISSING_ORG_SCOPE
    );
    await settle();
    expect(requests.count()).toBe(0);
  });

  it('useDeleteProject refuses to send an unscoped delete', async () => {
    server.use(
      noContent('delete', `/projects/${PROJECT_ID}`, { record: requests })
    );

    const { result } = renderApiHook(() => useDeleteProject(), {
      orgId: undefined,
    });
    result.current.mutate({ projectId: PROJECT_ID });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.code).toBe(
      ClientErrorCode.CLIENT_MISSING_ORG_SCOPE
    );
    await settle();
    expect(requests.count()).toBe(0);
  });

  it('reports the refusal as an ApiError the UI can render, not a crash', async () => {
    // The global mutation handler only forwards ApiError to the snackbar, and
    // the retry policy only reads ApiError — a bare Error here would be
    // swallowed and the user would see a button that silently did nothing.
    server.use(accepts('post', '/projects', aProject(), { record: requests }));

    const { result } = renderApiHook(() => useCreateProject(), {
      orgId: undefined,
    });
    result.current.mutate({ displayName: 'Wholesale' });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.name).toBe('ApiError');
    expect(result.current.error?.operation).toBe('CreateProject');
    expect(result.current.error?.isRetryable).toBe(false);
  });
});

describe('project mutations with an organization scope', () => {
  it('still sends the scope header once the organization is known', async () => {
    server.use(accepts('post', '/projects', aProject({ id: 'wholesale' }), { record: requests }));

    const { result } = renderApiHook(() => useCreateProject());
    result.current.mutate({ displayName: 'Wholesale' });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(requests.last()?.headers.get('X-Org-Id')).toBe('acme-org');
  });

  it('seeds the detail cache from the create response', async () => {
    server.use(accepts('post', '/projects', aProject({ id: 'wholesale' })));

    const { result, queryClient, org } = renderApiHook(() => useCreateProject());
    result.current.mutate({ displayName: 'Wholesale' });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(
      queryClient.getQueryData(projectKeys.detail(org, 'wholesale'))
    ).toMatchObject({ id: 'wholesale' });
  });
});

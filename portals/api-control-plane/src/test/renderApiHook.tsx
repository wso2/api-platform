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

import { type ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { renderHook } from '@testing-library/react';

import { ApiScopeProvider } from '../api/core/ApiScopeProvider';
import { orgScope, type OrgScope } from '../api/core/queryKeys';
import { createQueryClient } from '../api/core/queryClient';

/**
 * Harness for resource-hook tests.
 *
 * Deliberately smaller than `renderWithProviders`: a data hook needs a query
 * client and a scope and nothing else, so leaving out the router, theme and
 * auth keeps a failure attributable to the hook rather than the app shell.
 *
 * Requests are **not** stubbed — they run through the real transport to MSW, so
 * these tests exercise the same path production does. Stubbing the fetcher
 * would only assert that the test's own mock was called.
 */

export const TEST_ORG = 'acme-org';
export const TEST_PROJECT = 'retail';

export function renderApiHook<TResult>(
  hook: () => TResult,
  scope: { orgId?: string; projectId?: string } = {}
) {
  const queryClient = createQueryClient();
  const orgId = 'orgId' in scope ? scope.orgId : TEST_ORG;
  const projectId = 'projectId' in scope ? scope.projectId : TEST_PROJECT;

  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <ApiScopeProvider orgId={orgId} projectId={projectId}>
        {children}
      </ApiScopeProvider>
    </QueryClientProvider>
  );

  return {
    ...renderHook(hook, { wrapper }),
    queryClient,
    /** The branded scope the hooks build their keys from. */
    org: orgScope(orgId) as OrgScope,
  };
}

/**
 * Waits a beat so an assertion about something *not* happening is meaningful.
 *
 * `waitFor` cannot express absence — it passes on the first tick and proves
 * nothing. To show that no request fired or no cache entry was written, let the
 * work that would have done it have a chance to run first.
 */
export const settle = (ms = 40): Promise<void> =>
  new Promise((resolve) => {
    setTimeout(resolve, ms);
  });

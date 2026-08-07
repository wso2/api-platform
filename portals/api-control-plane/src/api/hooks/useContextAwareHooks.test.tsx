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

import { describe, expect, it, vi } from 'vitest';

import type { Api } from '../../types/domain';
import { makeConsoleScope } from '../../test/mockScope';
import { renderWithProviders, screen, waitFor } from '../../test/utils';
import { useApis } from './useMvpQueries';

// Verifies the Phase-3 wiring end-to-end with NO vi.mock: a context-aware hook
// (`useApis()` with no args) resolves org/project from the injected
// ConsoleScope and calls the API client provided via ApiClientProvider.
function Probe() {
  const query = useApis();
  return (
    <div data-testid="result">
      {query.isLoading ? 'loading' : `count:${query.data?.length ?? 0}`}
    </div>
  );
}

describe('context-aware hooks', () => {
  it('useApis() resolves scope from context and calls the injected client', async () => {
    const scope = makeConsoleScope();
    const listApis = vi
      .fn<(org: string, project: string) => Promise<Api[]>>()
      .mockResolvedValue([{ handler: 'a' } as Api, { handler: 'b' } as Api]);

    renderWithProviders(<Probe />, { scope, apiClient: { listApis } });

    await waitFor(() =>
      expect(screen.getByTestId('result')).toHaveTextContent('count:2')
    );
    // Called with the token-ready active scope (not undefined) — no args passed.
    expect(listApis).toHaveBeenCalledWith(
      scope.activeScope.orgHandle,
      scope.activeScope.projectHandler
    );
  });
});

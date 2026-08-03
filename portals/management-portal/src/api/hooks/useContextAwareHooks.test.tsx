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

import { describe, expect, it } from 'vitest';

import type { Api } from '../../types/domain';
import { getApiCapabilities } from './apiCapabilities';

const baseApi: Api = {
  id: 'component-1',
  projectId: 'project-1',
  name: 'orders-api',
  displayName: 'Orders API',
  handler: 'orders-api',
  kind: 'API_PROXY',
  status: 'ACTIVE',
  httpBased: true,
};

describe('getApiCapabilities', () => {
  it('enables cURL testing and deploy for API proxy components', () => {
    const capabilities = getApiCapabilities(baseApi);

    expect(capabilities.canDeploy).toBe(true);
    expect(capabilities.canTest).toBe(true);
    expect(capabilities.testMode).toBe('curl');
  });

  it('disables component scoped entries without component data', () => {
    const capabilities = getApiCapabilities();

    expect(capabilities.canDeploy).toBe(false);
    expect(capabilities.testMode).toBe('none');
  });
});

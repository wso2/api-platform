import type { ApiProxy } from '../../types/domain';
import { toApiProxy } from '../adapters';
import { apiProxies } from '../mocks/data';
import { readDerivedApiProxy } from '../apis/apiDerivedDataStore';
import { delay, useMockApi } from '../shared/apiClientUtils';

export async function getApiProxy(
  componentId: string
): Promise<ApiProxy | undefined> {
  if (useMockApi()) {
    await delay();
    const apiProxy = apiProxies.find((proxy) => proxy.componentId === componentId);
    return apiProxy ? toApiProxy(apiProxy) : undefined;
  }

  return readDerivedApiProxy(componentId);
}

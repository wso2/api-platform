import type { Environment } from '../../types/domain';
import { toEnvironment } from '../adapters';
import { environments } from '../mocks/data';
import { delay, useMockApi } from '../shared/apiClientUtils';
import { readDerivedEnvironments } from '../apis/apiDerivedDataStore';

export async function listEnvironments(): Promise<Environment[]> {
  if (useMockApi()) {
    await delay();
    return environments.map(toEnvironment);
  }

  return readDerivedEnvironments();
}

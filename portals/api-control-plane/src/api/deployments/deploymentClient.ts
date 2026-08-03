import type { Deployment } from '../../types/domain';
import { toDeployment } from '../adapters';
import { readDerivedDeployments } from '../apis/apiDerivedDataStore';
import { deployments } from '../mocks/data';
import { delay, useMockApi } from '../shared/apiClientUtils';

export async function listDeployments(
  componentId: string
): Promise<Deployment[]> {
  if (useMockApi()) {
    await delay();
    return deployments
      .filter((deployment) => deployment.componentId === componentId)
      .map(toDeployment);
  }

  return readDerivedDeployments(componentId);
}

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

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import DeployFeature from './DeployFeature';
import type { ArtifactKind } from './deployApi';
import { featureArtifactKind } from './featureUsage';
import type { CloudHostPort } from './hostPort';

/*
 * This page replaces the console's built-in Deploy page in the cloud, so it
 * reports the same console features through the host's `trackFeature`.
 */

/** A pipeline with one environment whose one gateway runs a deployment. */
const portWithDeployment = () => {
  const apiFetch = vi.fn(async (method: string, path: string) => {
    if (method === 'POST') return {};
    if (path.includes('/deployments')) {
      return {
        list: [{ environment: 'dev', gateways: [{ gatewayId: 'edge', deploymentId: 'd-1', status: 'DEPLOYED' }] }],
      };
    }
    if (path.startsWith('/managed-gateways')) return { list: [{ id: 'edge', displayName: 'Edge', isActive: true }] };
    if (path.includes('/builds')) return { list: [] };
    return { name: 'Orders' };
  });
  const trackFeature = vi.fn();
  const port: CloudHostPort = {
    orgHandle: 'acme',
    projectHandle: 'shop',
    apiHandle: 'orders',
    navigate: () => {},
    notify: () => {},
    apiFetch: apiFetch as unknown as CloudHostPort['apiFetch'],
    trackFeature,
  };
  return { apiFetch, port, trackFeature };
};

const stop = async (kind: ArtifactKind) => {
  const { apiFetch, port, trackFeature } = portWithDeployment();
  render(<DeployFeature kind={kind} port={port} />);

  // The row's actions sit in a collapsed panel, so the button is found by its text.
  const button = (await screen.findByText('Stop deployment')).closest('button')!;
  await waitFor(() => expect(button.disabled).toBe(false));
  fireEvent.click(button);
  await waitFor(() =>
    expect(apiFetch.mock.calls.some(([method, path]) => method === 'POST' && String(path).includes('/undeploy'))).toBe(
      true
    )
  );
  return trackFeature;
};

describe('DeployFeature feature usage', () => {
  it('reports Stop as an undeploy click, with the kind of API', async () => {
    const trackFeature = await stop('RestApi');

    expect(trackFeature).toHaveBeenCalledWith('apip.lifecycle.deployment-undeploy-click', {
      artifact_kind: 'rest-api',
    });
  });

  it('reports nothing for an artifact that is not a console API', async () => {
    const trackFeature = await stop('Mcp');

    expect(trackFeature).not.toHaveBeenCalled();
  });
});

describe('featureArtifactKind', () => {
  it('names only the console API kinds', () => {
    expect(featureArtifactKind('RestApi')).toBe('rest-api');
    expect(featureArtifactKind('GraphQLApi')).toBe('graphql-api');
    expect(featureArtifactKind('LlmProxy')).toBeUndefined();
  });
});

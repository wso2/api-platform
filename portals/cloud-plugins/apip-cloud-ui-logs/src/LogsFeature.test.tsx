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

import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { AIWorkspaceHostPort } from './hostPort';
import LogsFeature from './LogsFeature';

/** A port whose user holds exactly `held`, over an apiFetch that answers every read empty. */
const portHolding = (held: string[]) => {
  const apiFetch = vi.fn(async () => ({ list: [] }));
  const port: AIWorkspaceHostPort = {
    orgHandle: 'acme',
    navigate: () => {},
    notify: () => {},
    apiFetch: apiFetch as unknown as AIWorkspaceHostPort['apiFetch'],
    permissions: { canAny: (scopes) => scopes.some((scope) => held.includes(scope)) },
  };
  return { port, apiFetch };
};

describe('LogsFeature permissions', () => {
  it('says so and queries nothing when the user may not read gateway logs', () => {
    const { port, apiFetch } = portHolding(['ap:environment:read']);
    render(<LogsFeature port={port} />);

    expect(screen.getByText(/don't have permission to view runtime logs/i)).toBeTruthy();
    expect(apiFetch).not.toHaveBeenCalled();
  });

  it('queries logs for a user holding a gateway read scope', async () => {
    const { port, apiFetch } = portHolding(['ap:gateway:read']);
    render(<LogsFeature port={port} />);

    await waitFor(() =>
      expect(apiFetch).toHaveBeenCalledWith('GET', expect.stringMatching(/^\/logs\?/))
    );
    // Environments are their own scope, which this user lacks, so that read is skipped.
    expect(apiFetch).not.toHaveBeenCalledWith('GET', '/environments');
  });
});

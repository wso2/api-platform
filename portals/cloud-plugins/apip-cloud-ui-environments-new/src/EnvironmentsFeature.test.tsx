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

import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import EnvironmentsFeature from './EnvironmentsFeature';
import type { AIWorkspaceHostPort } from './hostPort';

const environment = { id: 'dev', name: 'dev', isProduction: false, createdAt: '2026-01-01T00:00:00Z' };

/** A port whose user holds exactly `held`, over an apiFetch that serves one environment. */
const portHolding = (held: string[]) => {
  const apiFetch = vi.fn(async () => ({ list: [environment] }));
  const port: AIWorkspaceHostPort = {
    orgHandle: 'acme',
    navigate: () => {},
    notify: () => {},
    apiFetch: apiFetch as unknown as AIWorkspaceHostPort['apiFetch'],
    permissions: { canAny: (scopes) => scopes.some((scope) => held.includes(scope)) },
  };
  return { port, apiFetch };
};

const isDisabled = (element: HTMLElement) => (element as HTMLButtonElement).disabled;

describe('EnvironmentsFeature permissions', () => {
  it('says so and loads nothing when the user may not list environments', () => {
    const { port, apiFetch } = portHolding(['ap:project:read']);
    render(<EnvironmentsFeature port={port} />);

    expect(screen.getByText(/don't have permission to view environments/i)).toBeTruthy();
    expect(apiFetch).not.toHaveBeenCalled();
  });

  it('disables Create and Delete for a user who may only read', async () => {
    const { port } = portHolding(['ap:environment:read']);
    render(<EnvironmentsFeature port={port} />);

    expect(isDisabled(screen.getByRole('button', { name: 'Create' }))).toBe(true);
    expect(isDisabled(await screen.findByRole('button', { name: 'Delete dev' }))).toBe(true);
  });

  it('offers Create and Delete to a user holding the broader manage scope', async () => {
    const { port } = portHolding(['ap:environment:manage']);
    render(<EnvironmentsFeature port={port} />);

    expect(isDisabled(screen.getByRole('button', { name: 'Create' }))).toBe(false);
    expect(isDisabled(await screen.findByRole('button', { name: 'Delete dev' }))).toBe(false);
  });
});

/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// An in-memory EnvironmentPort. This is the only EnvironmentPort that exists
// today — there is no real backend yet, so EnvironmentsPage constructs one of
// these per mount. Swap in a real, BFF-backed implementation later without
// touching EnvironmentsList/useEnvironmentList, which only ever see the
// EnvironmentPort interface.

import type { CloudEnvironment, CreateEnvironmentInput, EnvironmentPort } from './types';

const toName = (displayName: string) =>
  displayName
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

const delay = <T>(value: T, ms = 350): Promise<T> =>
  new Promise((resolve) => setTimeout(() => resolve(value), ms));

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value));

/** Builds a fresh in-memory port, optionally seeded with environments. */
export function createMockEnvironmentPort(seed?: CloudEnvironment[]): EnvironmentPort {
  let seq = 1;
  // Skips any candidate already present in `environments` — guards against a
  // caller-supplied `seed` that happens to contain an `env-<n>`-shaped id.
  const nextId = (prefix: string) => {
    let candidate: string;
    do {
      candidate = `${prefix}-${seq++}`;
    } while (environments.some((e) => e.id === candidate));
    return candidate;
  };
  const nowIso = () => new Date().toISOString();
  const environments: CloudEnvironment[] =
    seed ?? [
      {
        id: 'env-development',
        name: 'development',
        displayName: 'Development',
        isProduction: false,
        createdAt: nowIso(),
      },
      {
        id: 'env-staging',
        name: 'staging',
        displayName: 'Staging',
        isProduction: false,
        createdAt: nowIso(),
      },
      {
        id: 'env-production',
        name: 'production',
        displayName: 'Production',
        isProduction: true,
        createdAt: nowIso(),
      },
    ];

  return {
    async list() {
      return delay(clone(environments));
    },
    async create(input: CreateEnvironmentInput) {
      const name = toName(input.displayName);
      if (!name) throw new Error('A valid environment name is required');
      if (environments.some((e) => e.name === name)) {
        throw new Error(`An environment named "${name}" already exists`);
      }
      const env: CloudEnvironment = {
        id: nextId('env'),
        name,
        displayName: input.displayName.trim(),
        isProduction: input.isProduction,
        createdAt: nowIso(),
      };
      environments.push(env);
      return delay(clone(env));
    },
    async remove(envId: string) {
      const idx = environments.findIndex((e) => e.id === envId);
      if (idx === -1) throw new Error('Environment not found');
      environments.splice(idx, 1);
      await delay(undefined);
    },
  };
}

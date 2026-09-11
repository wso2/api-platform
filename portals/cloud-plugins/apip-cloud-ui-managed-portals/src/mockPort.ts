/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// In-memory PortalPort fallback for tests / storybook when no platform-api base is configured.

import type {
  CreateManagedPortalInput,
  ManagedPortal,
  OrgEnvironment,
  PortalPort,
  UpdateManagedPortalInput,
} from './types';

const delay = <T>(value: T, ms = 300): Promise<T> =>
  new Promise((resolve) => setTimeout(() => resolve(value), ms));

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value));

/** Builds an in-memory PortalPort, optionally seeded. Seed is copied so callers can reuse it across instances. */
export function createMockPortalPort(seed?: ManagedPortal[]): PortalPort {
  const portals: ManagedPortal[] = seed ? clone(seed) : [];

  return {
    async list() {
      return delay(clone(portals));
    },
    async get(id: string) {
      const found = portals.find((p) => p.id === id);
      if (!found) throw new Error('Portal not found');
      return delay(clone(found));
    },
    async create(input: CreateManagedPortalInput) {
      const handle = input.handle.trim();
      const name = input.name.trim();
      if (!handle) throw new Error('A portal handle is required');
      if (!name) throw new Error('A portal name is required');
      if (portals.some((p) => p.handle === handle)) {
        throw new Error(`A portal "${handle}" already exists`);
      }
      const portal: ManagedPortal = {
        id: handle,
        handle,
        name,
        description: input.description?.trim() || undefined,
        loginEnvironment: input.loginEnvironment?.trim() || 'production',
        url: `https://pending-${handle}.portals.invalid`,
        updatedAt: new Date().toISOString(),
      };
      portals.push(portal);
      return delay(clone(portal));
    },
    async update(id: string, input: UpdateManagedPortalInput) {
      const portal = portals.find((p) => p.id === id);
      if (!portal) throw new Error('Portal not found');
      if (input.name !== undefined) {
        const trimmed = input.name.trim();
        if (!trimmed) throw new Error('Name cannot be empty');
        portal.name = trimmed;
      }
      if (input.description !== undefined) {
        portal.description = input.description.trim() || undefined;
      }
      if (input.loginEnvironment !== undefined) {
        const trimmed = input.loginEnvironment.trim();
        if (!trimmed) throw new Error('loginEnvironment cannot be empty');
        portal.loginEnvironment = trimmed;
      }
      portal.updatedAt = new Date().toISOString();
      return delay(clone(portal));
    },
    async remove(id: string) {
      const idx = portals.findIndex((p) => p.id === id);
      if (idx === -1) throw new Error('Portal not found');
      portals.splice(idx, 1);
      await delay(undefined);
    },
    async listEnvironments(): Promise<OrgEnvironment[]> {
      // Fixed non-empty list so form interactions render sensibly offline.
      return delay([
        { name: 'development', displayName: 'Development' },
        { name: 'staging', displayName: 'Staging' },
        { name: 'production', displayName: 'Production' },
      ]);
    },
  };
}

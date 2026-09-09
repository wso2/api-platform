/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 *
 * The real PortalPort — talks to apip-platform-api through the console BFF's
 * same-origin proxy. Mirrors how ManagedGatewaysPage's realPort reaches
 * apip-platform-api's `/managed-gateways` resource; here we hit the sibling
 * `/managed-api-portals` resource added by the same plugin.
 *
 * LIST projects the cloud OAS ManagedApiPortalList; the core row's list
 * projection strips metadata by design, so `loginEnvironment` is empty on list
 * results and populated only by GET (matching the plugin service's own
 * projection). CREATE, PUT and DELETE go to the same resource.
 */

import type {
  CreateManagedPortalInput,
  ManagedPortal,
  OrgEnvironment,
  PortalPort,
  UpdateManagedPortalInput,
} from './types';

const CSRF_HEADER = 'X-Requested-By';
const CSRF_HEADER_VALUE = 'api-control-plane';
const ORG_HEADER = 'X-Org-Id';

type WindowRuntimeConfig = Partial<{
  platformApiBaseUrl: string;
  PLATFORM_API_BASE_URL: string;
  platformApiVersion: string;
  PLATFORM_API_VERSION: string;
}>;

function windowConfig(): WindowRuntimeConfig {
  if (typeof window === 'undefined') return {};
  const w = window as unknown as {
    __RUNTIME_CONFIG__?: WindowRuntimeConfig;
    config?: WindowRuntimeConfig;
  };
  return { ...(w.__RUNTIME_CONFIG__ ?? {}), ...(w.config ?? {}) };
}

/**
 * The same-origin request base for platform-api calls
 * (`${platformApiBaseUrl}/api/${version}`, e.g. `/proxy/api/v0.9`), or `null`
 * when the console has no platform-api base configured (tests → mock port).
 */
export function resolveApiBase(): string | null {
  const cfg = windowConfig();
  const base = cfg.platformApiBaseUrl || cfg.PLATFORM_API_BASE_URL || '';
  if (!base) return null;
  const version = cfg.platformApiVersion || cfg.PLATFORM_API_VERSION || 'v0.9';
  return `${base}/api/${version}`;
}

async function request<T>(
  base: string,
  orgRef: string,
  method: string,
  path: string,
  body?: unknown
): Promise<T> {
  const mutating = method !== 'GET';
  const response = await fetch(`${base}${path}`, {
    method,
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      [ORG_HEADER]: orgRef,
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...(mutating ? { [CSRF_HEADER]: CSRF_HEADER_VALUE } : {}),
    },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  });

  if (!response.ok) {
    let message = `Request failed (${response.status})`;
    try {
      const errBody = (await response.json()) as {
        message?: string;
        description?: string;
        error?: string;
      };
      message = errBody.description || errBody.message || errBody.error || message;
    } catch {
      // non-JSON error body — keep the status-based message
    }
    throw new Error(message);
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

// Wire shapes from the plugin's OAS (services/apip-platform-api/resources/openapi.yaml).
type WirePortal = {
  id?: string;
  handle?: string;
  name: string;
  description?: string | null;
  url?: string;
  loginEnvironment?: string;
  updatedAt?: string;
};
type WirePortalList = { count?: number; list?: WirePortal[] };

// EnvironmentList shape from the plugin's OAS. `name` is what the plugin's
// loginEnvironment field expects; the response carries more fields we ignore.
type WireEnvironment = { name?: string; displayName?: string };
type WireEnvironmentList = { count?: number; list?: WireEnvironment[] };

function fromWire(w: WirePortal): ManagedPortal {
  return {
    id: w.id ?? w.handle ?? '',
    handle: w.handle ?? w.id ?? '',
    name: w.name,
    description: w.description ?? undefined,
    url: w.url,
    loginEnvironment: w.loginEnvironment,
    updatedAt: w.updatedAt,
  };
}

/**
 * A PortalPort backed by the console BFF proxy at `base`
 * (e.g. `/proxy/api/v0.9`), scoped to `orgHandle` (sent as X-Org-Id).
 */
export function createRealPortalPort(base: string, orgHandle: string): PortalPort {
  const url = (id: string) => `/managed-api-portals/${encodeURIComponent(id)}`;
  return {
    async list() {
      const body = await request<WirePortalList>(base, orgHandle, 'GET', '/managed-api-portals');
      return (body.list ?? []).map(fromWire);
    },
    async get(id: string) {
      const body = await request<WirePortal>(base, orgHandle, 'GET', url(id));
      return fromWire(body);
    },
    async create(input: CreateManagedPortalInput) {
      const body = await request<WirePortal>(base, orgHandle, 'POST', '/managed-api-portals', {
        handle: input.handle,
        name: input.name,
        description: input.description,
        loginEnvironment: input.loginEnvironment,
      });
      return fromWire(body);
    },
    async update(id: string, input: UpdateManagedPortalInput) {
      const body = await request<WirePortal>(base, orgHandle, 'PUT', url(id), {
        name: input.name,
        description: input.description,
        loginEnvironment: input.loginEnvironment,
      });
      return fromWire(body);
    },
    async remove(id: string) {
      await request<void>(base, orgHandle, 'DELETE', url(id));
    },
    async listEnvironments(): Promise<OrgEnvironment[]> {
      // Sibling endpoint served by the same cloud plugin
      // (services/apip-platform-api/internal/environments/handler.go);
      // reached via the same BFF proxy + org-id header as the portal calls.
      const body = await request<WireEnvironmentList>(base, orgHandle, 'GET', '/environments');
      return (body.list ?? [])
        .filter((e): e is WireEnvironment & { name: string } => typeof e.name === 'string' && e.name.length > 0)
        .map((e) => ({ name: e.name, displayName: e.displayName }));
    },
  };
}

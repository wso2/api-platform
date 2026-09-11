/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

// PortalPort backed by apip-platform-api via the console BFF's same-origin proxy.
// LIST strips metadata by design so loginEnvironment is empty on list rows and only populated by GET.

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

// Bounded so a stalled BFF request can't leave UI mutations pending indefinitely.
const REQUEST_TIMEOUT_MS = 30_000;

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

/** Same-origin request base for platform-api calls, or null when unconfigured (tests fall back to mock). */
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
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
  let response: Response;
  try {
    response = await fetch(`${base}${path}`, {
      method,
      credentials: 'same-origin',
      signal: controller.signal,
      headers: {
        Accept: 'application/json',
        [ORG_HEADER]: orgRef,
        ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
        ...(mutating ? { [CSRF_HEADER]: CSRF_HEADER_VALUE } : {}),
      },
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
    });
  } catch (err) {
    if ((err as { name?: string }).name === 'AbortError') {
      throw new Error(`Request timed out after ${REQUEST_TIMEOUT_MS / 1000}s`);
    }
    throw err;
  } finally {
    clearTimeout(timeout);
  }

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
      // Non-JSON body; keep the status-based message.
    }
    throw new Error(message);
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

// Wire shapes match the plugin's OAS.
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

// `name` is the value loginEnvironment expects; other response fields are ignored.
type WireEnvironment = { name?: string; displayName?: string };
type WireEnvironmentList = { count?: number; list?: WireEnvironment[] };

function fromWire(w: WirePortal): ManagedPortal {
  const identifier = w.id ?? w.handle;
  if (!identifier) {
    throw new Error('Portal response is missing both id and handle');
  }
  return {
    id: identifier,
    handle: w.handle ?? identifier,
    name: w.name,
    description: w.description ?? undefined,
    url: w.url,
    loginEnvironment: w.loginEnvironment,
    updatedAt: w.updatedAt,
  };
}

/** PortalPort backed by the BFF proxy at `base`, scoped to `orgHandle` via the X-Org-Id header. */
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
      // Sibling endpoint reached via the same BFF proxy and org-id header as the portal calls.
      const body = await request<WireEnvironmentList>(base, orgHandle, 'GET', '/environments');
      return (body.list ?? [])
        .filter((e): e is WireEnvironment & { name: string } => typeof e.name === 'string' && e.name.length > 0)
        .map((e) => ({ name: e.name, displayName: e.displayName }));
    },
  };
}

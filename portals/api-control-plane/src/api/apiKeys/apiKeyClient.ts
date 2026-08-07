import type { Api, ApiKeySummary, CreateApiKeyInput } from '../../types/domain';
import {
  PLATFORM_API_BASE,
  platformDelete,
  platformGet,
  platformPost,
  usePlatformApi,
} from '../platform/platformClient';
import { asRecord, delay, useMockApi } from '../shared/apiClientUtils';
import { ApiError } from '../types/errors';

/**
 * API keys for an API proxy. platform-api's model is key *injection*: the
 * caller supplies the plain-text key, the server stores a hash and pushes it
 * to every gateway the API is deployed on. Listing goes through the caller's
 * key inventory (`/me/api-keys`) filtered down to this API.
 */

const asOptional = (value: unknown): string | undefined => {
  if (typeof value === 'string' && value) return value;
  return undefined;
};

const toApiKeySummary = (value: unknown): ApiKeySummary => {
  const source = asRecord(value);
  return {
    name: String(source.name || ''),
    maskedApiKey: String(source.maskedApiKey || ''),
    status: asOptional(source.status),
    createdAt: asOptional(source.createdAt),
    expiresAt: asOptional(source.expiresAt),
  };
};

// Mock mode: in-memory keys per API handler.
const mockKeys = new Map<string, ApiKeySummary[]>();

const requirePlatform = () => {
  throw new ApiError('API keys require the platform API', 'UNKNOWN');
};

export async function listApiKeys(
  orgHandle: string,
  api: Api
): Promise<ApiKeySummary[]> {
  if (useMockApi()) {
    await delay();
    return mockKeys.get(api.handler) || [];
  }
  if (usePlatformApi()) {
    const data = await platformGet<{ items?: unknown[] }>(
      `${PLATFORM_API_BASE}/me/api-keys`,
      orgHandle
    );
    return (data.items || [])
      .filter((item) => asRecord(item).artifactId === api.handler)
      .map(toApiKeySummary);
  }
  return [];
}

export async function createApiKey(
  orgHandle: string,
  api: Api,
  input: CreateApiKeyInput
): Promise<void> {
  if (useMockApi()) {
    await delay();
    const name = input.displayName.trim().toLowerCase().replace(/\s+/g, '-');
    const masked = `${input.apiKey.slice(0, 4)}****${input.apiKey.slice(-2)}`;
    mockKeys.set(api.handler, [
      ...(mockKeys.get(api.handler) || []),
      {
        name,
        maskedApiKey: masked,
        status: 'ACTIVE',
        createdAt: new Date().toISOString(),
        expiresAt: input.expiresAt,
      },
    ]);
    return;
  }
  if (usePlatformApi()) {
    await platformPost<unknown>(
      `${PLATFORM_API_BASE}/rest-apis/${encodeURIComponent(api.handler)}/api-keys`,
      orgHandle,
      {
        displayName: input.displayName,
        apiKey: input.apiKey,
        ...(input.expiresAt ? { expiresAt: input.expiresAt } : {}),
      }
    );
    return;
  }
  return requirePlatform();
}

export async function revokeApiKey(
  orgHandle: string,
  api: Api,
  keyName: string
): Promise<void> {
  if (useMockApi()) {
    await delay();
    mockKeys.set(
      api.handler,
      (mockKeys.get(api.handler) || []).filter((key) => key.name !== keyName)
    );
    return;
  }
  if (usePlatformApi()) {
    await platformDelete<void>(
      `${PLATFORM_API_BASE}/rest-apis/${encodeURIComponent(
        api.handler
      )}/api-keys/${encodeURIComponent(keyName)}`,
      orgHandle
    );
    return;
  }
  return requirePlatform();
}

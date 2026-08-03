import type {
  CreateGatewayInput,
  Gateway,
  GatewayToken,
} from '../../types/domain';
import { toGateway } from '../adapters';
import { gateways } from '../mocks/data';
import {
  PLATFORM_API_BASE,
  platformGet,
  platformPost,
  usePlatformApi,
} from '../platform/platformClient';
import { delay, useMockApi } from '../shared/apiClientUtils';
import { ApiError } from '../types/errors';

export async function listGateways(orgHandle: string): Promise<Gateway[]> {
  if (useMockApi()) {
    await delay();
    return gateways.map(toGateway);
  }
  if (usePlatformApi()) {
    const data = await platformGet<{ list?: unknown[] }>(
      `${PLATFORM_API_BASE}/gateways`,
      orgHandle
    );
    return (data.list || []).map(toGateway);
  }
  return [];
}

export async function getGateway(
  orgHandle: string,
  gatewayId: string
): Promise<Gateway | undefined> {
  if (useMockApi()) {
    await delay();
    const gateway = gateways.find((item) => item.id === gatewayId);
    return gateway ? toGateway(gateway) : undefined;
  }
  if (usePlatformApi()) {
    try {
      return toGateway(
        await platformGet<unknown>(
          `${PLATFORM_API_BASE}/gateways/${encodeURIComponent(gatewayId)}`,
          orgHandle
        )
      );
    } catch (error) {
      if (error instanceof ApiError && error.code === 'NOT_FOUND')
        return undefined;
      throw error;
    }
  }
  return undefined;
}

export async function createGateway(
  orgHandle: string,
  input: CreateGatewayInput
): Promise<Gateway> {
  if (useMockApi()) {
    await delay();
    const gateway: Gateway = {
      ...input,
      id: `gw-${Date.now()}`,
      mode: 'self-hosted',
      isActive: false,
      version: '1.0',
    };
    gateways.push(gateway);
    return toGateway(gateway);
  }
  if (usePlatformApi()) {
    // Map the console input to platform-api's CreateGatewayRequest: `name` is
    // the slug handle (`id`), and the single vhost the UI collects is sent as
    // the required `endpoints` array. Tag self-hosted via free-form properties.
    const { name, vhost, ...rest } = input;
    return toGateway(
      await platformPost<unknown>(`${PLATFORM_API_BASE}/gateways`, orgHandle, {
        ...rest,
        id: name,
        endpoints: [vhost],
        properties: { gatewayMode: 'self-hosted' },
      })
    );
  }
  throw new ApiError('Gateway creation requires the platform API', 'UNKNOWN');
}

/**
 * Generates a self-hosted gateway registration token. Mirrors platform-api
 * POST /gateways/{id}/tokens → { id, token, createdAt, message }. The token is
 * shown once; the gateway agent uses it to connect over WebSocket.
 */
export async function createGatewayToken(
  orgHandle: string,
  gatewayId: string
): Promise<GatewayToken> {
  if (useMockApi()) {
    await delay();
    return {
      id: `tok-${Date.now()}`,
      token: 'mock-gateway-token-' + Math.random().toString(36).slice(2, 18),
      createdAt: new Date().toISOString(),
      message: 'Mock token. The gateway agent uses this to connect.',
    };
  }
  if (usePlatformApi()) {
    const data = await platformPost<{
      id?: string;
      token?: string;
      createdAt?: string;
      message?: string;
    }>(
      `${PLATFORM_API_BASE}/gateways/${encodeURIComponent(gatewayId)}/tokens`,
      orgHandle,
      {}
    );
    return {
      id: data.id || '',
      token: data.token || '',
      createdAt: data.createdAt,
      message: data.message,
    };
  }
  throw new ApiError('Token generation requires the platform API', 'UNKNOWN');
}

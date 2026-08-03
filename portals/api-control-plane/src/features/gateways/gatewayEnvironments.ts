import type { Gateway } from '../../types/domain';

/**
 * Client-side mock environments. Until the dedicated environment service exists,
 * gateways are grouped under these for the listing UX. Replace
 * `environmentForGateway` with the real gateway↔environment association later.
 */
export type GatewayEnvironment = {
  id: string;
  name: string;
};

export const MOCK_ENVIRONMENTS: GatewayEnvironment[] = [
  { id: 'development', name: 'Development' },
  { id: 'production', name: 'Production' },
];

/**
 * Deterministically buckets a gateway into a mock environment by a stable hash
 * of its id (demo placeholder — no backend environment data yet).
 */
export const environmentForGateway = (gateway: Gateway): GatewayEnvironment => {
  const hash = [...gateway.id].reduce((sum, ch) => sum + ch.charCodeAt(0), 0);
  return MOCK_ENVIRONMENTS[hash % MOCK_ENVIRONMENTS.length];
};

export type GatewayEnvironmentGroup = {
  environment: GatewayEnvironment;
  gateways: Gateway[];
};

/** Groups gateways under their (mock) environment, preserving env order. */
export const groupGatewaysByEnvironment = (
  gateways: Gateway[]
): GatewayEnvironmentGroup[] => {
  const byEnv = new Map<string, Gateway[]>();
  for (const gateway of gateways) {
    const env = environmentForGateway(gateway);
    const list = byEnv.get(env.id) || [];
    list.push(gateway);
    byEnv.set(env.id, list);
  }
  return MOCK_ENVIRONMENTS.filter((env) => byEnv.has(env.id)).map((env) => ({
    environment: env,
    gateways: byEnv.get(env.id) || [],
  }));
};

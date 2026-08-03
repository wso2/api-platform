import type { Gateway, GatewayDeployment } from '../../types/domain';

/** Newest-first by createdAt (platform stamps it on every deployment). */
export const byNewestFirst = (a: GatewayDeployment, b: GatewayDeployment) => {
  const timeA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
  const timeB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
  return timeB - timeA;
};

export const deploymentsForGateway = (
  deployments: GatewayDeployment[],
  gatewayId: string
): GatewayDeployment[] =>
  deployments
    .filter((item) => item.gatewayId === gatewayId)
    .sort(byNewestFirst);

/**
 * The deployment that represents the gateway's current state: the active
 * (DEPLOYED) one if any, otherwise the newest — mirrors ai-workspace.
 */
export const currentDeploymentFor = (
  deployments: GatewayDeployment[],
  gatewayId: string
): GatewayDeployment | undefined => {
  const list = deploymentsForGateway(deployments, gatewayId);
  return list.find((item) => item.status === 'DEPLOYED') ?? list[0];
};

const normalizeGatewayName = (name: string): string =>
  name.trim().replace(/\s+/g, '_') || 'gateway';

const deploymentNumber = (name: string | undefined): number | null => {
  if (!name) return null;
  const match = name.match(/_(\d+)$/);
  return match ? parseInt(match[1], 10) : null;
};

/**
 * Auto-generated deployment name, ai-workspace convention:
 * `{gateway-name}_{YYYY-MM-DD}_{n}` where n increments per gateway per day.
 */
export const nextDeploymentName = (
  gateway: Gateway,
  deployments: GatewayDeployment[]
): string => {
  const prefix = normalizeGatewayName(gateway.name);
  const dateStr = new Date().toISOString().slice(0, 10);
  const todays = deploymentsForGateway(deployments, gateway.id).filter(
    (item) => item.name.includes(`_${dateStr}_`) && /_\d+$/.test(item.name)
  );
  const maxNumber = todays.reduce((max, item) => {
    const num = deploymentNumber(item.name);
    return num !== null && num > max ? num : max;
  }, 0);
  return `${prefix}_${dateStr}_${maxNumber + 1}`;
};

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

import type { Gateway } from '@/api/resources/gateways';
import type { Deployment } from '@/api/resources/restApis/deployments';

/** Newest-first by createdAt (platform stamps it on every deployment). */
export const byNewestFirst = (a: Deployment, b: Deployment) => {
  const timeA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
  const timeB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
  return timeB - timeA;
};

export const deploymentsForGateway = (deployments: Deployment[], gatewayId: string): Deployment[] =>
  deployments.filter((item) => item.gatewayId === gatewayId).sort(byNewestFirst);

/**
 * The deployment that represents the gateway's current state: the active
 * (DEPLOYED) one if any, otherwise the newest — mirrors ai-workspace.
 */
export const currentDeploymentFor = (
  deployments: Deployment[],
  gatewayId: string,
): Deployment | undefined => {
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
export const nextDeploymentName = (gateway: Gateway, deployments: Deployment[]): string => {
  const prefix = normalizeGatewayName(gateway.displayName);
  const dateStr = new Date().toISOString().slice(0, 10);
  const todays = deploymentsForGateway(deployments, gateway.id ?? '').filter(
    (item) => item.name.includes(`_${dateStr}_`) && /_\d+$/.test(item.name),
  );
  const maxNumber = todays.reduce((max, item) => {
    const num = deploymentNumber(item.name);
    return num !== null && num > max ? num : max;
  }, 0);
  return `${prefix}_${dateStr}_${maxNumber + 1}`;
};

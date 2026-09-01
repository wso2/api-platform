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

import { isoMinutesAgo } from '../utils/time';
import type { Gateway, Environment, GatewayInput } from '../types';

/**
 * In-memory stand-in for the gateways backend. Mutated in place for the
 * lifetime of the tab, so create/update/delete are reflected immediately
 * without a real API — swap for real client calls once this plugin talks to
 * the platform API directly.
 */
let gateways: Gateway[] = [
  {
    id: 'gateway-test',
    name: 'test',
    type: 'ai',
    environmentId: 'development',
    url: 'https://localhost:8443',
    status: 'inactive',
    updatedAt: isoMinutesAgo(600),
  },
  {
    id: 'gateway-eu-prod',
    name: 'eu-production',
    description: 'Primary EU AI gateway',
    type: 'ai',
    environmentId: 'production',
    url: 'https://eu.gateway.example.com',
    status: 'active',
    updatedAt: isoMinutesAgo(60),
  },
  {
    id: 'gateway-events',
    name: 'order-events',
    description: 'Event gateway for order pipeline notifications',
    type: 'event',
    environmentId: 'stage',
    url: 'https://events.gateway.example.com',
    status: 'active',
    updatedAt: isoMinutesAgo(1440),
  },
];

const environments: Environment[] = [
  { id: 'development', name: 'Development' },
  { id: 'stage', name: 'Stage' },
  { id: 'production', name: 'Production' },
];

export function listEnvironments(): Environment[] {
  return environments;
}

export function listGateways(): Gateway[] {
  return gateways;
}

export function getGateway(id: string): Gateway | undefined {
  return gateways.find((gateway) => gateway.id === id);
}

export function createGateway(input: GatewayInput): Gateway {
  const created: Gateway = {
    id: `gateway-${Date.now()}`,
    ...input,
    status: 'inactive',
    updatedAt: new Date().toISOString(),
  };
  gateways = [...gateways, created];
  return created;
}

export function updateGateway(id: string, input: GatewayInput): Gateway {
  const updated: Gateway = {
    ...(getGateway(id) as Gateway),
    ...input,
    id,
    updatedAt: new Date().toISOString(),
  };
  gateways = gateways.map((gateway) => (gateway.id === id ? updated : gateway));
  return updated;
}

export function deleteGateway(id: string): void {
  gateways = gateways.filter((gateway) => gateway.id !== id);
}

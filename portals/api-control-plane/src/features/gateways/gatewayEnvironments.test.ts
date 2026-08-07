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

import { describe, expect, it } from 'vitest';

import type { Gateway } from '../../types/domain';
import {
  environmentForGateway,
  groupGatewaysByEnvironment,
  MOCK_ENVIRONMENTS,
} from './gatewayEnvironments';

const gw = (id: string): Gateway => ({
  id,
  name: id,
  displayName: id,
  vhost: `${id}.example.com`,
  functionalityType: 'regular',
  mode: 'self-hosted',
});

describe('gateway environments', () => {
  it('assigns a gateway to one of the mock environments deterministically', () => {
    const env1 = environmentForGateway(gw('alpha'));
    const env2 = environmentForGateway(gw('alpha'));
    expect(env1).toEqual(env2);
    expect(MOCK_ENVIRONMENTS).toContainEqual(env1);
  });

  it('groups gateways under their environment, omitting empty environments', () => {
    const groups = groupGatewaysByEnvironment([gw('a'), gw('b'), gw('c')]);
    const total = groups.reduce((n, g) => n + g.gateways.length, 0);
    expect(total).toBe(3);
    groups.forEach((g) => expect(g.gateways.length).toBeGreaterThan(0));
  });

  it('returns nothing for an empty gateway list', () => {
    expect(groupGatewaysByEnvironment([])).toEqual([]);
  });
});

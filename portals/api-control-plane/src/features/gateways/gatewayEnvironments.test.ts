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

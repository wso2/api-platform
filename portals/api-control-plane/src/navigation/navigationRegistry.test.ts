import { describe, expect, it } from 'vitest';

import { navigationRegistry } from './navigationRegistry';

const matcherFor = (id: string) => {
  const item = navigationRegistry.find((entry) => entry.id === id);
  if (!item?.match) throw new Error(`No match predicate for ${id}`);
  return item.match;
};

const ORG = '/organizations/acme';
const PROJECT = `${ORG}/projects/orders`;
const API = `${PROJECT}/apis/api-1`;

describe('navigation match predicates', () => {
  it('matches    home only at the org home path', () => {
    const match = matcherFor('organization-home');
    expect(match(`${ORG}/home`)).toBe(true);
    expect(match(`${PROJECT}/home`)).toBe(false);
  });

  it('requires the org prefix for project home', () => {
    const match = matcherFor('project-home');
    expect(match(`${PROJECT}/home`)).toBe(true);
    // A bare /projects/x/home without the org segment must not match.
    expect(match('/projects/orders/home')).toBe(false);
  });

  it('anchors settings to the end of the path', () => {
    const match = matcherFor('settings');
    expect(match(`${PROJECT}/settings`)).toBe(true);
    expect(match(`${PROJECT}/settings/advanced`)).toBe(false);
    expect(match(`${ORG}/projects/settings/home`)).toBe(false);
  });

  it('matches runtime logs only at the runtimelogs path', () => {
    const match = matcherFor('runtime-logs');
    expect(match(`${PROJECT}/observe/runtimelogs`)).toBe(true);
    expect(match(`${PROJECT}/observe/runtimelogs/x`)).toBe(false);
  });

  it.each(['deploy', 'test', 'manage'])(
    'anchors the %s api tab',
    (id) => {
      const match = matcherFor(id);
      expect(match(`${API}/${id}`)).toBe(true);
      expect(match(`${API}/${id}/extra`)).toBe(false);
    }
  );

  it('matches api overview but not its sub-tabs', () => {
    const match = matcherFor('api-overview');
    expect(match(API)).toBe(true);
    expect(match(`${API}/deploy`)).toBe(false);
  });
});

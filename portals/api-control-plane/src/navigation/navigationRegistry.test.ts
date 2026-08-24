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

  it('stays active on a settings tab but not two levels deep', () => {
    const match = matcherFor('settings');
    expect(match(`${PROJECT}/settings`)).toBe(true);
    expect(match(`${PROJECT}/settings/general`)).toBe(true);
    expect(match(`${PROJECT}/settings/general/extra`)).toBe(false);
    expect(match(`${ORG}/projects/settings/home`)).toBe(false);
  });

  it('matches org-level settings without colliding with project settings', () => {
    const match = matcherFor('org-settings');
    expect(match(`${ORG}/settings`)).toBe(true);
    expect(match(`${ORG}/settings/general`)).toBe(true);
    // A project's own /settings must not also light up org-settings.
    expect(match(`${PROJECT}/settings`)).toBe(false);
  });

  it('hides org-settings once a project is selected', () => {
    const item = navigationRegistry.find((entry) => entry.id === 'org-settings');
    if (!item?.isVisible) throw new Error('org-settings has no isVisible predicate');
    expect(item.isVisible({ isProjectScope: false } as never)).toBe(true);
    expect(item.isVisible({ isProjectScope: true } as never)).toBe(false);
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

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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import {
  decideAnyOperation,
  decideOperation,
  decideScope,
  reportForbiddenDrift,
  resetPermissionWarnings,
  toGrantedSet,
  type PermissionInput,
} from './evaluate';

const ready = (
  scopes: string[] | undefined,
  mode: 'enforce' | 'permissive' = 'enforce',
): PermissionInput => ({ status: 'ready', granted: toGrantedSet(scopes), mode });

describe('evaluate', () => {
  // The unknown-operation warning is memoized per operation id, so a suite that
  // asserts on it more than once would see it only the first time.
  beforeEach(resetPermissionWarnings);

  it('grants on the narrow scope', () => {
    const d = decideOperation('DeployAPI', ready(['ap:api:rest:deployment:manage']));
    expect([d.allowed, d.reason]).toEqual([true, 'granted']);
    // The node's own action and :manage, plus :manage on every ancestor, as the
    // spec lists them. A parent's create does not flow down.
    expect(d.required).toEqual([
      'ap:api:manage',
      'ap:api:rest:deployment:create',
      'ap:api:rest:deployment:manage',
      'ap:api:rest:manage',
      'ap:manage',
    ]);
  });

  it('grants on a parent or root read/manage, because the spec lists them', () => {
    expect(decideOperation('DeployAPI', ready(['ap:api:rest:manage'])).allowed).toBe(true);
    expect(decideOperation('DeployAPI', ready(['ap:api:manage'])).allowed).toBe(true);
    expect(decideOperation('GetBuilds', ready(['ap:api:rest:read'])).allowed).toBe(true);
    expect(decideOperation('GetBuilds', ready(['ap:api:read'])).allowed).toBe(true);
    expect(decideOperation('DeployAPI', ready(['ap:manage'])).allowed).toBe(true);
    expect(decideOperation('GetBuilds', ready(['ap:read'])).allowed).toBe(true);
    expect(decideOperation('DeployAPI', ready(['ap:read'])).allowed).toBe(false);
    // …but a parent's own create/update does not flow down.
    expect(decideOperation('DeployAPI', ready(['ap:api:rest:create'])).allowed).toBe(false);
    expect(decideOperation('DeployAPI', ready(['ap:api:rest:update'])).allowed).toBe(false);
    expect(decideOperation('DeployAPI', ready(['ap:api:read'])).allowed).toBe(false);
  });

  it('grants on the broader manage scope without any client-side implication', () => {
    expect(decideOperation('CreateProject', ready(['ap:project:manage'])).allowed).toBe(true);
    // …and manage does NOT leak across resources
    expect(decideOperation('CreateProject', ready(['ap:gateway:manage'])).allowed).toBe(false);
  });

  it('denies with missing-scope when scopes are known but insufficient', () => {
    const d = decideOperation('DeleteProject', ready(['ap:project:read']));
    expect([d.allowed, d.reason]).toEqual([false, 'missing-scope']);
  });

  it('distinguishes an empty scope set from an absent claim', () => {
    expect(decideOperation('ListProjects', ready([])).reason).toBe('missing-scope');
    expect(decideOperation('ListProjects', ready(undefined)).reason).toBe('unknown-scopes');
  });

  it('defers to mode when scopes are unknown', () => {
    expect(decideOperation('ListProjects', ready(undefined, 'enforce')).allowed).toBe(false);
    expect(decideOperation('ListProjects', ready(undefined, 'permissive')).allowed).toBe(true);
  });

  it('never denies while loading', () => {
    const d = decideOperation('DeleteProject', { status: 'loading', mode: 'enforce' });
    expect([d.allowed, d.reason]).toEqual([true, 'loading']);
  });

  it('allows an unknown operation rather than hiding a control silently', () => {
    const d = decideOperation('CreateProjects', ready([]));
    expect([d.allowed, d.reason]).toEqual([true, 'unknown-operation']);
  });

  it('drops blank scope entries', () => {
    expect(toGrantedSet(['', '  '])?.size).toBe(0);
    expect(toGrantedSet(null)).toBeUndefined();
  });

  it('translates pre-tree scope names the way the Platform API does', () => {
    // The old name stays (the API accepts it too) and its tree permission joins it.
    expect([...(toGrantedSet(['ap:rest_api:deployment:create']) ?? [])]).toEqual([
      'ap:rest_api:deployment:create',
      'ap:api:rest:deployment:create',
    ]);
    expect(decideOperation('DeployAPI', ready(['ap:rest_api:deployment:create'])).allowed).toBe(
      true,
    );
    // Parent privilege through a legacy name: ap:rest_api:manage -> ap:api:rest:manage.
    expect(decideOperation('DeployAPI', ready(['ap:rest_api:manage'])).allowed).toBe(true);
    expect(decideOperation('GetBuilds', ready(['ap:rest_api:read'])).allowed).toBe(true);
    expect(decideOperation('DeployAPI', ready(['ap:rest_api:read'])).allowed).toBe(false);
    // A name the mapping does not know grants nothing.
    expect(decideOperation('DeployAPI', ready(['ap:rest_api:deploy'])).allowed).toBe(false);
    expect(toGrantedSet(['ap:rest_api:deploy'])?.size).toBe(1);
  });

  it('any-of across operations, unioning required scopes when all are denied', () => {
    const input = ready(['ap:api:rest:read']);
    expect(decideAnyOperation(['DeployAPI', 'UndeployDeployment'], input).allowed).toBe(false);
    // Two API kinds share only the ap:api:manage ancestor, so the union grows.
    const d = decideAnyOperation(['DeployAPI', 'DeployMCPProxy'], input);
    expect(d.required.length).toBeGreaterThan(decideOperation('DeployAPI', input).required.length);
    expect(decideAnyOperation(['DeployAPI', 'ListRESTAPIs'], input).allowed).toBe(true);
    expect(decideAnyOperation([], input).allowed).toBe(true);
  });

  it('checks a raw override scope, by its tree name or its pre-tree name', () => {
    expect(
      decideScope('ap:credential:api_key:all:manage', ready(['ap:api_key:all:manage'])).allowed,
    ).toBe(true);
    expect(
      decideScope('ap:credential:api_key:all:manage', ready(['ap:credential:api_key:all:manage']))
        .allowed,
    ).toBe(true);
    expect(
      decideScope('ap:credential:api_key:all:manage', ready(['ap:credential:api_key:read']))
        .allowed,
    ).toBe(false);
  });
});

describe('reportForbiddenDrift', () => {
  beforeEach(resetPermissionWarnings);

  it('warns when the server refused something the console had allowed', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    reportForbiddenDrift(
      'DeleteProject',
      decideOperation('DeleteProject', ready(['ap:project:manage'])),
    );

    expect(warn).toHaveBeenCalledTimes(1);
    expect(warn.mock.calls[0][0]).toMatch(/predicted was allowed/);
    expect(warn.mock.calls[0][0]).toMatch(/api:codegen/);
    warn.mockRestore();
  });

  it('stays silent when the denial is what the console already predicted', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    reportForbiddenDrift(
      'DeleteProject',
      decideOperation('DeleteProject', ready(['ap:project:read'])),
    );

    // Not drift: the console hid or disabled the control and the server agreed.
    expect(warn).not.toHaveBeenCalled();
    warn.mockRestore();
  });

  it('warns once per operation, so a remount loop does not bury the finding', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const allowed = decideOperation('DeleteProject', ready(['ap:project:manage']));

    reportForbiddenDrift('DeleteProject', allowed);
    reportForbiddenDrift('DeleteProject', allowed);
    reportForbiddenDrift('DeleteProject', allowed);

    expect(warn).toHaveBeenCalledTimes(1);
    warn.mockRestore();
  });
});

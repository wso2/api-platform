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
    const d = decideOperation('CreateProject', ready(['ap:project:create']));
    expect([d.allowed, d.reason]).toEqual([true, 'granted']);
    expect(d.required).toEqual(['ap:project:create', 'ap:project:manage']);
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

  it('any-of across operations, unioning required scopes when all are denied', () => {
    const input = ready(['ap:rest_api:read']);
    expect(decideAnyOperation(['DeployAPI', 'UndeployDeployment'], input).allowed).toBe(false);
    const d = decideAnyOperation(['DeployAPI', 'UndeployDeployment'], input);
    expect(d.required.length).toBeGreaterThan(decideOperation('DeployAPI', input).required.length);
    expect(decideAnyOperation(['DeployAPI', 'ListRESTAPIs'], input).allowed).toBe(true);
    expect(decideAnyOperation([], input).allowed).toBe(true);
  });

  it('checks a raw override scope', () => {
    expect(decideScope('ap:api_key:all:manage', ready(['ap:api_key:all:manage'])).allowed).toBe(
      true,
    );
    expect(decideScope('ap:api_key:all:manage', ready(['ap:api_key:read'])).allowed).toBe(false);
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

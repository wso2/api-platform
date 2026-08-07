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

import {
  toApi,
  toApiProxy,
  toDeployment,
  toEnvironment,
  toGateway,
  toOrganization,
  toProject,
} from './adapters';

describe('DTO adapters', () => {
  it('normalizes organization fallbacks', () => {
    expect(toOrganization({ id: 42, uuid: 'org-1', name: 'Acme' })).toEqual({
      id: '42',
      numericId: 42,
      uuid: 'org-1',
      name: 'Acme',
      handle: '42',
      description: undefined,
    });
  });

  it('normalizes wrapped validate-user organization responses', () => {
    expect(
      toOrganization({
        organization: {
          handle: 'acme',
          id: '84',
          name: 'Acme wrapped',
          uuid: 'org-84',
        },
      })
    ).toMatchObject({
      handle: 'acme',
      id: '84',
      numericId: 84,
      name: 'Acme wrapped',
      uuid: 'org-84',
    });
  });

  it('maps legacy proxy component types to API_PROXY', () => {
    expect(toApi({ id: 'c1', name: 'orders', displayType: 'proxy' }).kind).toBe(
      'API_PROXY'
    );
  });

  it('keeps unsupported component status UI-safe', () => {
    expect(toApi({ id: 'c1', name: 'orders', status: 'UNKNOWN' }).status).toBe(
      'DRAFT'
    );
  });

  it('carries organization status through', () => {
    expect(
      toOrganization({ id: '1', handle: 'acme', status: 'ACTIVE' }).status
    ).toBe('ACTIVE');
  });

  it('normalizes project detail fields', () => {
    expect(
      toProject({
        id: 'p1',
        orgId: '1',
        name: 'Retail',
        handler: 'retail',
        region: 'us-east-1',
        version: '1.2.0',
        createdDate: '2026-05-01T00:00:00Z',
        type: 'MULTI_REPO',
        gitProvider: 'github',
        repository: 'acme/retail',
      })
    ).toMatchObject({
      id: 'p1',
      handler: 'retail',
      version: '1.2.0',
      createdDate: '2026-05-01T00:00:00Z',
      type: 'MULTI_REPO',
      gitProvider: 'github',
      repository: 'acme/retail',
    });
  });

  it('drops unknown project repo types', () => {
    expect(
      toProject({ id: 'p1', name: 'x', type: 'WEIRD' }).type
    ).toBeUndefined();
  });
});

describe('toApi edge cases', () => {
  it('falls back name→id and displayName/handler→name', () => {
    expect(toApi({ id: 'svc-1' })).toMatchObject({
      id: 'svc-1',
      name: 'svc-1',
      displayName: 'svc-1',
      handler: 'svc-1',
    });
    expect(toApi({ id: 'c1', name: 'orders' })).toMatchObject({
      displayName: 'orders',
      handler: 'orders',
    });
  });

  it('passes through httpBased only when boolean', () => {
    expect(toApi({ id: 'c1', httpBased: false }).httpBased).toBe(false);
    expect(toApi({ id: 'c1' }).httpBased).toBeUndefined();
    expect(toApi({ id: 'c1', httpBased: 'yes' }).httpBased).toBeUndefined();
  });

  it('maps legacy build-plane kinds', () => {
    expect(toApi({ id: 'c1', displayType: 'byocWebApp' }).kind).toBe('WEB_APP');
    expect(toApi({ id: 'c1', displayType: 'ballerinaService' }).kind).toBe(
      'SERVICE'
    );
    expect(toApi({ id: 'c1', kind: 'WEB_APP' }).kind).toBe('WEB_APP');
  });

  it('normalizes legacy status aliases', () => {
    expect(toApi({ id: 'c1', status: 'RUNNING' }).status).toBe('ACTIVE');
    expect(toApi({ id: 'c1', initStatus: 'IN_PROGRESS' }).status).toBe(
      'PENDING'
    );
    expect(toApi({ id: 'c1', status: 'ERROR' }).status).toBe('FAILED');
  });
});

describe('toProject fallbacks', () => {
  it('resolves handler and org id from alternate fields', () => {
    expect(
      toProject({ id: 'p1', organizationId: 'o9', projectHandler: 'retail' })
    ).toMatchObject({ orgId: 'o9', handler: 'retail' });
  });

  it('accepts createdAt/updatedDate aliases', () => {
    expect(
      toProject({
        id: 'p1',
        createdAt: '2026-01-01',
        updatedDate: '2026-02-02',
      })
    ).toMatchObject({ createdDate: '2026-01-01', updatedAt: '2026-02-02' });
  });
});

describe('toEnvironment / toDeployment', () => {
  it('maps environment type with a DEVELOPMENT default', () => {
    expect(
      toEnvironment({ id: 'e1', name: 'Prod', type: 'PRODUCTION' }).type
    ).toBe('PRODUCTION');
    expect(toEnvironment({ id: 'e1', type: 'STAGING' }).type).toBe(
      'DEVELOPMENT'
    );
  });

  it('keeps known deployment statuses and defaults the rest', () => {
    expect(toDeployment({ id: 'd1', status: 'READY' }).status).toBe('READY');
    expect(toDeployment({ id: 'd1', status: 'WAT' }).status).toBe(
      'NOT_DEPLOYED'
    );
  });
});

describe('toGateway', () => {
  it('maps functionality type with a regular default', () => {
    expect(
      toGateway({ name: 'g', functionalityType: 'ai' }).functionalityType
    ).toBe('ai');
    expect(
      toGateway({ name: 'g', functionalityType: 'event' }).functionalityType
    ).toBe('event');
    expect(
      toGateway({ name: 'g', functionalityType: 'weird' }).functionalityType
    ).toBe('regular');
  });

  it('derives mode from source.mode or properties.gatewayMode, default managed', () => {
    expect(toGateway({ name: 'g', mode: 'self-hosted' }).mode).toBe(
      'self-hosted'
    );
    expect(
      toGateway({ name: 'g', properties: { gatewayMode: 'self-hosted' } }).mode
    ).toBe('self-hosted');
    expect(toGateway({ name: 'g' }).mode).toBe('managed');
  });

  it('defaults id to name and passes booleans through', () => {
    expect(
      toGateway({ name: 'g', isActive: true, isCritical: false })
    ).toMatchObject({ id: 'g', isActive: true, isCritical: false });
  });

  it('maps the gateway vhost', () => {
    expect(toGateway({ id: 'g', vhost: 'mg.example' }).vhost).toBe(
      'mg.example'
    );
    expect(toGateway({ id: 'g' }).vhost).toBe('');
  });

  it('prefers displayName as the name', () => {
    expect(toGateway({ id: 'g', displayName: 'Gateway One' }).name).toBe(
      'Gateway One'
    );
  });
});

describe('toApiProxy', () => {
  it('applies context/version defaults and PUBLIC visibility default', () => {
    expect(toApiProxy({ id: 'a1' })).toMatchObject({
      context: '/',
      version: '1.0.0',
      visibility: 'PUBLIC',
    });
    expect(toApiProxy({ id: 'a1', visibility: 'PRIVATE' }).visibility).toBe(
      'PRIVATE'
    );
  });
});

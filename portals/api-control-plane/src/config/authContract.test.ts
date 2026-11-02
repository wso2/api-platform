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
  DEFAULT_CHECK_SESSION_INTERVAL,
  DEFAULT_CLAIM_MAPPINGS,
  buildLoginUrl,
  parseCheckSessionInterval,
  parseClaimMappings,
  readAuthMode,
  resolveAuthContract,
  withAppBasePath,
} from './authContract';

const BFF_ENDPOINTS = {
  loginUrl: '/api/auth/login',
  credentialsUrl: '/api/login',
  returnParam: 'return',
  sessionUrl: '/api/session',
  logoutUrl: '/api/logout',
  // The BFF has no read-only session endpoint: no polling.
  checkSessionUrl: '',
  checkSessionInterval: DEFAULT_CHECK_SESSION_INTERVAL,
};

describe('readAuthMode', () => {
  it.each([
    ['basic', 'basic'],
    ['oidc', 'oidc'],
    ['external', 'external'],
    [undefined, 'basic'],
    ['', 'basic'],
    ['OIDC', 'basic'],
    ['something-else', 'basic'],
  ])('%s → %s', (value, expected) => {
    expect(readAuthMode(value)).toBe(expected);
  });
});

describe('resolveAuthContract', () => {
  // The BFF modes must resolve to exactly the BFF's routes and behaviour, so
  // every existing deployment keeps working without new configuration.
  it('basic mode is the BFF with form sign-in', () => {
    expect(resolveAuthContract('basic')).toEqual({
      ...BFF_ENDPOINTS,
      loginStyle: 'form',
      sessionFormat: 'bff',
      logoutStyle: 'post-json',
      claimMappings: DEFAULT_CLAIM_MAPPINGS,
    });
  });

  it('oidc mode is the BFF with redirect sign-in', () => {
    expect(resolveAuthContract('oidc')).toEqual({
      ...BFF_ENDPOINTS,
      loginStyle: 'redirect',
      sessionFormat: 'bff',
      logoutStyle: 'post-json',
      claimMappings: DEFAULT_CLAIM_MAPPINGS,
    });
  });

  it('external mode redirects, reads claims and logs out by navigation', () => {
    const contract = resolveAuthContract('external');

    expect(contract.loginStyle).toBe('redirect');
    expect(contract.sessionFormat).toBe('claims');
    expect(contract.logoutStyle).toBe('navigate');
  });

  it('takes every endpoint from the overrides', () => {
    const contract = resolveAuthContract('external', {
      loginUrl: '/api-platform/auth/login',
      credentialsUrl: '/creds',
      returnParam: 'returnTo',
      sessionUrl: '/api-platform/auth/session',
      checkSessionUrl: ' /api-platform/auth/checksession ',
      checkSessionInterval: '60',
      logoutUrl: '/api-platform/auth/logout',
    });

    expect(contract).toMatchObject({
      loginUrl: '/api-platform/auth/login',
      credentialsUrl: '/creds',
      returnParam: 'returnTo',
      sessionUrl: '/api-platform/auth/session',
      checkSessionUrl: '/api-platform/auth/checksession',
      checkSessionInterval: 60,
      logoutUrl: '/api-platform/auth/logout',
    });
  });

  it.each([
    ['loginStyle', 'form'],
    ['loginStyle', 'redirect'],
    ['sessionFormat', 'bff'],
    ['sessionFormat', 'claims'],
    ['sessionFormat', 'sdk'],
    ['logoutStyle', 'post-json'],
    ['logoutStyle', 'navigate'],
  ] as const)('accepts %s %s from the overrides', (key, value) => {
    expect(resolveAuthContract('external', { [key]: value })[key]).toBe(value);
  });

  it('lets a single style override its preset', () => {
    const contract = resolveAuthContract('oidc', {
      logoutStyle: 'navigate',
      sessionFormat: 'claims',
    });

    expect(contract.loginStyle).toBe('redirect');
    expect(contract.logoutStyle).toBe('navigate');
    expect(contract.sessionFormat).toBe('claims');
  });

  it('ignores unknown style values and blank URLs', () => {
    const contract = resolveAuthContract('basic', {
      loginStyle: 'popup',
      sessionFormat: 'xml',
      logoutStyle: 'delete',
      loginUrl: '   ',
      sessionUrl: '',
    });

    expect(contract.loginStyle).toBe('form');
    expect(contract.sessionFormat).toBe('bff');
    expect(contract.logoutStyle).toBe('post-json');
    expect(contract.loginUrl).toBe('/api/auth/login');
    expect(contract.sessionUrl).toBe('/api/session');
  });
});

describe('parseCheckSessionInterval', () => {
  it.each([
    [undefined, 30],
    ['abc', 30],
    ['0', 30],
    [' 45 ', 45],
    [120, 120],
    ['1', 5],
  ] as const)('%s → %i seconds', (value, expected) => {
    expect(parseCheckSessionInterval(value)).toBe(expected);
  });
});

describe('parseClaimMappings', () => {
  it('overlays a JSON string on the defaults', () => {
    expect(parseClaimMappings('{"username":"name","organization":"ouId"}')).toEqual({
      ...DEFAULT_CLAIM_MAPPINGS,
      username: 'name',
      organization: 'ouId',
    });
  });

  it('accepts an already-parsed object', () => {
    expect(parseClaimMappings({ org_handle: 'ouHandle' }).org_handle).toBe('ouHandle');
  });

  it('ignores unknown keys, blank and non-string values', () => {
    expect(parseClaimMappings({ nickname: 'nick', email: '  ', roles: 7 })).toEqual(
      DEFAULT_CLAIM_MAPPINGS,
    );
  });

  it.each([undefined, '', '{broken', '["username"]', 'null'])(
    'falls back to the defaults for %j',
    (value) => {
      expect(parseClaimMappings(value)).toEqual(DEFAULT_CLAIM_MAPPINGS);
    },
  );
});

describe('withAppBasePath', () => {
  it.each([
    ['/orgs/acme', '', '/orgs/acme'],
    ['orgs/acme', '', '/orgs/acme'],
    ['/', '', '/'],
    ['/orgs/acme', '/api-platform', '/api-platform/orgs/acme'],
    ['/', '/api-platform', '/api-platform/'],
  ])('%s under %j → %s', (path, base, expected) => {
    expect(withAppBasePath(path, base)).toBe(expected);
  });
});

describe('buildLoginUrl', () => {
  it('adds the return parameter, encoded', () => {
    const contract = resolveAuthContract('oidc');

    expect(buildLoginUrl(contract, '/orgs/a b')).toBe('/api/auth/login?return=%2Forgs%2Fa%20b');
  });

  it('appends to a login URL that already has a query', () => {
    const contract = resolveAuthContract('external', {
      loginUrl: '/oauth2/start?provider=idp',
      returnParam: 'rd',
    });

    expect(buildLoginUrl(contract, '/x')).toBe('/oauth2/start?provider=idp&rd=%2Fx');
  });
});

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

import { normalizeKeyManager, sameKeyManagers, validateKeyManager } from './keyManagers';

const thunder = {
  issuer: 'http://thunder.local:8080',
  jwksUri: 'http://thunder-service.thunder:8090/oauth2/jwks',
  name: 'ThunderKeyManager',
};
const asgardeo = {
  issuer: 'https://api.asgardeo.io/t/acme/oauth2/token',
  jwksUri: 'https://api.asgardeo.io/t/acme/oauth2/jwks',
  name: 'asgardeo',
};

describe('validateKeyManager', () => {
  it('accepts a well-formed entry, and an http URL the gateway already trusts', () => {
    expect(validateKeyManager(asgardeo, [thunder], [thunder.jwksUri])).toEqual({});
    expect(validateKeyManager(thunder, [asgardeo], [thunder.jwksUri])).toEqual({});
  });

  it('refuses a new http URL, a bad name and a collision', () => {
    expect(
      validateKeyManager({ ...asgardeo, jwksUri: 'http://idp.example.com/jwks' }, [], [])
    ).toEqual({ jwksUri: 'Must be an https URL.' });
    expect(validateKeyManager({ ...asgardeo, name: 'Acme Prod' }, [], []).name).toMatch(/Letters/);
    expect(validateKeyManager(asgardeo, [asgardeo], [])).toEqual({
      issuer: 'Another key manager has this issuer.',
      name: 'Another key manager has this name.',
    });
  });

  it('needs the key source it uses', () => {
    expect(validateKeyManager({ ...asgardeo, jwksUri: ' ' }, [], []).jwksUri).toBe('Required.');
    const keyed = { issuer: 'https://idp', name: 'idp', publicKey: 'not a key' };
    expect(validateKeyManager(keyed, [], []).publicKey).toMatch(/PEM/);
  });
});

describe('sameKeyManagers and normalizeKeyManager', () => {
  it('compares entries in order', () => {
    expect(sameKeyManagers([thunder, asgardeo], [thunder, asgardeo])).toBe(true);
    expect(sameKeyManagers([thunder, asgardeo], [asgardeo, thunder])).toBe(false);
  });

  it('trims, and keeps only the key source in use', () => {
    expect(normalizeKeyManager({ ...asgardeo, name: ' asgardeo ', scopeClaim: ' ' })).toEqual(asgardeo);
  });
});

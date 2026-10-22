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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, it, expect } from 'vitest';
import { oidcLoginUrl } from './BFFAuthProvider';

// The login link is the one place the SPA decides what the IDP gets to see about the
// user's choice on the login page. These pin its shape: which names carry the provider
// and the account hint, and that nothing else rides along.
const parse = (href: string) => {
  const u = new URL(href, 'https://portal.example');
  return { path: u.pathname, q: u.searchParams };
};

describe('oidcLoginUrl', () => {
  it('points at the BFF login endpoint with only the return path by default', () => {
    const { path, q } = parse(oidcLoginUrl('/projects?tab=1'));
    expect(path.endsWith('/api/auth/login')).toBe(true);
    expect(q.get('return')).toBe('/projects?tab=1');
    expect([...q.keys()]).toEqual(['return']);
  });

  it('forwards a provider choice on its own, with no account hint', () => {
    const { q } = parse(oidcLoginUrl('/', { fidp: 'google' }));
    expect(q.get('fidp')).toBe('google');
    expect(q.has('login_hint')).toBe(false);
    expect(q.has('username')).toBe(false);
  });

  it('sends the account hint as both login_hint and username', () => {
    // login_hint is the OIDC name; WSO2 Identity Server / Asgardeo's EnterpriseIDP
    // connection reads the address from username. Both carry the same value.
    const { q } = parse(
      oidcLoginUrl('/', { fidp: 'EnterpriseIDP', loginHint: 'someone@example.com' })
    );
    expect(q.get('fidp')).toBe('EnterpriseIDP');
    expect(q.get('login_hint')).toBe('someone@example.com');
    expect(q.get('username')).toBe('someone@example.com');
    expect([...q.keys()].sort()).toEqual(['fidp', 'login_hint', 'return', 'username']);
  });
});

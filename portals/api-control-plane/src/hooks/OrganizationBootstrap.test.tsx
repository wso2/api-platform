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
import { render } from '@testing-library/react';

import { ApiError } from '../api/core/errors';
import { OrganizationBootstrap } from './OrganizationBootstrap';

const mutate = vi.fn();
const refetch = vi.fn();
let lookup: { isPending: boolean; data?: unknown; error?: unknown };

vi.mock('../contexts/auth/AuthProvider', () => ({
  useAuth: () => ({ isAuthenticated: true, user: { org: { handle: 'acme', name: 'Acme' } } }),
}));

vi.mock('../api/resources/organizations', () => ({
  useOrganization: () => ({ ...lookup, refetch }),
  useBootstrapOrganization: () => ({ mutate }),
}));

const httpError = (status: number, code: string) =>
  new ApiError('failed', { kind: 'http', status, code });

describe('OrganizationBootstrap', () => {
  beforeEach(() => {
    mutate.mockReset();
    refetch.mockReset();
  });

  it.each(['ORGANIZATION_NOT_FOUND', 'NOT_FOUND'])(
    'registers the organization when the lookup answers %s',
    (code) => {
      lookup = { isPending: false, error: httpError(404, code) };
      render(<OrganizationBootstrap />);

      expect(mutate).toHaveBeenCalledTimes(1);
      expect(mutate.mock.calls[0][0]).toMatchObject({ id: 'acme', displayName: 'Acme' });
    },
  );

  it.each([
    ['an expired session', 401, 'UNAUTHORIZED'],
    ['a denial', 403, 'FORBIDDEN'],
    ['a server failure', 500, 'INTERNAL_ERROR'],
  ])('does not register on %s', (_label, status, code) => {
    lookup = { isPending: false, error: httpError(status, code) };
    render(<OrganizationBootstrap />);

    expect(mutate).not.toHaveBeenCalled();
  });

  it('does not register an organization the platform already has', () => {
    lookup = { isPending: false, data: { id: 'acme' } };
    render(<OrganizationBootstrap />);

    expect(mutate).not.toHaveBeenCalled();
  });

  it.each(['ORGANIZATION_EXISTS', 'CONFLICT'])(
    're-reads the organization when registering answers %s',
    (code) => {
      lookup = { isPending: false, error: httpError(404, 'ORGANIZATION_NOT_FOUND') };
      render(<OrganizationBootstrap />);

      const { onError } = mutate.mock.calls[0][1];
      onError(httpError(409, code));

      expect(refetch).toHaveBeenCalledTimes(1);
    },
  );
});

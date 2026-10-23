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

import React from 'react';
import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';

import {
  AppAuthContext,
  useAppAuth,
  useAppAuthScopes,
  type AppAuthContextType,
} from './AppAuthContext';
import {
  ResourceLimitsProvider,
  useResourceLimits,
} from '../hooks/useResourceLimits';
import { SCOPES, isScopeBlockedByReadOnly } from '../auth/permissions';

/** A session that holds every scope asked of it. */
const ALL_SCOPES: AppAuthContextType = {
  isAuthenticated: true,
  isLoading: false,
  sessionUnavailable: false,
  refreshSession: async () => {},
  user: null,
  getAccessToken: async () => null,
  hasPermission: () => true,
  login: async () => {},
  logout: async () => {},
};

/**
 * Both permission hooks plus the control that supplies the read-only verdict —
 * the arrangement the portal itself has, with the cloud plugin's `setReadOnly`
 * standing in for the subscription extension.
 */
function renderPermissions() {
  return renderHook(
    () => ({
      auth: useAppAuth(),
      scopes: useAppAuthScopes(),
      limits: useResourceLimits(),
    }),
    {
      wrapper: ({ children }: { children: React.ReactNode }) => (
        <AppAuthContext.Provider value={ALL_SCOPES}>
          <ResourceLimitsProvider>{children}</ResourceLimitsProvider>
        </AppAuthContext.Provider>
      ),
    }
  );
}

describe('isScopeBlockedByReadOnly', () => {
  it('keeps reads and deletes, blocks everything that adds or changes', () => {
    expect(isScopeBlockedByReadOnly(SCOPES.LLM_PROVIDER_READ)).toBe(false);
    expect(isScopeBlockedByReadOnly(SCOPES.GATEWAY_TOKEN_READ)).toBe(false);
    // Deleting is how an organization gets back under its limits.
    expect(isScopeBlockedByReadOnly(SCOPES.MCP_PROXY_DELETE)).toBe(false);
    expect(isScopeBlockedByReadOnly(SCOPES.APPLICATION_DELETE)).toBe(false);

    expect(isScopeBlockedByReadOnly(SCOPES.LLM_PROVIDER_CREATE)).toBe(true);
    expect(isScopeBlockedByReadOnly(SCOPES.LLM_PROXY_UPDATE)).toBe(true);
    expect(isScopeBlockedByReadOnly(SCOPES.AGENT_PROXY_MANAGE)).toBe(true);
    expect(
      isScopeBlockedByReadOnly(SCOPES.AGENT_PROXY_DEPLOYMENT_UNDEPLOY)
    ).toBe(true);
    expect(isScopeBlockedByReadOnly(SCOPES.MCP_PROXY_API_PORTAL_PUBLISH)).toBe(
      true
    );
  });
});

describe('useAppAuth while the organization is read-only', () => {
  it('holds every scope while the organization may write', () => {
    const { result } = renderPermissions();

    expect(result.current.auth.hasPermission(SCOPES.LLM_PROVIDER_CREATE)).toBe(
      true
    );
    expect(result.current.auth.hasPermission(SCOPES.LLM_PROVIDER_READ)).toBe(
      true
    );
  });

  it('refuses write scopes and keeps read scopes once read-only', () => {
    const { result } = renderPermissions();

    act(() => {
      result.current.limits.setReadOnly(true, 'Trial ended.');
    });

    expect(result.current.auth.hasPermission(SCOPES.LLM_PROVIDER_CREATE)).toBe(
      false
    );
    expect(result.current.auth.hasPermission(SCOPES.GATEWAY_UPDATE)).toBe(false);
    expect(
      result.current.auth.hasPermission(SCOPES.MCP_PROXY_DEPLOYMENT_CREATE)
    ).toBe(false);

    expect(result.current.auth.hasPermission(SCOPES.LLM_PROVIDER_READ)).toBe(
      true
    );
    expect(result.current.auth.hasPermission(SCOPES.GATEWAY_READ)).toBe(true);
    // Deletes stay available: a read-only organization can still remove what it
    // has, which is how it gets back under a plan's limits.
    expect(result.current.auth.hasPermission(SCOPES.APPLICATION_DELETE)).toBe(
      true
    );
    expect(result.current.auth.hasPermission(SCOPES.LLM_PROVIDER_DELETE)).toBe(
      true
    );
  });

  it('leaves what the user may SEE to the un-overlaid scopes', () => {
    const { result } = renderPermissions();

    act(() => {
      result.current.limits.setReadOnly(true, 'Trial ended.');
    });

    // Navigation and page variants read this one, so a read-only organization
    // keeps the shape of the product it had.
    expect(
      result.current.scopes.hasPermission(SCOPES.LLM_TEMPLATE_MANAGE)
    ).toBe(true);
  });

  it('restores write scopes when the verdict is lifted', () => {
    const { result } = renderPermissions();

    act(() => {
      result.current.limits.setReadOnly(true, 'Trial ended.');
    });
    act(() => {
      result.current.limits.setReadOnly(false);
    });

    expect(result.current.auth.hasPermission(SCOPES.LLM_PROVIDER_CREATE)).toBe(
      true
    );
  });
});

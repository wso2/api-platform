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

/**
 * Unit tests for the client-side scope gate in permissions.ts.
 *
 * `treePermissionsFor` must list exactly the permission-tree scopes the Platform
 * API accepts for a gate (the node's action and :manage, and the same on every
 * ancestor up to ap:manage / ap:read), `effectiveScopes` must translate a user's
 * pre-tree names the way the API does, and `checkPermission` must grant from
 * either vocabulary while never deriving the API-key ownership override.
 */
import { describe, expect, it } from 'vitest';

import { checkPermission, effectiveScopes, SCOPES, treePermissionsFor } from './permissions';

describe('treePermissionsFor', () => {
  it('maps a pre-tree read scope to its node and every ancestor, read before manage', () => {
    expect(treePermissionsFor(SCOPES.LLM_PROXY_API_KEY_READ)).toEqual([
      'ap:api:llm:api_key:read',
      'ap:api:llm:api_key:manage',
      'ap:api:llm:read',
      'ap:api:llm:manage',
      'ap:api:read',
      'ap:api:manage',
      'ap:read',
      'ap:manage',
    ]);
  });

  it('keeps a mutating scope\'s action on its own node and adds only :manage from the ancestors', () => {
    expect(treePermissionsFor('ap:llm_proxy:api_key:create')).toEqual([
      'ap:api:llm:api_key:create',
      'ap:api:llm:api_key:manage',
      'ap:api:llm:manage',
      'ap:api:manage',
      'ap:manage',
    ]);
    expect(treePermissionsFor(SCOPES.LLM_PROXY_DEPLOYMENT_CREATE)).toEqual([
      'ap:api:llm:deployment:create',
      'ap:api:llm:deployment:manage',
      'ap:api:llm:manage',
      'ap:api:manage',
      'ap:manage',
    ]);
    // The parent's own create is its own operation; it does not flow down.
    expect(treePermissionsFor('ap:llm_proxy:deployment:undeploy')).toEqual([
      'ap:api:llm:deployment:undeploy',
      'ap:api:llm:deployment:manage',
      'ap:api:llm:manage',
      'ap:api:manage',
      'ap:manage',
    ]);
  });

  it('resolves the event-gateway kinds, which live under ap:api on a plugin build', () => {
    expect(treePermissionsFor(SCOPES.WEBSUB_API_READ)).toEqual([
      'ap:api:websub:read',
      'ap:api:websub:manage',
      'ap:api:read',
      'ap:api:manage',
      'ap:read',
      'ap:manage',
    ]);
  });

  it('accepts a tree permission as the gate itself', () => {
    expect(treePermissionsFor('ap:api:mcp:deployment:read')).toEqual([
      'ap:api:mcp:deployment:read',
      'ap:api:mcp:deployment:manage',
      'ap:api:mcp:read',
      'ap:api:mcp:manage',
      'ap:api:read',
      'ap:api:manage',
      'ap:read',
      'ap:manage',
    ]);
  });

  it('follows a pre-tree resource that moved under a different parent', () => {
    expect(treePermissionsFor(SCOPES.LLM_TEMPLATE_READ)).toEqual([
      'ap:llm_provider:template:read',
      'ap:llm_provider:template:manage',
      'ap:llm_provider:read',
      'ap:llm_provider:manage',
      'ap:read',
      'ap:manage',
    ]);
    expect(treePermissionsFor(SCOPES.SECRET_MANAGE)).toEqual(['ap:credential:secret:manage', 'ap:manage']);
  });

  it('resolves every gate in SCOPES to tree permissions', () => {
    // A gate the tree cannot place would silently deny everyone; a feature
    // whose API has not shipped yet is a reserved tree node instead.
    for (const [name, scope] of Object.entries(SCOPES)) {
      expect(treePermissionsFor(scope), name).not.toEqual([]);
    }
  });

  it('keeps MCP proxy portal publishing for admins and operators, not publishers', () => {
    // Reserved node ap:portal:mcp_proxy, carried over from ap:api_portal:manage.
    expect(treePermissionsFor(SCOPES.MCP_PROXY_API_PORTAL_PUBLISH)).toEqual([
      'ap:portal:mcp_proxy:publish', 'ap:portal:mcp_proxy:manage', 'ap:portal:manage', 'ap:manage',
    ]);
    expect(checkPermission(['ap:manage'], SCOPES.MCP_PROXY_API_PORTAL_PUBLISH)).toBe(true);
    expect(checkPermission(['ap:portal:read', 'ap:portal:mcp_proxy:manage'], SCOPES.MCP_PROXY_API_PORTAL_UNPUBLISH)).toBe(true);
    expect(checkPermission(['ap:portal:read', 'ap:portal:draft:manage', 'ap:portal:rest_api:manage'], SCOPES.MCP_PROXY_API_PORTAL_PUBLISH)).toBe(false);
    // The pre-tree names still work as before.
    expect(checkPermission(['ap:api_portal:manage'], SCOPES.MCP_PROXY_API_PORTAL_PUBLISH)).toBe(true);
  });

  it('returns nothing for a resource the tree does not know, or for the ownership override', () => {
    expect(treePermissionsFor('ap:gateway:artifact:read')).toEqual([]);
    expect(treePermissionsFor('ap:api_key:all:manage')).toEqual([]);
    expect(treePermissionsFor('ap:credential:api_key:all:manage')).toEqual([]);
  });
});

describe('checkPermission', () => {
  it('still honours pre-tree scopes exactly and through an ancestor :manage', () => {
    expect(checkPermission([SCOPES.LLM_PROXY_READ], SCOPES.LLM_PROXY_READ)).toBe(true);
    expect(checkPermission([SCOPES.LLM_PROVIDER_MANAGE], 'ap:llm_provider:api_key:create')).toBe(true);
    expect(checkPermission([SCOPES.LLM_PROXY_READ], SCOPES.LLM_PROXY_CREATE)).toBe(false);
  });

  it('grants a read gate from the node read, an ancestor read, or an ancestor manage', () => {
    expect(checkPermission(['ap:api:llm:read'], SCOPES.LLM_PROXY_READ)).toBe(true);
    expect(checkPermission(['ap:api:read'], SCOPES.LLM_PROXY_DEPLOYMENT_READ)).toBe(true);
    expect(checkPermission(['ap:api:manage'], SCOPES.LLM_PROXY_API_KEY_READ)).toBe(true);
    expect(checkPermission(['ap:llm_provider:read'], SCOPES.LLM_TEMPLATE_READ)).toBe(true);
  });

  it('grants a mutating gate from its own action or a :manage on the node or an ancestor', () => {
    expect(checkPermission(['ap:api:llm:manage'], SCOPES.LLM_PROXY_CREATE)).toBe(true);
    expect(checkPermission(['ap:api:manage'], SCOPES.LLM_PROXY_DEPLOYMENT_CREATE)).toBe(true);
    expect(checkPermission(['ap:api:llm:create'], SCOPES.LLM_PROXY_CREATE)).toBe(true);
    expect(checkPermission(['ap:api:llm:create'], SCOPES.LLM_PROXY_DEPLOYMENT_CREATE)).toBe(false);
    expect(checkPermission(['ap:api:llm:deployment:create'], SCOPES.LLM_PROXY_CREATE)).toBe(false);
    expect(checkPermission(['ap:api:read', 'ap:api:llm:read'], SCOPES.LLM_PROXY_CREATE)).toBe(false);
    expect(checkPermission(['ap:api:llm:api_key:read'], 'ap:llm_proxy:api_key:create')).toBe(false);
  });

  it('translates the pre-tree names a user holds the way the Platform API does', () => {
    expect([...effectiveScopes(['ap:llm_proxy:create', 'ap:api:read'])]).toEqual([
      'ap:llm_proxy:create',
      'ap:api:llm:create',
      'ap:api:read',
    ]);
    // A pre-tree parent read reaches the sub-resource through the tree; a parent create does not.
    expect(checkPermission(['ap:llm_proxy:read'], SCOPES.LLM_PROXY_DEPLOYMENT_READ)).toBe(true);
    expect(checkPermission(['ap:llm_proxy:create'], SCOPES.LLM_PROXY_CREATE)).toBe(true);
    expect(checkPermission(['ap:llm_proxy:create'], SCOPES.LLM_PROXY_DEPLOYMENT_CREATE)).toBe(false);
    expect(checkPermission(['ap:llm_proxy:read'], SCOPES.LLM_PROXY_DEPLOYMENT_CREATE)).toBe(false);
    // A pre-tree name satisfies a gate written with the tree name.
    expect(checkPermission(['ap:mcp_proxy:read'], 'ap:api:mcp:deployment:read')).toBe(true);
    expect(checkPermission(['ap:mcp_proxy:deployment:read'], 'ap:api:mcp:read')).toBe(false);
    // A name the mapping does not know grants nothing beyond itself.
    expect([...effectiveScopes(['ap:llm_proxy:deploy'])]).toEqual(['ap:llm_proxy:deploy']);
    expect(checkPermission(['ap:llm_proxy:deploy'], SCOPES.LLM_PROXY_DEPLOYMENT_CREATE)).toBe(false);
  });

  it('grants from the root: ap:manage admits everything, ap:read every read', () => {
    expect(checkPermission(['ap:manage'], SCOPES.LLM_PROXY_CREATE)).toBe(true);
    expect(checkPermission(['ap:manage'], SCOPES.SECRET_READ)).toBe(true);
    expect(checkPermission(['ap:read'], SCOPES.LLM_PROXY_READ)).toBe(true);
    expect(checkPermission(['ap:read'], SCOPES.LLM_PROXY_CREATE)).toBe(false);
  });

  it('does not let a sibling node satisfy a gate', () => {
    expect(checkPermission(['ap:api:mcp:manage'], SCOPES.LLM_PROXY_CREATE)).toBe(false);
    expect(checkPermission(['ap:credential:secret:manage'], SCOPES.LLM_PROXY_READ)).toBe(false);
  });

  it('matches the ownership override by either name and never derives it', () => {
    expect(checkPermission(['ap:credential:api_key:all:manage'], 'ap:api_key:all:manage')).toBe(true);
    expect(checkPermission(['ap:api_key:all:manage'], 'ap:credential:api_key:all:manage')).toBe(true);
    expect(checkPermission(['ap:api:manage', 'ap:credential:api_key:read'], 'ap:api_key:all:manage')).toBe(false);
    expect(checkPermission(['ap:manage'], 'ap:credential:api_key:all:manage')).toBe(false);
  });
});

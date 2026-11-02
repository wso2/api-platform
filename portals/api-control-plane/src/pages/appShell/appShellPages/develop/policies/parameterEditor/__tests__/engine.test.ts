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

import { describe, it, expect } from 'vitest';
import {
  createPolicyForm,
  type PolicyDefinition as EngineDefinition,
  type ViewNode,
} from '../engine';
import { fixture, type FixtureName } from './fixtures';

const formFor = (name: FixtureName, initialConfig?: Record<string, unknown>) =>
  createPolicyForm(fixture(name) as unknown as EngineDefinition, { initialConfig });

function find(n: ViewNode, key: string): ViewNode | undefined {
  if (n.pathKey === key) return n;
  for (const c of [
    ...(n.children ?? []),
    ...(n.items ?? []),
    ...(n.entries ?? []).map((e) => e.node),
  ]) {
    const hit = find(c, key);
    if (hit) return hit;
  }
  return undefined;
}
const node = (f: ReturnType<typeof formFor>, key: string) => find(f.getResolution().view, key);
const visible = (f: ReturnType<typeof formFor>, key: string) => node(f, key)?.visible === true;

describe('oauth2-generator: oneOf modes, password grant, key/value params', () => {
  it('shows the token endpoint mode first and hides the bearer token', () => {
    const f = formFor('oauth2-generator');
    const g = f.getResolution().view.groups?.[0];
    expect(g?.group.multi).toBe(false);
    expect(g?.group.options).toHaveLength(2);
    expect(visible(f, 'tokenEndpoint')).toBe(true);
    expect(visible(f, 'bearerToken')).toBe(false);
    expect(visible(f, 'username')).toBe(false);
    expect(node(f, 'clientId')?.errors).toContain('Required');
  });

  it('requires username and password only for the password grant', () => {
    const f = formFor('oauth2-generator');
    f.setValue(['tokenEndpoint'], 'https://idp.example.com/token');
    f.setValue(['clientId'], 'gw');
    f.setValue(['clientSecret'], 's3cret');
    expect(f.getResolution().valid).toBe(true);
    f.setValue(['grantType'], 'password');
    expect(node(f, 'username')?.required).toBe(true);
    expect(node(f, 'username')?.errors[0]).toMatch(/^Required when Grant type is password/);
    f.setValue(['username'], 'u');
    f.setValue(['password'], 'p');
    expect(f.getResolution().valid).toBe(true);
  });

  it('switching to bearer token drops the endpoint fields but keeps what was typed', () => {
    const f = formFor('oauth2-generator');
    f.setValue(['clientId'], 'gw');
    const g = f.getResolution().view.groups![0];
    f.setChoice(g.instanceKey, ['1']);
    expect(visible(f, 'tokenEndpoint')).toBe(false);
    expect(f.getResolution().config).not.toHaveProperty('clientId');
    expect(node(f, 'bearerToken')?.errors).toContain('Required');
    f.setChoice(g.instanceKey, ['0']);
    expect(f.getResolution().config.clientId).toBe('gw');
  });

  it('infers the bearer mode when editing an existing config', () => {
    const f = formFor('oauth2-generator', { bearerToken: 'abc' });
    expect(f.getResolution().view.groups![0].selected).toEqual(['1']);
    expect(f.getResolution().valid).toBe(true);
  });

  it('turns key/value rows into an object and flags duplicate keys', () => {
    const f = formFor('oauth2-generator');
    f.addEntry(['tokenRequestParams']);
    f.setEntryKey(['tokenRequestParams'], 0, 'scope');
    f.setValue(['tokenRequestParams', 0, 'value'], 'read');
    expect(f.getResolution().config.tokenRequestParams).toEqual({ scope: 'read' });
    f.addEntry(['tokenRequestParams']);
    f.setEntryKey(['tokenRequestParams'], 1, 'scope');
    expect(node(f, 'tokenRequestParams')?.entries?.[1].keyErrors).toContain('Duplicate name');
  });

  it('renders integer lists (tokenPurgeStatusCodes) instead of dropping them', () => {
    const f = formFor('oauth2-generator');
    expect(node(f, 'tokenPurgeStatusCodes')?.field.kind).toBe('list');
    expect(node(f, 'tokenPurgeStatusCodes')?.value).toEqual([401]);
  });

  it('accepts template expressions in typed fields', () => {
    const f = formFor('oauth2-generator', { bearerToken: 'x' });
    f.setValue(['tokenRequestMaxRetries'], '{{ env "RETRIES" }}');
    expect(f.getResolution().valid).toBe(true);
  });
});

describe('choice groups', () => {
  it('token-based-ratelimit: a oneOf listing every combination becomes "pick any"', () => {
    const f = formFor('token-based-ratelimit');
    const g = f.getResolution().view.groups![0];
    expect(g.group.multi).toBe(true);
    expect(g.group.options).toHaveLength(3);
    f.setChoice(g.instanceKey, ['0', '2']);
    const keys = f
      .getResolution()
      .view.children!.filter((c) => c.visible)
      .map((c) => c.field.key);
    expect(keys).toEqual(expect.arrayContaining(['promptTokenLimits', 'totalTokenLimits']));
    expect(keys).not.toContain('completionTokenLimits');
    f.setChoice(g.instanceKey, []);
    expect(f.getResolution().valid).toBe(false);
  });

  it('log-message: only the configured side counts as chosen', () => {
    const f = formFor('log-message', { response: { headers: true } });
    expect(Object.keys(f.getResolution().config)).toEqual(['response']);
  });

  it('set-headers: request/response are chosen by the group, not a separate toggle', () => {
    const f = formFor('set-headers');
    const g = f.getResolution().view.groups![0];
    expect(node(f, 'request')?.field.toggleable).toBe(false);
    f.setChoice(g.instanceKey, ['1']);
    expect(f.getResolution().config).not.toHaveProperty('request');
  });
});

describe('fields that depend on another field', () => {
  it('request-rewrite: the rewrite type picks which value field shows', () => {
    const f = formFor('request-rewrite');
    f.setEnabled(['pathRewrite'], true);
    f.setValue(['pathRewrite', 'type'], 'ReplaceFullPath');
    expect(visible(f, 'pathRewrite.replaceFullPath')).toBe(true);
    expect(visible(f, 'pathRewrite.replacePrefixMatch')).toBe(false);
  });

  it('advanced-ratelimit: rules work inside list items', () => {
    const f = formFor('advanced-ratelimit');
    if (!node(f, 'quotas')?.items?.length) f.addItem(['quotas']);
    f.addItem(['quotas', 0, 'keyExtraction']);
    f.setValue(['quotas', 0, 'keyExtraction', 0, 'type'], 'cel');
    expect(visible(f, 'quotas[0].keyExtraction[0].expression')).toBe(true);
    expect(visible(f, 'quotas[0].keyExtraction[0].key')).toBe(false);
  });

  it('aws-bedrock-guardrail: auth type reveals the matching credentials', () => {
    const f = formFor('aws-bedrock-guardrail');
    f.setEnabled(['awsAuth'], true);
    f.setValue(['awsAuth', 'authenticationType'], 'iam-user-access-key');
    expect(visible(f, 'awsAuth.awsAccessKeyID')).toBe(true);
    expect(visible(f, 'awsAuth.awsRoleARN')).toBe(false);
    expect(node(f, 'awsAuth.awsSecretAccessKey')?.field.kind).toBe('secret');
  });
});

describe('validation', () => {
  it('json-xml-mediator: rejects the same format in both directions', () => {
    const f = formFor('json-xml-mediator');
    f.setValue(['upstreamPayloadFormat'], 'xml');
    f.setValue(['downsteamPayloadFormat'], 'xml');
    expect(f.getResolution().errors.map((e) => e.message)).toEqual(
      expect.arrayContaining([expect.stringMatching(/^This combination isn't allowed/)]),
    );
  });

  it('set-headers: pattern errors land on the field', () => {
    const f = formFor('set-headers');
    f.addItem(['request', 'headers']);
    f.setValue(['request', 'headers', 0, 'name'], 'bad header!');
    f.setValue(['request', 'headers', 0, 'value'], 'v');
    expect(node(f, 'request.headers[0].name')?.errors[0]).toMatch(/^Doesn't match/);
  });

  it('api-key-auth: reads the non-standard validation block', () => {
    const f = formFor('api-key-auth');
    expect(f.getResolution().view.children![0].field.constraints.maxLength).toBe(128);
  });

  it('cors: a new form with defaults is valid (Cypress TC-96 adds CORS and submits straight away)', () => {
    const f = formFor('cors');
    expect(f.getResolution().valid).toBe(true);
    expect(f.getResolution().config.allowedOrigins).toEqual(['*']);
  });
});

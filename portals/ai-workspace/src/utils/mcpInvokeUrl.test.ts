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

import { describe, expect, it } from 'vitest';

import type { Gateway } from '../apis/gatewayTypes';
import { buildMcpInvokeUrl } from './mcpInvokeUrl';

const gateway = (fields: Partial<Gateway>) => fields as Gateway;

describe('buildMcpInvokeUrl', () => {
  it('serves the proxy at its context under the gateway, on the /mcp path', () => {
    expect(buildMcpInvokeUrl(gateway({ endpoints: ['https://gw.example.com'] }), '/travel')).toBe(
      'https://gw.example.com/travel/mcp'
    );
  });

  it('assumes https for a host written without a scheme', () => {
    expect(buildMcpInvokeUrl(gateway({ endpoints: ['gw.example.com'] }), '/travel')).toBe(
      'https://gw.example.com/travel/mcp'
    );
  });

  it('keeps the scheme it was given', () => {
    expect(buildMcpInvokeUrl(gateway({ endpoints: ['http://localhost:8080/'] }), '/travel')).toBe(
      'http://localhost:8080/travel/mcp'
    );
  });

  it('falls back to the older vhost field when the gateway lists no endpoints', () => {
    expect(buildMcpInvokeUrl(gateway({ vhost: 'old.example.com' }), '/travel')).toBe(
      'https://old.example.com/travel/mcp'
    );
  });

  it('adds the missing leading slash to a context and drops a trailing one', () => {
    expect(buildMcpInvokeUrl(gateway({ endpoints: ['https://gw.example.com'] }), 'travel/')).toBe(
      'https://gw.example.com/travel/mcp'
    );
  });

  it('serves a proxy without a context at the root', () => {
    expect(buildMcpInvokeUrl(gateway({ endpoints: ['https://gw.example.com'] }))).toBe(
      'https://gw.example.com/mcp'
    );
    expect(buildMcpInvokeUrl(gateway({ endpoints: ['https://gw.example.com'] }), '   ')).toBe(
      'https://gw.example.com/mcp'
    );
  });

  it('is empty when there is no host to build from', () => {
    expect(buildMcpInvokeUrl(undefined, '/travel')).toBe('');
    expect(buildMcpInvokeUrl(gateway({ endpoints: [] }), '/travel')).toBe('');
    expect(buildMcpInvokeUrl(gateway({ endpoints: ['  '] }), '/travel')).toBe('');
  });
});

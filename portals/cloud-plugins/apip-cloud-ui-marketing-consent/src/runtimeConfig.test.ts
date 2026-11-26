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

import { afterEach, describe, expect, it } from 'vitest';

import { consentUrl, proxyBase } from './runtimeConfig';

type Win = Window & { config?: Record<string, string>; __RUNTIME_CONFIG__?: Record<string, string> };
const win = window as Win;

afterEach(() => {
  delete win.config;
  delete win.__RUNTIME_CONFIG__;
});

describe('proxyBase', () => {
  it('defaults to the BFF proxy', () => {
    expect(proxyBase()).toBe('/proxy');
  });

  it('uses the gateway path the console is served under, without a trailing slash', () => {
    win.__RUNTIME_CONFIG__ = { platformApiBaseUrl: '/_api/api-management/' };
    expect(proxyBase()).toBe('/_api/api-management');
  });

  it('reads the upper-case key too', () => {
    win.config = { PLATFORM_API_BASE_URL: '/gw' };
    expect(proxyBase()).toBe('/gw');
  });

  it.each([
    ['an absolute URL', 'https://platform.example.com'],
    ['a protocol-relative URL', '//evil.example.com'],
    ['a backslash host', '/\\evil.example.com'],
    ['a relative path', 'proxy'],
  ])('ignores %s and falls back to /proxy', (_label, value) => {
    win.__RUNTIME_CONFIG__ = { platformApiBaseUrl: value };
    expect(proxyBase()).toBe('/proxy');
  });
});

describe('proxyBase with the base the console resolved', () => {
  it('prefers the base the host console resolved over the window keys', () => {
    win.__RUNTIME_CONFIG__ = { platformApiBaseUrl: '/from-window' };
    expect(proxyBase('/_api/api-management/')).toBe('/_api/api-management');
  });

  it('covers a deployment that only sets the build-time override (neither window key)', () => {
    expect(proxyBase('/_api/api-management')).toBe('/_api/api-management');
  });

  it.each([
    ['an absolute URL', 'https://platform.example.com'],
    ['a protocol-relative URL', '//evil.example.com'],
  ])('refuses %s and uses /proxy, as the console ignores its window keys once it has resolved one', (_label, value) => {
    win.config = { PLATFORM_API_BASE_URL: '/gw' };
    expect(proxyBase(value)).toBe('/proxy');
  });

  it('falls back to the window keys when the host passes an empty base', () => {
    win.config = { PLATFORM_API_BASE_URL: '/gw' };
    expect(proxyBase('')).toBe('/gw');
  });
});

describe('consentUrl', () => {
  it('sits under the marketing-consent route of the proxy base', () => {
    expect(consentUrl()).toBe('/proxy/marketing-consent/marketing-consent');
    win.__RUNTIME_CONFIG__ = { platformApiBaseUrl: '/_api/api-management' };
    expect(consentUrl()).toBe('/_api/api-management/marketing-consent/marketing-consent');
  });

  it('uses the resolved base when given one', () => {
    expect(consentUrl('/gw')).toBe('/gw/marketing-consent/marketing-consent');
  });
});

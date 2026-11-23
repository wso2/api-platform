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

import { describe, expect, it, vi } from 'vitest';
import { readTrackingConfig } from './config';

const ID = '01a1a5f4-f908-7fef-aa9a-e3d16219021b';

describe('readTrackingConfig', () => {
  it('reads both values', () => {
    expect(readTrackingConfig({ cookieProDomainScript: ID, gtmContainerId: 'GTM-ABC123' })).toEqual({
      cookieProDomainScript: ID,
      gtmContainerId: 'GTM-ABC123',
    });
    expect(readTrackingConfig({ cookieProDomainScript: `${ID}-test`, gtmContainerId: 'GTM-ABC123' })).not.toBeNull();
  });
  it('is silent and off when neither is set', () => {
    const warn = vi.fn();
    expect(readTrackingConfig({}, warn)).toBeNull();
    expect(readTrackingConfig(undefined, warn)).toBeNull();
    expect(warn).not.toHaveBeenCalled();
  });
  it('warns and stays off with only one of the two', () => {
    const warn = vi.fn();
    expect(readTrackingConfig({ gtmContainerId: 'GTM-ABC123' }, warn)).toBeNull();
    expect(readTrackingConfig({ cookieProDomainScript: ID }, warn)).toBeNull();
    expect(warn).toHaveBeenCalledTimes(2);
  });
  it('rejects values in the wrong format', () => {
    const warn = vi.fn();
    expect(readTrackingConfig({ cookieProDomainScript: 'x" onload="y', gtmContainerId: 'GTM-ABC123' }, warn)).toBeNull();
    expect(readTrackingConfig({ cookieProDomainScript: ID, gtmContainerId: 'GTM-1&x=2' }, warn)).toBeNull();
    expect(warn).toHaveBeenCalledTimes(2);
  });
  it('reads window.config over window.__RUNTIME_CONFIG__, as the console does', () => {
    const runtimeWindow = window as Window & {
      __RUNTIME_CONFIG__?: Record<string, unknown>;
      config?: Record<string, unknown>;
    };
    runtimeWindow.__RUNTIME_CONFIG__ = { cookieProDomainScript: ID, gtmContainerId: 'GTM-ABC123' };
    runtimeWindow.config = { gtmContainerId: 'GTM-OVERRIDE1' };
    try {
      expect(readTrackingConfig()).toEqual({
        cookieProDomainScript: ID,
        gtmContainerId: 'GTM-OVERRIDE1',
      });
    } finally {
      delete runtimeWindow.__RUNTIME_CONFIG__;
      delete runtimeWindow.config;
    }
  });
});

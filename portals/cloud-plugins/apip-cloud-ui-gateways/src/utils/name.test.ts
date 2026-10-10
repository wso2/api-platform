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

import { MAX_GATEWAY_HANDLE_LENGTH, gatewayHandleFromName, validateGatewayName } from './name';

describe('gatewayHandleFromName', () => {
  it('lowercases and folds spaces and underscores to hyphens', () => {
    expect(gatewayHandleFromName('  My Payments_Gateway ')).toBe('my-payments-gateway');
  });

  it('drops anything a handle cannot carry, and collapses the gaps', () => {
    expect(gatewayHandleFromName('Orders (v2)!')).toBe('orders-v2');
  });

  it('returns an empty handle when nothing usable survives', () => {
    expect(gatewayHandleFromName('!!!')).toBe('');
  });
});

describe('validateGatewayName', () => {
  it('leaves an empty name to the field’s own required handling', () => {
    expect(validateGatewayName('   ')).toBeUndefined();
  });

  it('reports a name no handle can be built from', () => {
    expect(validateGatewayName('!!!')).toMatch(/at least one letter or number/i);
  });

  it('accepts a name whose handle fits the handle column', () => {
    expect(validateGatewayName('Payments')).toBeUndefined();
  });

  it('reports a handle longer than the handle column', () => {
    expect(validateGatewayName('a'.repeat(MAX_GATEWAY_HANDLE_LENGTH + 1))).toMatch(/too long/i);
    expect(validateGatewayName('a'.repeat(MAX_GATEWAY_HANDLE_LENGTH))).toBeUndefined();
  });

  // The environment is bound through properties.environment, not the handle, so
  // a long environment id (a 36-character UUID) must not shrink the name budget.
  it('does not let the environment eat into the handle budget', () => {
    expect(validateGatewayName('mygd')).toBeUndefined();
  });

  // The name an environment gives its own gateway is not reserved. Rejecting it
  // outright meant that once that gateway was deleted the name was unusable, so
  // the environment was left with a name nothing could take back.
  it('accepts the name an environment gives its own gateway', () => {
    expect(validateGatewayName('default')).toBeUndefined();
    expect(validateGatewayName('Default')).toBeUndefined();
    expect(gatewayHandleFromName('Default')).toBe('default');
  });
});

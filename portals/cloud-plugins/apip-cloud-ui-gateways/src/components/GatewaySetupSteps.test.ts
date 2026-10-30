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

// Only the pure version check is under test; keep the UI library out of the module graph.
vi.mock('@wso2/oxygen-ui', () => ({}));
vi.mock('@wso2/oxygen-ui-icons-react', () => ({}));

import { isGatewayV12OrAbove } from './GatewaySetupSteps';

describe('isGatewayV12OrAbove', () => {
  it('is false for releases before 1.2', () => {
    expect(isGatewayV12OrAbove('1.0')).toBe(false);
    expect(isGatewayV12OrAbove('1.0.0')).toBe(false);
    expect(isGatewayV12OrAbove('1.1.9')).toBe(false);
    expect(isGatewayV12OrAbove('0.9.0')).toBe(false);
  });

  it('is true from 1.2 onwards', () => {
    expect(isGatewayV12OrAbove('1.2')).toBe(true);
    expect(isGatewayV12OrAbove('1.2.0')).toBe(true);
    expect(isGatewayV12OrAbove('1.10.0')).toBe(true);
    expect(isGatewayV12OrAbove('2.0.0')).toBe(true);
  });

  it('defaults to the current flow when the version cannot be parsed', () => {
    expect(isGatewayV12OrAbove('')).toBe(true);
    expect(isGatewayV12OrAbove('latest')).toBe(true);
    expect(isGatewayV12OrAbove('1')).toBe(true);
    expect(isGatewayV12OrAbove('2026.05.13')).toBe(true);
  });
});

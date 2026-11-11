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

import { defaultText, helperText, optionLabel } from './display';
import type { EditableField } from '../types';

const TLS: EditableField = {
  default: 'TLS1_2',
  description: 'The oldest TLS version the gateway accepts.',
  labels: { TLS1_2: 'TLS 1.2', TLS1_3: 'TLS 1.3' },
  path: 'gateway.config.router.upstream.tls.minimum_protocol_version',
  type: 'enum',
  values: ['TLS1_2', 'TLS1_3'],
};

describe('display', () => {
  it('shows enum values and defaults by their labels', () => {
    expect(optionLabel(TLS, 'TLS1_3')).toBe('TLS 1.3');
    expect(optionLabel(TLS, 'TLS1_4')).toBe('TLS1_4');
    expect(defaultText(TLS)).toBe('TLS 1.2');
  });

  it('shows a boolean default as On or Off', () => {
    expect(defaultText({ default: true, path: 'a', type: 'boolean' })).toBe('On');
    expect(defaultText({ default: false, path: 'a', type: 'boolean' })).toBe('Off');
  });

  it('puts the hint before the default, and leaves out what is absent', () => {
    expect(helperText({ ...TLS, hint: 'Backends on older TLS are refused.' })).toBe(
      'Backends on older TLS are refused. Default: TLS 1.2.'
    );
    expect(helperText(TLS)).toBe('Default: TLS 1.2.');
    expect(helperText({ path: 'a', type: 'integer', default: 10 })).toBe('Default: 10.');
    expect(helperText({ path: 'a', type: 'boolean' })).toBe('');
  });
});

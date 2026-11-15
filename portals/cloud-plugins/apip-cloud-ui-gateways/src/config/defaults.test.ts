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

import { isChange, shownValue } from './defaults';
import type { EditableField } from '../types';

const LOG_LEVEL: EditableField = {
  path: 'gateway.config.controller.logging.level',
  type: 'enum',
  values: ['debug', 'info', 'warn', 'error'],
  default: 'info',
};

const ACCESS_LOGS: EditableField = {
  path: 'gateway.config.router.access_logs.enabled',
  type: 'boolean',
  default: true,
};

const NO_DEFAULT: EditableField = {
  path: 'gateway.config.api_key.api_keys_per_user_per_api',
  type: 'integer',
};

describe('shownValue', () => {
  it('shows the default for a path the organization has not set', () => {
    expect(shownValue(LOG_LEVEL, {}, {})).toBe('info');
    expect(shownValue(ACCESS_LOGS, {}, {})).toBe(true);
  });

  it('shows the stored value over the default', () => {
    expect(shownValue(LOG_LEVEL, { [LOG_LEVEL.path]: 'debug' }, {})).toBe('debug');
    expect(shownValue(ACCESS_LOGS, { [ACCESS_LOGS.path]: false }, {})).toBe(false);
  });

  it('shows a pending edit over both', () => {
    expect(
      shownValue(LOG_LEVEL, { [LOG_LEVEL.path]: 'debug' }, { [LOG_LEVEL.path]: 'warn' })
    ).toBe('warn');
  });

  it('shows nothing for an unset path with no published default', () => {
    expect(shownValue(NO_DEFAULT, {}, {})).toBeUndefined();
  });
});

describe('isChange', () => {
  it('does not count picking the default on an unset path', () => {
    // Saving it would pin today's platform default onto the binding for good.
    expect(isChange(LOG_LEVEL, {}, 'info')).toBe(false);
    expect(isChange(ACCESS_LOGS, {}, true)).toBe(false);
  });

  it('counts any other value on an unset path', () => {
    expect(isChange(LOG_LEVEL, {}, 'debug')).toBe(true);
    expect(isChange(ACCESS_LOGS, {}, false)).toBe(true);
  });

  it('does not count clearing an unset input', () => {
    expect(isChange(NO_DEFAULT, {}, '')).toBe(false);
    expect(isChange(NO_DEFAULT, {}, undefined)).toBe(false);
  });

  it('counts setting a stored path to the default, since that changes the binding', () => {
    expect(isChange(LOG_LEVEL, { [LOG_LEVEL.path]: 'debug' }, 'info')).toBe(true);
  });

  it('does not count putting a stored value back', () => {
    expect(isChange(LOG_LEVEL, { [LOG_LEVEL.path]: 'debug' }, 'debug')).toBe(false);
  });
});

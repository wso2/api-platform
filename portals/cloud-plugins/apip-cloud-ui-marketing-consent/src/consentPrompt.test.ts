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

import { afterEach, describe, expect, it, vi } from 'vitest';

import { markAsked, shouldPrompt, wasAsked } from './consentPrompt';

describe('shouldPrompt', () => {
  it.each(['No Opt-in', 'Unconfirmed'] as const)('asks someone whose status is %s', (consentStatus) => {
    expect(shouldPrompt({ consentStatus, optoutOfEmails: false })).toBe(true);
  });

  it.each(['Confirmed', 'Unsubscribed'] as const)('does not ask once the status is %s', (consentStatus) => {
    expect(shouldPrompt({ consentStatus, optoutOfEmails: false })).toBe(false);
  });

  it.each(['No Opt-in', 'Unconfirmed', 'Confirmed', 'Unsubscribed'] as const)(
    'never asks someone who opted out of all email (%s)',
    (consentStatus) => {
      expect(shouldPrompt({ consentStatus, optoutOfEmails: true })).toBe(false);
    }
  );

  it('does not ask when the record could not be read', () => {
    expect(shouldPrompt(null)).toBe(false);
  });
});

describe('wasAsked / markAsked', () => {
  afterEach(() => {
    window.sessionStorage.clear();
    vi.restoreAllMocks();
  });

  it('is false until the prompt has been dealt with, then true', () => {
    expect(wasAsked()).toBe(false);
    markAsked();
    expect(wasAsked()).toBe(true);
  });

  it('survives storage being unavailable: reads false, writes do not throw', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    expect(wasAsked()).toBe(false);
    expect(() => markAsked()).not.toThrow();
  });
});

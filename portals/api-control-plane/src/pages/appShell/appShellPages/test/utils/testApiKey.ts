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

import type { CreateApiKeyBody, CreateApiKeyResponse } from '@/api/resources/apiKeys';

/**
 * Short-lived API key used by the test console.
 *
 * The console requests a recognisable API key with a short expiry. Its plaintext
 * is returned only once, and the response does not report expiry, so this
 * module keeps the value and derives its expiration from the requested TTL.
 */

/** How long a console-minted key stays valid. */
export const TEST_KEY_TTL_HOURS = 1;

/** Stable, recognisable name for console-minted keys. */
export const TEST_KEY_DISPLAY_NAME = 'Test console key';

/** A minted key, plus what the console needs to present it. */
export type TestApiKey = {
  /** The plaintext credential. Never log or persist this. */
  value: string;
  /** Platform id of the key, for revocation. Absent if the server omitted it. */
  keyId?: string;
  /**
   * When the key stops working, as an epoch millisecond timestamp.
   *
    * Estimated from `TEST_KEY_TTL_HOURS`; the response does not report expiry.
   */
  expiresAt: number;
};

/**
 * The body the console asks `CreateAPIKey` for.
 *
 * Sends no `apiKey` of its own, so the server generates the value and returns
 * it in that one response — supplying one suppresses the generated value and
 * would leave the console with a key it cannot display.
 */
export const testApiKeyBody = (): CreateApiKeyBody => ({
  displayName: TEST_KEY_DISPLAY_NAME,
  expiresIn: { duration: TEST_KEY_TTL_HOURS, unit: 'hours' },
});

/**
 * Reads a created key out of the response.
 *
 * `requestedAt` starts the deadline before the request, making the countdown
 * conservative if the round-trip is slow.
 *
 * Throws if the response is unsuccessful or lacks `apiKey`; an empty credential
 * must never be handed to the console.
 */
export const toTestApiKey = (
  response: CreateApiKeyResponse,
  requestedAt: number,
): TestApiKey => {
  if (response.status !== 'success' || !response.apiKey) {
    throw new Error('Test key creation returned no key value');
  }

  return {
    expiresAt: requestedAt + TEST_KEY_TTL_HOURS * 60 * 60 * 1000,
    keyId: response.keyId,
    value: response.apiKey,
  };
};

/** Milliseconds until a key expires, floored at zero. */
export const testKeyRemainingMs = (key: TestApiKey | undefined, now: number): number => {
  if (!key) return 0;
  return Math.max(0, key.expiresAt - now);
};

/** Whether a key has expired. */
export const isTestKeyExpired = (key: TestApiKey | undefined, now: number): boolean =>
  testKeyRemainingMs(key, now) === 0;

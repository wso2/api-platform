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

import { consentUrl } from './runtimeConfig';
import type { ConsentRecord, ConsentStatus, SettableConsentStatus } from './types';

// Must match the console BFF's CSRFHeaderName / the host's CSRF_HEADER_VALUE.
const CSRF_HEADER = 'X-Requested-By';
const CSRF_HEADER_VALUE = 'api-control-plane';

// The read can cost a token exchange plus the Pardot lookup; the API asks callers
// to allow up to 12 seconds.
const READ_TIMEOUT_MS = 15_000;

// The write answers 202 before delivery to Pardot, so it is quick; the bound only
// keeps a stalled request from leaving the dialog locked.
const WRITE_TIMEOUT_MS = 15_000;

const KNOWN_STATUSES: readonly ConsentStatus[] = [
  'No Opt-in',
  'Unconfirmed',
  'Confirmed',
  'Unsubscribed',
];

/**
 * What Pardot holds for the signed-in person, or `null` when it cannot be read —
 * not configured here (404), no email on the token (403), Pardot down (502), a
 * timeout, or an unrecognised body. Never throws: consent is a courtesy prompt and
 * must not get in the way of the console, and an unknown answer is not a reason to
 * ask.
 */
export async function fetchConsent(proxyBase?: string): Promise<ConsentRecord | null> {
  try {
    const response = await fetch(consentUrl(proxyBase), {
      credentials: 'same-origin',
      cache: 'no-store',
      headers: { Accept: 'application/json' },
      signal: AbortSignal.timeout(READ_TIMEOUT_MS),
    });
    if (!response.ok) return null;
    const body = (await response.json()) as Partial<ConsentRecord> | null;
    const status = body?.consentStatus;
    if (!status || !KNOWN_STATUSES.includes(status)) return null;
    return { consentStatus: status, optoutOfEmails: body?.optoutOfEmails === true };
  } catch {
    return null;
  }
}

/**
 * Records the person's decision. `202` means accepted, not delivered, and the call
 * is safe to retry. Resolves whether it was accepted; a refusal, a network failure
 * and a timeout all answer false, so the caller can offer a retry.
 */
export async function recordConsent(
  status: SettableConsentStatus,
  proxyBase?: string
): Promise<boolean> {
  try {
    const response = await fetch(consentUrl(proxyBase), {
      method: 'POST',
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        [CSRF_HEADER]: CSRF_HEADER_VALUE,
      },
      body: JSON.stringify({ consentstatus: status }),
      signal: AbortSignal.timeout(WRITE_TIMEOUT_MS),
    });
    return response.status === 202;
  } catch {
    return false;
  }
}

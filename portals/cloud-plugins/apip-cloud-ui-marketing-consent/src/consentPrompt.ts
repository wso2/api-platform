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

import type { ConsentRecord } from './types';

/**
 * Whether to ask. Only for someone who has never answered (`No Opt-in` or
 * `Unconfirmed`) and has not opted out of all WSO2 email: asking someone who
 * unsubscribed, or already confirmed, would be nagging.
 */
export function shouldPrompt(record: ConsentRecord | null): boolean {
  if (!record || record.optoutOfEmails) return false;
  return record.consentStatus === 'No Opt-in' || record.consentStatus === 'Unconfirmed';
}

const ASKED_KEY = 'apip.marketingConsent.asked';

/**
 * Whether this browser session has already dealt with the prompt, either because
 * the person closed it ("Maybe later" by Esc or a click outside, which records
 * nothing with Pardot) or because they answered.
 *
 * The second case matters: a `202` means accepted, not delivered, so a re-read
 * right after answering can still say `No Opt-in`, and a remount of the gate would
 * ask again. Storage can be unavailable (private windows, blocked site data), and
 * then the prompt simply stays per page load.
 */
export function wasAsked(): boolean {
  try {
    return window.sessionStorage.getItem(ASKED_KEY) === '1';
  } catch {
    return false;
  }
}

/** Stop asking for the rest of this browser session. */
export function markAsked(): void {
  try {
    window.sessionStorage.setItem(ASKED_KEY, '1');
  } catch {
    /* nothing to remember it with */
  }
}

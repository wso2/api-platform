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

import type { CustomFormats } from 'react-intl';

/**
 * Time zone for timestamps. `undefined` uses the viewer's zone.
 * Set to `'UTC'` for cross-region consistency.
 */
export const DISPLAY_TIME_ZONE: string | undefined = undefined;

/**
 * Shared named date/time formats used via intl (e.g. format: 'short').
 * Uses `dateStyle: 'medium'` to avoid locale-dependent numeric month/day order.
 */
export const INTL_FORMATS: CustomFormats = {
  date: {
    /** "Aug 17, 2026" — a calendar day, where the time of day is noise. */
    short: { dateStyle: 'medium' },
    /** "Aug 17, 2026, 5:45 PM" — an event with a moment attached. */
    withTime: { dateStyle: 'medium', timeStyle: 'short' },
  },
};

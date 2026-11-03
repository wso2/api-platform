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

import { createIntl } from 'react-intl';
import { describe, expect, it } from 'vitest';

import type { Meter } from '@/api/resources/meters';
import type { SubscriptionPlanPricing } from '@/api/resources/subscriptionPlans';
import { formatAmount, pricingSummary } from './pricingFormat';

const intl = createIntl({ defaultLocale: 'en', locale: 'en', messages: {} });

type Price = NonNullable<SubscriptionPlanPricing['prices']>[number];

const meters: Meter[] = [
  { displayName: 'Successful calls', id: 'calls', type: 'REQUEST_COUNT', typeKey: null },
  {
    displayName: 'LLM input tokens',
    id: 'llm-input',
    type: 'TOKENS',
    typeKey: 'metadata.aiTokenUsage.promptTokens',
  },
  {
    displayName: 'LLM output tokens',
    id: 'llm-output',
    type: 'TOKENS',
    typeKey: 'metadata.aiTokenUsage.completionTokens',
  },
];
const perUnit = (meterId: string, unitAmount: string, unitQuantity = 1): Price => ({
  meterId,
  pricingModel: 'PER_UNIT',
  unitAmount,
  unitQuantity,
});
const tiered = (pricingModel: 'VOLUME' | 'GRADUATED'): Price => ({
  meterId: 'calls',
  pricingModel,
  tiers: [{ flatAmount: null, unitAmount: '0.01', upTo: null }],
});
const usage = (prices: Price[], billingPeriod: 'MONTH' | 'YEAR' = 'MONTH') =>
  pricingSummary(
    intl,
    { billingPeriod, currency: 'USD', flatAmount: null, prices, pricingType: 'USAGE_BASED' },
    meters,
  );

describe('pricingSummary', () => {
  it('reads as the plans list shows it', () => {
    expect(pricingSummary(intl, undefined)).toBe('Free');
    expect(
      pricingSummary(intl, {
        billingPeriod: 'MONTH',
        currency: 'USD',
        flatAmount: '20',
        prices: [],
        pricingType: 'FLAT_RATE',
      }),
    ).toBe('$20.00 / month');
    expect(
      pricingSummary(intl, {
        billingPeriod: 'YEAR',
        currency: 'USD',
        flatAmount: '200',
        prices: [],
        pricingType: 'FLAT_RATE',
      }),
    ).toBe('$200.00 / year');
    expect(usage([perUnit('calls', '0.01')])).toBe('$0.01 per request, billed monthly');
    // A token meter's unit is a token, whichever tokens it adds up; its name says which.
    expect(usage([perUnit('llm-input', '2.50', 1000000)], 'YEAR')).toBe(
      '$2.50 per 1,000,000 tokens, billed yearly',
    );
    expect(usage([perUnit('llm-output', '0.00001')])).toBe('$0.00001 per token, billed monthly');
    expect(usage([perUnit('unknown', '0.01')])).toBe('$0.01 per unit, billed monthly');
    expect(usage([tiered('VOLUME')])).toBe('Volume tiers, billed monthly');
    expect(usage([tiered('GRADUATED')], 'YEAR')).toBe('Graduated tiers, billed yearly');
    expect(usage([perUnit('calls', '0.01'), tiered('VOLUME'), tiered('GRADUATED')])).toBe(
      '3 usage prices, billed monthly',
    );
  });
});

describe('formatAmount', () => {
  it('keeps the precision an amount was entered with', () => {
    expect(formatAmount(intl, '0.008', 'USD')).toBe('$0.008');
    expect(formatAmount(intl, '500', 'JPY')).toBe('¥500');
  });

  it("shows the currency's own decimals, dropping trailing zeros beyond them", () => {
    expect(formatAmount(intl, '12.00', 'JPY')).toBe('¥12');
    expect(formatAmount(intl, '12', 'KWD')).toBe('KWD\u00a012.000'); // a code is followed by a no-break space
    expect(formatAmount(intl, '0.0100', 'USD')).toBe('$0.01');
    expect(formatAmount(intl, '12', 'AUD')).toBe('A$12.00');
  });
});

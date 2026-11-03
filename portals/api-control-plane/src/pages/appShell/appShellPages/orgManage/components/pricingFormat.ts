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
import { defineMessages, type IntlShape } from 'react-intl';

import type { Meter } from '@/api/resources/meters';
import type { SubscriptionPlanPricing } from '@/api/resources/subscriptionPlans';
import { meterUnit } from './meterLabels';

/** How a plan's pricing reads in the plans list, matching the API Portal. */

const messages = defineMessages({
  free: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.pricingFormat.free',
    defaultMessage: 'Free',
  },
  fee: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.pricingFormat.fee',
    defaultMessage: '{amount} / {period, select, YEAR {year} other {month}}',
    description: 'A flat rate plan, e.g. "$20.00 / month".',
  },
  perUnit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.pricingFormat.perUnit',
    defaultMessage:
      '{amount} per {unit, select, request {{quantity, plural, one {request} other {# requests}}} token {{quantity, plural, one {token} other {# tokens}}} other {{quantity, plural, one {unit} other {# units}}}}, billed {period, select, YEAR {yearly} other {monthly}}',
    description:
      'A usage based plan with one per-unit price, e.g. "$0.01 per request, billed monthly" or "$2.50 per 1,000,000 tokens, billed monthly".',
  },
  volume: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.pricingFormat.volume',
    defaultMessage: 'Volume tiers, billed {period, select, YEAR {yearly} other {monthly}}',
  },
  graduated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.pricingFormat.graduated',
    defaultMessage: 'Graduated tiers, billed {period, select, YEAR {yearly} other {monthly}}',
  },
  usagePrices: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.pricingFormat.usagePrices',
    defaultMessage:
      '{prices, plural, one {# usage price} other {# usage prices}}, billed {period, select, YEAR {yearly} other {monthly}}',
    description: 'A usage based plan with several prices, e.g. "3 usage prices, billed monthly".',
  },
});

/**
 * An amount as given (a decimal string) in its currency: the currency's own
 * decimals, and more only where the given digits matter: "99" -> "$99.00",
 * "0.008" -> "$0.008", "12.00" in yen -> "¥12", "12" in dinars -> "KWD 12.000".
 */
export const formatAmount = (intl: IntlShape, amount: string, currency: string): string => {
  // Trailing zeros beyond the currency's own decimals say nothing.
  const decimals = amount.includes('.') ? amount.split('.')[1].replace(/0+$/, '').length : 0;
  try {
    const minimum =
      new Intl.NumberFormat(intl.locale, { currency, style: 'currency' }).resolvedOptions()
        .minimumFractionDigits ?? 2;
    const digits = Math.max(minimum, decimals);
    return intl.formatNumber(Number(amount), {
      currency,
      maximumFractionDigits: digits,
      minimumFractionDigits: digits,
      style: 'currency',
    });
  } catch {
    return `${amount} ${currency}`;
  }
};

/** What one unit of a meter is: a call or a token, or a unit when the meter is not known. */
const unitOf = (meterId: string, meters: Meter[]): ReturnType<typeof meterUnit> | 'unit' => {
  const meter = meters.find((candidate) => candidate.id === meterId);
  return meter ? meterUnit(meter) : 'unit';
};

/**
 * The meters pricingSummary names a unit from: the meter of a usage based plan's only price, when
 * that price is per unit. A screen reads just these for the plans it shows.
 */
export const summaryMeterIds = (pricing: SubscriptionPlanPricing | undefined): string[] => {
  const prices = pricing?.pricingType === 'USAGE_BASED' ? (pricing.prices ?? []) : [];
  if (prices.length !== 1) return [];
  const [price] = prices;
  return price.pricingModel === 'VOLUME' || price.pricingModel === 'GRADUATED'
    ? []
    : [price.meterId];
};

/**
 * "Free"; a flat rate as "$20.00 / month"; usage based with one price as "$0.01
 * per call, billed monthly" or "Graduated tiers, billed monthly", and with
 * several as "3 usage prices, billed monthly". `meters` names the unit a
 * per-unit price counts.
 */
export const pricingSummary = (
  intl: IntlShape,
  pricing: SubscriptionPlanPricing | undefined,
  meters: Meter[] = [],
): string => {
  if (!pricing) return intl.formatMessage(messages.free);
  const currency = pricing.currency ?? 'USD';
  const period = pricing.billingPeriod;
  const prices = pricing.prices ?? [];
  if (pricing.pricingType === 'FLAT_RATE') {
    return intl.formatMessage(messages.fee, {
      amount: formatAmount(intl, pricing.flatAmount ?? '0', currency),
      period,
    });
  }
  if (prices.length !== 1)
    return intl.formatMessage(messages.usagePrices, { period, prices: prices.length });
  const [price] = prices;
  switch (price.pricingModel) {
    case 'VOLUME':
      return intl.formatMessage(messages.volume, { period });
    case 'GRADUATED':
      return intl.formatMessage(messages.graduated, { period });
    default:
      return intl.formatMessage(messages.perUnit, {
        amount: formatAmount(intl, price.unitAmount ?? '0', currency),
        period,
        quantity: price.unitQuantity ?? 1,
        unit: unitOf(price.meterId, meters),
      });
  }
};

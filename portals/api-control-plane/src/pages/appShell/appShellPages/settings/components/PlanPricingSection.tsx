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
import {
  Autocomplete,
  Box,
  Button,
  Form,
  FormControl,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  IconButton,
  InputAdornment,
  MenuItem,
  OutlinedInput,
  Select,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowRight, Plus, X } from '@wso2/oxygen-ui-icons-react';
import { useMemo } from 'react';
import {
  defineMessages,
  FormattedMessage,
  useIntl,
  type IntlShape,
  type MessageDescriptor,
} from 'react-intl';

import type { Meter } from '@/api/resources/meters';
import { hairline } from '@/theme/receipes';
import { CURRENCIES, type Currency } from './currencies';
import { MeterPicker } from './MeterPicker';
import { ReadOnlyField } from './ReadOnlyField';
import {
  API_TYPES,
  FILTER_RULES,
  HTTP_METHODS,
  MAX_TOKEN_CONDITIONS,
  fieldMessages,
  meterUnit,
  operatorMessage,
  priceFieldsOf,
  takesOneValue,
  tokenFieldLabel,
  typeLabel,
  type FilterOperator,
} from './meterLabels';
import {
  MAX_PRICES,
  MAX_TIERS,
  PRICING_MODELS,
  PRICING_TYPES,
  emptyFilterRow,
  filterRows,
  isAllowed,
  isTiered,
  newPrice,
  newTier,
  priceLabel,
  pricesByName,
  type FilterDraft,
  type PriceDraft,
  type PricingDraft,
  type PricingModel,
  type PricingProblem,
  type PricingType,
  type TierDraft,
} from './planPricingDraft';

const CURRENCY_FIELD = 'plan-pricing-currency';
const PERIOD_FIELD = 'plan-pricing-period';
const TYPE_FIELD = 'plan-pricing-type';
const AMOUNT_FIELD = 'plan-pricing-amount';
const MONEY_WIDTH = 130;
const TIER_COLUMNS = `minmax(0,1.3fr) auto ${MONEY_WIDTH}px auto ${MONEY_WIDTH}px 32px`;
/**
 * Filter rows: the filter's name, its operator, its values, then adding or removing a Token
 * condition.
 */
const FILTER_COLUMNS = '150px minmax(0,1fr) minmax(0,1.4fr) 32px';

const messages = defineMessages({
  addPrice: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.addPrice',
    defaultMessage: 'Add price',
  },
  amountLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.amountLabel',
    defaultMessage: 'Amount',
    description: 'What a flat rate plan charges every billing period.',
  },
  addCondition: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.addCondition',
    defaultMessage: 'Add a condition on {field}',
    description:
      'Accessible name of the button adding a second condition on a Token field, e.g. "Token · Prompt tokens".',
  },
  addTier: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.addTier',
    defaultMessage: 'Add tier',
  },
  chargeEvery: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.chargeEvery',
    defaultMessage: 'Charge every',
  },
  createHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.createHint',
    defaultMessage: "Pricing can't be updated once the plan is created.",
    description:
      'Shown under the Paid plan switch of a new plan once it is turned on: the pricing given now is final.',
  },
  currencyLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.currency',
    defaultMessage: 'Currency',
  },
  apiTypePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.apiTypePlaceholder',
    defaultMessage: '{many, select, true {RestApi, Mcp} other {RestApi}}',
    description: 'Example API types, as the gateway names them. Do not translate the names.',
  },
  any: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.any',
    defaultMessage: 'Any',
    description:
      'The empty choice of a filter whose one value is picked from a list, such as an HTTP method: the price counts every value.',
  },
  eventLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.eventLabel',
    defaultMessage: 'Event',
    description: 'What the chosen meter counts, shown beside it: "API requests" or "Tokens".',
  },
  filterOperator: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.filterOperator',
    defaultMessage: 'Filter operator',
  },
  filterValues: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.filterValues',
    defaultMessage: 'Filter values',
  },
  filtersHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.filtersHint',
    defaultMessage: 'Leave a filter empty to count every value.',
  },
  filtersLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.filtersLabel',
    defaultMessage: 'Filters',
    description: "Which of the meter's requests a usage price counts.",
  },
  meterLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.meter',
    defaultMessage: 'Meter',
  },
  methodsPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.methodsPlaceholder',
    defaultMessage: 'Pick methods',
  },
  modelGraduated: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.model.graduated',
    defaultMessage: 'Graduated tiers',
  },
  modelLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.model',
    defaultMessage: 'Pricing model',
  },
  modelPerUnit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.model.perUnit',
    defaultMessage: 'Per unit',
  },
  lockedHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.lockedHint',
    defaultMessage: "Pricing can't be updated once a plan is created.",
    description:
      'Shown under the Paid plan switch of an existing plan, whose pricing is shown but cannot be edited.',
  },
  modelVolume: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.model.volume',
    defaultMessage: 'Volume tiers',
  },
  noCurrencies: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.noCurrencies',
    defaultMessage: 'No currencies match',
    description: 'Shown in the currency list when no currency matches what was typed.',
  },
  noFilters: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.noFilters',
    defaultMessage: 'This meter has no filters.',
    description: 'Shown under Filters of a usage price whose meter lists no fields to filter on.',
  },
  noMeters: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.noMeters',
    defaultMessage: 'Create a meter in Settings › Meters to charge for usage.',
  },
  paidHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.paidHelper',
    defaultMessage:
      'Subscribers pay through each API Portal the plan is published to. An API Portal gets this pricing when the plan is first published there.',
  },
  paidLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.paidLabel',
    defaultMessage: 'Paid plan',
  },
  per: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.per',
    defaultMessage: 'per',
    description:
      'Between a unit price and its batch size, e.g. "$ 0.01 per 1 request" or "$ 2.50 per 1000000 tokens".',
  },
  periodMonth: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.period.month',
    defaultMessage: 'Month',
  },
  periodYear: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.period.year',
    defaultMessage: 'Year',
  },
  priceTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.priceTitle',
    defaultMessage: 'Price {position}',
    description: 'Heading of one usage price of a plan, numbered from 1.',
  },
  pricingFlatRate: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.pricing.flatRate',
    defaultMessage: 'Flat rate',
  },
  pricingLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.pricing',
    defaultMessage: 'Pricing',
    description: 'Whether a paid plan charges a flat rate or for usage.',
  },
  pricingUsageBased: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.pricing.usageBased',
    defaultMessage: 'Usage based',
  },
  pricesLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.pricesLabel',
    defaultMessage: 'Prices',
  },
  problemAmount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.amount',
    defaultMessage: 'Enter the unit price of price {position} as an amount such as 0.01.',
  },
  problemFlatAmount: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.flatAmount',
    defaultMessage: 'Enter the flat rate as an amount above zero, such as 20.00.',
  },
  problemMeterMissing: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.meterMissing',
    defaultMessage: 'Pick the meter that counts the units of price {position}.',
  },
  problemOverlap: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.overlap',
    defaultMessage:
      'Prices {position} and {other} both count the same requests of the meter {meter}; give them different filters.',
  },
  problemFilterNotAllowed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.filterNotAllowed',
    defaultMessage: 'Price {position} filters on {field}, which the meter {meter} does not allow.',
    description:
      '{meter} is the meter\'s id; {field} is the name of a filter field, such as "HTTP method".',
  },
  problemTooManyFilters: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.tooManyFilters',
    defaultMessage: 'Price {position} has more than 20 filters; empty some of them.',
  },
  problemTokenKey: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.tokenKey',
    defaultMessage: 'Price {position}: the meter has no Token field with the key {key}.',
  },
  problemTokenRange: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.tokenRange',
    defaultMessage: "Price {position}: the conditions on {field} can't all be true.",
    description:
      '{field} names a Token field, e.g. "Token · Prompt tokens"; its two conditions leave no number, e.g. more than 500 and at most 100.',
  },
  problemTokenValue: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.tokenValue',
    defaultMessage:
      'Price {position}: the value of {field} must be a whole number from 0 to 1,000,000,000,000.',
    description: '{field} names a Token field, e.g. "Token · Prompt tokens".',
  },
  problemNoPrices: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.noPrices',
    defaultMessage: 'Add at least one price.',
  },
  problemQuantity: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.quantity',
    defaultMessage: 'The units of price {position} must be a whole number from 1 to 1,000,000,000.',
  },
  problemTiers: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.tiers',
    defaultMessage:
      'Each tier of price {position} needs a unit price, a flat fee or both; upper bounds must increase, and the last tier has none.',
  },
  problemValues: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.problem.values',
    defaultMessage:
      'Check the filters of price {position}: one value for equals, does not equal and starts with, status codes such as 200, and a resource starting with /.',
  },
  quantityLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.quantity',
    defaultMessage: 'Units per price',
  },
  removePrice: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.removePrice',
    defaultMessage: 'Remove price {position}',
  },
  removeCondition: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.removeCondition',
    defaultMessage: 'Remove this condition on {field}',
    description: 'Accessible name of the button removing the second condition on a Token field.',
  },
  removeTier: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.removeTier',
    defaultMessage: 'Remove tier',
  },
  resourcePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.resourcePlaceholder',
    defaultMessage: "{prefix, select, true {/orders} other {/orders/'{id}'}}",
    description:
      'Example resource path from an API definition, or the start of one. Do not translate.',
  },
  statusPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.statusPlaceholder',
    defaultMessage: '{many, select, true {200, 201} other {200}}',
    description: 'Example status codes. Do not translate.',
  },
  tierFlat: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.tierFlat',
    defaultMessage: 'Flat fee',
  },
  tierFrom: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.tierFrom',
    defaultMessage: '{start} –',
    description: 'Start of a tier\'s unit range, before its upper bound, e.g. "1001 –".',
  },
  tierUnit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.tierUnit',
    defaultMessage: 'Per unit',
  },
  tierUnits: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.tierUnits',
    defaultMessage: 'Units',
  },
  tierUpTo: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.tierUpTo',
    defaultMessage: 'Up to',
  },
  unitPrice: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.unitPrice',
    defaultMessage: 'Unit price',
  },
  units: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.PlanPricingSection.units',
    defaultMessage:
      '{unit, select, request {{quantity, plural, one {request} other {requests}}} token {{quantity, plural, one {token} other {tokens}}} other {{quantity, plural, one {unit} other {units}}}}',
    description:
      'Follows the batch size of a unit price, in what the chosen meter counts, e.g. "per 1 request" or "per 1000000 tokens"; "unit" until a meter is chosen.',
  },
});

const TYPE_MESSAGES: Record<PricingType, MessageDescriptor> = {
  FLAT_RATE: messages.pricingFlatRate,
  USAGE_BASED: messages.pricingUsageBased,
};

const MODEL_MESSAGES: Record<PricingModel, MessageDescriptor> = {
  GRADUATED: messages.modelGraduated,
  PER_UNIT: messages.modelPerUnit,
  VOLUME: messages.modelVolume,
};

/** The problem as one sentence, naming the price at fault by its position. */
export const problemMessage = (intl: IntlShape, problem: PricingProblem): string => {
  switch (problem.code) {
    case 'NO_PRICES':
      return intl.formatMessage(messages.problemNoPrices);
    case 'FLAT_AMOUNT_INVALID':
      return intl.formatMessage(messages.problemFlatAmount);
    case 'METER_MISSING':
      return intl.formatMessage(messages.problemMeterMissing, { position: problem.price + 1 });
    case 'FILTER_INVALID':
      return intl.formatMessage(messages.problemValues, { position: problem.price + 1 });
    case 'FILTER_NOT_ALLOWED':
      return intl.formatMessage(messages.problemFilterNotAllowed, {
        field: intl.formatMessage(fieldMessages[problem.field]),
        meter: problem.meterId,
        position: problem.price + 1,
      });
    case 'AMOUNT_INVALID':
      return intl.formatMessage(messages.problemAmount, { position: problem.price + 1 });
    case 'QUANTITY_INVALID':
      return intl.formatMessage(messages.problemQuantity, { position: problem.price + 1 });
    case 'TIERS_INVALID':
      return intl.formatMessage(messages.problemTiers, { position: problem.price + 1 });
    case 'TOO_MANY_FILTERS':
      return intl.formatMessage(messages.problemTooManyFilters, { position: problem.price + 1 });
    case 'TOKEN_KEY_UNKNOWN':
      return intl.formatMessage(messages.problemTokenKey, {
        key: problem.eventKey,
        position: problem.price + 1,
      });
    case 'TOKEN_VALUE_INVALID':
      return intl.formatMessage(messages.problemTokenValue, {
        field: tokenFieldLabel(intl, problem.eventKey),
        position: problem.price + 1,
      });
    case 'TOKEN_RANGE_EMPTY':
      return intl.formatMessage(messages.problemTokenRange, {
        field: tokenFieldLabel(intl, problem.eventKey),
        position: problem.price + 1,
      });
    case 'OVERLAP':
      return intl.formatMessage(messages.problemOverlap, {
        meter: problem.meterId,
        other: problem.other + 1,
        position: problem.price + 1,
      });
  }
};

/** The first unit of each tier: 1, then one past the previous tier's upper bound. */
const tierStarts = (tiers: TierDraft[]): string[] =>
  tiers.map((_, index) => {
    if (index === 0) return '1';
    const previous = Number(tiers[index - 1].upTo.trim());
    return Number.isSafeInteger(previous) && previous > 0 ? String(previous + 1) : '…';
  });

const splitText = (text: string): string[] =>
  text
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean);

type MoneyProps = {
  label: MessageDescriptor;
  value: string;
  onChange: (next: string) => void;
  error?: boolean;
  disabled?: boolean;
};

function MoneyInput({ label, value, onChange, error, disabled }: MoneyProps) {
  const intl = useIntl();
  return (
    <OutlinedInput
      disabled={disabled}
      error={error}
      inputProps={{ 'aria-label': intl.formatMessage(label), inputMode: 'decimal' }}
      onChange={(event) => onChange(event.target.value)}
      placeholder="0.00"
      size="small"
      sx={{ fontFamily: 'monospace', width: MONEY_WIDTH }}
      value={value}
    />
  );
}

type PriceFieldsProps = {
  price: PriceDraft;
  index: number;
  meters: Meter[];
  problem: PricingProblem | null;
  disabled?: boolean;
  /** Shown, not changed: no price or tier is added or removed. */
  readOnly?: boolean;
  onChange: (patch: Partial<PriceDraft>) => void;
  onRemove: () => void;
  /** False for the only price: a usage based plan needs one. */
  removable?: boolean;
};

/**
 * One usage price: the meter and what it counts, a filter row for each field
 * the meter allows to narrow which of its requests the price counts, and the
 * pricing model with its amounts.
 */
function PriceFields({
  price,
  index,
  meters,
  problem,
  disabled,
  readOnly,
  onChange,
  onRemove,
  removable = true,
}: PriceFieldsProps) {
  const intl = useIntl();
  const id = `plan-price-${price.key}`;
  const position = index + 1;
  const meter = meters.find((candidate) => candidate.id === price.meterId);
  const fault = problem && 'price' in problem && problem.price === index ? problem : null;
  const setTier = (key: number, patch: Partial<TierDraft>) =>
    onChange({
      tiers: price.tiers.map((tier) => (tier.key === key ? { ...tier, ...patch } : tier)),
    });
  const allowed = priceFieldsOf(meter);
  const rows = filterRows(price.filters, allowed);
  // Rows are changed by position: a Token key can have two condition rows.
  const setFilter = (at: number, patch: Partial<FilterDraft>) =>
    onChange({
      filters: rows.map((row, position) => (position === at ? { ...row, ...patch } : row)),
    });
  const addCondition = (at: number) =>
    onChange({
      filters: [
        ...rows.slice(0, at + 1),
        emptyFilterRow({ eventKey: rows[at].eventKey, field: 'TOKEN' }),
        ...rows.slice(at + 1),
      ],
    });
  const removeCondition = (at: number) =>
    onChange({ filters: rows.filter((_, position) => position !== at) });
  const starts = tierStarts(price.tiers);

  // The new meter's rows keep the operator and values of a filter on a field (or Token key) it
  // also allows.
  const changeMeter = (picked: Meter) => {
    const next = priceFieldsOf(picked);
    onChange({
      filters: filterRows(price.filters, next).filter((row) => isAllowed(row, next)),
      meterId: picked.id ?? '',
    });
  };

  const valuesInput = (filter: FilterDraft, at: number, invalid: boolean) => {
    const many = !takesOneValue(filter.operator);
    const label = intl.formatMessage(messages.filterValues);
    if (filter.field === 'HTTP_METHOD') {
      return (
        <Select
          disabled={disabled}
          displayEmpty
          error={invalid}
          inputProps={{ 'aria-label': label }}
          multiple={many}
          onChange={(event) => {
            const value = event.target.value as string | string[];
            setFilter(at, { text: Array.isArray(value) ? value.join(', ') : value });
          }}
          renderValue={(selected) => {
            const picked = Array.isArray(selected) ? selected : selected ? [selected] : [];
            if (picked.length > 0) return picked.join(', ');
            return intl.formatMessage(many ? messages.methodsPlaceholder : messages.any);
          }}
          size="small"
          value={many ? splitText(filter.text) : filter.text}
        >
          {!many && (
            <MenuItem value="">
              <FormattedMessage {...messages.any} />
            </MenuItem>
          )}
          {HTTP_METHODS.map((method) => (
            <MenuItem key={method} value={method}>
              {method}
            </MenuItem>
          ))}
        </Select>
      );
    }
    if (filter.field === 'API_TYPE') {
      // Our gateway's API types are suggested; any text is accepted.
      const listed = splitText(filter.text);
      return (
        <Autocomplete<string, false, false, true>
          disabled={disabled}
          // Every suggestion not listed yet, whatever is typed: there are only six.
          filterOptions={(options) =>
            many ? options.filter((option) => !listed.includes(option)) : options
          }
          freeSolo
          inputValue={filter.text}
          // A pick replaces the one value, or adds to several.
          onChange={(_event, value, reason) => {
            if (reason !== 'selectOption' || !value) return;
            setFilter(at, { text: many ? [...listed, value].join(', ') : value });
          }}
          onInputChange={(_event, value, reason) => {
            if (reason === 'input' || reason === 'clear') setFilter(at, { text: value });
          }}
          options={API_TYPES}
          renderInput={({ InputLabelProps, InputProps, inputProps, ...params }) => (
            <TextField
              {...params}
              error={invalid}
              placeholder={intl.formatMessage(messages.apiTypePlaceholder, { many: String(many) })}
              slotProps={{
                htmlInput: { ...inputProps, 'aria-label': label },
                input: { ...InputProps, sx: { fontFamily: 'monospace' } },
                inputLabel: InputLabelProps,
              }}
            />
          )}
          size="small"
          value={null}
        />
      );
    }
    if (filter.field === 'TOKEN') {
      // One whole number, such as a token count.
      return (
        <OutlinedInput
          disabled={disabled}
          error={invalid}
          inputProps={{ 'aria-label': label, inputMode: 'numeric' }}
          onChange={(event) => setFilter(at, { text: event.target.value })}
          placeholder="0"
          size="small"
          sx={{ fontFamily: 'monospace' }}
          value={filter.text}
        />
      );
    }
    return (
      <OutlinedInput
        disabled={disabled}
        error={invalid}
        inputProps={{ 'aria-label': label }}
        onChange={(event) => setFilter(at, { text: event.target.value })}
        placeholder={
          filter.field === 'RESOURCE'
            ? intl.formatMessage(messages.resourcePlaceholder, {
                prefix: String(filter.operator === 'STARTS_WITH'),
              })
            : intl.formatMessage(messages.statusPlaceholder, { many: String(many) })
        }
        size="small"
        sx={{ fontFamily: 'monospace' }}
        value={filter.text}
      />
    );
  };

  return (
    <Box
      aria-labelledby={`${id}-title`}
      role="group"
      sx={(theme) => ({ border: hairline(theme), borderColor: 'divider', borderRadius: 1, p: 1.5 })}
    >
      <Stack spacing={1.5}>
        <Stack alignItems="center" direction="row" spacing={1}>
          <Typography id={`${id}-title`} sx={{ flexShrink: 0, fontWeight: 600 }} variant="body2">
            <FormattedMessage {...messages.priceTitle} values={{ position }} />
          </Typography>
          <Typography color="text.secondary" noWrap sx={{ flex: 1, minWidth: 0 }} variant="body2">
            {price.meterId ? priceLabel(price, meter) : null}
          </Typography>
          {!readOnly && removable && (
            <IconButton
              aria-label={intl.formatMessage(messages.removePrice, { position })}
              disabled={disabled}
              onClick={onRemove}
              size="small"
            >
              <X size={16} />
            </IconButton>
          )}
        </Stack>

        <Box sx={{ display: 'grid', gap: 1.5, gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
          <FormControl error={fault?.code === 'METER_MISSING'} fullWidth>
            <FormLabel id={`${id}-meter-label`}>
              <FormattedMessage {...messages.meterLabel} />
            </FormLabel>
            <MeterPicker
              disabled={disabled}
              error={fault?.code === 'METER_MISSING'}
              labelId={`${id}-meter-label`}
              meter={meter}
              meterId={price.meterId}
              onChange={changeMeter}
            />
          </FormControl>
          <FormControl fullWidth>
            <FormLabel htmlFor={`${id}-event`}>
              <FormattedMessage {...messages.eventLabel} />
            </FormLabel>
            <ReadOnlyField
              id={`${id}-event`}
              label={intl.formatMessage(messages.eventLabel)}
              value={meter ? typeLabel(intl, meter) : ''}
            />
          </FormControl>
        </Box>

        {price.meterId && (
          <Stack spacing={1}>
            <Box>
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.filtersLabel} />
              </Typography>
              {rows.length > 0 && (
                <Typography color="text.secondary" display="block" variant="caption">
                  <FormattedMessage {...messages.filtersHint} />
                </Typography>
              )}
            </Box>
            {meter && allowed.length === 0 && (
              <Typography color="text.secondary" variant="caption">
                <FormattedMessage {...messages.noFilters} />
              </Typography>
            )}
            {rows.map((filter, at) => {
              const token = filter.field === 'TOKEN';
              const onKey = (row: FilterDraft) =>
                row.field === 'TOKEN' && row.eventKey === filter.eventKey;
              const tokenFault =
                token && fault && 'eventKey' in fault && fault.eventKey === filter.eventKey
                  ? fault
                  : null;
              const invalid =
                (fault?.code === 'FILTER_INVALID' && fault.field === filter.field) ||
                tokenFault?.code === 'TOKEN_VALUE_INVALID' ||
                tokenFault?.code === 'TOKEN_RANGE_EMPTY';
              const refused =
                (fault?.code === 'FILTER_NOT_ALLOWED' && fault.field === filter.field) ||
                tokenFault?.code === 'TOKEN_KEY_UNKNOWN';
              const name = token
                ? tokenFieldLabel(intl, filter.eventKey ?? '')
                : intl.formatMessage(fieldMessages[filter.field]);
              // A Token key's first row can add a second condition; the second can be removed.
              const firstOnKey = token && rows.findIndex(onKey) === at;
              const nameId = `${id}-filter-${at}`;
              return (
                <Box
                  aria-labelledby={nameId}
                  key={token ? `TOKEN:${filter.eventKey}:${at}` : filter.field}
                  role="group"
                  sx={{
                    alignItems: 'center',
                    display: 'grid',
                    gap: 1,
                    gridTemplateColumns: FILTER_COLUMNS,
                  }}
                >
                  <Typography
                    color={refused ? 'error' : 'text.primary'}
                    id={nameId}
                    noWrap
                    title={name}
                    variant="body2"
                  >
                    {name}
                  </Typography>
                  <Select
                    disabled={disabled}
                    inputProps={{ 'aria-label': intl.formatMessage(messages.filterOperator) }}
                    onChange={(event) => {
                      const operator = event.target.value as FilterOperator;
                      // A one-value operator keeps only the first method picked.
                      setFilter(at, {
                        operator,
                        text:
                          filter.field === 'HTTP_METHOD' && takesOneValue(operator)
                            ? (splitText(filter.text)[0] ?? '')
                            : filter.text,
                      });
                    }}
                    size="small"
                    value={filter.operator}
                  >
                    {FILTER_RULES[filter.field].operators.map((operator) => (
                      <MenuItem key={operator} value={operator}>
                        <FormattedMessage {...operatorMessage(filter.field, operator)} />
                      </MenuItem>
                    ))}
                  </Select>
                  {valuesInput(filter, at, invalid || refused)}
                  {token &&
                  !readOnly &&
                  firstOnKey &&
                  rows.filter(onKey).length < MAX_TOKEN_CONDITIONS ? (
                    <IconButton
                      aria-label={intl.formatMessage(messages.addCondition, { field: name })}
                      disabled={disabled}
                      onClick={() => addCondition(at)}
                      size="small"
                    >
                      <Plus size={16} />
                    </IconButton>
                  ) : token && !readOnly && !firstOnKey ? (
                    <IconButton
                      aria-label={intl.formatMessage(messages.removeCondition, { field: name })}
                      disabled={disabled}
                      onClick={() => removeCondition(at)}
                      size="small"
                    >
                      <X size={16} />
                    </IconButton>
                  ) : (
                    <span />
                  )}
                </Box>
              );
            })}
          </Stack>
        )}

        <Box sx={{ display: 'grid', gap: 1.5, gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }}>
          <FormControl fullWidth>
            <FormLabel id={`${id}-model-label`}>
              <FormattedMessage {...messages.modelLabel} />
            </FormLabel>
            <Select
              disabled={disabled}
              labelId={`${id}-model-label`}
              onChange={(event) => {
                const pricingModel = event.target.value as PricingModel;
                onChange({
                  pricingModel,
                  tiers:
                    isTiered(pricingModel) && price.tiers.length === 0
                      ? [newTier(), newTier()]
                      : price.tiers,
                });
              }}
              size="small"
              value={price.pricingModel}
            >
              {PRICING_MODELS.map((model) => (
                <MenuItem key={model} value={model}>
                  <FormattedMessage {...MODEL_MESSAGES[model]} />
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>

        {price.pricingModel === 'PER_UNIT' && (
          <Stack alignItems="center" direction="row" spacing={1}>
            <MoneyInput
              disabled={disabled}
              error={fault?.code === 'AMOUNT_INVALID'}
              label={messages.unitPrice}
              onChange={(unitAmount) => onChange({ unitAmount })}
              value={price.unitAmount}
            />
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.per} />
            </Typography>
            <OutlinedInput
              disabled={disabled}
              error={fault?.code === 'QUANTITY_INVALID'}
              inputProps={{
                'aria-label': intl.formatMessage(messages.quantityLabel),
                inputMode: 'numeric',
              }}
              onChange={(event) => onChange({ unitQuantity: event.target.value })}
              size="small"
              sx={{ fontFamily: 'monospace', width: 120 }}
              value={price.unitQuantity}
            />
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage
                {...messages.units}
                values={{
                  quantity: Number(price.unitQuantity) || 1,
                  unit: meter ? meterUnit(meter) : 'unit',
                }}
              />
            </Typography>
          </Stack>
        )}

        {isTiered(price.pricingModel) && (
          <Stack spacing={1}>
            <Box sx={{ display: 'grid', gap: 1, gridTemplateColumns: TIER_COLUMNS }}>
              <Typography color="text.secondary" variant="caption">
                <FormattedMessage {...messages.tierUnits} />
              </Typography>
              <span />
              <Typography color="text.secondary" variant="caption">
                <FormattedMessage {...messages.tierUnit} />
              </Typography>
              <span />
              <Typography color="text.secondary" variant="caption">
                <FormattedMessage {...messages.tierFlat} />
              </Typography>
              <span />
            </Box>
            {price.tiers.map((tier, tierIndex) => {
              const last = tierIndex === price.tiers.length - 1;
              return (
                <Box
                  key={tier.key}
                  sx={{
                    alignItems: 'center',
                    display: 'grid',
                    gap: 1,
                    gridTemplateColumns: TIER_COLUMNS,
                  }}
                >
                  <OutlinedInput
                    disabled={disabled || last}
                    inputProps={{
                      'aria-label': intl.formatMessage(messages.tierUpTo),
                      inputMode: 'numeric',
                    }}
                    onChange={(event) => setTier(tier.key, { upTo: event.target.value })}
                    size="small"
                    startAdornment={
                      <InputAdornment position="start">
                        {intl.formatMessage(messages.tierFrom, { start: starts[tierIndex] })}
                      </InputAdornment>
                    }
                    sx={{ fontFamily: 'monospace' }}
                    value={last ? '∞' : tier.upTo}
                  />
                  <Box aria-hidden color="text.secondary" sx={{ display: 'flex' }}>
                    <ArrowRight size={16} />
                  </Box>
                  <MoneyInput
                    disabled={disabled}
                    label={messages.tierUnit}
                    onChange={(unitAmount) => setTier(tier.key, { unitAmount })}
                    value={tier.unitAmount}
                  />
                  <Box aria-hidden color="text.secondary" sx={{ display: 'flex' }}>
                    <Plus size={14} />
                  </Box>
                  <MoneyInput
                    disabled={disabled}
                    label={messages.tierFlat}
                    onChange={(flatAmount) => setTier(tier.key, { flatAmount })}
                    value={tier.flatAmount}
                  />
                  {tierIndex > 0 && !readOnly ? (
                    <IconButton
                      aria-label={intl.formatMessage(messages.removeTier)}
                      disabled={disabled}
                      onClick={() =>
                        onChange({ tiers: price.tiers.filter((t) => t.key !== tier.key) })
                      }
                      size="small"
                    >
                      <X size={16} />
                    </IconButton>
                  ) : (
                    <span />
                  )}
                </Box>
              );
            })}
            {!readOnly && (
              <Box>
                <Button
                  disabled={disabled || price.tiers.length >= MAX_TIERS}
                  onClick={() => onChange({ tiers: [...price.tiers, newTier()] })}
                  size="small"
                  startIcon={<Plus size={16} />}
                  variant="text"
                >
                  <FormattedMessage {...messages.addTier} />
                </Button>
              </Box>
            )}
          </Stack>
        )}
      </Stack>
    </Box>
  );
}

export type PlanPricingSectionProps = {
  value: PricingDraft;
  onChange: (next: PricingDraft) => void;
  /** The meters the prices use, once read; a price's meter is picked by searching (MeterPicker). */
  meters: Meter[];
  /** Whether the organization has any meter, so a usage price can be added. */
  hasMeters: boolean;
  /** The problem to show, once the user has tried to save. */
  problem: PricingProblem | null;
  disabled?: boolean;
  /** An existing plan's pricing, which is set when the plan is created: shown, not changed. */
  readOnly?: boolean;
};

/**
 * The plan dialog's Paid plan section: the currency and billing period, then
 * either a flat rate's amount or the usage prices, each on a meter.
 */
export function PlanPricingSection({
  value,
  onChange,
  meters,
  hasMeters,
  problem,
  disabled: busy,
  readOnly,
}: PlanPricingSectionProps) {
  const intl = useIntl();
  const disabled = busy || readOnly;
  // A saved currency that is not in the list is still offered, so it shows as selected.
  const currencies = useMemo<readonly Currency[]>(
    () =>
      CURRENCIES.some((currency) => currency.code === value.currency)
        ? CURRENCIES
        : [{ code: value.currency, name: '' }, ...CURRENCIES],
    [value.currency],
  );
  const set = (patch: Partial<PricingDraft>) => onChange({ ...value, ...patch });
  const setPrice = (key: number, patch: Partial<PriceDraft>) =>
    set({
      prices: value.prices.map((price) => (price.key === key ? { ...price, ...patch } : price)),
    });
  // A saved plan's prices are listed by name; prices being entered keep the order they were
  // added in.
  const prices = readOnly ? pricesByName(value.prices, meters) : value.prices;

  return (
    <Stack spacing={2}>
      <FormControl fullWidth>
        <FormControlLabel
          control={
            <Switch
              checked={value.paid}
              disabled={disabled}
              onChange={(event) => set({ paid: event.target.checked })}
            />
          }
          label={<FormattedMessage {...messages.paidLabel} />}
          labelPlacement="start"
          sx={{ justifyContent: 'space-between', ml: 0 }}
        />
        <FormHelperText>
          <FormattedMessage {...(readOnly ? messages.lockedHint : messages.paidHelper)} />
        </FormHelperText>
        {value.paid && !readOnly && (
          <FormHelperText>
            <FormattedMessage {...messages.createHint} />
          </FormHelperText>
        )}
      </FormControl>

      {value.paid && (
        <Stack spacing={2}>
          <Box sx={{ display: 'grid', gap: 1.5, gridTemplateColumns: 'repeat(3, minmax(0, 1fr))' }}>
            <FormControl fullWidth>
              <FormLabel htmlFor={CURRENCY_FIELD} id={`${CURRENCY_FIELD}-label`}>
                <FormattedMessage {...messages.currencyLabel} />
              </FormLabel>
              <Autocomplete
                disableClearable
                disabled={disabled}
                getOptionLabel={(currency) =>
                  currency.name ? `${currency.code} — ${currency.name}` : currency.code
                }
                id={CURRENCY_FIELD}
                isOptionEqualToValue={(option, selected) => option.code === selected.code}
                noOptionsText={intl.formatMessage(messages.noCurrencies)}
                onChange={(_event, next) => set({ currency: next.code })}
                options={currencies}
                renderInput={(params) => <TextField {...params} size="small" />}
                size="small"
                value={currencies.find((currency) => currency.code === value.currency)!}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel id={`${PERIOD_FIELD}-label`}>
                <FormattedMessage {...messages.chargeEvery} />
              </FormLabel>
              <Select
                disabled={disabled}
                labelId={`${PERIOD_FIELD}-label`}
                onChange={(event) =>
                  set({ billingPeriod: event.target.value as PricingDraft['billingPeriod'] })
                }
                size="small"
                value={value.billingPeriod}
              >
                <MenuItem value="MONTH">
                  <FormattedMessage {...messages.periodMonth} />
                </MenuItem>
                <MenuItem value="YEAR">
                  <FormattedMessage {...messages.periodYear} />
                </MenuItem>
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <FormLabel id={`${TYPE_FIELD}-label`}>
                <FormattedMessage {...messages.pricingLabel} />
              </FormLabel>
              <Select
                disabled={disabled}
                labelId={`${TYPE_FIELD}-label`}
                onChange={(event) => {
                  const pricingType = event.target.value as PricingType;
                  // A usage based plan starts with one price to fill in.
                  set(
                    pricingType === 'USAGE_BASED' && value.prices.length === 0 && hasMeters
                      ? { pricingType, prices: [newPrice()] }
                      : { pricingType },
                  );
                }}
                size="small"
                value={value.pricingType}
              >
                {PRICING_TYPES.map((type) => (
                  <MenuItem key={type} value={type}>
                    <FormattedMessage {...TYPE_MESSAGES[type]} />
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Box>

          {value.pricingType === 'FLAT_RATE' ? (
            <Box
              sx={{ display: 'grid', gap: 1.5, gridTemplateColumns: 'repeat(3, minmax(0, 1fr))' }}
            >
              <FormControl error={problem?.code === 'FLAT_AMOUNT_INVALID'} fullWidth>
                <FormLabel htmlFor={AMOUNT_FIELD}>
                  <FormattedMessage {...messages.amountLabel} />
                </FormLabel>
                <OutlinedInput
                  disabled={disabled}
                  error={problem?.code === 'FLAT_AMOUNT_INVALID'}
                  id={AMOUNT_FIELD}
                  inputProps={{ inputMode: 'decimal' }}
                  onChange={(event) => set({ flatAmount: event.target.value })}
                  placeholder="0.00"
                  size="small"
                  sx={{ fontFamily: 'monospace' }}
                  value={value.flatAmount}
                />
              </FormControl>
            </Box>
          ) : (
            <Stack spacing={1.5}>
              <Stack alignItems="center" direction="row" justifyContent="space-between">
                <Form.Header sx={{ fontWeight: 600, typography: 'body2' }}>
                  <FormattedMessage {...messages.pricesLabel} />
                </Form.Header>
                {!readOnly && (
                  <Button
                    disabled={disabled || !hasMeters || value.prices.length >= MAX_PRICES}
                    onClick={() => set({ prices: [...value.prices, newPrice()] })}
                    size="small"
                    startIcon={<Plus size={16} />}
                    variant="outlined"
                  >
                    <FormattedMessage {...messages.addPrice} />
                  </Button>
                )}
              </Stack>
              {!hasMeters && !readOnly && (
                <Typography color="text.secondary" variant="caption">
                  <FormattedMessage {...messages.noMeters} />
                </Typography>
              )}
              {prices.map((price, index) => (
                <PriceFields
                  disabled={disabled}
                  index={index}
                  key={price.key}
                  meters={meters}
                  onChange={(patch) => setPrice(price.key, patch)}
                  onRemove={() =>
                    set({ prices: value.prices.filter((other) => other.key !== price.key) })
                  }
                  price={price}
                  problem={problem}
                  readOnly={readOnly}
                  removable={value.prices.length > 1}
                />
              ))}
            </Stack>
          )}

          {problem && (
            <Typography color="error" variant="caption">
              {problemMessage(intl, problem)}
            </Typography>
          )}
        </Stack>
      )}
    </Stack>
  );
}

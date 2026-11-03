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
import { defineMessages, type IntlShape, type MessageDescriptor } from 'react-intl';

import type { Meter } from '@/api/resources/meters';
import type { PriceFilter } from '@/api/resources/subscriptionPlans';

/**
 * What meters count and the properties plan prices filter on, with their labels:
 *   - A meter counts requests, or adds up the number at one event key of AI
 *     API requests, and lists the properties its prices may filter on: each a
 *     name the administrator gives and the event key it reads.
 *   - The meter form offers our gateway's properties, whose names and keys fill
 *     in a new property and stay editable.
 *   - Each plan price that uses a meter gives its own filters on those
 *     properties. Any operator works on any property.
 *
 * The Platform API checks meters and prices with the API Portal's rules; these
 * mirror them so the forms offer only what is accepted.
 */

export type MeterType = Meter['type'] & string;
export type FilterOperator = PriceFilter['operator'];

/** What a meter counts: requests, each counting one, or the tokens of AI API requests. */
export const METER_TYPES: MeterType[] = ['REQUEST_COUNT', 'TOKENS'];

/** The event key a new Tokens meter starts with: the prompt's tokens. */
export const DEFAULT_TOKEN_KEY = 'metadata.aiTokenUsage.promptTokens';

/** At most this many properties in one meter. */
export const MAX_METER_FIELDS = 10;
/** The longest name a property may have, once trimmed. */
const MAX_NAME_LENGTH = 40;
/** At most this many comparisons on one property in one price, such as more than 100 and at most 500. */
export const MAX_COMPARISONS = 2;
/** The largest number a comparison takes. */
const MAX_COMPARE_VALUE = 1_000_000_000_000;
/** At most this many values in one filter, each at most this long. */
const MAX_VALUES = 20;
const MAX_VALUE_LENGTH = 255;

/** Every operator, in the order the forms offer them. */
export const OPERATORS: FilterOperator[] = [
  'EQUALS',
  'NOT_EQUALS',
  'IN',
  'NOT_IN',
  'STARTS_WITH',
  'GREATER_THAN',
  'GREATER_THAN_OR_EQUAL',
  'LESS_THAN',
  'LESS_THAN_OR_EQUAL',
];

/** The operators that compare a whole number; the others match text. */
export const COMPARISONS: FilterOperator[] = [
  'GREATER_THAN',
  'GREATER_THAN_OR_EQUAL',
  'LESS_THAN',
  'LESS_THAN_OR_EQUAL',
];

export const isComparison = (operator: FilterOperator): boolean => COMPARISONS.includes(operator);

/** Is one of and is not one of take several values; every other operator takes one. */
export const takesOneValue = (operator: FilterOperator): boolean =>
  operator !== 'IN' && operator !== 'NOT_IN';

/**
 * An event key as the API Portal accepts it, once trimmed: 1 to 255 characters, with no spaces or
 * control characters.
 */
export const isEventKey = (key: string): boolean => /^[^\s\p{Cc}]{1,255}$/u.test(key.trim());

/** A property's name, once trimmed: 1 to 40 characters, with no control characters. */
export const isPropertyName = (name: string): boolean => {
  const trimmed = name.trim();
  return trimmed.length > 0 && [...trimmed].length <= MAX_NAME_LENGTH && !/\p{Cc}/u.test(trimmed);
};

/** Whether two property names are the same, ignoring case: "Region" and "region" are. */
export const sameName = (a: string, b: string): boolean =>
  a.trim().toLowerCase() === b.trim().toLowerCase();

/** The names of the properties a meter's prices may filter on, in the meter's order; none for a meter not known. */
export const propertiesOf = (meter: Meter | undefined): string[] =>
  (meter?.filterFields ?? []).map(({ field }) => field);

/**
 * The values of a filter as typed, or null when they are not valid:
 *   - equals, does not equal and starts with take one text value;
 *   - is one of and is not one of take 1 to 20 comma-separated text values, each once, in the order
 *     given;
 *   - a comparison takes one whole number from 0 to 1000000000000, written without leading zeros.
 * Text values are trimmed, 1 to 255 characters, and otherwise kept as written.
 */
export const parseFilterValues = (operator: FilterOperator, text: string): string[] | null => {
  if (isComparison(operator)) {
    const value = text.trim();
    return /^\d+$/.test(value) && Number(value) <= MAX_COMPARE_VALUE
      ? [String(Number(value))]
      : null;
  }
  const values = takesOneValue(operator)
    ? [text.trim()].filter(Boolean)
    : text
        .split(',')
        .map((value) => value.trim())
        .filter(Boolean);
  if (values.length === 0 || values.length > MAX_VALUES) return null;
  return values.every((value) => value.length <= MAX_VALUE_LENGTH) ? [...new Set(values)] : null;
};

export const meterTypeMessages: Record<MeterType, MessageDescriptor> = defineMessages({
  REQUEST_COUNT: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.type.requestCount',
    defaultMessage: 'Requests',
  },
  TOKENS: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.type.tokens',
    defaultMessage: 'Tokens',
  },
});

const presetMessages = defineMessages({
  statusCode: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.preset.statusCode',
    defaultMessage: 'Status code',
    description:
      'A property our gateway reports for each request, offered as a filter property: the HTTP status code of the response.',
  },
  method: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.preset.method',
    defaultMessage: 'HTTP method',
    description:
      'A property our gateway reports for each request, offered as a filter property: its HTTP method, such as GET.',
  },
  resource: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.preset.resource',
    defaultMessage: 'Resource',
    description:
      'A property our gateway reports for each request, offered as a filter property: the resource of the API definition it matched, such as /orders/{id}.',
  },
  apiType: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.preset.apiType',
    defaultMessage: 'API type',
    description:
      'A property our gateway reports for each request, offered as a filter property: the kind of API it went to, such as a REST API or an LLM provider.',
  },
  mcpMethod: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.preset.mcpMethod',
    defaultMessage: 'MCP method',
    description:
      'A property our gateway reports for each request, offered as a filter property: the JSON-RPC method of a request to an MCP server, such as tools/call. Keep "MCP".',
  },
  tool: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.preset.tool',
    defaultMessage: 'Tool',
    description:
      'A property our gateway reports for each request, offered as a filter property: the name of the MCP server tool a tool call runs.',
  },
  promptTokens: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.preset.promptTokens',
    defaultMessage: 'Prompt tokens',
    description:
      'A token count our gateway reports for each AI API request: the tokens of the prompt sent to the model.',
  },
  completionTokens: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.preset.completionTokens',
    defaultMessage: 'Completion tokens',
    description:
      'A token count our gateway reports for each AI API request: the tokens of the answer the model returns.',
  },
  totalTokens: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.preset.totalTokens',
    defaultMessage: 'Total tokens',
    description:
      'A token count our gateway reports for each AI API request: the prompt and completion tokens together.',
  },
});

/** A property our gateway reports: the name it is offered by, and the event key it reads. */
export type PropertyPreset = { name: MessageDescriptor; eventKey: string };

/**
 * The token counts our gateway reports, offered for a Tokens meter's Token key: the prompt's (its
 * default), the completion's, and both. Any other key may be named.
 */
export const TOKEN_KEYS: readonly PropertyPreset[] = [
  { eventKey: DEFAULT_TOKEN_KEY, name: presetMessages.promptTokens },
  { eventKey: 'metadata.aiTokenUsage.completionTokens', name: presetMessages.completionTokens },
  { eventKey: 'metadata.aiTokenUsage.totalTokens', name: presetMessages.totalTokens },
];

/**
 * Our gateway's properties, offered in this order when a property is added to a meter. Picking one
 * fills in its name and key, which both stay editable.
 */
export const PROPERTY_PRESETS: readonly PropertyPreset[] = [
  { eventKey: 'response.status', name: presetMessages.statusCode },
  { eventKey: 'request.verb', name: presetMessages.method },
  { eventKey: 'request.route', name: presetMessages.resource },
  { eventKey: 'metadata.apiType', name: presetMessages.apiType },
  { eventKey: 'metadata.mcpAnalytics.jsonRpcMethod', name: presetMessages.mcpMethod },
  { eventKey: 'metadata.mcpAnalytics.capabilityName', name: presetMessages.tool },
  ...TOKEN_KEYS,
];

export const operatorMessages: Record<FilterOperator, MessageDescriptor> = defineMessages({
  EQUALS: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.equals',
    defaultMessage: 'equals',
  },
  NOT_EQUALS: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.notEquals',
    defaultMessage: 'does not equal',
  },
  IN: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.in',
    defaultMessage: 'is one of',
  },
  NOT_IN: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.notIn',
    defaultMessage: 'is not one of',
  },
  STARTS_WITH: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.startsWith',
    defaultMessage: 'starts with',
    description:
      'A filter that matches the values beginning with the given text: /orders matches /orders/{id} and /orders-archive.',
  },
  GREATER_THAN: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.greaterThan',
    defaultMessage: 'more than',
  },
  GREATER_THAN_OR_EQUAL: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.greaterThanOrEqual',
    defaultMessage: 'at least',
  },
  LESS_THAN: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.lessThan',
    defaultMessage: 'less than',
  },
  LESS_THAN_OR_EQUAL: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.lessThanOrEqual',
    defaultMessage: 'at most',
  },
});

/** What a meter counts, in the meters list and beside it in a plan price: "Requests" or "Tokens". */
export const typeLabel = (intl: IntlShape, meter: Meter): string =>
  intl.formatMessage(meterTypeMessages[meter.type === 'TOKENS' ? 'TOKENS' : 'REQUEST_COUNT']);

/**
 * The names of the properties a meter's prices may filter on, such as "API type, HTTP method"; empty
 * when there are none.
 */
export const propertiesLabel = (meter: Meter): string => propertiesOf(meter).join(', ');

/** The unit a meter counts in, for prices: a request, or a token; the meter's name says which tokens. */
export const meterUnit = (meter: Meter): 'request' | 'token' =>
  meter.type === 'TOKENS' ? 'token' : 'request';

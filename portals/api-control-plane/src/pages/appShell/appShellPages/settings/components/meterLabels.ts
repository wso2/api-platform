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
 * What meters count and the filters plan prices narrow them with, with their
 * labels and the keys they read by default in the gateway's events:
 *   - A meter counts requests, or adds up the number at one event key of AI
 *     API requests, and lists the fields its prices may filter on, each with its
 *     event key.
 *   - The keys are stored on the meter and can be edited, so a gateway that names
 *     its analytics fields differently works too; our gateway's keys are the
 *     defaults.
 *   - Each plan price that uses a meter gives its own filters on those fields.
 *
 * The Platform API checks meters and prices with the API Portal's rules; these
 * mirror them so the forms offer only what is accepted.
 */

export type MeterType = Meter['type'] & string;
export type FilterField = PriceFilter['field'];
export type FilterOperator = PriceFilter['operator'];

/** What a meter counts: requests, each counting one, or the tokens of AI API requests. */
export const METER_TYPES: MeterType[] = ['REQUEST_COUNT', 'TOKENS'];

// Keep these default keys in step with the API Portal's
// (portals/api-portal/src/services/billing/meterValidation.js). They are full paths in our gateway's
// Moesif events.

/** The event key a new Tokens meter starts with: the prompt's tokens. */
export const DEFAULT_TOKEN_KEY = 'metadata.aiTokenUsage.promptTokens';

const MATCH: FilterOperator[] = ['EQUALS', 'NOT_EQUALS', 'IN', 'NOT_IN'];
/** A Token condition compares a token count with one whole number. */
const COMPARE: FilterOperator[] = [
  'GREATER_THAN',
  'GREATER_THAN_OR_EQUAL',
  'LESS_THAN',
  'LESS_THAN_OR_EQUAL',
  'EQUALS',
];

/**
 * The operators each filter field takes, and the key it reads in the gateway's
 * events unless the meter gives another. The MCP method is the JSON-RPC method
 * of a request to an MCP server, such as tools/call, and the tool the name of the
 * tool a tool call runs. Token, a number such as a token count, has no default
 * key: each Token field names its own, and a meter may list it once per key.
 */
export const FILTER_RULES: Record<FilterField, { operators: FilterOperator[]; eventKey: string }> =
  {
    HTTP_STATUS_CODE: { eventKey: 'response.status', operators: MATCH },
    HTTP_METHOD: { eventKey: 'request.verb', operators: MATCH },
    RESOURCE: {
      eventKey: 'metadata.apiResourceTemplate',
      operators: ['EQUALS', 'NOT_EQUALS', 'STARTS_WITH'],
    },
    API_TYPE: { eventKey: 'metadata.apiType', operators: MATCH },
    MCP_METHOD: { eventKey: 'metadata.mcpAnalytics.jsonRpcMethod', operators: MATCH },
    MCP_TOOL: { eventKey: 'metadata.mcpAnalytics.capabilityName', operators: MATCH },
    TOKEN: { eventKey: '', operators: COMPARE },
  };

/** At most this many fields in one meter, besides a Tokens meter's Token key (typeKey). */
export const MAX_METER_FIELDS = 10;
/** At most this many Token conditions on one key in one price, such as more than 100 and at most 500. */
export const MAX_TOKEN_CONDITIONS = 2;
/** The largest number a Token condition compares with. */
export const MAX_TOKEN_VALUE = 1_000_000_000_000;

/**
 * The API types our gateway reports, suggested for an API type filter; any
 * other value is accepted. Matched exactly as written.
 */
export const API_TYPES = ['RestApi', 'WebSubApi', 'LlmProvider', 'LlmProxy', 'Mcp', 'Agent'];

/** The MCP method of a tool call. */
export const MCP_TOOL_CALL = 'tools/call';

/**
 * MCP methods suggested for an MCP method filter; any other value is accepted.
 * Matched exactly as written.
 */
export const MCP_METHODS = [
  MCP_TOOL_CALL,
  'resources/read',
  'prompts/get',
  'tools/list',
  'resources/list',
  'prompts/list',
  'initialize',
  'ping',
];

/**
 * An event key as the API Portal accepts it, once trimmed: 1 to 255 characters, with no spaces or
 * control characters.
 */
export const isEventKey = (key: string): boolean => /^[^\s\p{Cc}]{1,255}$/u.test(key.trim());

/** Equals, does not equal and starts with take one value; is one of and is not one of take several. */
export const takesOneValue = (operator: FilterOperator): boolean =>
  operator !== 'IN' && operator !== 'NOT_IN';

export const FILTER_FIELDS = Object.keys(FILTER_RULES) as FilterField[];

/** The fields a meter lists at most once; Token may be listed once per key. */
export const FIXED_FIELDS = FILTER_FIELDS.filter((field) => field !== 'TOKEN');

export const HTTP_METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'];

/** A field a price may filter on: a fixed field, or Token with the key it reads. */
export type PriceField = { field: FilterField; eventKey?: string };

/**
 * The token keys a meter's prices may set conditions on: a Tokens meter's Token
 * key (typeKey) first, then the keys of its Token fields, in the meter's order.
 */
export const tokenKeysOf = (meter: Meter | undefined): string[] => [
  ...(meter?.type === 'TOKENS' && meter.typeKey ? [meter.typeKey] : []),
  ...(meter?.filterFields ?? [])
    .filter((entry) => entry.field === 'TOKEN')
    .map((entry) => entry.eventKey),
];

/**
 * The fields a meter's prices may filter on: its fixed fields in the meter's
 * order, then a Token field for each of its token keys; none for a meter not
 * known.
 */
export const priceFieldsOf = (meter: Meter | undefined): PriceField[] => [
  ...(meter?.filterFields ?? [])
    .filter((entry) => entry.field !== 'TOKEN')
    .map(({ field }) => ({ field })),
  ...tokenKeysOf(meter).map((eventKey) => ({ eventKey, field: 'TOKEN' as const })),
];

/** At most this many values in one filter, each at most this long. */
const MAX_VALUES = 20;
const MAX_VALUE_LENGTH = 255;
// One status code, from 100 to 599, as the API Portal accepts; not a class such as 4xx.
const STATUS_PATTERN = /^[1-5]\d\d$/;

/**
 * The values of a filter, as typed or picked, or null when they are not valid:
 * status codes and API types comma-separated, one resource starting
 * with /, methods as picked (comma-separated here too). Equals, does not equal
 * and starts with take exactly one value.
 */
export const parseFilterValues = (
  field: FilterField,
  operator: FilterOperator,
  text: string,
): string[] | null => {
  const values =
    field === 'RESOURCE'
      ? [text.trim()].filter(Boolean)
      : text
          .split(',')
          .map((value) => value.trim())
          .filter(Boolean);
  if (values.length === 0 || values.length > MAX_VALUES) return null;
  if (takesOneValue(operator) && values.length !== 1) return null;
  const valid = (value: string) => {
    if (value.length > MAX_VALUE_LENGTH) return false;
    switch (field) {
      case 'HTTP_STATUS_CODE':
        return STATUS_PATTERN.test(value);
      case 'HTTP_METHOD':
        return HTTP_METHODS.includes(value);
      case 'RESOURCE':
        return value.startsWith('/');
      case 'TOKEN':
        return /^\d{1,13}$/.test(value) && Number(value) <= MAX_TOKEN_VALUE;
      default:
        return true;
    }
  };
  return values.every(valid) ? values : null;
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

const tokenKeyMessages = defineMessages({
  prompt: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.tokenKey.prompt',
    defaultMessage: 'Prompt tokens',
    description:
      'A token count our gateway reports for each AI API request: the tokens of the prompt sent to the model.',
  },
  completion: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.tokenKey.completion',
    defaultMessage: 'Completion tokens',
    description:
      'A token count our gateway reports for each AI API request: the tokens of the answer the model returns.',
  },
  total: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.tokenKey.total',
    defaultMessage: 'Total tokens',
    description:
      'A token count our gateway reports for each AI API request: the prompt and completion tokens together.',
  },
});

/**
 * The token counts our gateway reports, offered for a Tokens meter's Token key
 * and for every Token field's key: the prompt's (the Token key's default), the
 * completion's, and both. Any other key may be named. `name` is how a price name
 * reads the key, as the API Portal derives it, the same in every language.
 */
export const TOKEN_KEYS: readonly { key: string; label: MessageDescriptor; name: string }[] = [
  { key: DEFAULT_TOKEN_KEY, label: tokenKeyMessages.prompt, name: 'prompt tokens' },
  {
    key: 'metadata.aiTokenUsage.completionTokens',
    label: tokenKeyMessages.completion,
    name: 'completion tokens',
  },
  { key: 'metadata.aiTokenUsage.totalTokens', label: tokenKeyMessages.total, name: 'total tokens' },
];

/** A token key as shown: "Prompt tokens" for one of our gateway's, else the key itself. */
export const tokenKeyLabel = (intl: IntlShape, key: string): string => {
  const option = TOKEN_KEYS.find((candidate) => candidate.key === key);
  return option ? intl.formatMessage(option.label) : key;
};

export const fieldMessages: Record<FilterField, MessageDescriptor> = defineMessages({
  HTTP_STATUS_CODE: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.field.statusCode',
    defaultMessage: 'Status code',
  },
  HTTP_METHOD: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.field.method',
    defaultMessage: 'HTTP method',
  },
  RESOURCE: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.field.resource',
    defaultMessage: 'Resource',
  },
  API_TYPE: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.field.apiType',
    defaultMessage: 'API type',
    description:
      'A filter field: the kind of API a request went to, such as a REST API or an LLM provider.',
  },
  MCP_METHOD: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.field.mcpMethod',
    defaultMessage: 'MCP method',
    description:
      'A filter field: the JSON-RPC method of a request to an MCP server, such as tools/call. Keep "MCP".',
  },
  MCP_TOOL: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.field.mcpTool',
    defaultMessage: 'Tool',
    description: 'A filter field: the name of the MCP server tool a tool call runs.',
  },
  TOKEN: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.field.token',
    defaultMessage: 'Token',
    description:
      'A filter field: a token count of each AI API request, which a price can compare with a number.',
  },
});

const labelMessages = defineMessages({
  tokenField: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.tokenField',
    defaultMessage: 'Token · {key}',
    description:
      'A Token field named by its key, e.g. "Token · Prompt tokens"; {key} is the key\'s label or the key itself.',
  },
  equalTo: {
    id: 'apiControlPlane.pages.appShell.appShellPages.settings.components.meterLabels.operator.equalTo',
    defaultMessage: 'equal to',
    description:
      'The operator of a Token condition that matches one number, e.g. "Token · Prompt tokens equal to 100".',
  },
});

/** A Token field as named in a price or a meter: "Token · Prompt tokens", or "Token · <key>". */
export const tokenFieldLabel = (intl: IntlShape, key: string): string =>
  intl.formatMessage(labelMessages.tokenField, { key: tokenKeyLabel(intl, key) });

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
      'A resource filter: the resource itself or one under it, by whole path segments (/orders and /orders/{id}).',
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

/** An operator as a field reads it: a Token condition's EQUALS is "equal to", every other field's "equals". */
export const operatorMessage = (field: FilterField, operator: FilterOperator): MessageDescriptor =>
  field === 'TOKEN' && operator === 'EQUALS' ? labelMessages.equalTo : operatorMessages[operator];

/** What a meter counts, in the meters list and beside it in a plan price: "Requests" or "Tokens". */
export const typeLabel = (intl: IntlShape, meter: Meter): string =>
  intl.formatMessage(meterTypeMessages[meter.type === 'TOKENS' ? 'TOKENS' : 'REQUEST_COUNT']);

/**
 * The names of the fields a meter's prices may filter on, such as "API type, HTTP method, Token ·
 * Prompt tokens"; empty when there are none.
 */
export const filterFieldsLabel = (intl: IntlShape, meter: Meter): string =>
  (meter.filterFields ?? [])
    .map(({ eventKey, field }) =>
      field === 'TOKEN'
        ? tokenFieldLabel(intl, eventKey)
        : intl.formatMessage(fieldMessages[field]),
    )
    .join(', ');

/** The unit a meter counts in, for prices: a request, or a token; the meter's name says which tokens. */
export const meterUnit = (meter: Meter): 'request' | 'token' =>
  meter.type === 'TOKENS' ? 'token' : 'request';

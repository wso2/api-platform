/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
'use strict';

/*
 * Display text for meters and prices, shared by the settings page and the API pages: "API
 * requests", "Tokens", "API type is one of LlmProvider", "LLM input tokens · LlmProvider".
 */

const METER_TYPE_LABELS = {
    REQUEST_COUNT: 'API requests',
    TOKENS: 'Tokens',
};

// The unit one charge bills per, for "$0.01 per request".
const METER_TYPE_UNITS = {
    REQUEST_COUNT: 'request',
    TOKENS: 'token',
};

const FIELD_LABELS = {
    HTTP_STATUS_CODE: 'Status code',
    HTTP_METHOD: 'HTTP method',
    RESOURCE: 'Resource',
    API_TYPE: 'API type',
    MCP_METHOD: 'MCP method',
    MCP_TOOL: 'Tool',
    TOKEN: 'Token',
};

const OPERATOR_LABELS = {
    EQUALS: 'equals',
    NOT_EQUALS: 'does not equal',
    IN: 'is one of',
    NOT_IN: 'is not one of',
    // The resource itself or one under it, by whole path segments (meterValidation.underPath).
    STARTS_WITH: 'starts with',
};

// A Token condition's operators, as they read in full ("more than 333") and in a price's name ("> 333").
const TOKEN_OPERATOR_LABELS = {
    GREATER_THAN: 'more than',
    GREATER_THAN_OR_EQUAL: 'at least',
    LESS_THAN: 'less than',
    LESS_THAN_OR_EQUAL: 'at most',
    EQUALS: 'equal to',
};
const TOKEN_OPERATOR_SIGNS = {
    GREATER_THAN: '>',
    GREATER_THAN_OR_EQUAL: '\u2265',
    LESS_THAN: '<',
    LESS_THAN_OR_EQUAL: '\u2264',
    EQUALS: '=',
};

// Our gateway's token counts: the keys the settings page suggests for a Token row or field, with
// their labels. The first is a new Tokens meter's key.
const TOKEN_EVENT_KEYS = Object.freeze([
    Object.freeze({ label: 'Prompt tokens', key: 'metadata.aiTokenUsage.promptTokens' }),
    Object.freeze({ label: 'Completion tokens', key: 'metadata.aiTokenUsage.completionTokens' }),
    Object.freeze({ label: 'Total tokens', key: 'metadata.aiTokenUsage.totalTokens' }),
]);

/** What a Token key reads as: its suggestion's label ("Prompt tokens"), or the key itself. */
function tokenKeyLabel(key) {
    const known = TOKEN_EVENT_KEYS.find((k) => k.key === key);
    return known ? known.label : key;
}

function meterTypeLabel(type) {
    return METER_TYPE_LABELS[type] || type;
}

function meterTypeUnit(type) {
    return METER_TYPE_UNITS[type] || 'unit';
}

/** The unit a meter counts in, as subscribers see it: "request" or "token". A missing meter counts requests. */
function meterUnit(meter) {
    return meter ? meterTypeUnit(meter.type) : 'request';
}

/**
 * The fields a meter's prices can filter on ([{ field, eventKey }]), as the meters list shows them:
 * "API type, HTTP method, Token · Total tokens".
 */
function filterFieldsLabel(fields) {
    return (fields || []).map((f) => (f.field === 'TOKEN' ? `${FIELD_LABELS.TOKEN} \u00b7 ${tokenKeyLabel(f.eventKey)}`
        : FIELD_LABELS[f.field] || f.field)).join(', ');
}

/**
 * A price filter as it reads in full: "API type is one of LlmProvider, LlmProxy", or for a Token condition
 * "Prompt tokens more than 333".
 */
function filterLabel(filter) {
    if (filter.field === 'TOKEN') {
        return `${tokenKeyLabel(filter.eventKey)} ${TOKEN_OPERATOR_LABELS[filter.operator] || filter.operator} ${(filter.values || [])[0]}`;
    }
    const field = FIELD_LABELS[filter.field] || filter.field;
    return `${field} ${OPERATOR_LABELS[filter.operator] || filter.operator} ${(filter.values || []).join(', ')}`;
}

/**
 * A filter in short form, for a price's name: "LlmProvider", "not DELETE, OPTIONS", "/orders…", or for a
 * Token condition its key's label in lower case (or the key) and the comparison: "prompt tokens > 333".
 */
function filterShort(filter) {
    if (filter.field === 'TOKEN') {
        const known = TOKEN_EVENT_KEYS.find((k) => k.key === filter.eventKey);
        const label = known ? known.label.toLowerCase() : filter.eventKey;
        return `${label} ${TOKEN_OPERATOR_SIGNS[filter.operator] || filter.operator} ${(filter.values || [])[0]}`;
    }
    const values = (filter.values || []).join(', ');
    if (filter.operator === 'NOT_EQUALS' || filter.operator === 'NOT_IN') return `not ${values}`;
    if (filter.operator === 'STARTS_WITH') return `${values}\u2026`;
    return values;
}

/**
 * The name of a price, which subscribers see: the meter's name, then each of the price's filters in
 * short form: "LLM input tokens · LlmProvider", "API calls · not DELETE, OPTIONS", "API calls".
 */
function priceName(meterName, filters) {
    return [meterName, ...(filters || []).map(filterShort)].join(' · ');
}

module.exports = {
    METER_TYPE_LABELS, FIELD_LABELS, OPERATOR_LABELS, TOKEN_OPERATOR_LABELS, TOKEN_EVENT_KEYS, meterTypeLabel,
    meterTypeUnit, meterUnit, filterFieldsLabel, filterLabel, tokenKeyLabel, priceName,
};

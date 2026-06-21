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

// Shared definitions for the LLM provider template forms (create / edit) and
// the read-only overview. Kept in one place so the field set and the location
// options stay consistent across all three.

import type { TokenLocation } from './types';

export const TOKEN_FIELDS = [
  { key: 'promptTokens', label: 'Prompt Tokens' },
  { key: 'completionTokens', label: 'Completion Tokens' },
  { key: 'totalTokens', label: 'Total Tokens' },
  { key: 'remainingTokens', label: 'Remaining Tokens' },
  { key: 'requestModel', label: 'Request Model' },
  { key: 'responseModel', label: 'Response Model' },
] as const;

export type TokenFieldKey = (typeof TOKEN_FIELDS)[number]['key'];

export const TOKEN_LOCATIONS = [
  { value: 'payload', label: 'Payload' },
  { value: 'header', label: 'Header' },
  { value: 'queryParam', label: 'Query Param' },
  { value: 'pathParam', label: 'Path Param' },
] as const;

export type TokenConfig = Record<
  TokenFieldKey,
  { identifier: string; location: string }
>;

export const EMPTY_TOKEN_CONFIG: TokenConfig = {
  promptTokens: { identifier: '', location: 'payload' },
  completionTokens: { identifier: '', location: 'payload' },
  totalTokens: { identifier: '', location: 'payload' },
  remainingTokens: { identifier: '', location: 'payload' },
  requestModel: { identifier: '', location: 'payload' },
  responseModel: { identifier: '', location: 'payload' },
};

// Sensible starting values (the OpenAI-style mappings used by the built-in
// templates). Pre-filled in the create form so the user starts from a working
// configuration and only tweaks what differs for their provider.
export const DEFAULT_TOKEN_CONFIG: TokenConfig = {
  promptTokens: { identifier: '$.usage.prompt_tokens', location: 'payload' },
  completionTokens: { identifier: '$.usage.completion_tokens', location: 'payload' },
  totalTokens: { identifier: '$.usage.total_tokens', location: 'payload' },
  remainingTokens: { identifier: 'x-ratelimit-remaining-tokens', location: 'header' },
  requestModel: { identifier: '$.model', location: 'payload' },
  responseModel: { identifier: '$.model', location: 'payload' },
};

/** Default auth config (Bearer token in the standard Authorization header) used
 * by most OpenAI-compatible providers. Pre-filled on the create form and shown
 * as the fallback on the Connection tab when a template specifies no auth. */
export const DEFAULT_AUTH_CONFIG = {
  type: 'bearer',
  header: 'Authorization',
  valuePrefix: 'Bearer',
} as const;

/** Build a TokenConfig draft from an entity that carries the six TokenLocation fields. */
export function toTokenConfig(source?: Partial<
  Record<TokenFieldKey, TokenLocation | undefined>
>): TokenConfig {
  const draft: TokenConfig = {
    promptTokens: { ...EMPTY_TOKEN_CONFIG.promptTokens },
    completionTokens: { ...EMPTY_TOKEN_CONFIG.completionTokens },
    totalTokens: { ...EMPTY_TOKEN_CONFIG.totalTokens },
    remainingTokens: { ...EMPTY_TOKEN_CONFIG.remainingTokens },
    requestModel: { ...EMPTY_TOKEN_CONFIG.requestModel },
    responseModel: { ...EMPTY_TOKEN_CONFIG.responseModel },
  };
  if (!source) return draft;
  TOKEN_FIELDS.forEach(({ key }) => {
    const value = source[key];
    if (value) {
      draft[key] = {
        identifier: value.identifier ?? '',
        location: value.location ?? 'payload',
      };
    }
  });
  return draft;
}

/**
 * Like toTokenConfig, but any field the source leaves blank falls back to the
 * OpenAI default value. Used by the edit form so the defaults are always shown
 * ("by default OpenAI values are here") rather than empty inputs.
 */
export function toTokenConfigWithDefaults(
  source?: Partial<Record<TokenFieldKey, TokenLocation | undefined>>
): TokenConfig {
  const cfg = toTokenConfig(source);
  TOKEN_FIELDS.forEach(({ key }) => {
    if (!cfg[key].identifier.trim()) {
      cfg[key] = { ...DEFAULT_TOKEN_CONFIG[key] };
    }
  });
  return cfg;
}

/** Collect only the filled-in token mappings into the API shape (skips blanks). */
export function fromTokenConfig(
  config: TokenConfig
): Partial<Record<TokenFieldKey, TokenLocation>> {
  const result: Partial<Record<TokenFieldKey, TokenLocation>> = {};
  TOKEN_FIELDS.forEach(({ key }) => {
    const cfg = config[key];
    if (cfg.identifier.trim()) {
      result[key] = {
        identifier: cfg.identifier.trim(),
        location: cfg.location,
      };
    }
  });
  return result;
}

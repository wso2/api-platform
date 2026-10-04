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

/**
 * The gateway handle limit enforced by platform-api's `gateways.handle` column.
 * The handle is built as `<environment>-<name>`, so the environment's own length
 * eats into what is left for the name.
 */
export const MAX_GATEWAY_HANDLE_LENGTH = 40;

/** The longest name any environment can accommodate (a one-character one). */
const MAX_GATEWAY_NAME_LENGTH = MAX_GATEWAY_HANDLE_LENGTH - 2;

/** Reserved for the gateway provisioned automatically with the environment. */
const RESERVED_GATEWAY_NAME = 'default';

/** How many characters a gateway handle may use in the given environment. */
export function gatewayNameBudget(environment: string): number {
  return environment
    ? MAX_GATEWAY_HANDLE_LENGTH - environment.length - 1
    : MAX_GATEWAY_NAME_LENGTH;
}

/**
 * Derives the handle a gateway will be addressed by from its display name, the
 * same conversion the backend applies (and the same one behind a project's or an
 * API's id): lowercased, with spaces and underscores folded to hyphens and
 * anything else dropped.
 *
 * Returns `''` when nothing usable survives, which the caller reports rather
 * than substituting a name of its own.
 */
export function gatewayHandleFromName(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[\s_]+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/-{2,}/g, '-')
    .replace(/^-+|-+$/g, '');
}

/**
 * Validates a new gateway's name against the same rules the backend applies,
 * returning the message to show or `undefined` when the name is fine.
 *
 * The name is a display name: it may be written however the user likes, and the
 * handle is derived from it, so casing and spaces are not errors. What it cannot
 * be is a name no handle can be built from, the reserved bootstrap name, or one
 * whose handle does not fit the handle column alongside the environment. An
 * empty name is left to the field's own `required` handling rather than reported
 * here.
 */
export function validateGatewayName(name: string, environment: string): string | undefined {
  if (!name.trim()) return undefined;

  const handle = gatewayHandleFromName(name);
  if (!handle) {
    return 'Include at least one letter or number.';
  }
  if (handle === RESERVED_GATEWAY_NAME) {
    return `"${RESERVED_GATEWAY_NAME}" is reserved for the gateway created with the environment.`;
  }

  const budget = gatewayNameBudget(environment);
  if (handle.length > budget) {
    return environment
      ? `Too long for the "${environment}" environment: the handle "${handle}" must be at most ${budget} characters (the full handle "${environment}-<handle>" has to fit ${MAX_GATEWAY_HANDLE_LENGTH}).`
      : `Use at most ${budget} characters.`;
  }
  return undefined;
}

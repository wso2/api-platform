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

const DNS1123_NAME = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

/** How many characters a gateway name may use in the given environment. */
export function gatewayNameBudget(environment: string): number {
  return environment
    ? MAX_GATEWAY_HANDLE_LENGTH - environment.length - 1
    : MAX_GATEWAY_NAME_LENGTH;
}

/**
 * Validates a new gateway's name against the same rules the backend applies to
 * it, returning the message to show or `undefined` when the name is fine.
 *
 * On create the name is what the gateway's handle is derived from, so it has to
 * be a DNS-1123 name that fits the handle column alongside the environment —
 * otherwise the create call fails after the form has already been submitted.
 * Case is not part of it: the backend lowercases before validating, so `Prod-1`
 * is accepted and becomes `prod-1`. An empty name is left to the field's own
 * `required` handling rather than reported here.
 */
export function validateGatewayName(name: string, environment: string): string | undefined {
  const handle = name.trim().toLowerCase();
  if (!handle) return undefined;

  if (handle === RESERVED_GATEWAY_NAME) {
    return `"${RESERVED_GATEWAY_NAME}" is reserved for the gateway created with the environment.`;
  }
  if (!DNS1123_NAME.test(handle)) {
    return 'Use only letters, digits and hyphens, starting and ending with a letter or digit.';
  }

  const budget = gatewayNameBudget(environment);
  if (handle.length > budget) {
    return environment
      ? `Too long for the "${environment}" environment: use at most ${budget} characters (the handle "${environment}-<name>" must fit ${MAX_GATEWAY_HANDLE_LENGTH} characters).`
      : `Use at most ${budget} characters.`;
  }
  return undefined;
}

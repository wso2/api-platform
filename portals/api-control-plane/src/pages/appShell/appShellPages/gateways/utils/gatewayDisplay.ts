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

import type { Gateway } from '@/api/resources/gateways';

/**
 * Display helpers for the spec's `GatewayResponse` shape — the presentation
 * half of `api/resources/gateways`, kept out of the resource layer so that
 * layer stays transport + cache only.
 *
 * Everything here is derived from spec fields (`endpoints`, `properties`,
 * `functionalityType`), so a spec change surfaces as a typecheck failure here
 * rather than as a wrong label on a card.
 */

/** Whether the gateway runs on the customer's own infra or is WSO2-managed. */
export type GatewayMode = 'managed' | 'self-hosted';

/**
 * Self-hosted vs WSO2-managed is not a first-class spec field: it is tagged in
 * the gateway's free-form `properties` bag under `gatewayMode`. Anything absent
 * or unrecognised reads as WSO2-managed, which is the safe default.
 */
export const gatewayMode = (gateway: Gateway): GatewayMode =>
  String(gateway.properties?.gatewayMode) === 'self-hosted' ? 'self-hosted' : 'managed';

/**
 * The address clients call. The spec models `endpoints` as a list, but a
 * gateway exposes one primary address and the cards/rows show that one; the
 * rest belong on the gateway's own page.
 */
export const gatewayEndpoint = (gateway: Gateway): string => gateway.endpoints?.[0] ?? '';

/** Kinds of traffic a gateway is provisioned to serve. */
export type GatewayFunctionality = NonNullable<Gateway['functionalityType']>;

/**
 * Fields the listing searches. Derived rather than inlined at the call site so
 * the search box and the card can never disagree about what a match means.
 */
export const gatewaySearchFields = (gateway: Gateway): string[] =>
  [gateway.displayName, gateway.id, gatewayEndpoint(gateway)].filter((field): field is string =>
    Boolean(field),
  );

/** Gateway tile monogram from the first two word initials, or first two letters. */
export const gatewayInitials = (displayName?: string): string => {
  const words = (displayName ?? '')
    .split(/\s+/)
    .map((word) => word.replace(/[^\p{L}\p{N}]/gu, ''))
    .filter((word) => word.length > 0);

  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();

  return (words[0][0] + words[1][0]).toUpperCase();
};

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

import type { AIWorkspaceHostPort } from '../hostPort';
import type { GatewayHosting } from '../types';

/**
 * Which hosting kinds this organization has no room for, and why.
 *
 * Each kind is capped separately — a full WSO2-managed quota leaves
 * self-hosted room untouched — so a control that commits to one kind consults
 * that kind alone. One shared helper so the empty state, the form's hosting
 * cards and anything added later cannot drift apart on the rule.
 *
 * A host with no entitlement supplier reports nothing, and every kind stays
 * choosable exactly as before.
 */
export function hostingBlocked(
  port: AIWorkspaceHostPort
): Partial<Record<GatewayHosting, string>> {
  const limits = port.resourceLimits;
  if (!limits?.canCreate) return {};
  const blocked: Partial<Record<GatewayHosting, string>> = {};
  if (!limits.canCreate('managedGateways')) {
    blocked.managed = limits.limitMessage?.('managedGateways') ?? '';
  }
  if (!limits.canCreate('selfHostedGateways')) {
    blocked['self-hosted'] = limits.limitMessage?.('selfHostedGateways') ?? '';
  }
  return blocked;
}

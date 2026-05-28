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

import React from 'react';
import { Outlet } from 'react-router-dom';
import { ProxiesProvider } from '../../../../contexts/proxy';
import { LLMProvidersProvider } from '../../../../contexts/llmProvider';
import { GuardrailsProvider } from '../../../../contexts/GuardrailsContext';

export function formatRelativeTime(value?: string): string {
  if (!value) return 'Unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Unknown';

  const now = Date.now();
  const diffMs = now - date.getTime();
  const diffSeconds = Math.abs(diffMs) / 1000;

  if (diffSeconds < 45) return 'Just now';
  if (diffSeconds < 90) return '1 minute ago';

  const diffMinutes = diffSeconds / 60;
  if (diffMinutes < 45) return `${Math.round(diffMinutes)} minutes ago`;
  if (diffMinutes < 90) return '1 hour ago';

  const diffHours = diffMinutes / 60;
  if (diffHours < 22) return `${Math.round(diffHours)} hours ago`;
  if (diffHours < 36) return '1 day ago';

  const diffDays = diffHours / 24;
  if (diffDays < 26) return `${Math.round(diffDays)} days ago`;
  if (diffDays < 45) return '1 month ago';

  const diffMonths = diffDays / 30;
  if (diffMonths < 11) return `${Math.round(diffMonths)} months ago`;
  if (diffMonths < 18) return '1 year ago';

  const diffYears = diffDays / 365;
  return `${Math.round(diffYears)} years ago`;
}

export function buildProxyId(name: string, takenIds: Set<string>): string {
  const base = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-');

  if (!takenIds.has(base)) return base;

  let suffix = 2;
  while (takenIds.has(`${base}-${suffix}`)) {
    suffix += 1;
  }
  return `${base}-${suffix}`;
}

export default function LLMProxyLayout() {
  return (
    <LLMProvidersProvider>
      <ProxiesProvider>
        <GuardrailsProvider>
          <Outlet />
        </GuardrailsProvider>
      </ProxiesProvider>
    </LLMProvidersProvider>
  );
}

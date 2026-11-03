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

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';

import useTransformerPolicies, {
  TRANSFORMER_POLICY_CATEGORY,
  FALLBACK_POLICY_CATEGORY,
} from './useTransformerPolicies';

const getGuardrails = vi.fn();
const getGatewayCustomPolicies = vi.fn();

vi.mock('../apis/policyHubApis', () => ({
  getGuardrails: (...args: unknown[]) => getGuardrails(...args),
}));
vi.mock('../apis/gatewayPolicyApis', () => ({
  getGatewayCustomPolicies: () => getGatewayCustomPolicies(),
}));
vi.mock('../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const page = (rows: unknown[], total = rows.length) => ({
  data: rows,
  pagination: { total },
});
const policy = (name: string, version = 'v0') => ({
  name,
  displayName: name,
  version,
});

/**
 * Both categories are read concurrently, so a mock keyed on call order would
 * depend on scheduling. These answer by category instead.
 */
const catalogueReturning = (
  byCategory: Record<string, (offset: number) => unknown>
) => {
  getGuardrails.mockImplementation((category: string, _size: number, offset: number) => {
    const answer = byCategory[category];
    return Promise.resolve(answer ? answer(offset) : page([]));
  });
};

beforeEach(() => {
  vi.clearAllMocks();
  getGuardrails.mockResolvedValue(page([]));
  getGatewayCustomPolicies.mockResolvedValue({ list: [] });
});

describe('reading the transformer catalogue', () => {
  it('asks for the dedicated category and the one that predates it', async () => {
    renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(getGuardrails).toHaveBeenCalled());
    const categories = getGuardrails.mock.calls.map((c) => c[0]);
    expect(categories).toContain(TRANSFORMER_POLICY_CATEGORY);
    expect(categories).toContain(FALLBACK_POLICY_CATEGORY);
  });

  it('asks for them as separate requests, not one request naming both', async () => {
    renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(getGuardrails).toHaveBeenCalled());
    // The catalogue reads several categories in one request as "carries all of
    // them", which matches nothing until every translator carries both labels.
    for (const [category] of getGuardrails.mock.calls) {
      expect(category).not.toContain(',');
    }
  });

  it("offers catalogue policies alongside the gateway's own", async () => {
    catalogueReturning({
      [TRANSFORMER_POLICY_CATEGORY]: (offset) =>
        offset === 0 ? page([policy('openai-to-anthropic-transformer')]) : page([]),
    });
    getGatewayCustomPolicies.mockResolvedValue({
      list: [{ name: 'acme-request-tagger', version: 'v1.0.0', policyDefinition: {} }],
    });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    const names = result.current.policies.map((p) => p.name);
    expect(names).toContain('openai-to-anthropic-transformer');
    expect(names).toContain('acme-request-tagger');
  });

  it('marks where each policy came from, so a reader can tell them apart', async () => {
    catalogueReturning({
      [TRANSFORMER_POLICY_CATEGORY]: (offset) =>
        offset === 0 ? page([policy('openai-to-gemini-transformer')]) : page([]),
    });
    getGatewayCustomPolicies.mockResolvedValue({
      list: [{ name: 'acme-request-tagger', version: 'v1.0.0', policyDefinition: {} }],
    });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    const sources = Object.fromEntries(result.current.policies.map((p) => [p.name, p.source]));
    expect(sources['openai-to-gemini-transformer']).toBe('catalogue');
    expect(sources['acme-request-tagger']).toBe('custom');
  });

  it('lists a policy once even when it answers both category requests', async () => {
    const both = (offset: number) =>
      offset === 0 ? page([policy('openai-to-bedrock-transformer')]) : page([]);
    catalogueReturning({
      [TRANSFORMER_POLICY_CATEGORY]: both,
      [FALLBACK_POLICY_CATEGORY]: both,
    });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    // A translator carrying both labels comes back from each read.
    expect(
      result.current.policies.filter((p) => p.name === 'openai-to-bedrock-transformer')
    ).toHaveLength(1);
  });

  it('keeps two versions of the same policy apart', async () => {
    catalogueReturning({
      [TRANSFORMER_POLICY_CATEGORY]: (offset) =>
        offset === 0
          ? page([policy('openai-to-anthropic-transformer', 'v0'), policy('openai-to-anthropic-transformer', 'v1')])
          : page([]),
    });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    // De-duplication is on name and version together: a new major is a
    // different policy to attach, not a duplicate of the old one.
    expect(result.current.policies).toHaveLength(2);
  });

  it('reads a policy its own parameter declarations, so the form follows the policy', async () => {
    catalogueReturning({
      [TRANSFORMER_POLICY_CATEGORY]: (offset) =>
        offset === 0
          ? page([
              {
                ...policy('openai-to-anthropic-transformer'),
                parameters: [{ name: 'maxTokens', type: 'integer', required: true }],
              },
            ])
          : page([]),
    });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    expect(result.current.policies[0].parameters).toEqual([
      { name: 'maxTokens', type: 'integer', required: true },
    ]);
  });

  it('reads parameters published as a map just as well as a list', async () => {
    getGatewayCustomPolicies.mockResolvedValue({
      list: [
        {
          name: 'acme-request-tagger',
          version: 'v1.0.0',
          policyDefinition: {
            parameters: { tag: { type: 'string', required: true, default: 'acme' } },
          },
        },
      ],
    });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    expect(result.current.policies[0].parameters).toEqual([
      {
        name: 'tag',
        type: 'string',
        displayName: undefined,
        description: undefined,
        required: true,
        default: 'acme',
      },
    ]);
  });

  it('gives a policy that declares no parameters an empty list, not nothing', async () => {
    catalogueReturning({
      [TRANSFORMER_POLICY_CATEGORY]: (offset) =>
        offset === 0 ? page([policy('openai-to-anthropic-transformer')]) : page([]),
    });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    // Callers build the form straight off this, so undefined would be a crash.
    expect(result.current.policies[0].parameters).toEqual([]);
  });
});

describe('paging through a large catalogue', () => {
  it('keeps reading until it has every policy the catalogue reports', async () => {
    const first = Array.from({ length: 100 }, (_, i) => policy(`p-${i}`));
    catalogueReturning({
      [TRANSFORMER_POLICY_CATEGORY]: (offset) => {
        if (offset === 0) return page(first, 101);
        if (offset === 100) return page([policy('p-100')], 101);
        return page([], 101);
      },
    });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    // A translator on a later page is one this application cannot match, so a
    // provider reads as having none available when one exists.
    expect(result.current.policies.map((p) => p.name)).toContain('p-100');
    expect(result.current.policies).toHaveLength(101);
  });

  it('stops on an empty page even when the reported total says otherwise', async () => {
    // A catalogue that ignores paging, or reports a stale total, must not spin
    // this forever.
    catalogueReturning({ [TRANSFORMER_POLICY_CATEGORY]: () => page([], 999) });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.policies).toEqual([]);
  });
});

describe('when the catalogue cannot be read', () => {
  it('stays unloaded rather than reporting an empty catalogue', async () => {
    getGuardrails.mockRejectedValue(new Error('policy hub offline'));

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    // An empty catalogue reported as loaded would make the resolver call every
    // attached translator invalid and every provider unresolved — an outage
    // rendered as a configuration fault on every proxy at once.
    expect(result.current.isLoaded).toBe(false);
    expect(result.current.error).toBeInstanceOf(Error);
  });

  it('keeps what one category returned when only the other failed', async () => {
    getGuardrails.mockImplementation((category: string, _size: number, offset: number) => {
      if (category === TRANSFORMER_POLICY_CATEGORY) {
        return Promise.reject(new Error('labelled category missing'));
      }
      return Promise.resolve(
        offset === 0 ? page([policy('openai-to-mistral-transformer')]) : page([])
      );
    });

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    // A Policy Hub that predates the dedicated label still answers the old one.
    expect(result.current.policies.map((p) => p.name)).toContain('openai-to-mistral-transformer');
    expect(result.current.error).toBeNull();
  });

  it('treats a gateway with no policies of its own as normal', async () => {
    catalogueReturning({
      [TRANSFORMER_POLICY_CATEGORY]: (offset) =>
        offset === 0 ? page([policy('openai-to-anthropic-transformer')]) : page([]),
    });
    getGatewayCustomPolicies.mockRejectedValue(new Error('no custom policies'));

    const { result } = renderHook(() => useTransformerPolicies());

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    expect(result.current.error).toBeNull();
    expect(result.current.policies).toHaveLength(1);
  });

  it('retries on reload', async () => {
    getGuardrails.mockRejectedValue(new Error('offline'));
    const { result } = renderHook(() => useTransformerPolicies());
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isLoaded).toBe(false);

    catalogueReturning({
      [TRANSFORMER_POLICY_CATEGORY]: (offset) =>
        offset === 0 ? page([policy('openai-to-anthropic-transformer')]) : page([]),
    });
    await act(async () => {
      await result.current.reload();
    });

    await waitFor(() => expect(result.current.isLoaded).toBe(true));
    expect(result.current.error).toBeNull();
  });
});

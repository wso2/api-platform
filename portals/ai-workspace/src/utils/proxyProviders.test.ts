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

import { describe, expect, it } from 'vitest';
import {
  canRemoveProvider,
  effectiveProviderName,
  majorPolicyVersion,
  primaryProviderEntry,
  proxyProviderEntries,
  withPrimaryFirst,
  resolvedTransformerFor,
  transformerFromPolicy,
  withPrimaryProvider,
} from './proxyProviders';
import { resolveTransformer } from './transformerResolution';
import type { Proxy, ProxyProviderEntry, SelectablePolicy } from './types';

const entry = (
  id: string,
  extra: Partial<ProxyProviderEntry> = {}
): ProxyProviderEntry => ({ id, isPrimary: false, ...extra });

const proxyWith = (providers: ProxyProviderEntry[]): Proxy =>
  ({ id: 'p', displayName: 'p', providers }) as Proxy;

describe('effectiveProviderName', () => {
  it('is the alias where there is one, because that is what routes', () => {
    expect(effectiveProviderName(entry('openai-provider', { alias: 'openai-eu' })))
      .toBe('openai-eu');
  });

  it('falls back to the id, and treats a blank alias as none', () => {
    expect(effectiveProviderName(entry('openai-provider'))).toBe('openai-provider');
    expect(effectiveProviderName(entry('openai-provider', { alias: '   ' })))
      .toBe('openai-provider');
  });
});

describe('proxyProviderEntries', () => {
  it('reads the list in the order it is stored, primary or not', () => {
    // Display order. Marking a provider primary must not move its row out from
    // under the pointer that marked it — the list is put in order as the
    // request is built instead, by withPrimaryFirst.
    const entries = proxyProviderEntries(
      proxyWith([entry('a'), entry('b', { isPrimary: true }), entry('c')])
    );
    expect(entries.map((e) => e.id)).toEqual(['a', 'b', 'c']);
  });

  it('returns nothing for a proxy that has no providers', () => {
    expect(proxyProviderEntries(null)).toEqual([]);
    expect(proxyProviderEntries(proxyWith([]))).toEqual([]);
  });

  it('keeps the stored objects, so a caller can find one by identity', () => {
    // The Providers tab edits a position in this view and writes back into the
    // stored list, so the two must be the same objects.
    const stored = entry('a');
    const entries = proxyProviderEntries(proxyWith([stored, entry('b', { isPrimary: true })]));
    expect(entries).toContain(stored);
  });
});

describe('withPrimaryFirst', () => {
  it('leads with the primary, keeping the rest in order', () => {
    const ordered = withPrimaryFirst([
      entry('a'),
      entry('b', { isPrimary: true }),
      entry('c'),
    ]);
    expect(ordered.map((e) => e.id)).toEqual(['b', 'a', 'c']);
  });

  it('leaves a list that cannot be out of order alone', () => {
    const single = [entry('a', { isPrimary: true })];
    expect(withPrimaryFirst(single)).toBe(single);
    expect(withPrimaryFirst([])).toEqual([]);
  });

  it('keeps every attachment, even one no entry claims to lead', () => {
    // A list with no primary is not this function's to repair: dropping or
    // inventing one would send a different proxy than the page is showing.
    const ordered = withPrimaryFirst([entry('a'), entry('b')]);
    expect(ordered.map((e) => e.id)).toEqual(['a', 'b']);
  });
});

describe('primaryProviderEntry and withPrimaryProvider', () => {
  it('finds the primary, and moving it leaves exactly one', () => {
    const moved = withPrimaryProvider(
      [entry('a', { isPrimary: true }), entry('b')],
      'b'
    );
    expect(moved.filter((e) => e.isPrimary).map((e) => e.id)).toEqual(['b']);
    expect(primaryProviderEntry(proxyWith(moved))?.id).toBe('b');
  });
});

describe('canRemoveProvider', () => {
  it('refuses the last one: a proxy always has a provider', () => {
    expect(canRemoveProvider([entry('a')])).toBe(false);
    expect(canRemoveProvider([entry('a'), entry('b')])).toBe(true);
  });
});

describe('majorPolicyVersion', () => {
  it('reduces a published version to the major a gateway resolves', () => {
    // A gateway rejects anything more specific, so `0.9` stored as published
    // is accepted on create and then fails at invocation.
    expect(majorPolicyVersion('0.9')).toBe('v0');
    expect(majorPolicyVersion('v0.9.1')).toBe('v0');
    expect(majorPolicyVersion('1')).toBe('v1');
    expect(majorPolicyVersion('  2.3  ')).toBe('v2');
  });

  it('yields nothing where there is nothing to reduce', () => {
    expect(majorPolicyVersion(undefined)).toBe('');
    expect(majorPolicyVersion('')).toBe('');
  });
});

describe('transformerFromPolicy', () => {
  const policy: SelectablePolicy = {
    name: 'openai-to-gemini-transformer',
    displayName: 'OpenAI to Gemini Transformer',
    version: '0.9',
    source: 'catalogue',
  };

  it('records the name and the major, and omits empty parameters', () => {
    expect(transformerFromPolicy(policy)).toEqual({
      type: 'openai-to-gemini-transformer',
      version: 'v0',
    });
    expect(transformerFromPolicy(policy, {})).not.toHaveProperty('params');
  });

  it('carries parameters where there are any', () => {
    expect(transformerFromPolicy(policy, { model: 'gemini-2.5-pro' })).toEqual({
      type: 'openai-to-gemini-transformer',
      version: 'v0',
      params: { model: 'gemini-2.5-pro' },
    });
  });
});

describe('resolvedTransformerFor', () => {
  const matched: SelectablePolicy = {
    name: 'openai-to-gemini-transformer',
    displayName: 'OpenAI to Gemini Transformer',
    version: '0.9',
    source: 'catalogue',
  };

  it('keeps what was chosen, untouched', () => {
    const chosen = { type: 'custom-translator', version: 'v3', params: { a: 1 } };
    expect(resolvedTransformerFor(chosen, { status: 'auto', policy: matched }))
      .toBe(chosen);
  });

  it('records a match where nothing was chosen', () => {
    expect(resolvedTransformerFor(null, { status: 'auto', policy: matched }))
      .toEqual({ type: 'openai-to-gemini-transformer', version: 'v0' });
  });

  it('records nothing for any other verdict', () => {
    // Only a match is a translator. Recording one for "needs none" or "cannot
    // be matched" would attach something the screen never offered.
    for (const status of ['none', 'unresolved', 'invalid', 'unknown'] as const) {
      expect(resolvedTransformerFor(null, { status })).toBeUndefined();
    }
  });

  // The two halves of what a page saves, composed the way a page composes them.
  // A removal only survives the save if it reaches the resolver; drop it on the
  // way and the match is found again and written back, which is the remove
  // control doing nothing at all.
  describe('composed with the resolver, as a save does', () => {
    const forProvider = (hasNoTransformer: boolean) =>
      resolvedTransformerFor(
        null,
        resolveTransformer({
          inboundTemplate: 'openai',
          providerTemplate: 'gemini',
          hasNoTransformer,
          policies: [matched],
        })
      );

    it('writes the match a provider has not answered for yet', () => {
      expect(forProvider(false)).toEqual({
        type: 'openai-to-gemini-transformer',
        version: 'v0',
      });
    });

    it('writes nothing once the translator was taken off', () => {
      expect(forProvider(true)).toBeUndefined();
    });
  });
});

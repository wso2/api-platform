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
import { autoMatchedPolicyName, resolveTransformer } from './transformerResolution';
import type { SelectablePolicy } from './types';

/**
 * What a provider is told about its translator.
 *
 * A wrong verdict here renders perfectly well — the screen simply says
 * something untrue about the proxy — so this is the one place in the feature
 * where a defect is silent. Every branch is pinned, and so is the order they
 * are considered in, because that order is itself a decision: what a proxy
 * carries is read before what its formats would need.
 */

const policy = (name: string): SelectablePolicy => ({
  name,
  displayName: name.replace(/-/g, ' '),
  version: '0.9',
  source: 'catalogue',
});

const catalogue = [
  policy('openai-to-anthropic-transformer'),
  policy('openai-to-gemini-transformer'),
];

describe('autoMatchedPolicyName', () => {
  it('names the policy from handles on both sides', () => {
    expect(autoMatchedPolicyName('openai', 'anthropic')).toBe(
      'openai-to-anthropic-transformer'
    );
  });
});

describe('resolveTransformer', () => {
  it('says nothing at all about a provider whose format is not yet known', () => {
    // A provider list still loading has no template to compare. Announcing a
    // verdict here states a conclusion nothing has reached.
    const resolution = resolveTransformer({ inboundTemplate: 'openai' });
    expect(resolution.status).toBe('none');
    expect(resolution.title).toBeUndefined();
  });

  it('says none is needed where the formats already agree', () => {
    const resolution = resolveTransformer({
      inboundTemplate: 'openai',
      providerTemplate: 'openai',
      interfaceLabel: 'OpenAI',
      providerLabel: 'test-openai',
    });
    expect(resolution.status).toBe('none');
    expect(resolution.title).toBe('No transformer needed');
    expect(resolution.detail).toContain('test-openai');
    expect(resolution.detail).toContain('OpenAI');
  });

  it('reports what is attached even once the formats have come to agree', () => {
    // The translator is still on the proxy and still runs. Reporting "none
    // needed" over it would describe a proxy that does not exist, and would
    // hide the only thing that could be removed.
    const resolution = resolveTransformer({
      inboundTemplate: 'anthropic',
      providerTemplate: 'anthropic',
      chosenTransformer: { type: 'openai-to-anthropic-transformer', version: 'v0' },
      policies: catalogue,
      interfaceLabel: 'Anthropic',
      providerLabel: 'test-anthropic',
    });
    expect(resolution.status).toBe('manual');
    expect(resolution.detail).toBe('openai-to-anthropic-transformer');
    expect(resolution.note).toContain('Not needed');
  });

  it('matches a translator from the two formats when nothing is attached', () => {
    const resolution = resolveTransformer({
      inboundTemplate: 'openai',
      providerTemplate: 'gemini',
      policies: catalogue,
      interfaceLabel: 'OpenAI',
    });
    expect(resolution.status).toBe('auto');
    expect(resolution.policy?.name).toBe('openai-to-gemini-transformer');
    expect(resolution.note).toBeUndefined();
  });

  it('stops offering a match once going without one is a decision', () => {
    // A provider whose translator was just removed must not be shown that same
    // match back as its state, or removing appears to do nothing.
    const resolution = resolveTransformer({
      inboundTemplate: 'openai',
      providerTemplate: 'gemini',
      hasNoTransformer: true,
      policies: catalogue,
      interfaceLabel: 'OpenAI',
      providerLabel: 'test-gemini',
    });
    expect(resolution.status).toBe('unresolved');
    expect(resolution.title).toBe('No transformer configured');
    expect(resolution.detail).toContain('test-gemini');
  });

  it('reports a translator the catalogue no longer carries as no longer valid', () => {
    const resolution = resolveTransformer({
      inboundTemplate: 'openai',
      providerTemplate: 'mistral',
      chosenTransformer: { type: 'openai-to-mistral-transformer', version: 'v0' },
      policies: catalogue,
      interfaceLabel: 'OpenAI',
    });
    expect(resolution.status).toBe('invalid');
    expect(resolution.detail).toContain('openai-to-mistral-transformer');
  });

  it('says the catalogue has not answered rather than blaming the proxy', () => {
    // Reporting "unresolved" on a failed fetch turns somebody else's outage
    // into a configuration problem the user goes looking for.
    for (const chosenTransformer of [
      undefined,
      { type: 'openai-to-gemini-transformer', version: 'v0' },
    ]) {
      const resolution = resolveTransformer({
        inboundTemplate: 'openai',
        providerTemplate: 'gemini',
        chosenTransformer,
        policies: [],
        policiesLoaded: false,
      });
      expect(resolution.status).toBe('unknown');
    }
  });

  it('reports a needed translator nothing provides, and says where to look', () => {
    const resolution = resolveTransformer({
      inboundTemplate: 'gemini',
      providerTemplate: 'openai',
      policies: catalogue,
      interfaceLabel: 'Gemini',
      providerLabel: 'test-openai',
    });
    expect(resolution.status).toBe('unresolved');
    expect(resolution.detail).toContain('Policy Hub');
  });
});

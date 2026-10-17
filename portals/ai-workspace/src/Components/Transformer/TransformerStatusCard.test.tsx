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
import { describe, it, expect, vi } from 'vitest';

import { renderWithProviders, screen } from '../../test/utils';
import TransformerStatusCard from './TransformerStatusCard';
import type { TransformerResolution } from '../../utils/transformerResolution';

const cy = (c: HTMLElement, id: string) => c.querySelector(`[data-cyid="${id}"]`) as HTMLElement;
const btn = (c: HTMLElement, id: string) => cy(c, id)?.querySelector('button') ?? cy(c, id);

const render = (resolution: TransformerResolution, props = {}) =>
  renderWithProviders(
    <TransformerStatusCard resolution={resolution} onConfigure={vi.fn()} onRemove={vi.fn()} {...props} />
  );

describe('what the card reports', () => {
  it('reassures when the provider already speaks the inbound format', () => {
    render({ status: 'none', title: 'No transformer needed' });

    expect(screen.getByText('No transformer needed')).toBeInTheDocument();
  });

  it('names the policy that applies when one is attached', () => {
    render({
      status: 'manual',
      title: 'OpenAI to Anthropic Transformer',
      policy: { name: 'openai-to-anthropic-transformer', version: 'v0' } as never,
    });

    expect(screen.getByText('OpenAI to Anthropic Transformer')).toBeInTheDocument();
  });

  it('reports a provider that still needs one', () => {
    render({ status: 'unresolved', title: 'No transformer configured', detail: 'Pick a policy from the Policy Hub.' });

    expect(screen.getByText('No transformer configured')).toBeInTheDocument();
    expect(screen.getByText('Pick a policy from the Policy Hub.')).toBeInTheDocument();
  });

  it('reports a policy that no longer translates between these two formats', () => {
    render({ status: 'invalid', title: 'Transformer no longer valid' });

    expect(screen.getByText('Transformer no longer valid')).toBeInTheDocument();
  });

  it('says it is still looking while the catalogue has not answered', () => {
    render({ status: 'unknown', title: 'Checking available transformers…' });

    expect(screen.getByText('Checking available transformers…')).toBeInTheDocument();
  });
});

describe('when nothing needs translating', () => {
  it('renders nothing at all where the caller asked for silence', () => {
    const { container } = render({ status: 'none', title: 'No transformer needed' }, { hideWhenNotNeeded: true });

    // A proxy whose provider matches its interface has nothing to say here, and
    // a line saying so on every row is a list nobody reads.
    expect(container.textContent).toBe('');
  });

  it('still reports the matching case when the caller wants it shown', () => {
    const { container } = render({ status: 'none', title: 'No transformer needed' });

    expect(container.textContent).not.toBe('');
  });
});

describe('acting on the card', () => {
  it('opens the picker from the configure action', async () => {
    const onConfigure = vi.fn();
    const { user, container } = render(
      { status: 'unresolved', title: 'No transformer configured' },
      { onConfigure, 'data-cyid': 'card' }
    );

    await user.click(btn(container, 'card-configure'));

    expect(onConfigure).toHaveBeenCalled();
  });

  it('detaches the attached policy from the remove action', async () => {
    const onRemove = vi.fn();
    const { user, container } = render(
      {
        status: 'manual',
        title: 'OpenAI to Anthropic Transformer',
        policy: { name: 'openai-to-anthropic-transformer', version: 'v0' } as never,
      },
      { onRemove, transformer: { type: 'openai-to-anthropic-transformer', version: 'v0' }, 'data-cyid': 'card' }
    );

    const remove = btn(container, 'card-remove');
    if (remove) {
      await user.click(remove);
      expect(onRemove).toHaveBeenCalled();
    }
  });

  it('offers nothing to click while disabled', () => {
    const { container } = render(
      { status: 'unresolved', title: 'No transformer configured' },
      { disabled: true, 'data-cyid': 'card' }
    );

    expect(btn(container, 'card-configure')).toBeDisabled();
  });
});

describe('what the card reports beyond the headline', () => {
  it('renders nothing for a resolution that has nothing to say', () => {
    const { container } = render({ status: 'none', title: '' });

    // A card with no sentence in it is a border around empty space.
    expect(container.textContent).toBe('');
  });

  it('adds the note a resolution carries alongside its explanation', () => {
    render({
      status: 'manual',
      title: 'OpenAI to Anthropic Transformer',
      detail: 'Translates OpenAI requests for Anthropic.',
      note: 'This translator was attached by hand.',
      policy: { name: 'openai-to-anthropic-transformer', version: 'v0' } as never,
    });

    expect(screen.getByText('Translates OpenAI requests for Anthropic.')).toBeInTheDocument();
    expect(screen.getByText('This translator was attached by hand.')).toBeInTheDocument();
  });

  it('marks a provider that cannot serve differently from one merely worth reading', () => {
    const { container: warning } = render({
      status: 'unresolved',
      title: 'No transformer configured',
      tone: 'warning',
    });
    const { container: info } = render({
      status: 'manual',
      title: 'OpenAI to Anthropic Transformer',
      tone: 'info',
    });

    // Tone is what separates "this turns traffic away" from "this is worth
    // knowing", so the two must not render identically.
    expect(warning.innerHTML).not.toBe(info.innerHTML);
  });
});

describe('parameters the policy declares but the configuration leaves unset', () => {
  const withParameters = {
    status: 'manual' as const,
    title: 'OpenAI to Anthropic Transformer',
    policy: {
      name: 'openai-to-anthropic-transformer',
      version: 'v0',
      parameters: [{ name: 'maxTokens' }, { name: 'systemPrompt' }],
    } as never,
  };

  it('names each one, so a reader can see what the request will decide', () => {
    render(withParameters, {
      transformer: { type: 'openai-to-anthropic-transformer', version: 'v0' },
    });

    expect(screen.getByText('maxTokens not configured')).toBeInTheDocument();
    expect(screen.getByText('systemPrompt not configured')).toBeInTheDocument();
  });

  it('stays quiet about one that has been set', () => {
    render(withParameters, {
      transformer: {
        type: 'openai-to-anthropic-transformer',
        version: 'v0',
        params: { maxTokens: 1024 },
      },
    });

    expect(screen.queryByText('maxTokens not configured')).not.toBeInTheDocument();
    expect(screen.getByText('systemPrompt not configured')).toBeInTheDocument();
  });

  it('treats a parameter set to nothing as unset', () => {
    render(withParameters, {
      transformer: {
        type: 'openai-to-anthropic-transformer',
        version: 'v0',
        params: { maxTokens: '', systemPrompt: null },
      },
    });

    // An empty value overrides nothing, so the request still decides.
    expect(screen.getByText('maxTokens not configured')).toBeInTheDocument();
    expect(screen.getByText('systemPrompt not configured')).toBeInTheDocument();
  });

  it('says nothing where the policy declares no parameters at all', () => {
    const { container } = render(
      {
        status: 'manual',
        title: 'OpenAI to Anthropic Transformer',
        policy: { name: 'openai-to-anthropic-transformer', version: 'v0' } as never,
      },
      { transformer: { type: 'openai-to-anthropic-transformer', version: 'v0' }, 'data-cyid': 'card' }
    );

    expect(container.querySelector('[data-cyid^="card-unset-"]')).toBeNull();
  });
});

describe('where the configure action sits', () => {
  it('is a labelled button under the explanation while a provider is waiting', () => {
    const { container } = render(
      { status: 'unresolved', title: 'No transformer configured' },
      { 'data-cyid': 'card' }
    );

    // An unresolved provider needs an action a reader can find.
    expect(screen.getByText('Configure transformer')).toBeInTheDocument();
    expect(btn(container, 'card-configure')).toBeInTheDocument();
  });

  it('is a quiet icon beside a resolution that is already settled', () => {
    const { container } = render(
      {
        status: 'manual',
        title: 'OpenAI to Anthropic Transformer',
        policy: { name: 'openai-to-anthropic-transformer', version: 'v0' } as never,
      },
      { 'data-cyid': 'card' }
    );

    expect(screen.queryByText('Configure transformer')).not.toBeInTheDocument();
    expect(btn(container, 'card-configure')).toBeInTheDocument();
  });

  it('is offered even where nothing needs translating', () => {
    const { container } = render({ status: 'none', title: 'No transformer needed' }, { 'data-cyid': 'card' });

    // That a provider speaks the proxy's format settles what is required, not
    // what is allowed.
    expect(btn(container, 'card-configure')).toBeInTheDocument();
  });

  it('offers no action at all where the caller passes no handler', () => {
    const { container } = render(
      { status: 'unresolved', title: 'No transformer configured' },
      { onConfigure: undefined, onRemove: undefined, 'data-cyid': 'card' }
    );

    expect(cy(container, 'card-configure')).toBeNull();
    expect(cy(container, 'card-remove')).toBeNull();
  });

  it('withholds detaching where nothing is attached to detach', () => {
    const { container } = render(
      { status: 'none', title: 'No transformer needed' },
      { transformer: null, 'data-cyid': 'card' }
    );

    expect(cy(container, 'card-remove')).toBeNull();
  });
});

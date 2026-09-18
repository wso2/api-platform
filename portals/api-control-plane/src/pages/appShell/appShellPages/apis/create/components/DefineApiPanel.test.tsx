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

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import { PLACEHOLDER_UPSTREAM_URL } from '../utils/apiSkeleton';
import { DefineApiPanel } from './DefineApiPanel';

// The contract view renders a spec preview pane. swagger-ui-react bundles its
// own copy of React, which react-dom refuses to render inside this suite ("a
// React Element from an older version of React"), and what that pane draws is
// covered by its own suite — so it is stubbed rather than worked around.
vi.mock('swagger-ui-react', () => ({ default: () => null }));

// The same pane's Source view is Monaco, which needs a canvas and real font
// metrics, neither of which jsdom has. Standing it in with a text area keeps
// the contract view renderable; none of these tests read from it.
vi.mock('@/components/CodeEditor/CodeEditor', () => ({
  CodeEditor: ({
    ariaLabel,
    onChange,
    readOnly,
    value,
  }: {
    ariaLabel?: string;
    onChange?: (next: string) => void;
    readOnly?: boolean;
    value: string;
  }) => (
    <textarea
      aria-label={ariaLabel}
      onChange={(event) => onChange?.(event.target.value)}
      readOnly={readOnly}
      value={value}
    />
  ),
}));

/**
 * The step offers two approaches side by side. "Start from Scratch" resolves
 * entirely inside this panel — a skeleton document plus an optional backend —
 * so it is the half these tests drive; the contract half delegates to
 * `ContractSourceForm`, which has its own suite.
 */
const renderPanel = (onDraftChange = vi.fn()) => {
  const { user } = renderWithProviders(<DefineApiPanel onDraftChange={onDraftChange} />);
  return { onDraftChange, user };
};

/** The draft the panel last handed the wizard footer. */
const lastDraft = (onDraftChange: ReturnType<typeof vi.fn>) =>
  onDraftChange.mock.calls.at(-1)?.[0];

describe('DefineApiPanel — choosing an approach', () => {
  it('offers nothing to continue with until an approach is picked', () => {
    const { onDraftChange } = renderPanel();

    expect(lastDraft(onDraftChange)).toBeNull();
  });

  it('hands over the skeleton and a placeholder backend for a scratch API', async () => {
    const { onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('button', { name: /Start from Scratch/ }));

    const draft = lastDraft(onDraftChange);
    expect(draft.upstream).toEqual({ main: { url: PLACEHOLDER_UPSTREAM_URL } });
    // The skeleton travels as a file: the create step submits it to
    // import-openapi exactly as it would an imported contract.
    expect(draft.contractImport.specFile).toBeInstanceOf(File);
  });

  it('uses the endpoint the user gave instead of the placeholder', async () => {
    const { onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('radio', { name: 'I have an endpoint URL' }));
    await user.type(screen.getByRole('textbox'), 'https://orders.example.com');

    expect(lastDraft(onDraftChange).upstream).toEqual({
      main: { url: 'https://orders.example.com' },
    });
  });

  it('opens the contract form on the other card, and comes back from it', async () => {
    const { onDraftChange, user } = renderPanel();

    await user.click(screen.getByRole('button', { name: /Start with a Contract/ }));

    // Nothing is fetched yet, so there is no draft to continue with.
    expect(screen.getByText('Point us at your contract')).toBeInTheDocument();
    expect(lastDraft(onDraftChange)).toBeNull();

    await user.click(screen.getByRole('button', { name: 'Change source' }));
    expect(screen.getByRole('button', { name: /Start with a Contract/ })).toBeInTheDocument();
  });
});

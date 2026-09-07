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

import { fireEvent, renderWithProviders, screen } from '@/test/utils';
import { DefineApiPanel } from './DefineApiPanel';

// swagger-ui-react bundles its own copy of React, which react-dom refuses to
// render inside this suite ("a React Element from an older version of React").
// The rendered resources are not what these tests are about — the pane's other
// half is; so the component is stubbed rather than the assertions bent around
// it. The Source view under test renders no Swagger UI at all.
vi.mock('swagger-ui-react', () => ({ default: () => null }));

// Monaco needs a canvas and real font metrics, neither of which jsdom has.
// These tests are about what the step does with an edited definition, not
// about how the text got typed, so the editor is a plain text area here.
vi.mock('./SpecCodeEditor', () => ({
  SpecCodeEditor: ({
    onChange,
    readOnly,
    value,
  }: {
    onChange?: (next: string) => void;
    readOnly?: boolean;
    value: string;
  }) => (
    <textarea
      aria-label="API definition source"
      onChange={(event) => onChange?.(event.target.value)}
      readOnly={readOnly}
      value={value}
    />
  ),
}));

/**
 * The step is exercised through "Design from scratch": it is the approach that
 * has a definition on screen without a fetch standing between the test and the
 * editor, and the edit path under test is the same one a fetched contract takes.
 */
const openScratchSource = async (onDataFetched = vi.fn()) => {
  const { user } = renderWithProviders(<DefineApiPanel onDataFetched={onDataFetched} />);

  await user.click(screen.getByRole('button', { name: /Design from scratch/ }));
  await user.click(screen.getByRole('checkbox', { name: 'Source' }));
  return { onDataFetched, user };
};

/** The editor arrives in its own chunk, so the first look at it is awaited. */
const editor = async (): Promise<HTMLTextAreaElement> =>
  (await screen.findByRole('textbox', {
    name: 'API definition source',
  })) as HTMLTextAreaElement;

describe('DefineApiPanel — editing the definition', () => {
  it('carries the edited definition forward instead of the one it started from', async () => {
    const { onDataFetched, user } = await openScratchSource();

    await user.click(screen.getByRole('button', { name: 'Edit' }));
    fireEvent.change(await editor(), {
      target: {
        value: JSON.stringify({
          openapi: '3.0.3',
          info: { title: 'Edited by hand', version: '3.2.1' },
          servers: [{ url: 'https://orders.example.com' }],
          paths: { '/orders': { get: { responses: { '200': { description: 'A page.' } } } } },
        }),
      },
    });
    await user.click(screen.getByRole('button', { name: 'Save' }));
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    expect(onDataFetched).toHaveBeenCalledTimes(1);
    expect(onDataFetched.mock.calls[0][0]).toMatchObject({
      displayName: 'Edited by hand',
      upstream: { main: { url: 'https://orders.example.com' } },
      version: '3.2.1',
    });
  });

  it('reports what the edited definition is missing, once it is the one on screen', async () => {
    const { user } = await openScratchSource();

    await user.click(screen.getByRole('button', { name: 'Edit' }));
    // Importable, but with nothing to read a backend off.
    fireEvent.change(await editor(), {
      target: {
        value: JSON.stringify({
          openapi: '3.0.3',
          info: { title: 'No backend', version: '1.0.0' },
          paths: { '/orders': { get: { responses: { '200': { description: 'A page.' } } } } },
        }),
      },
    });
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText(/No server URL in this definition/)).toBeInTheDocument();
  });

  it('keeps each approach’s edit while the other one is looked at', async () => {
    const { user } = await openScratchSource();

    await user.click(screen.getByRole('button', { name: 'Edit' }));
    fireEvent.change(await editor(), {
      target: {
        value: JSON.stringify({
          openapi: '3.0.3',
          info: { title: 'Scratch edit', version: '1.0.0' },
          paths: { '/orders': { get: { responses: { '200': { description: 'A page.' } } } } },
        }),
      },
    });
    await user.click(screen.getByRole('button', { name: 'Save' }));

    // Over to the contract approach, which has nothing fetched, and back.
    await user.click(screen.getByRole('button', { name: /Start with a contract/ }));
    await user.click(screen.getByRole('button', { name: /Design from scratch/ }));

    await user.click(screen.getByRole('button', { name: 'Edit' }));
    expect((await editor()).value).toContain('Scratch edit');
  });
});

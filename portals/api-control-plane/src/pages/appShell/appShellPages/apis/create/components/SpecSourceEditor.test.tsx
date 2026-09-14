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
import { SpecSourceEditor } from './SpecSourceEditor';

// Monaco measures fonts and lays out on a canvas, neither of which jsdom has,
// so the editor is stood in for by a plain text area carrying the same
// accessible name and the same value/onChange contract. What these tests are
// about; reading text back, checking it, and what Save does with the result —
// lives entirely on this side of that boundary.
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

/** A definition that passes `validateApiSpec` with no warnings at all. */
const VALID_SPEC = {
  openapi: '3.0.3',
  info: { title: 'Orders API', version: '1.0.0' },
  servers: [{ url: 'https://orders.example.com' }],
  paths: { '/orders': { get: { responses: { '200': { description: 'A page.' } } } } },
};

/** The editor arrives in its own chunk, so the first look at it is awaited. */
const editor = async (): Promise<HTMLTextAreaElement> =>
  (await screen.findByRole('textbox', {
    name: 'API definition source',
  })) as HTMLTextAreaElement;

/** Replaces the whole document, the way selecting all and pasting would. */
const retype = async (text: string) =>
  fireEvent.change(await editor(), { target: { value: text } });

const openEditor = async (onSave = vi.fn(), spec: Record<string, unknown> = VALID_SPEC) => {
  const { user } = renderWithProviders(<SpecSourceEditor onSave={onSave} spec={spec} />);
  await user.click(screen.getByRole('button', { name: 'Edit' }));
  return { onSave, user };
};

describe('SpecSourceEditor', () => {
  it('adopts an edited definition, with the warnings its own re-check raised', async () => {
    const { onSave, user } = await openEditor();

    // Same document, minus the servers block — still importable, but the
    // re-check has something new to say about it.
    await retype(
      JSON.stringify({
        ...VALID_SPEC,
        info: { title: 'Renamed API', version: '2.0.0' },
        servers: [],
      }),
    );
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(onSave).toHaveBeenCalledTimes(1);
    const [spec, warnings] = onSave.mock.calls[0];
    expect(spec.info).toEqual({ title: 'Renamed API', version: '2.0.0' });
    expect(warnings.map((warning: { code: string }) => warning.code)).toContain('noServers');

    // Saving closes the editor and hands the document back to the pane.
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
  });

  it('refuses to save text that is not valid JSON, and says which line', async () => {
    const { onSave, user } = await openEditor();

    await retype('{ "openapi": "3.0.3", }');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText(/could not be read as JSON/)).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();
    // The editor stays open over the text that needs fixing.
    expect(await editor()).toBeInTheDocument();
  });

  it('refuses to save a document the wizard could not use, naming what is wrong', async () => {
    const { onSave, user } = await openEditor();

    // Parses fine, but declares no operation — an error, not a warning.
    await retype(
      JSON.stringify({ openapi: '3.0.3', info: { title: 'A', version: '1' }, paths: {} }),
    );
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(
      await screen.findByText(/declares no GET, POST, PUT, PATCH or DELETE operation/),
    ).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();
  });

  it('rejects a top level that is a list rather than an object', async () => {
    const { onSave, user } = await openEditor();

    await retype('[1, 2, 3]');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText(/has to be an object at the top level/)).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();
  });

  it('carries unsaved edits across a switch to YAML rather than discarding them', async () => {
    const { onSave, user } = await openEditor();

    await retype(
      JSON.stringify({ ...VALID_SPEC, info: { title: 'Half typed', version: '1.0.0' } }),
    );
    await user.click(screen.getByRole('button', { name: 'YAML' }));

    // Re-printed as YAML, with the edit intact.
    expect((await editor()).value).toContain('title: Half typed');
    expect((await editor()).value).not.toContain('"title"');

    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(onSave.mock.calls[0][0].info.title).toBe('Half typed');
  });

  it('parses YAML on save, so a definition can be written in it', async () => {
    const { onSave, user } = await openEditor();

    await user.click(screen.getByRole('button', { name: 'YAML' }));
    await retype(
      [
        'openapi: 3.0.3',
        'info:',
        '  title: Written in YAML',
        '  version: 1.0.0',
        'servers:',
        '  - url: https://orders.example.com',
        'paths:',
        '  /orders:',
        '    get:',
        '      responses:',
        "        '200':",
        '          description: A page.',
      ].join('\n'),
    );
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave.mock.calls[0][0].info.title).toBe('Written in YAML');
  });

  it('refuses the format switch while the text does not parse, keeping it as typed', async () => {
    const { user } = await openEditor();

    await retype('{ not: json');
    await user.click(screen.getByRole('button', { name: 'YAML' }));

    expect(await screen.findByText(/could not be read as JSON/)).toBeInTheDocument();
    // Nothing was re-printed over what the user is in the middle of fixing.
    expect((await editor()).value).toBe('{ not: json');
  });

  it('discards the edit on Cancel, leaving the document alone', async () => {
    const { onSave, user } = await openEditor();

    await retype(JSON.stringify({ ...VALID_SPEC, info: { title: 'Discarded', version: '1.0.0' } }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(onSave).not.toHaveBeenCalled();

    // Re-opening starts from the document again, not from the abandoned draft.
    await user.click(screen.getByRole('button', { name: 'Edit' }));
    expect((await editor()).value).toContain('Orders API');
    expect((await editor()).value).not.toContain('Discarded');
  });
});

describe('SpecSourceEditor — reading and the expanded panel', () => {
  it('prints the document as YAML without having to enter edit mode', async () => {
    const { user } = renderWithProviders(<SpecSourceEditor onSave={vi.fn()} spec={VALID_SPEC} />);

    expect((await editor()).value).toContain('"openapi": "3.0.3"');
    await user.click(screen.getByRole('button', { name: 'YAML' }));

    expect((await editor()).value).toContain('openapi: 3.0.3');
    // Still read-only: the document is being looked at, not edited.
    expect(await editor()).toHaveAttribute('readonly');
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
  });

  it('moves the editor into the panel rather than leaving a second copy behind', async () => {
    const { user } = renderWithProviders(<SpecSourceEditor onSave={vi.fn()} spec={VALID_SPEC} />);

    await user.click(screen.getByRole('button', { name: 'Expand editor' }));

    // Exactly one editor and one set of controls, wherever they now live —
    // two would be ambiguous to anyone reaching them by name.
    expect(screen.getAllByRole('textbox', { name: 'API definition source' })).toHaveLength(1);
    expect(screen.getAllByRole('button', { name: 'Edit' })).toHaveLength(1);
    expect(screen.getByText('This definition is open in the expanded editor.')).toBeInTheDocument();
  });

  it('carries an unsaved edit between the pane and the panel, in both directions', async () => {
    const { onSave, user } = await openEditor();

    await retype(
      JSON.stringify({ ...VALID_SPEC, info: { title: 'Typed in the pane', version: '1.0.0' } }),
    );
    await user.click(screen.getByRole('button', { name: 'Expand editor' }));
    expect((await editor()).value).toContain('Typed in the pane');

    await user.click(screen.getByRole('button', { name: 'Collapse editor' }));
    expect((await editor()).value).toContain('Typed in the pane');

    // And it is still the edit that gets saved, not the document it started from.
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(onSave.mock.calls[0][0].info.title).toBe('Typed in the pane');
  });

  it('saves from inside the panel, closing the editor behind it', async () => {
    const { onSave, user } = await openEditor();

    await user.click(screen.getByRole('button', { name: 'Expand editor' }));
    await retype(
      JSON.stringify({ ...VALID_SPEC, info: { title: 'Saved while expanded', version: '4.0.0' } }),
    );
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave.mock.calls[0][0].info.version).toBe('4.0.0');
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
  });
});

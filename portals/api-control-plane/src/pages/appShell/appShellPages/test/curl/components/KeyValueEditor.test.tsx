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

import { useState } from 'react';
import { describe, expect, it } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import { KeyValueEditor } from './KeyValueEditor';
import type { KeyValueRow } from '../../utils/types';

/**
 * Tests for the query/header/body-field table.
 *
 * The first block is a regression test for a bug that made every one of those
 * tables unusable: the trailing "type here" row was a pair of inputs hardcoded
 * to `value=""` that appended a brand-new row on each `onChange`. Because the
 * inputs never held what was typed, focus never left them, so typing `test`
 * produced four rows reading `t`, `e`, `s`, `t`.
 *
 * These tests drive the component through a real controlled parent, because
 * that is the only way to catch it — asserting on a single `onChange` call
 * would have passed against the broken version.
 */

/** Hosts the editor the way the cURL builder does: state lives in the parent. */
function Harness({
  initial = [],
  nameOptions,
}: {
  initial?: KeyValueRow[];
  nameOptions?: readonly string[];
}) {
  const [rows, setRows] = useState<KeyValueRow[]>(initial);

  return (
    <>
      <KeyValueEditor
        helperText="help"
        nameOptions={nameOptions}
        namePlaceholder="New field"
        onChange={setRows}
        rows={rows}
        valuePlaceholder="Value"
      />
      {/* The committed state, so assertions read what the parent actually holds
          rather than what the inputs happen to display. */}
      <pre data-testid="rows">{JSON.stringify(rows.map((r) => [r.name, r.value]))}</pre>
    </>
  );
}

const committed = (): [string, string][] =>
  JSON.parse(screen.getByTestId('rows').textContent || '[]');

/** The blank row's name input is always the last one rendered. */
const lastNameInput = (): HTMLElement => {
  const inputs = screen.getAllByLabelText('Key');
  return inputs[inputs.length - 1];
};

describe('KeyValueEditor — typing into the blank row', () => {
  it('builds one row from a multi-character name', async () => {
    const { user } = renderWithProviders(<Harness />);

    await user.type(lastNameInput(), 'test');

    // The bug produced [['t',''],['e',''],['s',''],['t','']].
    expect(committed()).toEqual([['test', '']]);
  });

  it('keeps focus in the same input across keystrokes', async () => {
    const { user } = renderWithProviders(<Harness />);

    const input = lastNameInput();
    await user.type(input, 'abc');

    // Losing focus is what caused the one-row-per-character behaviour, so this
    // is the property that actually has to hold.
    expect(input).toHaveFocus();
    expect(input).toHaveValue('abc');
  });

  it('offers a fresh blank row once the previous one is used', async () => {
    const { user } = renderWithProviders(<Harness />);

    expect(screen.getAllByLabelText('Key')).toHaveLength(1);

    await user.type(lastNameInput(), 'first');

    // One committed row plus a new blank to type the next one into.
    expect(screen.getAllByLabelText('Key')).toHaveLength(2);
    expect(committed()).toEqual([['first', '']]);
  });

  it('builds a second row without disturbing the first', async () => {
    const { user } = renderWithProviders(<Harness />);

    await user.type(lastNameInput(), 'first');
    await user.type(lastNameInput(), 'second');

    expect(committed()).toEqual([
      ['first', ''],
      ['second', ''],
    ]);
  });

  it('accepts a value typed before a name', async () => {
    const { user } = renderWithProviders(<Harness />);

    const values = screen.getAllByLabelText('Value');
    await user.type(values[values.length - 1], 'orphan');

    expect(committed()).toEqual([['', 'orphan']]);
  });

  it('types a full name and value into one row', async () => {
    const { user } = renderWithProviders(<Harness />);

    await user.type(lastNameInput(), 'limit');
    const values = screen.getAllByLabelText('Value');
    await user.type(values[0], '10');

    expect(committed()).toEqual([['limit', '10']]);
  });
});

describe('KeyValueEditor — existing rows', () => {
  const rows: KeyValueRow[] = [
    { enabled: true, id: 'r1', name: 'limit', value: '10' },
    { enabled: true, id: 'r2', name: 'status', value: 'paid' },
  ];

  it('renders one blank row after the supplied ones', () => {
    renderWithProviders(<Harness initial={rows} />);

    expect(screen.getAllByLabelText('Key')).toHaveLength(3);
  });

  it('does not add a second blank row when one is already trailing', () => {
    renderWithProviders(
      <Harness initial={[...rows, { enabled: true, id: 'r3', name: '', value: '' }]} />,
    );

    // Three supplied rows, the last already blank — so no extra is appended.
    expect(screen.getAllByLabelText('Key')).toHaveLength(3);
  });

  it('edits an existing row in place rather than appending', async () => {
    const { user } = renderWithProviders(<Harness initial={rows} />);

    await user.type(screen.getAllByLabelText('Key')[0], '!');

    expect(committed()).toEqual([
      ['limit!', '10'],
      ['status', 'paid'],
    ]);
  });

  it('removes a row', async () => {
    const { user } = renderWithProviders(<Harness initial={rows} />);

    await user.click(screen.getAllByRole('button', { name: /Remove row/i })[0]);

    expect(committed()).toEqual([['status', 'paid']]);
  });

  it('offers no remove button on the trailing blank row', () => {
    renderWithProviders(<Harness initial={rows} />);

    // Two supplied rows are removable; the blank one has nothing to remove.
    expect(screen.getAllByRole('button', { name: /Remove row/i })).toHaveLength(2);
  });

  it('excludes a row without deleting it', async () => {
    const { user } = renderWithProviders(<Harness initial={rows} />);

    await user.click(screen.getAllByRole('checkbox')[0]);

    // Still present, so it can be put back without retyping.
    expect(committed()).toHaveLength(2);
  });
});

describe('KeyValueEditor — picking a name from the offered list', () => {
  const options = ['Accept', 'Authorization', 'Content-Type'];

  it('offers no picker when no names are supplied', () => {
    renderWithProviders(<Harness />);

    expect(screen.queryByRole('button', { name: /Pick a standard name/i })).toBeNull();
  });

  it('commits the picked name to the row', async () => {
    const { user } = renderWithProviders(<Harness nameOptions={options} />);

    await user.click(screen.getByRole('button', { name: /Pick a standard name/i }));
    await user.click(await screen.findByRole('option', { name: 'Content-Type' }));

    expect(committed()).toEqual([['Content-Type', '']]);
  });

  it('narrows the list to what has been typed', async () => {
    const { user } = renderWithProviders(<Harness nameOptions={options} />);

    await user.type(lastNameInput(), 'auth');

    expect(await screen.findByRole('option', { name: 'Authorization' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Accept' })).toBeNull();
  });

  it('still accepts a name that is not on the list', async () => {
    const { user } = renderWithProviders(<Harness nameOptions={options} />);

    // The picker adds a way to fill the field; it does not turn it into a
    // closed set, since a backend may read any header it likes.
    await user.type(lastNameInput(), 'X-Tenant-Id');

    expect(committed()).toEqual([['X-Tenant-Id', '']]);
  });

  it('leaves an existing row editable by hand', async () => {
    const { user } = renderWithProviders(
      <Harness
        initial={[{ enabled: true, id: 'r1', name: 'Accept', value: '*/*' }]}
        nameOptions={options}
      />,
    );

    await user.type(screen.getAllByLabelText('Key')[0], '-Language');

    expect(committed()).toEqual([['Accept-Language', '*/*']]);
  });
});

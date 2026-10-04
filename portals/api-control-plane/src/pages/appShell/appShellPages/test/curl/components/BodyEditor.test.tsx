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
import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import { BodyEditor } from './BodyEditor';
import type { BodyMode, KeyValueRow, RawFormat } from '../../utils/types';

/**
 * Monaco does not run in jsdom, so the editor is a plain text area here — the
 * same substitution the creation wizard's tests make.
 *
 * The mock still surfaces the two props the body editor is responsible for
 * choosing: `language`, which is the whole point of picking a format, and
 * `ariaLabel`, which is what gives the field a name. Asserting on them is what
 * makes these tests about this component rather than about Monaco.
 */
vi.mock('@/components/CodeEditor/CodeEditor', () => ({
  CodeEditor: ({
    ariaLabel,
    language,
    onChange,
    value,
  }: {
    ariaLabel?: string;
    language: string;
    onChange?: (next: string) => void;
    value: string;
  }) => (
    <textarea
      aria-label={ariaLabel}
      data-language={language}
      onChange={(event) => onChange?.(event.target.value)}
      value={value}
    />
  ),
}));

/** Hosts the editor the way the cURL builder does: state lives in the parent. */
function Harness({
  initialFormat = 'json',
  initialMode = 'raw',
  initialValue = '',
  sample,
}: {
  initialFormat?: RawFormat;
  initialMode?: BodyMode;
  initialValue?: string;
  sample?: string;
}) {
  const [mode, setMode] = useState<BodyMode>(initialMode);
  const [rawFormat, setRawFormat] = useState<RawFormat>(initialFormat);
  const [value, setValue] = useState(initialValue);
  const [fields, setFields] = useState<KeyValueRow[]>([]);

  return (
    <BodyEditor
      fields={fields}
      mode={mode}
      onChange={setValue}
      onFieldsChange={setFields}
      onModeChange={setMode}
      onRawFormatChange={setRawFormat}
      rawFormat={rawFormat}
      sample={sample}
      value={value}
    />
  );
}

/** The mocked editor, found by the name the component gives it. */
const editor = () => screen.findByRole('textbox', { name: /request body/i });

describe('BodyEditor — the raw body editor', () => {
  it('hands Monaco the language matching the selected format', async () => {
    renderWithProviders(<Harness initialFormat="xml" />);

    // The reason a code editor was worth loading at all: an XML body has to be
    // highlighted as XML, not as whatever the previous format was.
    expect(await editor()).toHaveAttribute('data-language', 'xml');
  });

  it('switches the language when the format dropdown changes', async () => {
    const { user } = renderWithProviders(<Harness initialFormat="json" />);

    await editor();
    await user.click(screen.getByRole('combobox', { name: /raw body format/i }));
    await user.click(screen.getByRole('option', { name: 'XML' }));

    expect(await editor()).toHaveAttribute('data-language', 'xml');
  });

  it('maps plain text to plaintext rather than leaving it unset', async () => {
    // 'text' is this app's name for the format and 'plaintext' is Monaco's;
    // passing ours through would silently give the editor no grammar at all.
    renderWithProviders(<Harness initialFormat="text" />);

    expect(await editor()).toHaveAttribute('data-language', 'plaintext');
  });

  it('reports what was typed to the parent', async () => {
    const { user } = renderWithProviders(<Harness />);

    await user.type(await editor(), '{{"a":1}');

    expect(await editor()).toHaveValue('{"a":1}');
  });

  it('reports a malformed body without blocking editing', async () => {
    renderWithProviders(<Harness initialValue="{" />);

    await editor();
    expect(await screen.findByText(/invalid json/i)).toBeInTheDocument();
  });

  it('confirms a well-formed body', async () => {
    renderWithProviders(<Harness initialValue='{"a":1}' />);

    await editor();
    expect(await screen.findByText(/valid json/i)).toBeInTheDocument();
  });

  it('re-indents JSON in place when Format is pressed', async () => {
    const { user } = renderWithProviders(<Harness initialValue='{"a":1}' />);

    await editor();
    await user.click(screen.getByRole('button', { name: /format/i }));

    expect(await editor()).toHaveValue('{\n  "a": 1\n}');
  });

  it('does not mount an editor when the request carries no body', () => {
    renderWithProviders(<Harness initialMode="none" />);

    // Worth its own test because the editor is lazily imported: rendering it in
    // 'none' mode would fetch ~3.9 MB of Monaco for a request that has no body.
    expect(screen.queryByRole('textbox', { name: /request body/i })).not.toBeInTheDocument();
  });
});

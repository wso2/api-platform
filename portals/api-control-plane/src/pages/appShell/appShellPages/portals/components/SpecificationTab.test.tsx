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
import type { SpecFormat } from '../../apis/create/utils/specText';
import { SpecificationTab } from './SpecificationTab';

// Monaco does not run in jsdom; a textarea stands in for it.
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

const JSON_DEFINITION = JSON.stringify({ openapi: '3.0.3', info: { title: 'Loans' } }, null, 2);

function Harness({
  initialText,
  initialFormat = 'json',
  parseError,
}: {
  initialText: string;
  initialFormat?: SpecFormat;
  parseError?: string;
}) {
  const [text, setText] = useState(initialText);
  const [format, setFormat] = useState<SpecFormat>(initialFormat);
  return (
    <SpecificationTab
      format={format}
      onChange={setText}
      onFormatChange={setFormat}
      parseError={parseError}
      text={text}
    />
  );
}

describe('SpecificationTab', () => {
  it('opens an existing definition read-only until Edit is clicked', async () => {
    const { user } = renderWithProviders(<Harness initialText={JSON_DEFINITION} />);

    const editor = await screen.findByRole('textbox', { name: 'API definition (JSON)' });
    expect(editor).toHaveAttribute('readonly');

    await user.click(screen.getByRole('button', { name: 'Edit' }));

    expect(screen.getByRole('textbox', { name: 'API definition (JSON)' })).not.toHaveAttribute(
      'readonly',
    );
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
  });

  it('is editable straight away, with no Edit button, when there is no definition yet', async () => {
    renderWithProviders(<Harness initialText="" />);

    const editor = await screen.findByRole('textbox', { name: 'API definition (JSON)' });
    expect(editor).not.toHaveAttribute('readonly');
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
  });

  it('switches a definition to YAML and back, re-printing it in the chosen format', async () => {
    const { user } = renderWithProviders(<Harness initialText={JSON_DEFINITION} />);
    await screen.findByRole('textbox', { name: 'API definition (JSON)' });

    await user.click(screen.getByRole('button', { name: 'YAML' }));

    const yamlEditor = await screen.findByRole('textbox', { name: 'API definition (YAML)' });
    expect(yamlEditor).toHaveValue('openapi: 3.0.3\ninfo:\n  title: Loans\n');
    expect(screen.getByRole('button', { name: 'YAML' })).toHaveAttribute('aria-pressed', 'true');

    await user.click(screen.getByRole('button', { name: 'JSON' }));

    expect(await screen.findByRole('textbox', { name: 'API definition (JSON)' })).toHaveValue(
      JSON_DEFINITION,
    );
  });

  it('only changes the language when the text cannot be read, leaving it untouched', async () => {
    const broken = '{ "openapi": ';
    const { user } = renderWithProviders(<Harness initialText={broken} />);
    await screen.findByRole('textbox', { name: 'API definition (JSON)' });

    await user.click(screen.getByRole('button', { name: 'YAML' }));

    expect(await screen.findByRole('textbox', { name: 'API definition (YAML)' })).toHaveValue(
      broken,
    );
  });

  it('offers no import, download or resources view', async () => {
    renderWithProviders(<Harness initialText={JSON_DEFINITION} />);
    await screen.findByRole('textbox', { name: 'API definition (JSON)' });

    for (const name of [/import/i, /download/i, /upload/i, /resources/i, /add resource/i]) {
      expect(screen.queryByRole('button', { name })).not.toBeInTheDocument();
    }
  });

  it('names the format and the parser’s complaint, and stays editable so it can be fixed', async () => {
    renderWithProviders(
      <Harness
        initialFormat="yaml"
        initialText="openapi: ["
        parseError="unexpected end of the stream"
      />,
    );

    expect(
      await screen.findByText('This is not valid YAML: unexpected end of the stream'),
    ).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'API definition (YAML)' })).not.toHaveAttribute(
      'readonly',
    );
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
  });
});

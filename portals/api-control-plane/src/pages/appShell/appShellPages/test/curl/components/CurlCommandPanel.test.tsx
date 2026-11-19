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

import { renderWithProviders, screen } from '@/test/utils';
import type { ConsoleRequest, KeyValueRow } from '../../utils/types';
import { CurlCommandPanel } from './CurlCommandPanel';

const fileRow = (name: string, filename: string): KeyValueRow => ({
  enabled: true,
  file: true,
  id: 'f1',
  name,
  value: filename,
});

const request = (overrides: Partial<ConsoleRequest> = {}): ConsoleRequest => ({
  baseUrl: 'https://gw.example.com/p',
  body: '',
  bodyMode: 'none',
  formFields: [],
  headers: [],
  method: 'POST',
  path: '/orders',
  queryParams: [],
  rawFormat: 'json',
  ...overrides,
});

describe('CurlCommandPanel', () => {
  it('shows the command, with URL globbing turned off', () => {
    renderWithProviders(<CurlCommandPanel request={request()} />);

    // The code block splits the command into highlighted spans, so read its whole text.
    expect(document.body.textContent).toContain('curl --globoff -X POST');
  });

  it('describes a mirrored binary upload', () => {
    renderWithProviders(
      <CurlCommandPanel request={request({ binaryFile: 'photo.png', bodyMode: 'binary' })} />,
    );

    expect(screen.getByText(/binary body/)).toBeInTheDocument();
    expect(screen.getByText(/--data-binary/)).toBeInTheDocument();
  });

  it('shows a warning instead of a command that would send the wrong file', () => {
    renderWithProviders(
      <CurlCommandPanel
        request={request({
          bodyMode: 'url-encoded',
          formFields: [fileRow('upload', 'report=final.csv')],
        })}
      />,
    );

    expect(screen.getByRole('alert')).toHaveTextContent('report=final.csv');
    expect(document.body.textContent).not.toContain('curl --globoff');
    expect(screen.queryByRole('button', { name: /copy/i })).toBeNull();
    // Nor does it describe a command that is not there.
    expect(screen.queryByText(/no headers/)).toBeNull();
  });
});

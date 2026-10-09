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
import { CopyableCommand } from './CopyableCommand';

describe('CopyableCommand', () => {
  it('overrides native copy with copyCode when placeholders are shown', () => {
    const setData = vi.fn();
    renderWithProviders(
      <CopyableCommand
        code="MOESIF_KEY=<your-moesif-key>"
        copyCode="MOESIF_KEY=real-secret"
      />,
    );

    fireEvent.copy(screen.getByTestId('copyable-command-body'), {
      clipboardData: { setData },
    });

    expect(setData).toHaveBeenCalledWith('text/plain', 'MOESIF_KEY=real-secret');
  });

  it('does not override native copy when display and copy text match', () => {
    const setData = vi.fn();
    renderWithProviders(<CopyableCommand code="cd gateway && docker compose up" />);

    fireEvent.copy(screen.getByTestId('copyable-command-body'), {
      clipboardData: { setData },
    });

    expect(setData).not.toHaveBeenCalled();
  });
});

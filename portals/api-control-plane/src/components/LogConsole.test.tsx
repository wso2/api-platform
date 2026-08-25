/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { afterEach, describe, expect, it, vi } from 'vitest';

import { fireEvent, renderWithProviders, screen, within } from '../test/utils';
import {
  consoleLineText,
  formatConsoleTimestamp,
  LogConsole,
  type ConsoleLine,
} from './LogConsole';

const line = (overrides: Partial<ConsoleLine> = {}): ConsoleLine => ({
  id: 'line-1',
  level: 'INFO',
  message: 'GET /orders -> 200',
  timestamp: '2026-08-22T10:00:00.500Z',
  ...overrides,
});

const renderConsole = (props: Partial<Parameters<typeof LogConsole>[0]> = {}) =>
  renderWithProviders(
    <LogConsole
      emptyMessage="Nothing yet"
      label="Gateway log output"
      lines={[line()]}
      {...props}
    />
  );

/*
 * `userEvent.setup()` installs its own `navigator.clipboard`, so these tests
 * redefine the property after render and drive the click with `fireEvent`.
 */
const originalClipboard = Object.getOwnPropertyDescriptor(
  navigator,
  'clipboard'
);

const stubClipboard = (clipboard: unknown) => {
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: clipboard,
    writable: true,
  });
};

afterEach(() => {
  if (originalClipboard) {
    Object.defineProperty(navigator, 'clipboard', originalClipboard);
  } else {
    delete (navigator as { clipboard?: unknown }).clipboard;
  }
});

describe('formatConsoleTimestamp', () => {
  it('renders a fixed-width local timestamp', () => {
    // Built from local parts, so assert the shape rather than a fixed zone.
    expect(formatConsoleTimestamp('2026-08-22T10:00:00.500Z')).toMatch(
      /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.500$/
    );
  });

  it('pads a missing or unparseable timestamp to the same width', () => {
    expect(formatConsoleTimestamp()).toHaveLength(23);
    expect(formatConsoleTimestamp('not a date')).toHaveLength(23);
  });
});

describe('consoleLineText', () => {
  it('aligns the level column and uses the summary by default', () => {
    expect(consoleLineText(line(), false)).toMatch(/ INFO {2}GET \/orders -> 200$/);
  });

  it('uses the raw payload when raw mode is on', () => {
    expect(consoleLineText(line({ raw: '{"status":200}' }), true)).toMatch(
      /\{"status":200\}$/
    );
  });

  it('falls back to the summary when a line has no raw payload', () => {
    expect(consoleLineText(line(), true)).toMatch(/GET \/orders -> 200$/);
  });
});

describe('LogConsole', () => {
  it('renders each record as a selectable console row', () => {
    renderConsole();

    const output = screen.getByRole('log', { name: 'Gateway log output' });
    expect(within(output).getByText('GET /orders -> 200')).toBeInTheDocument();
    expect(within(output).getByText(/INFO/)).toBeInTheDocument();
  });

  it('shows the empty message with no records', () => {
    renderConsole({ lines: [] });

    expect(screen.getByText('Nothing yet')).toBeInTheDocument();
  });

  it('copies every rendered line to the clipboard', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    renderConsole({
      lines: [line(), line({ id: 'line-2', level: 'ERROR', message: 'boom' })],
    });
    stubClipboard({ writeText });

    fireEvent.click(screen.getByRole('button', { name: 'Copy all' }));

    expect(writeText).toHaveBeenCalledTimes(1);
    const copied = writeText.mock.calls[0][0] as string;
    expect(copied.split('\n')).toHaveLength(2);
    expect(copied).toContain('INFO  GET /orders -> 200');
    expect(copied).toContain('ERROR boom');
    expect(await screen.findByText('Copied')).toBeInTheDocument();
  });

  it('reports a blocked clipboard instead of failing silently', async () => {
    renderConsole();
    stubClipboard(undefined);

    fireEvent.click(screen.getByRole('button', { name: 'Copy all' }));

    expect(
      await screen.findByText(/Could not copy to the clipboard/)
    ).toBeInTheDocument();
  });

  it('offers the raw toggle only when a record carries a raw payload', () => {
    const { unmount } = renderConsole();
    expect(
      screen.queryByRole('button', { name: 'Show raw log payload' })
    ).not.toBeInTheDocument();
    unmount();

    renderConsole({ lines: [line({ raw: '{"status":200}' })] });
    expect(
      screen.getByRole('button', { name: 'Show raw log payload' })
    ).toBeInTheDocument();
  });

  it('switches rows to the raw payload when raw mode is toggled on', async () => {
    const { user } = renderConsole({
      lines: [line({ raw: '{"status":200}' })],
    });

    await user.click(
      screen.getByRole('button', { name: 'Show raw log payload' })
    );

    expect(screen.getByText('{"status":200}')).toBeInTheDocument();
    expect(screen.queryByText('GET /orders -> 200')).not.toBeInTheDocument();
  });

  it('clears through the owner and disables the control when empty', async () => {
    const onClear = vi.fn();
    const { user, unmount } = renderConsole({ onClear });

    await user.click(screen.getByRole('button', { name: 'Clear console' }));
    expect(onClear).toHaveBeenCalledTimes(1);
    unmount();

    renderConsole({ lines: [], onClear });
    expect(screen.getByRole('button', { name: 'Clear console' })).toBeDisabled();
  });

  it('marks the output live so assistive tech follows new rows', () => {
    renderConsole({ live: true });

    expect(screen.getByRole('log')).toHaveAttribute('aria-live', 'polite');
    expect(screen.getByText('Live')).toBeInTheDocument();
  });

  it('leaves the output silent when not tailing', () => {
    renderConsole();

    expect(screen.getByRole('log')).toHaveAttribute('aria-live', 'off');
  });
});

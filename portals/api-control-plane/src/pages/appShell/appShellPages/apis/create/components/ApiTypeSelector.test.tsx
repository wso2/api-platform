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

import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import { ApiTypeSelector } from './ApiTypeSelector';

const card = (name: RegExp) => screen.getByRole('button', { name });

describe('ApiTypeSelector', () => {
  it('lists every API type with its one-line description', () => {
    renderWithProviders(<ApiTypeSelector onChange={vi.fn()} />);

    expect(screen.getByRole('group', { name: 'API type' })).toBeInTheDocument();
    expect(screen.getByText('Expose REST and HTTP backends.')).toBeInTheDocument();
    expect(screen.getByText('Serve a GraphQL schema.')).toBeInTheDocument();
    expect(screen.getByText('Stream data over WebSocket.')).toBeInTheDocument();
    expect(screen.getByText('Deliver events over WebSub.')).toBeInTheDocument();
    expect(screen.getByText('Expose gRPC services.')).toBeInTheDocument();
  });

  it('marks the unreleased types as coming soon and keeps them out of the tab order', () => {
    renderWithProviders(<ApiTypeSelector onChange={vi.fn()} />);

    expect(screen.getAllByText('Coming soon')).toHaveLength(3);
    for (const name of [/WebSocket API/, /WebSub API/, /gRPC API/]) {
      expect(card(name)).toHaveAttribute('aria-disabled', 'true');
      expect(card(name)).toHaveAttribute('tabindex', '-1');
    }
  });

  it('reports the whole catalog entry when an available card is clicked', async () => {
    const onChange = vi.fn();
    renderWithProviders(<ApiTypeSelector onChange={onChange} />);

    await userEvent.click(card(/GraphQL API/));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ enabled: true, key: 'graphql' }),
    );
  });

  it('selects with Enter and Space from the keyboard, ignoring other keys', async () => {
    const onChange = vi.fn();
    renderWithProviders(<ApiTypeSelector onChange={onChange} />);

    card(/REST API/).focus();
    await userEvent.keyboard('{Enter}');
    await userEvent.keyboard(' ');
    expect(onChange).toHaveBeenCalledTimes(2);

    await userEvent.keyboard('a');
    expect(onChange).toHaveBeenCalledTimes(2);
  });

  it('never reports a click on an unreleased card', async () => {
    const onChange = vi.fn();
    renderWithProviders(<ApiTypeSelector onChange={onChange} />);

    // The card takes no pointer events, so drive the handler path directly.
    card(/gRPC API/).click();

    expect(onChange).not.toHaveBeenCalled();
  });

  it('keeps its own selection when uncontrolled', async () => {
    renderWithProviders(<ApiTypeSelector onChange={vi.fn()} />);
    expect(card(/REST API/)).toHaveAttribute('aria-pressed', 'false');

    await userEvent.click(card(/REST API/));

    expect(card(/REST API/)).toHaveAttribute('aria-pressed', 'true');
    expect(card(/GraphQL API/)).toHaveAttribute('aria-pressed', 'false');
  });

  it('follows `value` when the parent controls the selection', () => {
    const { rerender } = renderWithProviders(
      <ApiTypeSelector onChange={vi.fn()} value="graphql" />,
    );
    expect(card(/GraphQL API/)).toHaveAttribute('aria-pressed', 'true');

    rerender(<ApiTypeSelector onChange={vi.fn()} value="rest" />);

    expect(card(/REST API/)).toHaveAttribute('aria-pressed', 'true');
    expect(card(/GraphQL API/)).toHaveAttribute('aria-pressed', 'false');
  });

  it('does not show an unreleased type as selected even if the parent passes it', () => {
    renderWithProviders(<ApiTypeSelector onChange={vi.fn()} value="grpc" />);

    expect(card(/gRPC API/)).not.toHaveAttribute('aria-pressed');
  });
});

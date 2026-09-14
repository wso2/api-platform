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

import { aGateway } from '@/test/msw';
import { renderWithProviders, screen } from '@/test/utils';
import { GatewayCard } from './GatewayCard';

const gateway = aGateway({
  displayName: 'Dev Gateway',
  functionalityType: 'regular',
  id: 'dev-gateway',
  properties: { gatewayMode: 'self-hosted' },
  version: '1.2.0-SNAPSHOT',
});

describe('GatewayCard', () => {
  it('shows what identifies the gateway and whether it is connected', () => {
    renderWithProviders(<GatewayCard gateway={gateway} onOpen={vi.fn()} />);

    // Identity: the name is never abbreviated in the markup, however long the
    // version beside it is.
    expect(screen.getByText('Dev Gateway')).toBeInTheDocument();
    expect(screen.getByText('v1.2.0-SNAPSHOT')).toBeInTheDocument();
    // Kind: two same-weight marks, neither of them the accent.
    expect(screen.getByText('Self-hosted')).toBeInTheDocument();
    expect(screen.getByText('Regular')).toBeInTheDocument();
    // State: the card's one accent.
    expect(screen.getByText('Connected')).toBeInTheDocument();
  });

  it('leaves out the endpoint and every other detail the gateway page owns', () => {
    renderWithProviders(
      <GatewayCard
        gateway={aGateway({
          displayName: 'Dev Gateway',
          endpoints: ['https://dev.test'],
          id: 'dev-gateway',
        })}
        onOpen={vi.fn()}
      />,
    );

    expect(screen.queryByText('https://dev.test')).not.toBeInTheDocument();
  });

  it('opens the gateway when the card is clicked', async () => {
    const onOpen = vi.fn();
    const { user } = renderWithProviders(<GatewayCard gateway={gateway} onOpen={onOpen} />);

    await user.click(screen.getByText('Dev Gateway'));

    expect(onOpen).toHaveBeenCalledWith(gateway);
  });
});

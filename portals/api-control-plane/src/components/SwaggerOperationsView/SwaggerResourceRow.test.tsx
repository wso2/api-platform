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

import { Button } from '@wso2/oxygen-ui';
import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import { SwaggerResourceRow } from './SwaggerResourceRow';

describe('SwaggerResourceRow', () => {
  it('renders the method, path and description', () => {
    renderWithProviders(
      <SwaggerResourceRow
        description="Create a conversation and append entries to it."
        method="POST"
        path="/v1/conversations"
      />,
    );

    expect(screen.getByText('POST')).toBeInTheDocument();
    expect(screen.getByText('/v1/conversations')).toBeInTheDocument();
    expect(screen.getByText('Create a conversation and append entries to it.')).toBeInTheDocument();
  });

  it('stays a flat row with no expand control when given no children', () => {
    renderWithProviders(<SwaggerResourceRow method="GET" path="/v1/agents" />);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('becomes expandable when given children, revealing them on click', async () => {
    const { user } = renderWithProviders(
      <SwaggerResourceRow method="GET" path="/v1/agents">
        <p>Attached policies</p>
      </SwaggerResourceRow>,
    );

    await user.click(screen.getByRole('button', { name: 'Show details for GET /v1/agents' }));
    expect(screen.getByText('Attached policies')).toBeVisible();
    // The control relabels itself, so a screen reader is told what it now does.
    expect(
      screen.getByRole('button', { expanded: true, name: 'Hide details for GET /v1/agents' }),
    ).toBeInTheDocument();
  });

  it('expands when the row surface itself is clicked', async () => {
    const { user } = renderWithProviders(
      <SwaggerResourceRow method="GET" path="/v1/agents">
        <p>Attached policies</p>
      </SwaggerResourceRow>,
    );

    await user.click(screen.getByText('/v1/agents'));
    expect(screen.getByText('Attached policies')).toBeVisible();
  });

  it('lets a trailing action be clicked without toggling the row', async () => {
    const onAction = vi.fn();
    const { user } = renderWithProviders(
      <SwaggerResourceRow
        actions={<Button onClick={onAction}>Remove</Button>}
        method="DELETE"
        path="/v1/models/{model_id}"
      >
        <p>Attached policies</p>
      </SwaggerResourceRow>,
    );

    await user.click(screen.getByRole('button', { name: 'Remove' }));
    expect(onAction).toHaveBeenCalledOnce();
    expect(screen.getByRole('button', { expanded: false })).toBeInTheDocument();
    expect(screen.queryByText('Attached policies')).not.toBeInTheDocument();
  });
});

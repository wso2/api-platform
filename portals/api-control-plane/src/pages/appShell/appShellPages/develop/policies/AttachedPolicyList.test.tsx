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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '@/test/utils';
import { AttachedPolicyList } from './AttachedPolicyList';

describe('AttachedPolicyList', () => {
  it('does not duplicate the version prefix after a saved policy is reloaded', () => {
    renderWithProviders(
      <AttachedPolicyList
        canAdd
        onAdd={vi.fn()}
        onEdit={vi.fn()}
        onRemove={vi.fn()}
        onReorder={vi.fn()}
        policies={[
          { name: 'stored-policy', version: 'v1' },
          { name: 'catalog-policy', version: '2.0' },
        ]}
      />,
    );

    expect(screen.getByText('v1')).toBeInTheDocument();
    expect(screen.queryByText('vv1')).not.toBeInTheDocument();
    expect(screen.getByText('v2.0')).toBeInTheDocument();
  });
});

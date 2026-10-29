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

import { renderWithProviders, screen } from '../../../../../test/utils';
import ExternalServerStepBanner, { type ExternalServerStepBannerProps } from './ExternalServerStepBanner';

const renderBanner = (props: Partial<ExternalServerStepBannerProps> = {}) => {
  const onStepClick = vi.fn();
  const view = renderWithProviders(
    <ExternalServerStepBanner
      hasDeployments={false}
      hasPolicies={false}
      onStepClick={onStepClick}
      serverName="Travel MCP"
      {...props}
    />
  );
  return { ...view, onStepClick };
};

describe('ExternalServerStepBanner', () => {
  describe('where publishing to hubs is not offered', () => {
    it('shows the two setup steps it always has, and counts out of three', () => {
      renderBanner();

      expect(screen.getByRole('button', { name: 'Configure Policies' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Deploy to Gateway & Test' })).toBeInTheDocument();
      expect(screen.getByText('1/3')).toBeInTheDocument();
    });

    it('shows no publish step', () => {
      renderBanner();

      expect(screen.queryByRole('button', { name: 'Publish to MCP Hub' })).not.toBeInTheDocument();
    });

    it('counts policies and deployments as before', () => {
      renderBanner({ hasDeployments: true, hasPolicies: true });

      expect(screen.getByText('3/3')).toBeInTheDocument();
    });
  });

  describe('where publishing to hubs is offered', () => {
    it('adds a publish step and counts out of four', () => {
      renderBanner({ hasPublication: false });

      expect(screen.getByRole('button', { name: 'Publish to MCP Hub' })).toBeInTheDocument();
      expect(screen.getByText('1/4')).toBeInTheDocument();
    });

    it('counts a published proxy as a completed step', () => {
      renderBanner({ hasPublication: true });

      expect(screen.getByText('2/4')).toBeInTheDocument();
    });

    it('is complete only when every step is', () => {
      renderBanner({ hasDeployments: true, hasPolicies: true, hasPublication: true });

      expect(screen.getByText('4/4')).toBeInTheDocument();
    });

    it('reports a click on the publish step', async () => {
      const { onStepClick, user } = renderBanner({ hasPublication: false });

      await user.click(screen.getByRole('button', { name: 'Publish to MCP Hub' }));

      expect(onStepClick).toHaveBeenCalledWith('publish-to-mcp-hub');
    });

    it('still reports the other steps by their own ids', async () => {
      const { onStepClick, user } = renderBanner({ hasPublication: false });

      await user.click(screen.getByRole('button', { name: 'Configure Policies' }));
      await user.click(screen.getByRole('button', { name: 'Deploy to Gateway & Test' }));

      expect(onStepClick).toHaveBeenNthCalledWith(1, 'add-policies');
      expect(onStepClick).toHaveBeenNthCalledWith(2, 'deploy-to-gateway');
    });
  });

  it('can be closed', async () => {
    const { user } = renderBanner();

    await user.click(screen.getByRole('button', { name: 'Close external server setup steps banner' }));

    expect(screen.queryByText('Configure Policies')).not.toBeInTheDocument();
  });
});

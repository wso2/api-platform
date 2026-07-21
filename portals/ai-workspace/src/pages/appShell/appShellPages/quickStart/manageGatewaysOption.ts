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

import AIGatewayImage from '../../../../assets/images/AIGateway.svg';
import SecondStepImage from '../../../../assets/images/quickStart/2ndStep.svg';
import type { QuickStartOption } from './types';

export function getManageGatewaysOption(getNextPath: () => string): QuickStartOption {
  return {
    id: 'manage-gateways',
    label: 'Manage AI Gateways',
    description:
      'Add, configure, and operate the gateways that handle your AI traffic.',
    title: 'Provision and manage your AI gateway layer',
    subtitle:
      'Set up the gateways that front your providers, proxies, and MCP workloads, then keep them ready for deployment.',
    steps: [
      {
        title: 'Add AI gateway',
        description:
          'Register a new gateway with the runtime details needed by your organization.',
      },
      {
        title: 'Configure connectivity',
        description:
          'Review endpoint, virtual host, and environment settings for the gateway.',
      },
      {
        title: 'Prepare for deployments',
        description:
          'Make the gateway available for App AI Proxies, providers, and MCP proxy deployments.',
      },
    ],
    accentColor: '#C96B00',
    imageSrc: AIGatewayImage,
    previewImageSrc: SecondStepImage,
    getNextPath,
    navigationScope: 'organization',
  };
}

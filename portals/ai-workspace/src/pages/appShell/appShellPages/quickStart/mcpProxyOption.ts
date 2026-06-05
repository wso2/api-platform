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

import QSMcpImage from '../../../../assets/images/QSMcp.svg';
import ThirdStepImage from '../../../../assets/images/quickStart/3rdStep.svg';
import type { QuickStartOption } from './types';

export function getMCPProxyOption(getNextPath: () => string): QuickStartOption {
  return {
    id: 'mcp-proxy',
    label: 'Publish my MCP servers securely',
    description:
      'Expose tools, prompts, and resources through an MCP proxy.',
    title: 'Create an MCP proxy for tools and resources',
    subtitle:
      'Connect an MCP endpoint, validate what it exposes, and publish it through your AI gateway workflows.',
    steps: [
      {
        title: 'Connect MCP endpoint',
        description:
          'Add the MCP server endpoint and provide any authentication required to access it.',
      },
      {
        title: 'Review exposed capabilities',
        description:
          'Validate the tools, prompts, and resources discovered from the connected endpoint.',
      },
      {
        title: 'Deploy through gateway',
        description:
          'Make the MCP proxy available for downstream AI applications and agents.',
      },
    ],
    accentColor: '#C96B00',
    imageSrc: QSMcpImage,
    previewImageSrc: ThirdStepImage,
    getNextPath,
    navigationScope: 'current',
  };
}

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

import LLMProviderImage from '../../../../assets/images/LLMProvider.svg';
import FirstStepImage from '../../../../assets/images/quickStart/1stStep.svg';
import type { QuickStartOption } from './types';

export function getProviderProxyOption(getNextPath: () => string): QuickStartOption {
  return {
    id: 'provider-proxy',
    label: 'Expose My LLM Providers Securely',
    description: 'Register an LLM provider your applications can use.',
    title: 'Set up an LLM provider and expose it with a proxy',
    subtitle:
      'Connect your model provider, define a reusable proxy, and prepare it for deployment through your AI workspace.',
    steps: [
      {
        title: 'Add LLM provider',
        description:
          'Register the provider, credentials, and models you want to make available.',
      },
      {
        title: 'Add guardrail policies',
        description:
          'Configure guardrail policies to control and protect model interactions.',
      },
      {
        title: 'Deploy and invoke',
        description:
          'Publish the proxy through an AI gateway and start calling it from your apps.',
      },
    ],
    badge: 'Recommended',
    accentColor: '#C96B00',
    imageSrc: LLMProviderImage,
    previewImageSrc: FirstStepImage,
    getNextPath,
    navigationScope: 'organization',
  };
}

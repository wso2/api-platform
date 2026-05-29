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

import type { JSX } from 'react';
import React from 'react';
import { ColorSchemeImage } from '@wso2/oxygen-ui';

import AIWorkspaceLogoDark from '../assets/images/AIWorkspaceLogo.svg';
import AIWorkspaceLogoLight from '../assets/images/AIWorkspaceLogoDark.svg';

type LogoProps = {
  height?: number;
};

export default function Logo({ height = 40 }: LogoProps): JSX.Element {
  return (
    <ColorSchemeImage
      src={{
        light: AIWorkspaceLogoLight,
        dark: AIWorkspaceLogoDark,
      }}
      alt={{
        light: 'AI Workspace Logo (Light)',
        dark: 'AI Workspace Logo (Dark)',
      }}
      height={height}
      width="auto"
    />
  );
}

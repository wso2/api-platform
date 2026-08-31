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

import type { FC } from 'react';
import { Alert, Box, Typography } from '@wso2/oxygen-ui';
import type { StatusTone } from '../utils/status';

export type DeploymentStatusBarProps = {
  tone: StatusTone;
};

/**
 * The full-width "Deployment Status" row shown per gateway. Reuses `Alert`'s
 * built-in severity tinting for success/warning/error (adapts to light/dark
 * automatically) and falls back to a neutral themed box for the "default"
 * tone (not deployed), since Alert has no neutral severity.
 */
const DeploymentStatusBar: FC<DeploymentStatusBarProps> = ({ tone }) => {
  const content = (
    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
      <Typography sx={{ fontSize: 13, fontWeight: 600 }}>Deployment Status</Typography>
      <Typography sx={{ fontSize: 13, fontWeight: 600 }}>{tone.label}</Typography>
    </Box>
  );

  if (tone.tone === 'default') {
    return (
      <Box sx={{ px: 2, py: 1, borderRadius: 1, bgcolor: 'action.hover', color: 'text.secondary' }}>{content}</Box>
    );
  }

  return (
    <Alert severity={tone.tone} icon={false} sx={{ py: 0.5 }}>
      {content}
    </Alert>
  );
};

export default DeploymentStatusBar;

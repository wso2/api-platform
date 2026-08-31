/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
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

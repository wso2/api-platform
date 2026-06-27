/*
 * Copyright (c) 2026, WSO2 Inc. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 Inc. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

import React, { useMemo } from 'react';
import { Box, Grid } from '@wso2/oxygen-ui';
import type { HybridGateway } from '../../../../../apis/gatewayTypes';
import GatewayDeployEnvCard from './GatewayDeployEnvCard';
import GatewayDeploymentHistory from './GatewayDeploymentHistory';

interface GatewayDeployCardContentProps {
  gateway: HybridGateway;
  hasDeployments: boolean;
  isGatewayActive: boolean;
}

/**
 * Content area for a gateway card
 */
export default function GatewayDeployCardContent({
  gateway,
  hasDeployments,
  isGatewayActive,
}: GatewayDeployCardContentProps) {
  return (
    <Grid container spacing={3}>
      <Grid
        size={{ xs: 12, md: hasDeployments ? 6 : 12 }}
        sx={{ minWidth: 240 }}
      >
        <GatewayDeployEnvCard gateway={gateway} isGatewayActive={isGatewayActive} />
      </Grid>
      {hasDeployments && (
        <Grid
          size={{ xs: 12, md: 6 }}
          sx={{
            minWidth: 280,
            borderLeft: { md: '1px solid', xs: 'none' },
            borderColor: 'divider',
            pl: { md: 3, xs: 0 },
          }}
        >
          <GatewayDeploymentHistory gatewayHandle={gateway.handle} />
        </Grid>
      )}
    </Grid>
  );
}

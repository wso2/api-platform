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

import { useState } from 'react';
import { Button, CircularProgress, ButtonGroup, Menu, MenuItem } from '@wso2/oxygen-ui';
import { ChevronDown } from '@wso2/oxygen-ui-icons-react';
import type { HybridGateway } from '../../apis/gateway/types';
import { useGatewayDeploy } from '../../contexts/GatewayDeployContext';

interface GatewayDeployCardActionsProps {
  gateway: HybridGateway;
  onConfigureAndDeploy?: (gateway: HybridGateway) => void;
  disabled?: boolean;
}

/**
 * Deploy action button for a gateway card header.
 */
export default function GatewayDeployCardActions({
  gateway,
  onConfigureAndDeploy,
  disabled,
}: GatewayDeployCardActionsProps) {
  const {
    deployToGateway,
    deployingGatewayId,
    isDeployingToGateway,
    isLoadingDeployments,
    readOnly,
  } = useGatewayDeploy();

  const [deployMenuAnchor, setDeployMenuAnchor] = useState<null | HTMLElement>(null);

  const isThisGatewayDeploying = deployingGatewayId === gateway.id;

  const handleDeploy = async () => {
    await deployToGateway(gateway.id, '');
  };

  const handleConfigureAndDeploy = () => {
    setDeployMenuAnchor(null);
    if (onConfigureAndDeploy) {
      onConfigureAndDeploy(gateway);
    }
  };

  const isButtonDisabled =
    disabled || readOnly || isDeployingToGateway || isLoadingDeployments;

  // If no onConfigureAndDeploy callback, show simple button
  if (!onConfigureAndDeploy) {
    return (
      <Button
        variant="contained"
        color="primary"
        size="small"
        onClick={handleDeploy}
        disabled={isButtonDisabled}
        startIcon={
          isThisGatewayDeploying ? (
            <CircularProgress color="inherit" size={14} />
          ) : undefined
        }
      >
        {isThisGatewayDeploying ? 'Deploying...' : 'Deploy'}
      </Button>
    );
  }

  // Otherwise show split button with configure option
  return (
    <>
      <ButtonGroup variant="contained" color="primary" size="small">
        <Button
          onClick={handleDeploy}
          disabled={isButtonDisabled}
          startIcon={
            isThisGatewayDeploying ? (
              <CircularProgress color="inherit" size={14} />
            ) : undefined
          }
        >
          {isThisGatewayDeploying ? 'Deploying...' : 'Deploy'}
        </Button>
        <Button
          size="small"
          onClick={(e) => setDeployMenuAnchor(e.currentTarget)}
          disabled={isButtonDisabled}
          sx={{ px: 1, minWidth: 'auto' }}
        >
          <ChevronDown />
        </Button>
      </ButtonGroup>
      <Menu
        anchorEl={deployMenuAnchor}
        open={Boolean(deployMenuAnchor)}
        onClose={() => setDeployMenuAnchor(null)}
      >
        <MenuItem onClick={() => { setDeployMenuAnchor(null); handleDeploy(); }}>
          Deploy
        </MenuItem>
        <MenuItem onClick={handleConfigureAndDeploy}>
          Configure and Deploy
        </MenuItem>
      </Menu>
    </>
  );
}

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

import React, { useMemo, useState } from 'react';
import { Link as RouterLink } from 'react-router-dom';
import { Box, Button, CircularProgress, Typography } from '@wso2/oxygen-ui';
import { Plus } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import type { Gateway } from '../../apis/gateway/types';
import { useGatewayDeploy } from '../../contexts/GatewayDeployContext';
import { useAppShell } from '../../contexts/AppShellContext';
import { buildOrgPath } from '../../utils/projectRouting';
import GatewayDeployList from './GatewayDeployList';
import GatewayDeployConfigDrawer from './GatewayDeployConfigDrawer';
import NoGW from '../../assets/images/NoGW.svg';

interface GatewayDeployMainSectionProps {
  /**
   * Whether to show the "Configure and Deploy" option.
   * If false, only the simple "Deploy" button will be shown.
   * @default true
   */
  showConfigureOption?: boolean;
}

/**
 * Main section for deploying an API to hybrid gateways.
 */
export default function GatewayDeployMainSection({
  showConfigureOption = true,
}: GatewayDeployMainSectionProps = {}) {
  const { gateways, isLoading, error } = useGatewayDeploy();
  const { currentOrganization } = useAppShell();
  const [searchQuery, setSearchQuery] = useState('');
  const [configDrawerOpen, setConfigDrawerOpen] = useState(false);
  const [selectedGateway, setSelectedGateway] = useState<Gateway | null>(null);

  const handleConfigureAndDeploy = (gateway: Gateway) => {
    setSelectedGateway(gateway);
    setConfigDrawerOpen(true);
  };

  const handleCloseConfigDrawer = () => {
    setConfigDrawerOpen(false);
    setSelectedGateway(null);
  };

  const initialExpandedIds = useMemo(() => {
    if (gateways.length > 0 && gateways[0]) {
      return [gateways[0].id];
    }
    return [];
  }, [gateways]);

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', p: 6 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Box sx={{ p: 3 }}>
        <Typography color="error">Failed to load gateways.</Typography>
      </Box>
    );
  }

  if (gateways.length === 0) {
    return (
      <Box
        sx={{
          textAlign: 'center',
          p: 6,
          borderRadius: 1,
          mt: 2,
        }}
      >
        <Box
          component="img"
          src={NoGW}
          alt="No gateways"
          sx={{ width: 120, maxWidth: '80%'}}
        />
        <Typography variant="h6" sx={{ mb: 1 }}>
          <FormattedMessage
            id="aiWorkspace.components.GatewayDeploy.GatewayDeployMainSection.add.gateway.to.deploy"
            defaultMessage="Add gateway to deploy"
          />
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
          <FormattedMessage
            id="aiWorkspace.components.GatewayDeploy.GatewayDeployMainSection.create.an.ai.gateway.to.enable.deployments"
            defaultMessage="Create an AI Gateway to enable deployments"
          />
        </Typography>
        <Button
          variant="contained"
          component={RouterLink}
          to={buildOrgPath(currentOrganization, '/gateways/new')}
          startIcon={<Plus size={20} />}
        >
          <FormattedMessage
            id="aiWorkspace.components.GatewayDeploy.GatewayDeployMainSection.add.ai.gateway"
            defaultMessage="Add AI Gateway"
          />
        </Button>
      </Box>
    );
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column' }}>
      <GatewayDeployList
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        initialExpandedIds={initialExpandedIds}
        onConfigureAndDeploy={
          showConfigureOption ? handleConfigureAndDeploy : undefined
        }
      />
      {showConfigureOption && (
        <GatewayDeployConfigDrawer
          open={configDrawerOpen}
          onClose={handleCloseConfigDrawer}
          gateway={selectedGateway}
        />
      )}
    </Box>
  );
}

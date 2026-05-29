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
import {
  Box,
  Button,
  CircularProgress,
  Drawer,
  IconButton,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import type { HybridGateway } from '../../apis/gateway/types';
import { useGatewayDeploy } from '../../contexts/GatewayDeployContext';

interface GatewayDeployConfigDrawerProps {
  open: boolean;
  onClose: () => void;
  gateway: HybridGateway | null;
}

/**
 * Configuration drawer for gateway deployment settings.
 */
export default function GatewayDeployConfigDrawer({
  open,
  onClose,
  gateway,
}: GatewayDeployConfigDrawerProps) {
  const { deployToGateway, isDeployingToGateway } = useGatewayDeploy();
  const [host, setHost] = useState('');

  const handleDeploy = async () => {
    if (!gateway) return;

    try {
      await deployToGateway(gateway.id, host);
      onClose();
      setHost('');
    } catch (err) {
      // Error is handled by context
    }
  };

  const handleClose = () => {
    if (isDeployingToGateway) return;
    onClose();
    setHost('');
  };

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={handleClose}
      sx={{
        '& .MuiDrawer-paper': {
          width: { xs: '100%', sm: 500 },
          maxWidth: '100%',
        },
      }}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          p: 2,
          borderBottom: 1,
          borderColor: 'divider',
        }}
      >
        <Typography variant="h6">Configure Deployment</Typography>
        <IconButton
          onClick={handleClose}
          disabled={isDeployingToGateway}
          size="small"
        >
          <X size={20} />
        </IconButton>
      </Box>

      <Box sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 3 }}>
        <TextField
          label="Host"
          placeholder="api.example.com"
          value={host}
          onChange={(e) => setHost(e.target.value)}
          fullWidth
          helperText="Gateway URL of the LLM Provider (optional)"
        />

        <Box sx={{ mt: 'auto', display: 'flex', gap: 2, pt: 2 }}>
          <Button
            variant="outlined"
            onClick={handleClose}
            disabled={isDeployingToGateway}
            fullWidth
            color="secondary"
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleDeploy}
            disabled={isDeployingToGateway || !gateway}
            startIcon={
              isDeployingToGateway ? <CircularProgress size={16} /> : null
            }
            fullWidth
          >
            {isDeployingToGateway ? 'Deploying...' : 'Deploy'}
          </Button>
        </Box>
      </Box>
    </Drawer>
  );
}

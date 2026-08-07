import { useState } from 'react';
import { Box, Button, Drawer, IconButton, Typography } from '@wso2/oxygen-ui';
import { RefreshCw } from '@wso2/oxygen-ui-icons-react';

import { useDeleteGatewayDeployment } from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import type { Api, GatewayDeployment } from '../../types/domain';
import { GatewayDeploymentRow } from './GatewayDeploymentRow';

type GatewayDeploymentHistoryProps = {
  api: Api;
  /** Deployments on this gateway, newest first. */
  deployments: GatewayDeployment[];
  onRefresh: () => void;
  refreshing: boolean;
};

/**
 * Right panel of an expanded gateway card: the API deployment history —
 * newest three rows inline, the full list (with delete) in a drawer
 * (ai-workspace GatewayDeploymentHistory).
 */
export function GatewayDeploymentHistory({
  api,
  deployments,
  onRefresh,
  refreshing,
}: GatewayDeploymentHistoryProps) {
  const [drawerOpen, setDrawerOpen] = useState(false);
  const { notify } = useNotifications();
  const deleteMutation = useDeleteGatewayDeployment();

  if (deployments.length === 0) return null;

  const currentDeploymentId = deployments[0]?.id ?? null;

  const handleDelete = (deployment: GatewayDeployment) => {
    deleteMutation.mutate(
      { api, deployment },
      {
        onSuccess: () => notify(`Deleted "${deployment.name}".`, 'success'),
        onError: (error) =>
          notify(
            error instanceof Error ? error.message : 'Delete failed',
            'error'
          ),
      }
    );
  };

  return (
    <Box>
      <Box
        sx={{
          alignItems: 'center',
          borderBottom: '1px solid',
          borderColor: 'divider',
          display: 'flex',
          pb: 2,
        }}
      >
        <Typography sx={{ flexGrow: 1, fontWeight: 500 }}>
          API Deployment History
        </Typography>
        <IconButton
          aria-label="Refresh deployment history"
          color="primary"
          disabled={refreshing}
          onClick={onRefresh}
          size="small"
        >
          <RefreshCw size={18} />
        </IconButton>
      </Box>

      <Box>
        {deployments.slice(0, 3).map((deployment) => (
          <GatewayDeploymentRow
            deployment={deployment}
            isCurrentDeployment={deployment.id === currentDeploymentId}
            key={deployment.id}
          />
        ))}
      </Box>

      {deployments.length > 3 && (
        <Box
          sx={{
            borderColor: 'divider',
            borderTop: '1px solid',
            pt: 1,
            textAlign: 'center',
          }}
        >
          <Button
            color="primary"
            onClick={() => setDrawerOpen(true)}
            variant="text"
          >
            View More
          </Button>
        </Box>
      )}

      <Drawer
        anchor="right"
        onClose={() => setDrawerOpen(false)}
        open={drawerOpen}
        sx={{ '& .MuiDrawer-paper': { width: { md: 600, xs: '100%' } } }}
      >
        <Box sx={{ p: 3 }}>
          <Typography sx={{ mb: 2 }} variant="h6">
            API Deployment History
          </Typography>
          {deployments.map((deployment) => (
            <GatewayDeploymentRow
              deleteDisabled={deleteMutation.isPending}
              deployment={deployment}
              isCurrentDeployment={deployment.id === currentDeploymentId}
              key={deployment.id}
              onDelete={handleDelete}
            />
          ))}
        </Box>
      </Drawer>
    </Box>
  );
}

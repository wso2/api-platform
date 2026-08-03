import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Chip,
  CircularProgress,
  Grid,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown } from '@wso2/oxygen-ui-icons-react';

import { useDeployApi } from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import type { Api, Gateway, GatewayDeployment } from '../../types/domain';
import { GatewayDeployEnvCard } from './GatewayDeployEnvCard';
import { GatewayDeploymentHistory } from './GatewayDeploymentHistory';
import {
  currentDeploymentFor,
  deploymentsForGateway,
  nextDeploymentName,
} from './gatewayDeployUtils';

type GatewayDeployCardProps = {
  api: Api;
  gateway: Gateway;
  /** All deployments of the API (across gateways). */
  deployments: GatewayDeployment[];
  isExpanded: boolean;
  onToggleExpand: (expanded: boolean) => void;
  onRefresh: () => void;
  refreshing: boolean;
};

/**
 * Expandable per-gateway card on the Deploy page: header with the gateway
 * name, connection state, current deployment and a one-click Deploy button;
 * expanded body with the status panel and deployment history (ai-workspace
 * GatewayDeployCard).
 */
export function GatewayDeployCard({
  api,
  gateway,
  deployments,
  isExpanded,
  onToggleExpand,
  onRefresh,
  refreshing,
}: GatewayDeployCardProps) {
  const { notify } = useNotifications();
  const deployMutation = useDeployApi();
  const isActive = gateway.isActive === true;
  const gatewayDeployments = deploymentsForGateway(deployments, gateway.id);
  const currentDeployment = currentDeploymentFor(deployments, gateway.id);
  const hasDeployments = gatewayDeployments.length > 0;

  const handleDeploy = () => {
    const name = nextDeploymentName(gateway, deployments);
    deployMutation.mutate(
      { api, input: { name, gatewayId: gateway.id, base: 'current' } },
      {
        onSuccess: (deployment) =>
          notify(`Deployment "${deployment.name}" started.`, 'success'),
        onError: (error) =>
          notify(
            error instanceof Error ? error.message : 'Deploy failed',
            'error'
          ),
      }
    );
  };

  return (
    <Accordion
      expanded={isExpanded}
      onChange={(_event, expanded) => onToggleExpand(expanded)}
      sx={{
        borderRadius: '8px',
        overflow: 'hidden',
        '&:before': { display: 'none' },
        '&.Mui-expanded': { borderRadius: '8px', margin: 0 },
        '&:first-of-type': {
          borderTopLeftRadius: '8px',
          borderTopRightRadius: '8px',
        },
        '&:last-of-type': {
          borderBottomLeftRadius: '8px',
          borderBottomRightRadius: '8px',
        },
      }}
      variant="outlined"
    >
      <AccordionSummary
        sx={{
          px: 3,
          '& .MuiAccordionSummary-content': {
            alignItems: 'center',
            flexWrap: 'wrap',
            justifyContent: 'space-between',
            m: 0,
          },
        }}
      >
        <Box
          sx={{
            alignItems: 'center',
            display: 'flex',
            flexWrap: 'wrap',
            justifyContent: 'space-between',
            width: '100%',
          }}
        >
          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              flexWrap: 'wrap',
              gap: 1.5,
            }}
          >
            <Typography sx={{ fontWeight: 500 }} variant="h6">
              {gateway.displayName}
            </Typography>
            <Chip
              color={isActive ? 'success' : 'error'}
              label={isActive ? 'Active' : 'Not Active'}
              size="small"
              variant="outlined"
            />
            {currentDeployment && (
              <Box sx={{ alignItems: 'center', display: 'flex', gap: 1 }}>
                <Typography
                  color="text.secondary"
                  component="span"
                  variant="body2"
                >
                  Current Deployment:
                </Typography>
                <Chip
                  label={currentDeployment.name}
                  size="small"
                  variant="outlined"
                />
              </Box>
            )}
          </Box>
          <Box sx={{ alignItems: 'center', display: 'flex', gap: 1.5 }}>
            <Box component="span" onClick={(event) => event.stopPropagation()}>
              <Button
                color="primary"
                disabled={!isActive || deployMutation.isPending}
                onClick={handleDeploy}
                size="small"
                startIcon={
                  deployMutation.isPending ? (
                    <CircularProgress color="inherit" size={14} />
                  ) : undefined
                }
                variant="contained"
              >
                {deployMutation.isPending ? 'Deploying...' : 'Deploy'}
              </Button>
            </Box>
            <ChevronDown
              size={20}
              style={{
                transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
                transition: 'transform 0.2s ease',
              }}
            />
          </Box>
        </Box>
      </AccordionSummary>
      <AccordionDetails sx={{ px: 3, py: 2 }}>
        <Grid container spacing={3}>
          <Grid
            size={{ md: hasDeployments ? 6 : 12, xs: 12 }}
            sx={{ minWidth: 240 }}
          >
            <GatewayDeployEnvCard
              api={api}
              currentDeployment={currentDeployment}
              deployments={gatewayDeployments}
              gateway={gateway}
              isGatewayActive={isActive}
            />
          </Grid>
          {hasDeployments && (
            <Grid
              size={{ md: 6, xs: 12 }}
              sx={{
                borderColor: 'divider',
                borderLeft: { md: '1px solid', xs: 'none' },
                minWidth: 280,
                pl: { md: 3, xs: 0 },
              }}
            >
              <GatewayDeploymentHistory
                api={api}
                deployments={gatewayDeployments}
                onRefresh={onRefresh}
                refreshing={refreshing}
              />
            </Grid>
          )}
        </Grid>
      </AccordionDetails>
    </Accordion>
  );
}

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
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import type { Gateway } from '@/api/resources/gateways';
import { useDeployApi, type Deployment } from '@/api/resources/restApis/deployments';
import { useNotifications } from '@/components/Notifications';
import { GatewayDeployEnvCard } from './GatewayDeployEnvCard';
import { GatewayDeploymentHistory } from '../GatewayDeploymentHistory';
import {
  currentDeploymentFor,
  deploymentsForGateway,
  nextDeploymentName,
} from '../utils/gatewayDeployUtils';

const messages = defineMessages({
  active: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeployCard.active',
    defaultMessage: 'Active',
    description: 'Gateway connection state: the control plane can reach this gateway.',
  },
  notActive: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeployCard.notActive',
    defaultMessage: 'Not Active',
    description: 'Gateway connection state: the control plane cannot reach this gateway.',
  },
  currentDeployment: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeployCard.currentDeployment',
    defaultMessage: 'Current Deployment:',
    description: 'Label before the name of the deployment currently on this gateway.',
  },
  deploy: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeployCard.deploy',
    defaultMessage: 'Deploy',
    description: "Button that deploys the API's working copy to this gateway. Verb.",
  },
  deploying: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeployCard.deploying',
    defaultMessage: 'Deploying...',
    description: 'Label on the Deploy button while the request is in flight.',
  },
  deployStarted: {
    id: 'apiControlPlane.pages.appShell.appShellPages.deploy.components.GatewayDeployCard.deployStarted',
    defaultMessage: 'Deployment "{deploymentName}" started.',
    description:
      'Toast confirming a deploy was requested. {deploymentName} is server-generated; do not translate it.',
  },
});

type GatewayDeployCardProps = {
  /** Handle of the API being deployed. */
  restApiId: string;
  gateway: Gateway;
  /** All deployments of the API (across gateways). */
  deployments: Deployment[];
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
  restApiId,
  gateway,
  deployments,
  isExpanded,
  onToggleExpand,
  onRefresh,
  refreshing,
}: GatewayDeployCardProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const deployMutation = useDeployApi();
  const isActive = gateway.isActive === true;
  // `id` is the gateway handle. The spec marks it optional (it is server-assigned
  // on create), but a gateway that reached this card came from a list response,
  // so it is always present here.
  const gatewayId = gateway.id ?? '';
  const gatewayDeployments = deploymentsForGateway(deployments, gatewayId);
  const currentDeployment = currentDeploymentFor(deployments, gatewayId);
  const hasDeployments = gatewayDeployments.length > 0;

  const handleDeploy = () => {
    const name = nextDeploymentName(gateway, deployments);
    deployMutation.mutate(
      { restApiId, body: { name, gatewayId, base: 'current' } },
      // No `onError`: the query client's `onMutationError` already notifies.
      {
        onSuccess: (deployment) =>
          notify(
            intl.formatMessage(messages.deployStarted, {
              deploymentName: deployment.name,
            }),
            'success',
          ),
      },
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
              label={intl.formatMessage(isActive ? messages.active : messages.notActive)}
              size="small"
              variant="outlined"
            />
            {currentDeployment && (
              <Box sx={{ alignItems: 'center', display: 'flex', gap: 1 }}>
                <Typography color="text.secondary" component="span" variant="body2">
                  <FormattedMessage {...messages.currentDeployment} />
                </Typography>
                <Chip label={currentDeployment.name} size="small" variant="outlined" />
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
                <FormattedMessage
                  {...(deployMutation.isPending ? messages.deploying : messages.deploy)}
                />
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
          <Grid size={{ md: hasDeployments ? 6 : 12, xs: 12 }} sx={{ minWidth: 240 }}>
            <GatewayDeployEnvCard
              currentDeployment={currentDeployment}
              deployments={gatewayDeployments}
              gateway={gateway}
              isGatewayActive={isActive}
              restApiId={restApiId}
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
                deployments={gatewayDeployments}
                onRefresh={onRefresh}
                refreshing={refreshing}
                restApiId={restApiId}
              />
            </Grid>
          )}
        </Grid>
      </AccordionDetails>
    </Accordion>
  );
}

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
import type { Deployment } from '@/api/resources/restApis/deployments';
import { useNotifications } from '@/components/Notifications';
import { GatewayDeployEnvCard } from './GatewayDeployEnvCard';
import type { UseDeploymentMutationHook } from './GatewayDeploymentSelector';
import { GatewayDeploymentHistory } from '../GatewayDeploymentHistory';
import {
  currentDeploymentFor,
  deploymentsForGateway,
  nextDeploymentName,
} from '../utils/gatewayDeployUtils';

/**
 * What a "deploy the current working copy" mutation needs to expose — REST's
 * and GraphQL's own hooks each expect a differently keyed variables object
 * (`restApiId`/`graphqlApiId`), so a caller adapts its own hook to this
 * uniform `apiId` shape rather than this shared component knowing about
 * either kind. See `DeployPage`/`GraphqlDeployPage` for the adapters.
 */
export type UseDeployMutationHook = () => {
  isPending: boolean;
  mutate: (
    variables: { apiId: string; body: { name: string; gatewayId: string; base: 'current' } },
    options?: { onSuccess?: (deployment: Deployment) => void },
  ) => void;
};

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
  apiId: string;
  gateway: Gateway;
  /** All deployments of the API (across gateways). */
  deployments: Deployment[];
  isExpanded: boolean;
  onToggleExpand: (expanded: boolean) => void;
  onRefresh: () => void;
  refreshing: boolean;
  /** The caller's own deploy/undeploy/restore/delete mutations, adapted to a
   * uniform shape — REST's and GraphQL's hooks each expect a differently
   * keyed variables object. `useUndeploy`/`useRestore` are forwarded to
   * `GatewayDeployEnvCard`, `useDelete` to `GatewayDeploymentHistory`. */
  useDeploy: UseDeployMutationHook;
  useUndeploy: UseDeploymentMutationHook;
  useRestore: UseDeploymentMutationHook;
  useDelete: UseDeploymentMutationHook;
};

/**
 * Expandable per-gateway card on the Deploy page: header with the gateway
 * name, connection state, current deployment and a one-click Deploy button;
 * expanded body with the status panel and deployment history (ai-workspace
 * GatewayDeployCard). Shared between REST and GraphQL APIs — see
 * `DeployPage`/`GraphqlDeployPage` for the mutation-hook adapters.
 */
export function GatewayDeployCard({
  apiId,
  gateway,
  deployments,
  isExpanded,
  onToggleExpand,
  onRefresh,
  refreshing,
  useDeploy,
  useUndeploy,
  useRestore,
  useDelete,
}: GatewayDeployCardProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const deployMutation = useDeploy();
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
      { apiId, body: { name, gatewayId, base: 'current' } },
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
      {/*
        The Deploy button lives outside `AccordionSummary` on purpose:
        `AccordionSummary` renders as an actual `<button>` in this MUI build
        (no `component="div"` override), so a `<Button>` nested inside it —
        even wrapped in a click-stopping `<span>` — is an invalid
        button-inside-a-button and triggers a React hydration warning. This
        wrapper box gives the floating action row something to position
        against that spans exactly the summary's own height (not the whole
        accordion, which would grow once expanded), while `pointerEvents:
        'none'` on the row itself lets a click on empty space between the
        button and the chevron still fall through to the summary's toggle.
      */}
      <Box sx={{ position: 'relative' }}>
        <AccordionSummary
          // Oxygen's theme sets a default `expandIcon` on every AccordionSummary
          // (`MuiAccordionSummary.defaultProps.expandIcon` in the theme
          // registry) — explicitly null it out since the floating row below
          // renders this card's own rotating chevron; otherwise both render.
          expandIcon={null}
          sx={{
            pl: 3,
            pr: 22,
            '& .MuiAccordionSummary-content': {
              alignItems: 'center',
              flexWrap: 'wrap',
              m: 0,
            },
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
        </AccordionSummary>
        <Box
          sx={{
            alignItems: 'center',
            bottom: 0,
            display: 'flex',
            gap: 1.5,
            pointerEvents: 'none',
            position: 'absolute',
            right: 24,
            top: 0,
          }}
        >
          <Box sx={{ pointerEvents: 'auto' }}>
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
      <AccordionDetails sx={{ px: 3, py: 2 }}>
        <Grid container spacing={3}>
          <Grid size={{ md: hasDeployments ? 6 : 12, xs: 12 }} sx={{ minWidth: 240 }}>
            <GatewayDeployEnvCard
              apiId={apiId}
              currentDeployment={currentDeployment}
              deployments={gatewayDeployments}
              gateway={gateway}
              isGatewayActive={isActive}
              useRestore={useRestore}
              useUndeploy={useUndeploy}
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
                apiId={apiId}
                deployments={gatewayDeployments}
                onRefresh={onRefresh}
                refreshing={refreshing}
                useDelete={useDelete}
              />
            </Grid>
          )}
        </Grid>
      </AccordionDetails>
    </Accordion>
  );
}

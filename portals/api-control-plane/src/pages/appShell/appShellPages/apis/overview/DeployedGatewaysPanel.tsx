/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import { Box, Button, Card, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { Server } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Link as RouterLink, useParams } from 'react-router-dom';

import type { Gateway } from '@/api/resources/gateways';
import type { Deployment } from '@/api/resources/restApis/deployments';
import { routes } from '@/routes/paths';

type Props = { gateways: Gateway[]; deployments: Deployment[] };

const messages = defineMessages({
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DeployedGatewaysPanel.title',
    defaultMessage: 'Deployed gateways',
  },
  seeMore: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DeployedGatewaysPanel.seeMore',
    defaultMessage: 'See more',
  },
  status: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.overview.DeployedGatewaysPanel.status',
    defaultMessage:
      '{status, select, DEPLOYED {Deployed} UNDEPLOYED {Undeployed} DEPLOYING {Deploying} UNDEPLOYING {Undeploying} FAILED {Failed} ARCHIVED {Archived} other {{status}}}',
  },
});

export function DeployedGatewaysPanel({ gateways, deployments }: Props) {
  const { orgHandle = '', projectHandler = '', apiHandler = '' } = useParams();
  const intl = useIntl();
  const latestByGateway = new Map<string, Deployment>();
  [...deployments]
    .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
    .forEach((deployment) => {
      if (!latestByGateway.has(deployment.gatewayId)) {
        latestByGateway.set(deployment.gatewayId, deployment);
      }
    });
  const gatewayDeployments = [...latestByGateway.values()];
  const latestDeployments = gatewayDeployments.slice(0, 5);
  const rows =
    latestDeployments.length > 0
      ? latestDeployments.map((deployment) => ({
          gateway: gateways.find((gateway) => gateway.id === deployment.gatewayId),
          gatewayId: deployment.gatewayId,
          id: deployment.deploymentId,
          status: deployment.status,
        }))
      : gateways.map((gateway) => ({
          gateway,
          gatewayId: gateway.id ?? gateway.displayName,
          id: gateway.id ?? gateway.displayName,
          status: 'DEPLOYED',
        }));

  return (
    <Card>
      <Box sx={{ px: 2, py: 1.5 }}>
        <Typography sx={{ fontWeight: 600 }} variant="h6">
          <FormattedMessage {...messages.title} />
        </Typography>
      </Box>
      <Divider />
      <Stack divider={<Divider />}>
        {rows.map(({ gateway, gatewayId, id, status }) => {
          const successful = status === 'DEPLOYED';
          return (
            <Stack
              alignItems="center"
              direction="row"
              key={id}
              spacing={1.25}
              sx={{ px: 2, py: 1.5 }}
            >
              <Box
                sx={{
                  alignItems: 'center',
                  bgcolor: 'action.hover',
                  borderRadius: 1,
                  color: 'primary.main',
                  display: 'flex',
                  height: 32,
                  justifyContent: 'center',
                  width: 32,
                }}
              >
                <Server size={17} />
              </Box>
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Typography noWrap variant="body2">
                  {gateway?.displayName || gatewayId}
                </Typography>
              </Box>
              <Stack alignItems="center" direction="row" spacing={0.5}>
                <Box
                  sx={{
                    bgcolor: successful ? 'success.main' : 'warning.main',
                    borderRadius: '50%',
                    height: 7,
                    width: 7,
                  }}
                />
                <Typography color={successful ? 'success.main' : 'warning.main'} variant="caption">
                  {intl.formatMessage(messages.status, { status })}
                </Typography>
              </Stack>
            </Stack>
          );
        })}
      </Stack>
      {gatewayDeployments.length > 5 && (
        <Box sx={{ borderTop: '1px solid', borderColor: 'divider', p: 1, textAlign: 'center' }}>
          <Button
            component={RouterLink}
            size="small"
            to={routes.apiDeploy(orgHandle, projectHandler, apiHandler)}
          >
            <FormattedMessage {...messages.seeMore} />
          </Button>
        </Box>
      )}
    </Card>
  );
}

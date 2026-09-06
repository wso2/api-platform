/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import { Box, Button, Card, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import { Server } from '@wso2/oxygen-ui-icons-react';
import { Link as RouterLink, useParams } from 'react-router-dom';

import type { Gateway } from '@/api/resources/gateways';
import type { Deployment } from '@/api/resources/restApis/deployments';
import { routes } from '@/routes/paths';

type Props = { gateways: Gateway[]; deployments: Deployment[] };

export function DeployedGatewaysPanel({ gateways, deployments }: Props) {
  const { orgHandle = '', projectHandler = '', apiHandler = '' } = useParams();
  const latestDeployments = [...deployments]
    .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
    .slice(0, 5);
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
          Deployed gateways
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
                  {status}
                </Typography>
              </Stack>
            </Stack>
          );
        })}
      </Stack>
      {deployments.length > 5 && (
        <Box sx={{ borderTop: '1px solid', borderColor: 'divider', p: 1, textAlign: 'center' }}>
          <Button
            component={RouterLink}
            size="small"
            to={routes.apiDeploy(orgHandle, projectHandler, apiHandler)}
          >
            See more
          </Button>
        </Box>
      )}
    </Card>
  );
}

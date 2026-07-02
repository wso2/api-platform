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

import { useState } from 'react';
import {
  Box,
  Chip,
  Divider,
  Drawer,
  Form,
  IconButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { BoxIcon, Network, Server, Workflow, X } from '@wso2/oxygen-ui-icons-react';
import type { Gateway } from '../../../../apis/gatewayTypes';
import type { DeploymentResponse, Proxy } from '../../../../utils/types';
import {
  GatewayCard,
  getProxyIdentifier,
  ProxyCard,
} from './ProviderMap/ProviderMapTab';

type ResourceDrawerCardsProps = {
  proxies: Proxy[];
  gateways: Gateway[];
  gatewayDeployments: Record<string, DeploymentResponse>;
  proxyDeployments: Record<string, DeploymentResponse>;
  onProxyClick: (proxyId: string, proxyProjectId?: string) => void;
  onCreateProxy?: () => void;
  onBlockedNavigation?: () => void;
};

export default function ResourceDrawerCards({
  proxies,
  gateways,
  gatewayDeployments,
  proxyDeployments,
  onProxyClick,
  onCreateProxy,
  onBlockedNavigation,
}: ResourceDrawerCardsProps) {
  const [resourceDrawerView, setResourceDrawerView] = useState<
    'proxies' | 'gateways' | null
  >(null);

  return (
    <>
      <Stack
        direction="row"
        spacing={1.5}
        alignItems="center"
        flexWrap="wrap"
        sx={{ mb: 2 }}
      >
        {[
          {
            id: 'proxies' as const,
            label: 'Available Proxies',
            count: proxies.length,
            icon: <Workflow size={18} />,
          },
          {
            id: 'gateways' as const,
            label: 'Deployed Gateways',
            count: gateways.length,
            icon: <Network size={18} />,
          },
        ].map((option) => (
          <Box key={option.id}>
            <Form.CardButton
              onClick={
                option.count > 0
                  ? () => setResourceDrawerView(option.id)
                  : option.id === 'proxies'
                    ? onCreateProxy
                    : onBlockedNavigation
              }
              disabled={
                option.count === 0 &&
                (option.id === 'proxies' ? !onCreateProxy : !onBlockedNavigation)
              }
              sx={{
                height: '100%',
                p: 1.5,
                minWidth: 220,
                '& .MuiCardHeader-root': {
                  width: '100%',
                  p: 0,
                  alignItems: 'center',
                },
                '& .MuiCardHeader-action': {
                  m: 0,
                  pl: 1,
                  alignSelf: 'center',
                },
              }}
            >
              <Form.CardHeader
                disableTypography
                title={
                  <Stack
                    direction="row"
                    spacing={1}
                    justifyContent="center"
                    alignItems="center"
                  >
                    <Box
                      sx={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        width: 36,
                        height: 36,
                        borderRadius: '50%',
                        background: '#fff1e5',
                        color: '#FF8A00',
                        flexShrink: 0,
                      }}
                    >
                      {option.icon}
                    </Box>
                    <Typography variant="body2" fontWeight={600} fontSize="0.8rem">
                      {option.label}
                    </Typography>
                  </Stack>
                }
                action={
                  <Chip
                    label={option.count}
                    size="small"
                    sx={{
                      height: 22,
                      fontSize: '0.75rem',
                      background: option.count > 0
                        ? 'linear-gradient(135deg, #FF8A00 0%, #FF2D00 100%)'
                        : '#e0e0e0',
                      color: option.count > 0 ? '#fff' : '#9e9e9e',
                      fontWeight: 600,
                      border: 'none',
                      '& .MuiChip-label': { px: 1 },
                    }}
                  />
                }
              />
            </Form.CardButton>
          </Box>
        ))}
      </Stack>

      <Drawer
        anchor="right"
        open={resourceDrawerView !== null}
        onClose={() => setResourceDrawerView(null)}
      >
        <Box sx={{ width: 420, maxWidth: '100vw', p: 2 }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              justifyContent: 'space-between',
              gap: 1,
            }}
          >
            <Stack spacing={0.5}>
              <Typography variant="subtitle1" fontWeight={600}>
                {resourceDrawerView === 'gateways'
                  ? 'Deployed Gateways'
                  : 'Available Proxies'}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {resourceDrawerView === 'gateways'
                  ? `${gateways.length} gateway${
                      gateways.length === 1 ? '' : 's'
                    } deployed for this provider.`
                  : `${proxies.length} prox${
                      proxies.length === 1 ? 'y' : 'ies'
                    } available for this provider.`}
              </Typography>
            </Stack>
            <IconButton
              size="small"
              aria-label="Close resource drawer"
              onClick={() => setResourceDrawerView(null)}
            >
              <X size={18} />
            </IconButton>
          </Box>

          <Divider sx={{ my: 2 }} />

          <Stack spacing={1.5}>
            {resourceDrawerView === 'proxies' &&
              (proxies.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  No proxies linked to this provider.
                </Typography>
              ) : (
                proxies.map((proxy) => {
                  const proxyId = getProxyIdentifier(proxy);

                  return (
                    <ProxyCard
                      key={proxyId ?? proxy.displayName}
                      proxy={proxy}
                      deployment={
                        proxyId ? proxyDeployments[proxyId] : undefined
                      }
                      onClick={
                        proxyId
                          ? () => onProxyClick(proxyId, proxy.projectId)
                          : undefined
                      }
                    />
                  );
                })
              ))}

            {resourceDrawerView === 'gateways' &&
              (gateways.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  No deployed gateways available for this provider.
                </Typography>
              ) : (
                gateways.map((gw) => (
                  <GatewayCard
                    key={gw.id}
                    gateway={gw}
                    deployment={gatewayDeployments[gw.id]}
                  />
                ))
              ))}
          </Stack>
        </Box>
      </Drawer>
    </>
  );
}

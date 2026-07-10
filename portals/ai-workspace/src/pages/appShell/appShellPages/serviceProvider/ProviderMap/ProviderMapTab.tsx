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

import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Avatar,
  Box,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  ArrowUpRight,
  Clock,
  Layers,
  MoreHorizontal,
  Network,
  Server,
  Tag,
  Zap,
} from '@wso2/oxygen-ui-icons-react';
import { useLLMProvider } from '../../../../../contexts/llmProvider';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import { getGateways } from '../../../../../apis/gatewayApis';
import {
  getLLMProviderDeployments,
  getLLMProviderProxies,
} from '../../../../../apis/llmProviderApis';
import { getLLMProxyDeployments } from '../../../../../apis/llmProxiesApis';
import type { Gateway } from '../../../../../apis/gatewayTypes';
import type { DeploymentResponse, Proxy } from '../../../../../utils/types';
import { PLATFORM_API_BASE_URL } from '../../../../../config.env';
import { logger } from '../../../../../utils/logger';
import { buildProjectPath } from '../../../../../utils/projectRouting';

import AnthropicLogo from '../../../../../assets/brands/Anthropic.jpg';
import AWSBedrockLogo from '../../../../../assets/brands/AWSBedrock.webp';
import AzureLogo from '../../../../../assets/brands/Azure.png';
import GoogleVertexLogo from '../../../../../assets/brands/GoogleVertex.png';
import GoogleGeminiLogo from '../../../../../assets/brands/googlegemini.png';
import MistralAILogo from '../../../../../assets/brands/mistralai.png';
import OpenAILogo from '../../../../../assets/brands/openAI.png';

const TEMPLATE_LOGO_MAP: Record<string, string> = {
  openai: OpenAILogo,
  anthropic: AnthropicLogo,
  'azure-openai': AzureLogo,
  'azureai-foundry': AzureLogo,
  'aws-bedrock': AWSBedrockLogo,
  awsbedrock: AWSBedrockLogo,
  'google-vertex': GoogleVertexLogo,
  gemini: GoogleGeminiLogo,
  mistralai: MistralAILogo,
  mistral: MistralAILogo,
};

function getProviderLogo(template?: string): string | undefined {
  if (!template) return undefined;
  const key = template.toLowerCase();
  return TEMPLATE_LOGO_MAP[key];
}

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

export function getProxyIdentifier(proxy: Proxy): string | undefined {
  const proxyRecord = proxy as Proxy & {
    proxyId?: unknown;
    uuid?: unknown;
  };
  const candidates = [
    proxyRecord.id,
    proxyRecord.proxyId,
    proxyRecord.uuid,
    proxyRecord.displayName,
  ];

  return candidates.find(
    (value): value is string =>
      typeof value === 'string' && value.trim().length > 0
  );
}

function formatRelative(value?: string): string {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  const diffMs = Date.now() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
  const diffHrs = Math.floor(diffMins / 60);
  if (diffHrs < 24) return `${diffHrs} hour${diffHrs > 1 ? 's' : ''} ago`;
  const diffDays = Math.floor(diffHrs / 24);
  return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
}

function getDeploymentTimestamp(deployment: DeploymentResponse): number {
  const value = deployment.updatedAt ?? deployment.createdAt;
  const timestamp = new Date(value).getTime();
  return Number.isNaN(timestamp) ? 0 : timestamp;
}

export function getCurrentDeployment(
  deployments: DeploymentResponse[]
): DeploymentResponse | undefined {
  return deployments.reduce<DeploymentResponse | undefined>((current, item) => {
    if (!current) return item;
    if (item.status === 'DEPLOYED' && current.status !== 'DEPLOYED') {
      return item;
    }
    if (item.status !== 'DEPLOYED' && current.status === 'DEPLOYED') {
      return current;
    }
    return getDeploymentTimestamp(item) > getDeploymentTimestamp(current)
      ? item
      : current;
  }, undefined);
}

function StatusChip({ deployed }: { deployed: boolean }) {
  return (
    <Chip
      label={deployed ? 'Deployed' : 'Not Deployed'}
      size="small"
      variant="outlined"
      color={deployed ? 'success' : 'default'}
      sx={{
        height: 20,
        fontSize: '0.68rem',
        '& .MuiChip-label': { px: 0.75 },
      }}
    />
  );
}

function DeploymentSection({
  deploymentId,
  createdAt,
  isActive,
}: {
  deploymentId?: string;
  createdAt?: string;
  isActive?: boolean;
}) {
  return (
    <>
      <Divider sx={{ my: 1.25 }} />
      <Typography
        variant="caption"
        color="text.secondary"
        fontWeight={500}
        sx={{ display: 'block', mb: 0.75 }}
      >
        Current Deployment:
      </Typography>
      {deploymentId ? (
        <Stack spacing={0.5}>
          <Stack direction="row" spacing={0.75} alignItems="center">
            <Tag size={12} color="var(--oxygen-palette-text-disabled)" />
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ fontFamily: 'monospace', fontSize: '0.72rem' }}
            >
              {deploymentId.slice(0, 8)}
            </Typography>
            {isActive && (
              <Chip
                label="Active"
                size="small"
                variant="outlined"
                color="success"
                sx={{
                  fontSize: '0.62rem',
                  height: 16,
                  '& .MuiChip-label': { px: 0.75 },
                }}
              />
            )}
          </Stack>
          {createdAt && (
            <Stack direction="row" spacing={0.75} alignItems="center">
              <Clock size={12} color="var(--oxygen-palette-text-disabled)" />
              <Typography variant="caption" color="text.disabled">
                {formatRelative(createdAt)}
              </Typography>
            </Stack>
          )}
        </Stack>
      ) : (
        <Stack direction="row" spacing={0.75} alignItems="center">
          <Tag size={12} color="var(--oxygen-palette-text-disabled)" />
          <Typography
            variant="caption"
            color="text.disabled"
            sx={{ fontStyle: 'italic' }}
          >
            None
          </Typography>
        </Stack>
      )}
    </>
  );
}

export function ProxyCard({
  proxy,
  deployment,
  onClick,
}: {
  proxy: Proxy;
  deployment?: DeploymentResponse;
  onClick?: () => void;
}) {
  const isDeployed = deployment?.status === 'DEPLOYED';

  return (
    <Card
      onClick={onClick}
      onKeyDown={(event) => {
        if (!onClick) return;
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onClick();
        }
      }}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      sx={{
        cursor: onClick ? 'pointer' : undefined,
      }}
    >
      <CardContent>
        <Stack direction="row" spacing={1.5} alignItems="flex-start">
          <Avatar
            sx={{
              width: 36,
              height: 36,
              borderRadius: 1.5,
              bgcolor: 'primary.50',
              color: 'primary.main',
              flexShrink: 0,
            }}
          >
            <ArrowUpRight size={18} />
          </Avatar>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Stack
              direction="row"
              alignItems="flex-start"
              justifyContent="space-between"
              spacing={0.5}
            >
              <Typography
                variant="subtitle2"
                fontWeight={600}
                noWrap
                title={proxy.displayName}
                sx={{ flex: 1, minWidth: 0 }}
              >
                {proxy.displayName}
              </Typography>
            </Stack>
            <Box>
              <StatusChip deployed={isDeployed} />
            </Box>
            <DeploymentSection
              deploymentId={deployment?.deploymentId}
              createdAt={deployment?.createdAt}
              isActive={isDeployed}
            />
          </Box>
        </Stack>
      </CardContent>
    </Card>
  );
}

interface GatewayCardProps {
  gateway: Gateway;
  deployment?: DeploymentResponse;
}

export function GatewayCard({ gateway, deployment }: GatewayCardProps) {
  const isDeployed = deployment?.status === 'DEPLOYED';

  return (
    <Card>
      <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
        <Stack direction="row" spacing={1.5} alignItems="flex-start">
          <Avatar
            sx={{
              width: 36,
              height: 36,
              borderRadius: 1.5,
              bgcolor: isDeployed ? 'success.50' : 'grey.100',
              color: isDeployed ? 'success.dark' : 'text.secondary',
              flexShrink: 0,
            }}
          >
            <Server size={18} />
          </Avatar>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Stack
              direction="row"
              alignItems="flex-start"
              justifyContent="space-between"
              spacing={0.5}
            >
              <Typography
                variant="subtitle2"
                fontWeight={600}
                noWrap
                title={gateway.displayName}
                sx={{
                  flex: 1,
                  minWidth: 0,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}
              >
                {gateway.displayName}
              </Typography>
              {gateway.isActive && (
                <Chip
                  label="Active"
                  size="small"
                  sx={{
                    bgcolor: 'success.light',
                    color: 'success.dark',
                    fontSize: '0.62rem',
                    height: 16,
                    flexShrink: 0,
                    '& .MuiChip-label': { px: 0.75 },
                  }}
                />
              )}
            </Stack>
            <Box sx={{ mt: 0.75 }}>
              <StatusChip deployed={isDeployed} />
            </Box>
            {(gateway.functionalityType ||
              gateway.endpoints?.[0] ||
              gateway.vhost) && (
              <Stack spacing={0.5} mt={1}>
                {(gateway.endpoints?.[0] || gateway.vhost) && (
                  <Stack direction="row" spacing={0.75} alignItems="center">
                    <Network
                      size={13}
                      color="var(--oxygen-palette-text-secondary)"
                    />
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      noWrap
                      title={gateway.endpoints?.[0] || gateway.vhost}
                    >
                      {gateway.endpoints?.[0] || gateway.vhost}
                    </Typography>
                  </Stack>
                )}
              </Stack>
            )}
            <DeploymentSection
              deploymentId={deployment?.deploymentId}
              createdAt={deployment?.createdAt}
              isActive={isDeployed}
            />
          </Box>
        </Stack>
      </CardContent>
    </Card>
  );
}

function ProviderCard({
  name,
  template,
  deploymentCount,
}: {
  name: string;
  template?: string;
  description?: string;
  status?: string;
  deploymentCount: number;
}) {
  const logoSrc = getProviderLogo(template);
  const isDeployed = deploymentCount > 0;

  return (
    <Card
      sx={{
        borderRadius: 2,
        transition: 'box-shadow 0.2s',
        '&:hover': { boxShadow: 2 },
        borderColor: isDeployed ? 'primary.light' : undefined,
      }}
    >
      <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
        <Stack direction="row" spacing={1.5} alignItems="flex-start">
          <Avatar
            src={logoSrc}
            sx={{
              width: 36,
              height: 36,
              borderRadius: 1.5,
              bgcolor: logoSrc ? 'transparent' : 'secondary.light',
              fontSize: '0.8rem',
              fontWeight: 600,
              flexShrink: 0,
              border: logoSrc ? '1px solid' : undefined,
              borderColor: logoSrc ? 'divider' : undefined,
            }}
          >
            {!logoSrc && getInitials(name)}
          </Avatar>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Stack
              direction="row"
              alignItems="flex-start"
              justifyContent="space-between"
              spacing={0.5}
            >
              <Typography
                variant="subtitle2"
                fontWeight={600}
                noWrap
                title={name}
                sx={{ flex: 1, minWidth: 0 }}
              >
                {name}
              </Typography>
              <IconButton
                size="small"
                sx={{ p: 0.25, flexShrink: 0, mt: -0.25 }}
              >
                <MoreHorizontal size={16} />
              </IconButton>
            </Stack>
            <Box sx={{ mt: 0.75 }}>
              <StatusChip deployed={isDeployed} />
            </Box>
            <DeploymentSection />
          </Box>
        </Stack>
      </CardContent>
    </Card>
  );
}

function ColumnConnector() {
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
        width: 44,
        pt: 4.5,
        color: 'text.disabled',
      }}
    >
      <ArrowRight size={18} />
    </Box>
  );
}

function EmptyColumnPlaceholder({ message }: { message: string }) {
  return (
    <Box
      sx={{
        border: '1px dashed',
        borderColor: 'divider',
        borderRadius: 2,
        p: 2.5,
        textAlign: 'center',
      }}
    >
      <Typography variant="caption" color="text.disabled">
        {message}
      </Typography>
    </Box>
  );
}

export default function ProviderMapTab() {
  const { provider } = useLLMProvider();
  const { currentOrganization, projectsForCurrentOrganization } = useAppShell();
  const navigate = useNavigate();

  const [proxies, setProxies] = useState<Proxy[]>([]);
  const [proxyDeployments, setProxyDeployments] = useState<
    Record<string, DeploymentResponse>
  >({});
  const [gateways, setGateways] = useState<Gateway[]>([]);
  const [deployments, setDeployments] = useState<DeploymentResponse[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const organizationId = currentOrganization?.uuid ?? '';

  const loadData = useCallback(async () => {
    if (!provider?.id || !organizationId) return;

    setIsLoading(true);
    setError(null);
    setProxyDeployments({});

    try {
      const [proxiesResult, gatewaysResult, deploymentsResult] =
        await Promise.allSettled([
          getLLMProviderProxies(
            provider.id,
            organizationId,
            PLATFORM_API_BASE_URL
          ),
          getGateways(organizationId),
          getLLMProviderDeployments(
            provider.id,
            organizationId,
            PLATFORM_API_BASE_URL
          ),
        ]);

      const gatewayList =
        gatewaysResult.status === 'fulfilled'
          ? gatewaysResult.value.list ?? []
          : [];

      if (gatewaysResult.status === 'fulfilled') {
        setGateways(gatewayList);
      } else {
        logger.error('Failed to load gateways:', gatewaysResult.reason);
        setGateways([]);
      }

      if (proxiesResult.status === 'fulfilled') {
        const proxyList = proxiesResult.value.list ?? [];
        setProxies(proxyList);

        const proxyDeploymentEntries = await Promise.all(
          proxyList.map(async (proxy) => {
            const proxyId = getProxyIdentifier(proxy);

            if (!proxyId) {
              logger.error(
                'Skipping proxy deployment lookup because proxy id is unavailable:',
                proxy
              );
              return undefined;
            }

            try {
              const deploymentResponses = await Promise.all(
                gatewayList.map((gateway) =>
                  getLLMProxyDeployments(
                    proxyId,
                    organizationId,
                    PLATFORM_API_BASE_URL,
                    gateway.id
                  ).catch((err) => {
                    logger.error(
                      `Failed to load deployments for proxy ${proxyId} on gateway ${gateway.id}:`,
                      err
                    );
                    return { list: [] as DeploymentResponse[], count: 0 };
                  })
                )
              );
              const proxyDeploymentsForGateways = deploymentResponses.flatMap(
                (response) => response.list ?? []
              );

              return [
                proxyId,
                getCurrentDeployment(proxyDeploymentsForGateways),
              ] as const;
            } catch (err) {
              logger.error(
                `Failed to load deployments for proxy ${proxyId}:`,
                err
              );
              return [proxyId, undefined] as const;
            }
          })
        );

        setProxyDeployments(
          proxyDeploymentEntries.reduce<Record<string, DeploymentResponse>>(
            (acc, entry) => {
              if (!entry) return acc;

              const [proxyId, deployment] = entry;
              if (deployment) {
                acc[proxyId] = deployment;
              }
              return acc;
            },
            {}
          )
        );
      } else {
        logger.error('Failed to load proxies:', proxiesResult.reason);
        setProxies([]);
      }

      if (deploymentsResult.status === 'fulfilled') {
        setDeployments(deploymentsResult.value.list ?? []);
      } else {
        logger.error('Failed to load deployments:', deploymentsResult.reason);
      }
    } catch (err) {
      logger.error('Failed to load provider map data:', err);
      setError('Failed to load provider map data.');
    } finally {
      setIsLoading(false);
    }
  }, [provider?.id, organizationId]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const handleProxyClick = useCallback(
    (proxyId: string, proxyProjectId?: string) => {
      // removed: ProjectBase no longer carries a `handler` alias field, so
      // match on `id` only.
      const proxyProject = projectsForCurrentOrganization.find(
        (project) => project.id === proxyProjectId
      );

      if (!currentOrganization || !proxyProject) {
        logger.error(
          `Unable to navigate to proxy ${proxyId} because project ${
            proxyProjectId ?? ''
          } is unavailable.`
        );
        return;
      }

      const proxyPath = `/proxies/${encodeURIComponent(proxyId)}`;
      navigate(buildProjectPath(currentOrganization, proxyProject, proxyPath));
    },
    [currentOrganization, navigate, projectsForCurrentOrganization]
  );

  const deploymentsByGateway = deployments.reduce<
    Record<string, DeploymentResponse>
  >((acc, dep) => {
    if (dep.gatewayId && dep.status === 'DEPLOYED') {
      acc[dep.gatewayId] = dep;
    } else if (dep.gatewayId && !acc[dep.gatewayId]) {
      acc[dep.gatewayId] = dep;
    }
    return acc;
  }, {});

  const activeDeploymentCount = Object.values(deploymentsByGateway).filter(
    (d) => d.status === 'DEPLOYED'
  ).length;

  if (isLoading) {
    return (
      <Box
        sx={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          minHeight: 240,
        }}
      >
        <CircularProgress size={32} />
      </Box>
    );
  }

  if (error) {
    return (
      <Box sx={{ p: 2 }}>
        <Typography color="error" variant="body2">
          {error}
        </Typography>
      </Box>
    );
  }

  return (
    <Box>
      <Stack direction="row" spacing={1} alignItems="center" mb={2.5}>
        <Typography variant="subtitle1" fontWeight={600}>
          Architecture Map
        </Typography>
        <Typography variant="body2" color="text.secondary">
          — Visualize the relationships between proxies, gateways, and this
          provider
        </Typography>
      </Stack>

      <Box
        sx={{
          display: 'flex',
          alignItems: 'flex-start',
          gap: 0,
          overflowX: 'auto',
          pb: 1,
        }}
      >
        {/* Proxies Column */}
        <Box sx={{ flex: 1, minWidth: 220 }}>
          <Stack
            direction="row"
            spacing={0.75}
            alignItems="center"
            mb={1.5}
            sx={{ px: 0.5 }}
          >
            <Zap size={14} color="var(--oxygen-palette-primary-main)" />
            <Typography variant="caption" fontWeight={600} color="primary.main">
              LLM PROXIES
            </Typography>
            <Chip
              label={proxies.length}
              size="small"
              sx={{ height: 18, fontSize: '0.65rem' }}
            />
          </Stack>
          <Stack spacing={1.5}>
            {proxies.length === 0 ? (
              <EmptyColumnPlaceholder message="No proxies linked to this provider" />
            ) : (
              proxies.map((proxy) => {
                const proxyId = getProxyIdentifier(proxy);

                return (
                  <ProxyCard
                    key={proxyId ?? proxy.displayName}
                    proxy={proxy}
                    deployment={proxyId ? proxyDeployments[proxyId] : undefined}
                    onClick={
                      proxyId
                        ? () => handleProxyClick(proxyId, proxy.projectId)
                        : undefined
                    }
                  />
                );
              })
            )}
          </Stack>
        </Box>

        <ColumnConnector />

        {/* Gateways Column */}
        <Box sx={{ flex: 1, minWidth: 220 }}>
          <Stack
            direction="row"
            spacing={0.75}
            alignItems="center"
            mb={1.5}
            sx={{ px: 0.5 }}
          >
            <Server size={14} color="var(--oxygen-palette-text-secondary)" />
            <Typography
              variant="caption"
              fontWeight={600}
              color="text.secondary"
            >
              GATEWAYS
            </Typography>
            <Chip
              label={gateways.length}
              size="small"
              sx={{ height: 18, fontSize: '0.65rem' }}
            />
          </Stack>
          <Stack spacing={1.5}>
            {gateways.length === 0 ? (
              <EmptyColumnPlaceholder message="No gateways available" />
            ) : (
              gateways.map((gw) => (
                <GatewayCard
                  key={gw.id}
                  gateway={gw}
                  deployment={deploymentsByGateway[gw.id]}
                />
              ))
            )}
          </Stack>
        </Box>

        <ColumnConnector />

        {/* Provider Column */}
        <Box sx={{ flex: 1, minWidth: 220 }}>
          <Stack
            direction="row"
            spacing={0.75}
            alignItems="center"
            mb={1.5}
            sx={{ px: 0.5 }}
          >
            <Network size={14} color="var(--oxygen-palette-text-secondary)" />
            <Typography
              variant="caption"
              fontWeight={600}
              color="text.secondary"
            >
              PROVIDER
            </Typography>
          </Stack>
          {provider && (
            <ProviderCard
              name={provider.displayName}
              template={provider.template}
              description={provider.description}
              status={provider.status}
              deploymentCount={activeDeploymentCount}
            />
          )}
        </Box>
      </Box>
    </Box>
  );
}

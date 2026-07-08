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

import { Link as RouterLink } from 'react-router-dom';
import {
  Avatar,
  Box,
  Button,
  Card,
  CardContent,
  CardHeader,
  Chip,
  Divider,
  ParticleBackground,
  Skeleton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Layers, Plus } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { useMCPServers } from '../../../../contexts/MCP';
import { formatRelativeTime } from '../../../../contexts/ApplicationsContext';
import NoMCPServers from '../../../../assets/images/NoMCPServers.svg';
import ErrorAlert from '../../../../Components/common/ErrorAlert';

function truncateWords(text: string, maxWords: number): string {
  const words = text.trim().split(/\s+/);
  if (words.length <= maxWords) return text.trim();
  return `${words.slice(0, maxWords).join(' ')}…`;
}

function getHttpStatusCode(error?: Error | null): number | null {
  if (!error) return null;

  const axiosStatus = (error as any)?.response?.status;
  if (typeof axiosStatus === 'number') return axiosStatus;

  const match = error.message?.match(/status:\s*(\d{3})/i);
  if (match) return parseInt(match[1], 10);

  return null;
}

type MCPProxiesSummaryCardSectionProps = {
  mcpProxiesPath: string;
  newMCPProxyPath: string;
  onMCPProxyClick: (proxyId: string) => void;
};

export default function MCPProxiesSummaryCardSection({
  mcpProxiesPath,
  newMCPProxyPath,
  onMCPProxyClick,
}: MCPProxiesSummaryCardSectionProps) {
  const { mcpServersResponse, isLoading, error, refreshMCPServers } =
    useMCPServers();
  const servers = mcpServersResponse.list;

  const errorStatusCode = getHttpStatusCode(error);
  const isNotFoundError = errorStatusCode === 404;
  const hasServers = !error && servers.length > 0;
  const isEmptyState =
    !isLoading && (isNotFoundError || (!error && servers.length === 0));
  const showSeeMore = mcpServersResponse.count > 6;

  return (
    <Card
      sx={{
        height: '100%',
        width: '100%',
        minHeight: 300,
        ...(isEmptyState
          ? {
              display: 'flex',
              position: 'relative',
              overflow: 'hidden',
            }
          : {}),
      }}
    >
      {isEmptyState ? <ParticleBackground opacity={0.6} /> : null}

      {!isEmptyState ? (
        <CardHeader
          title="MCP Proxies"
          subheader={
            isLoading
              ? 'Loading…'
              : error && !isNotFoundError
                ? 'Total: 0'
                : `Total: ${mcpServersResponse.count}`
          }
          slotProps={{
            title: { sx: { fontSize: '1rem', fontWeight: 700, marginBottom: 0.3 } },
            subheader: { sx: { fontSize: '0.82rem' } },
          }}
          action={
            !hasServers ? null : showSeeMore ? (
              <Button component={RouterLink} to={mcpProxiesPath} size="small">
                See more
              </Button>
            ) : (
              <Button component={RouterLink} to={newMCPProxyPath} size="small">
                + Add New
              </Button>
            )
          }
        />
      ) : null}

      <CardContent
        sx={{
          position: 'relative',
          zIndex: 1,
          ...(isEmptyState
            ? {
                flex: 1,
                minHeight: 300,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
              }
            : {}),
        }}
      >
        {isLoading ? (
          <Stack divider={<Divider />} spacing={1.5}>
            {[0, 1, 2].map((item) => (
              <Box
                key={item}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 1.5,
                  width: '100%',
                }}
              >
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1.25,
                    minWidth: 0,
                    flex: 1,
                  }}
                >
                  <Skeleton variant="circular" width={36} height={36} />
                  <Box sx={{ minWidth: 0, width: '100%' }}>
                    <Skeleton variant="text" width="50%" height={24} />
                    <Skeleton variant="text" width="75%" height={18} />
                  </Box>
                </Box>
                <Skeleton variant="text" width={80} height={18} />
              </Box>
            ))}
          </Stack>
        ) : error && !isNotFoundError ? (
          <Box sx={{ py: 2 }}>
            <ErrorAlert error={error} onRetry={refreshMCPServers} />
          </Box>
        ) : servers.length === 0 || isNotFoundError ? (
          <Stack
            spacing={1.5}
            alignItems="center"
            justifyContent="center"
            sx={{ textAlign: 'center', py: 2, width: '100%' }}
          >
            <Box
              component="img"
              src={NoMCPServers}
              alt="No MCP proxies"
              sx={{ width: 140, maxWidth: '80%' }}
            />
            <Typography variant="h6" sx={{ fontWeight: 700 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.your.first.mcp.server"
                defaultMessage="Create your first MCP Proxy"
              />
            </Typography>
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ maxWidth: 420 }}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.setup.an.mcp.server.description"
                defaultMessage="Set up an MCP Proxy to expose tools, prompts, and resources through your AI gateway workflows."
              />
            </Typography>
            <Button
              variant="contained"
              component={RouterLink}
              to={newMCPProxyPath}
              startIcon={<Plus size={20} />}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.external.server"
                defaultMessage="Create MCP Proxy"
              />
            </Button>
          </Stack>
        ) : (
          <Stack divider={<Divider />} spacing={1.5}>
            {servers.slice(0, 6).map((server) => (
              <Box
                key={server.id}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 1.5,
                  width: '100%',
                }}
              >
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1.25,
                    minWidth: 0,
                    cursor: 'pointer',
                  }}
                  onClick={() => onMCPProxyClick(server.id)}
                >
                  <Avatar
                    sx={{
                      width: 36,
                      height: 36,
                      bgcolor: 'primary.light',
                      color: 'primary.contrastText',
                    }}
                  >
                    <Layers size={18} />
                  </Avatar>
                  <Box sx={{ minWidth: 0, overflow: 'hidden' }}>
                    <Stack
                      direction="row"
                      spacing={0.75}
                      alignItems="center"
                      sx={{ minWidth: 0 }}
                    >
                      <Typography variant="body1" sx={{ fontWeight: 600 }} noWrap>
                        {truncateWords(server.displayName || 'No Name', 12)}
                      </Typography>
                      <Chip
                        label={server.version || '—'}
                        size="small"
                        variant="outlined"
                        sx={{ flexShrink: 0, marginTop: 0.5 }}
                      />
                    </Stack>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      fontSize="0.7rem"
                      noWrap
                    >
                      {truncateWords(server.description?.trim() || '', 12)}
                    </Typography>
                  </Box>
                </Box>

                <Stack
                  direction="row"
                  spacing={0.75}
                  alignItems="center"
                  sx={{ flexShrink: 0, whiteSpace: 'nowrap' }}
                >
                  <Clock size={14} />
                  <Typography variant="caption" color="text.secondary" noWrap>
                    {formatRelativeTime(server.updatedAt || server.createdAt)}
                  </Typography>
                </Stack>
              </Box>
            ))}
          </Stack>
        )}
      </CardContent>
    </Card>
  );
}

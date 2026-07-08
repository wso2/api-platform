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
import { useProxies } from '../../../../contexts/proxy';
import { formatRelativeTime } from '../../../../contexts/ApplicationsContext';
import NoProxies from '../../../../assets/images/NoProxies.svg';
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

type LLMProxiesSummaryCardSectionProps = {
  proxiesPath: string;
  newProxyPath: string;
  onProxyClick: (proxyId: string) => void;
};

export default function LLMProxiesSummaryCardSection({
  proxiesPath,
  newProxyPath,
  onProxyClick,
}: LLMProxiesSummaryCardSectionProps) {
  const { proxiesResponse, isLoading, error, refreshProxies } = useProxies();
  const proxies = proxiesResponse.list;

  const proxyErrorStatusCode = getHttpStatusCode(error);
  const isProxyNotFoundError = proxyErrorStatusCode === 404;
  const hasProxies = !error && proxies.length > 0;
  const isEmptyState =
    !isLoading && (isProxyNotFoundError || (!error && proxies.length === 0));
  const showSeeMore = proxiesResponse.count > 6;

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
          title="App LLM Proxies"
          subheader={
            isLoading
              ? 'Loading…'
              : error && !isProxyNotFoundError
                ? 'Total: 0'
                : `Total: ${proxiesResponse.count}`
          }
          slotProps={{
            title: { sx: { fontSize: '1rem', fontWeight: 700, marginBottom: 0.3 } },
            subheader: { sx: { fontSize: '0.82rem' } },
          }}
          action={
            !hasProxies ? null : showSeeMore ? (
              <Button component={RouterLink} to={proxiesPath} size="small">
                See more
              </Button>
            ) : (
              <Button component={RouterLink} to={newProxyPath} size="small">
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
        ) : error && !isProxyNotFoundError ? (
          <Box sx={{ py: 2 }}>
            <ErrorAlert error={error} onRetry={refreshProxies} />
          </Box>
        ) : proxies.length === 0 || isProxyNotFoundError ? (
          <Stack
            spacing={1.5}
            alignItems="center"
            justifyContent="center"
            sx={{ textAlign: 'center', py: 2, width: '100%' }}
          >
            <Box
              component="img"
              src={NoProxies}
              alt="No proxies"
              sx={{ width: 140, maxWidth: '80%' }}
            />
            <Typography variant="h6" sx={{ fontWeight: 700 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.overview.Overview.create.your.first.llm.proxy"
                defaultMessage={'Create your first App LLM Proxy'}
              />
            </Typography>
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ maxWidth: 420 }}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.overview.Overview.create.your.first.llm.proxy.description"
                defaultMessage={
                  'Set up an App LLM Proxy to route model traffic and manage AI access across your applications.'
                }
              />
            </Typography>
            <Button
              variant="contained"
              component={RouterLink}
              to={newProxyPath}
              startIcon={<Plus size={20} />}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.overview.Overview.create.llm.proxy"
                defaultMessage={'Create App LLM Proxy'}
              />
            </Button>
          </Stack>
        ) : (
          <Stack divider={<Divider />} spacing={1.5}>
            {proxies.slice(0, 6).map((proxy) => (
              <Box
                key={proxy.id}
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
                  onClick={() => onProxyClick(proxy.id)}
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
                        {truncateWords(proxy.displayName || 'No Name', 12)}
                      </Typography>
                      <Chip
                        label={proxy.version || '—'}
                        size="small"
                        variant="outlined"
                        sx={{ flexShrink: 0 }}
                      />
                    </Stack>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      fontSize="0.7rem"
                      noWrap
                    >
                      {truncateWords(proxy.description?.trim() || '', 12)}
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
                    {formatRelativeTime(proxy.updatedAt || proxy.createdAt)}
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

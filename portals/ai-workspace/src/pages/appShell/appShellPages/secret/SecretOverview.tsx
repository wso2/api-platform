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

import React, { useEffect, useRef, useState } from 'react';
import { Link as RouterLink, useNavigate, useParams } from 'react-router-dom';
import { Avatar, Box, Button, Card, Chip, Divider, IconButton, PageContent, Skeleton, Stack, Typography } from '@wso2/oxygen-ui';
import { ChevronLeft, Clock, Copy, KeyRound, RotateCw, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { getSecret, getSecretUsages, type SecretMetadata, type SecretReference } from '../../../../apis/secretApis';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import DeleteSecretDialog from './DeleteSecretDialog';
import NoData from '../../../../assets/images/NoData.svg';

// Maps the backend's artifact-type identifiers to display labels for the
// Usages section below.
const REFERENCE_TYPE_LABELS: Record<string, string> = {
  RestApi: 'REST API',
  LlmProvider: 'LLM Provider',
  LlmProxy: 'LLM Proxy',
  Mcp: 'MCP Proxy',
};

function formatReferenceType(type: string): string {
  return REFERENCE_TYPE_LABELS[type] ?? type;
}

// Matches the relative-time convention used elsewhere in the app (e.g.
// contexts/llmProvider/LLMProviderContext.tsx's formatRelativeTime).
function formatRelativeTime(value?: string): string {
  if (!value) return 'Unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Unknown';

  const diffSeconds = Math.abs(Date.now() - date.getTime()) / 1000;
  if (diffSeconds < 45) return 'Just now';
  if (diffSeconds < 90) return '1 minute ago';

  const diffMinutes = diffSeconds / 60;
  if (diffMinutes < 45) return `${Math.round(diffMinutes)} minutes ago`;
  if (diffMinutes < 90) return '1 hour ago';

  const diffHours = diffMinutes / 60;
  if (diffHours < 22) return `${Math.round(diffHours)} hours ago`;
  if (diffHours < 36) return '1 day ago';

  const diffDays = diffHours / 24;
  if (diffDays < 26) return `${Math.round(diffDays)} days ago`;
  if (diffDays < 45) return '1 month ago';

  const diffMonths = diffDays / 30;
  if (diffMonths < 11) return `${Math.round(diffMonths)} months ago`;
  const diffYears = diffDays / 365;
  return `${Math.round(diffYears)} year${Math.round(diffYears) === 1 ? '' : 's'} ago`;
}

function KeyValueRow({ label, children }: { label: string; children: React.ReactNode }): React.JSX.Element {
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: '200px 1fr', gap: 2, py: 1.25 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      <Box>{children}</Box>
    </Box>
  );
}

export default function SecretOverview(): React.JSX.Element {
  const { handle } = useParams<{ handle: string }>();
  const navigate = useNavigate();
  const { currentOrganization } = useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();

  const [secret, setSecret] = useState<SecretMetadata | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SecretMetadata | null>(null);

  const [usages, setUsages] = useState<SecretReference[]>([]);
  const [usagesLoading, setUsagesLoading] = useState(true);
  const [usagesError, setUsagesError] = useState<Error | null>(null);

  const listPath = buildOrgPath(currentOrganization, '/settings/secrets');

  const handleCopyHandle = async (value: string) => {
    await navigator.clipboard.writeText(value);
    showSnackbar('Handle copied to clipboard.', 'success');
  };

  // Guards against out-of-order responses: if the handle changes while a fetch for
  // the previous handle is still in flight, that response must not overwrite the
  // metadata now on screen for the newly-active handle.
  const requestIdRef = useRef(0);

  const fetchSecret = async () => {
    if (!handle) return;
    const requestId = ++requestIdRef.current;
    try {
      setIsLoading(true);
      setError(null);
      const response = await getSecret(handle);
      if (requestIdRef.current !== requestId) return; // superseded by a newer request
      setSecret(response);
    } catch (err) {
      if (requestIdRef.current !== requestId) return;
      setError(err instanceof Error ? err : new Error('Failed to load secret.'));
    } finally {
      if (requestIdRef.current === requestId) setIsLoading(false);
    }
  };

  useEffect(() => {
    void fetchSecret();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [handle]);

  // Separate request-id guard from fetchSecret's: the two fetches are independent
  // and must not clobber each other's staleness tracking.
  const usagesRequestIdRef = useRef(0);

  const fetchUsages = async () => {
    if (!handle) return;
    const requestId = ++usagesRequestIdRef.current;
    try {
      setUsagesLoading(true);
      setUsagesError(null);
      const response = await getSecretUsages(handle);
      if (usagesRequestIdRef.current !== requestId) return;
      setUsages(response);
    } catch (err) {
      if (usagesRequestIdRef.current !== requestId) return;
      setUsagesError(err instanceof Error ? err : new Error('Failed to load usages.'));
    } finally {
      if (usagesRequestIdRef.current === requestId) setUsagesLoading(false);
    }
  };

  useEffect(() => {
    void fetchUsages();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [handle]);

  return (
    <PageContent fullWidth>
      <Button component={RouterLink} to={listPath} size="small" startIcon={<ChevronLeft size={24} />}>
        Back to Secrets
      </Button>

      {isLoading ? (
        <Box sx={{ mt: 3, maxWidth: 820 }}>
          <Skeleton variant="text" width="40%" height={40} />
          <Skeleton variant="text" width="25%" />
          <Skeleton variant="rounded" height={220} sx={{ mt: 2 }} />
        </Box>
      ) : error || !secret ? (
        <Box sx={{ py: 2 }}>
          <ErrorAlert error={error ?? new Error('Secret not found.')} onRetry={fetchSecret} />
        </Box>
      ) : (
        <Stack spacing={3} sx={{ mt: 2, maxWidth: 820 }}>
          {/* Header card */}
          <Card>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                flexWrap: 'wrap',
                gap: 2,
                padding: 2,
              }}
            >
              <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2, minWidth: 0, flex: 1 }}>
                <Avatar
                  sx={{
                    width: 56,
                    height: 56,
                    bgcolor: 'primary.light',
                    color: 'primary.contrastText',
                    flexShrink: 0,
                  }}
                >
                  <KeyRound size={26} />
                </Avatar>
                <Stack spacing={0.75} sx={{ minWidth: 0 }}>
                  <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
                    <Typography variant="h5" sx={{ fontWeight: 700, wordBreak: 'break-word', minWidth: 0 }}>
                      {secret.displayName}
                    </Typography>
                    <Chip label={secret.type} size="small" variant="outlined" />
                  </Stack>
                  {secret.description && (
                    <Typography variant="body2" color="text.secondary">
                      {secret.description}
                    </Typography>
                  )}
                  <Stack direction="row" spacing={0.75} alignItems="center">
                    <Typography variant="caption" color="text.secondary">
                      Last updated :
                    </Typography>
                    <Clock size={14} />
                    <Typography variant="caption" color="text.secondary">
                      {formatRelativeTime(secret.updatedAt)}
                    </Typography>
                  </Stack>
                  {secret.createdBy && (
                    <Typography variant="caption" color="text.secondary">
                      Created by: {secret.createdBy}
                    </Typography>
                  )}
                </Stack>
              </Box>

              <Stack
                spacing={1}
                sx={{
                  alignSelf: 'flex-start',
                  ml: 'auto',
                  alignItems: 'stretch',
                  width: { xs: '100%', sm: 200 },
                  flexShrink: 0,
                }}
              >
                <Button
                  variant="outlined"
                  startIcon={<RotateCw size={16} />}
                  onClick={() => navigate(`${listPath}/${secret.id}/rotate`)}
                  data-cyid="rotate-secret-button"
                  fullWidth
                >
                  Update secret
                </Button>
                <Box sx={{ display: 'flex', justifyContent: 'flex-end', width: '100%' }}>
                  <IconButton
                    size="small"
                    color="error"
                    onClick={() => setDeleteTarget(secret)}
                    aria-label={`Delete ${secret.displayName}`}
                    data-cyid="delete-secret-button"
                  >
                    <Trash2 size={16} />
                  </IconButton>
                </Box>
              </Stack>
            </Box>

            <Divider />

            <Box sx={{ px: 3, py: 1 }}>
              <KeyValueRow label="Handle">
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1,
                    py: 0.5,
                    px: 1.5,
                    width: 'fit-content',
                    bgcolor: 'action.hover',
                    border: 1,
                    borderColor: 'divider',
                    borderRadius: 1,
                    fontFamily: 'monospace',
                    fontSize: '0.875rem',
                  }}
                >
                  <Box sx={{ wordBreak: 'break-all' }}>{secret.id}</Box>
                  <IconButton
                    size="small"
                    onClick={() => void handleCopyHandle(secret.id)}
                    aria-label="Copy handle"
                    sx={{ flexShrink: 0 }}
                  >
                    <Copy size={14} />
                  </IconButton>
                </Box>
              </KeyValueRow>
              <KeyValueRow label="Value">
                <Stack direction="row" spacing={1.5} alignItems="center">
                  <Typography sx={{ fontFamily: 'monospace', letterSpacing: 2 }}>••••••••••••</Typography>
                  <Typography variant="caption" color="text.secondary">
                    write-once — never returned after creation
                  </Typography>
                </Stack>
              </KeyValueRow>
            </Box>
          </Card>

          {/* Usages */}
          <Card>
            <Box sx={{ p: 3 }}>
              <Typography variant="h6" sx={{ mb: 1.5, fontWeight: 600 }}>
                Usages
              </Typography>
              {usagesLoading ? (
                <Stack spacing={1}>
                  <Skeleton variant="rounded" height={48} />
                  <Skeleton variant="rounded" height={48} />
                </Stack>
              ) : usagesError ? (
                <ErrorAlert error={usagesError} onRetry={fetchUsages} />
              ) : usages.length === 0 ? (
                <Stack
                  spacing={1}
                  alignItems="center"
                  justifyContent="center"
                  sx={{ py: 2, textAlign: 'center' }}
                >
                  <Box component="img" src={NoData} alt="No usages" sx={{ width: 80, maxWidth: '80%' }} />
                  <Typography variant="body2" color="text.secondary">
                    No resources currently reference this secret.
                  </Typography>
                </Stack>
              ) : (
                <Stack spacing={1}>
                  {usages.map((ref) => (
                    <Box
                      key={`${ref.type}-${ref.handle}`}
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: 2,
                        p: 1.5,
                        border: 1,
                        borderColor: 'divider',
                        borderRadius: 1,
                      }}
                    >
                      <Typography variant="body2" sx={{ fontWeight: 600, minWidth: 0 }}>
                        {ref.name || ref.handle}
                      </Typography>
                      <Chip label={formatReferenceType(ref.type)} size="small" variant="outlined" />
                    </Box>
                  ))}
                </Stack>
              )}
            </Box>
          </Card>
        </Stack>
      )}

      <DeleteSecretDialog
        secret={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onDeleted={() => navigate(listPath)}
      />
    </PageContent>
  );
}

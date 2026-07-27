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

import React, { useEffect, useState } from 'react';
import { Link as RouterLink, useNavigate, useParams } from 'react-router-dom';
import { Box, Button, Card, Chip, PageContent, Skeleton, Stack, Typography } from '@wso2/oxygen-ui';
import { ChevronLeft, RotateCw, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { getSecret, type SecretMetadata } from '../../../../apis/secretApis';
import { useAppShell } from '../../../../contexts/AppShellContext';
import { buildOrgPath } from '../../../../utils/projectRouting';
import ErrorAlert from '../../../../Components/common/ErrorAlert';
import DeleteSecretDialog from './DeleteSecretDialog';

function formatDateTime(value?: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return date.toLocaleString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

function KeyValueRow({ label, children }: { label: string; children: React.ReactNode }): React.JSX.Element {
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: '200px 1fr', gap: 2, py: 1.25 }}>
      <Typography variant="caption" sx={{ fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.04em' }} color="text.secondary">
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

  const [secret, setSecret] = useState<SecretMetadata | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SecretMetadata | null>(null);

  const listPath = buildOrgPath(currentOrganization, '/settings/secrets');

  const fetchSecret = async () => {
    if (!handle) return;
    try {
      setIsLoading(true);
      setError(null);
      const response = await getSecret(handle);
      setSecret(response);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to load secret.'));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void fetchSecret();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [handle]);

  return (
    <PageContent fullWidth>
      <Button component={RouterLink} to={listPath} size="small" startIcon={<ChevronLeft size={20} />}>
        Back to Secrets
      </Button>

      {isLoading ? (
        <Box sx={{ mt: 3, maxWidth: 720 }}>
          <Skeleton variant="text" width="40%" height={40} />
          <Skeleton variant="text" width="25%" />
          <Skeleton variant="rounded" height={220} sx={{ mt: 2 }} />
        </Box>
      ) : error || !secret ? (
        <Box sx={{ py: 2 }}>
          <ErrorAlert error={error ?? new Error('Secret not found.')} onRetry={fetchSecret} />
        </Box>
      ) : (
        <>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              justifyContent: 'space-between',
              gap: 2,
              flexWrap: 'wrap',
              mt: 2,
              mb: 3,
            }}
          >
            <Box sx={{ minWidth: 0 }}>
              <Typography variant="h5" sx={{ fontWeight: 700 }}>
                {secret.displayName}
              </Typography>
              <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mt: 0.5 }}>
                <Typography
                  variant="body2"
                  sx={{
                    fontFamily: 'monospace',
                    bgcolor: 'action.hover',
                    border: 1,
                    borderColor: 'divider',
                    borderRadius: 1,
                    px: 1,
                  }}
                >
                  {secret.id}
                </Typography>
                {secret.description && (
                  <Typography variant="body2" color="text.secondary">
                    {secret.description}
                  </Typography>
                )}
              </Stack>
            </Box>

            <Stack direction="row" spacing={1} sx={{ flexShrink: 0 }}>
              <Button
                variant="outlined"
                startIcon={<RotateCw size={16} />}
                onClick={() => navigate(`${listPath}/${secret.id}/rotate`)}
                data-cyid="rotate-secret-button"
              >
                Rotate secret
              </Button>
              <Button
                variant="outlined"
                color="error"
                startIcon={<Trash2 size={16} />}
                onClick={() => setDeleteTarget(secret)}
                data-cyid="delete-secret-button"
              >
                Delete
              </Button>
            </Stack>
          </Box>

          <Card sx={{ maxWidth: 720 }}>
            <Box sx={{ px: 3, py: 1 }}>
              <KeyValueRow label="Value">
                <Stack direction="row" spacing={1.5} alignItems="center">
                  <Typography sx={{ fontFamily: 'monospace', letterSpacing: 2 }}>••••••••••••</Typography>
                  <Typography variant="caption" color="text.secondary">
                    write-once — never returned after creation
                  </Typography>
                </Stack>
              </KeyValueRow>
              <KeyValueRow label="Type">
                <Chip label={secret.type} size="small" variant="outlined" />
              </KeyValueRow>
              <KeyValueRow label="Status">
                {secret.status === 'ACTIVE' ? (
                  <Chip label="Active" size="small" color="success" variant="outlined" />
                ) : (
                  <Chip label="Deprecated" size="small" color="warning" variant="outlined" />
                )}
              </KeyValueRow>
              <KeyValueRow label="Provider">
                <Typography variant="body2" color="text.secondary">
                  {secret.provider} (AES-GCM-256)
                </Typography>
              </KeyValueRow>
              <KeyValueRow label="Scope">
                <Typography variant="body2" color="text.secondary">
                  Organization — project and artifact-level scoping coming soon
                </Typography>
              </KeyValueRow>
              <KeyValueRow label="Created">
                <Typography variant="body2" color="text.secondary">
                  {formatDateTime(secret.createdAt)}
                </Typography>
              </KeyValueRow>
              <KeyValueRow label="Last updated">
                <Typography variant="body2" color="text.secondary">
                  {formatDateTime(secret.updatedAt)}
                </Typography>
              </KeyValueRow>
            </Box>
          </Card>
        </>
      )}

      <DeleteSecretDialog
        secret={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onDeleted={() => navigate(listPath)}
      />
    </PageContent>
  );
}

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


import { useEffect, useState } from 'react';
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItemButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Check } from '@wso2/oxygen-ui-icons-react';
import * as providerTemplateApis from '../../../../../apis/providerTemplateApis';
import { PLATFORM_API_BASE_URL } from '../../../../../config.env';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import { formatRelativeTime } from '../../../../../contexts/llmProvider';
import type { ProviderTemplate } from '../../../../../utils/types';

interface TemplateVersionDialogProps {
  open: boolean;
  templateId: string;
  templateName: string;
  onClose: () => void;
  onConfirm: (versionTemplate: ProviderTemplate) => void;
}

export default function TemplateVersionDialog({
  open,
  templateId,
  templateName,
  onClose,
  onConfirm,
}: TemplateVersionDialogProps) {
  const { currentOrganization } = useAppShell();
  const [versions, setVersions] = useState<ProviderTemplate[]>([]);
  const [selected, setSelected] = useState<string>('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const organizationId = currentOrganization?.uuid;
    if (!open || !templateId || !organizationId) return;

    let isMounted = true;
    setIsLoading(true);
    setError(null);
    setVersions([]);
    setSelected('');
    providerTemplateApis
      .getProviderTemplateVersions(templateId, organizationId, PLATFORM_API_BASE_URL)
      .then((list) => {
        if (!isMounted) return;
        // Only offer enabled versions (disabled ones are hidden everywhere).
        const enabled = list.filter((v) => v.enabled !== false);
        setVersions(enabled);
        // Default to the latest version, else the first returned.
        const latest = enabled.find((v) => v.isLatest) ?? enabled[0];
        setSelected(latest?.version ?? '');
      })
      .catch((err: unknown) => {
        if (isMounted) {
          setError(err instanceof Error ? err.message : 'Failed to load versions');
          setSelected('');
        }
      })
      .finally(() => {
        if (isMounted) setIsLoading(false);
      });

    return () => {
      isMounted = false;
    };
  }, [open, templateId, currentOrganization?.uuid]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="xs"
      fullWidth
      data-cyid="template-version-dialog"
    >
      <DialogTitle>Select a version of {templateName}</DialogTitle>
      <DialogContent dividers>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
          The provider will be based on the version you choose.
        </Typography>

        {isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }}>
            <CircularProgress size={24} />
          </Box>
        ) : error ? (
          <Typography variant="body2" color="error" sx={{ py: 2 }}>
            {error}
          </Typography>
        ) : versions.length === 0 ? (
          <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
            No versions found for this template.
          </Typography>
        ) : (
          <List disablePadding>
            {versions.map((v) => {
              const ver = v.version ?? '';
              return (
                <ListItemButton
                  key={ver}
                  selected={ver === selected}
                  onClick={() => setSelected(ver)}
                  data-cyid={`template-version-option-${ver}`}
                  sx={{
                    borderRadius: 1,
                    mb: 0.5,
                    border: 1,
                    borderColor: ver === selected ? 'primary.main' : 'divider',
                  }}
                >
                  <Stack sx={{ flexGrow: 1 }} spacing={0.25}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Typography variant="body2" sx={{ fontWeight: 600 }}>
                        {ver}
                      </Typography>
                      {v.isLatest ? (
                        <Chip label="latest" size="small" color="primary" variant="outlined" />
                      ) : null}
                    </Stack>
                    <Typography variant="caption" color="text.secondary">
                      {formatRelativeTime(v.createdAt ?? v.updatedAt)}
                    </Typography>
                  </Stack>
                  {ver === selected ? <Check size={16} /> : null}
                </ListItemButton>
              );
            })}
          </List>
        )}
      </DialogContent>
      <DialogActions>
        <Button
          onClick={onClose}
          color="secondary"
          variant="outlined"
          data-cyid="template-version-cancel-button"
        >
          Cancel
        </Button>
        <Button
          onClick={() => {
            const vt = versions.find((v) => v.version === selected) ?? versions[0];
            if (vt) onConfirm(vt);
          }}
          variant="contained"
          disabled={!selected || isLoading || Boolean(error) || versions.length === 0}
          data-cyid="template-version-continue-button"
        >
          Continue
        </Button>
      </DialogActions>
    </Dialog>
  );
}

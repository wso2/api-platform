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

import { useState, type FC } from 'react';
import {
  Avatar,
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  PageContent,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft, Edit } from '@wso2/oxygen-ui-icons-react';
import { GatewaySetupStepsPreV1_2, GatewaySetupStepsV1_2Plus, isGatewayV12OrAbove } from './GatewaySetupSteps';
import { gatewayStatusColor, gatewayStatusLabel, gatewayTypeLabel } from '../utils/gateway';
import type { GatewayVersionEntry } from '../hostPort';
import type { Gateway } from '../types';

export type GatewayOverviewProps = {
  gateway: Gateway;
  environmentNames: Map<string, string>;
  gatewayVersions: GatewayVersionEntry[];
  onBack: () => void;
  onEditClick: (gatewayId: string) => void;
  /** Mints (and revokes the previous) registration token — see `selfHostedGatewaysApi.generateToken`. */
  onGenerateRegistrationToken: (gatewayId: string) => Promise<string>;
  notify: (message: string, severity?: 'success' | 'info' | 'warning' | 'error') => void;
};

/** The release path segment and zip-name prefix for each gateway type's own distribution. */
const RELEASE_CHANNEL: Record<Gateway['type'], string> = {
  ai: 'ai-gateway',
  regular: 'api-gateway',
  event: 'event-gateway',
};

const INSTALL_METHODS = ['Quick Start', 'Virtual Machine', 'Docker', 'Kubernetes'] as const;
type InstallMethod = (typeof INSTALL_METHODS)[number];

const PREREQUISITES = ['cURL installed', 'unzip installed', 'Docker installed and running'];

const GatewayOverview: FC<GatewayOverviewProps> = ({
  gateway,
  environmentNames,
  gatewayVersions,
  onBack,
  onEditClick,
  onGenerateRegistrationToken,
  notify,
}) => {
  const [tab, setTab] = useState<'configurations' | 'policies'>('configurations');
  const [installMethod, setInstallMethod] = useState<InstallMethod>('Quick Start');
  const [registrationToken, setRegistrationToken] = useState<string | null>(null);
  const [hasJustRegeneratedToken, setHasJustRegeneratedToken] = useState(false);
  const [isRegeneratingToken, setIsRegeneratingToken] = useState(false);

  // The entries this gateway's own type offers, most recent first — matching
  // AI Workspace's `resolveGatewayVersion`: fall back to the gateway's own
  // reported version (prefixed with "v") when the catalog has nothing for it,
  // so an overview page still renders a usable download command rather than
  // nothing at all. Matched by the gateway's OWN declared version first — the
  // catalog can carry several entries per type (e.g. multiple regular-gateway
  // releases), so picking just any entry of the right type would offer
  // download commands for a release this gateway was never actually created
  // against. Only an untyped/legacy gateway (no `version` yet) falls back to
  // that type's first entry, which the catalog's own ordering treats as
  // "Latest" (see the "Add Gateway" form's version picker).
  const versionEntry: GatewayVersionEntry | undefined =
    gatewayVersions.find((entry) => entry.type === gateway.type && entry.version === gateway.version) ??
    gatewayVersions.find((entry) => entry.type === gateway.type);
  const resolvedVersion = versionEntry?.latestVersion ?? (gateway.version ? `v${gateway.version}` : 'v1.0.0');
  const distVersion = versionEntry?.distVersion ?? resolvedVersion.replace(/^v/, '');
  const releaseChannel = RELEASE_CHANNEL[gateway.type];
  const gatewayZipName = `wso2apip-${releaseChannel}-${distVersion}`;

  const handleRegenerateToken = async () => {
    setIsRegeneratingToken(true);
    try {
      const token = await onGenerateRegistrationToken(gateway.id);
      setRegistrationToken(token);
      setHasJustRegeneratedToken(true);
      notify(`Generated a new registration token for "${gateway.name}".`, 'success');
    } catch (generateError) {
      notify(
        generateError instanceof Error ? generateError.message : 'Unable to generate a registration token.',
        'error'
      );
    } finally {
      setIsRegeneratingToken(false);
    }
  };

  const setupStepsProps = {
    gatewayVersion: resolvedVersion,
    gatewayZipName,
    gatewayFolderName: gatewayZipName,
    releaseChannel,
    registrationToken,
    hasJustRegeneratedToken,
    isRegeneratingToken,
    onRegenerateToken: () => void handleRegenerateToken(),
  };

  return (
    <PageContent fullWidth>
      <Button size="small" startIcon={<ChevronLeft size={18} />} onClick={onBack}>
        Back to list
      </Button>

      <Box
        sx={{
          mt: 2,
          p: 2.5,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '10px',
          display: 'flex',
          alignItems: 'center',
          gap: 2,
        }}
      >
        <Avatar
          sx={{ width: 48, height: 48, backgroundColor: 'primary.light', color: 'primary.contrastText' }}
        >
          {gateway.name.trim().slice(0, 2).toUpperCase()}
        </Avatar>
        <Box sx={{ minWidth: 0 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography variant="h5" sx={{ fontWeight: 700 }}>
              {gateway.name}
            </Typography>
            {gateway.version ? (
              <Chip label={`${gatewayTypeLabel(gateway.type)} v${gateway.version}`} size="small" variant="outlined" />
            ) : null}
            <Chip
              size="small"
              variant="outlined"
              label={gatewayStatusLabel(gateway.status)}
              color={gatewayStatusColor(gateway.status)}
            />
            <IconButton size="small" onClick={() => onEditClick(gateway.id)} aria-label={`Edit ${gateway.name}`}>
              <Edit size={16} />
            </IconButton>
          </Box>
          <Typography variant="body2" color="text.secondary">
            {gateway.description || `This is a ${gatewayTypeLabel(gateway.type)}`}
          </Typography>
        </Box>
      </Box>

      <Box sx={{ mt: 2, borderBottom: '1px solid', borderColor: 'divider' }}>
        <Tabs value={tab} onChange={(_event, value) => setTab(value)}>
          <Tab label="Configurations" value="configurations" />
          <Tab label="Policies" value="policies" />
        </Tabs>
      </Box>

      {tab === 'policies' ? (
        <Box sx={{ py: 6, textAlign: 'center' }}>
          <Typography variant="body2" color="text.secondary">
            Gateway policies aren&apos;t available here yet.
          </Typography>
        </Box>
      ) : (
        <Box sx={{ mt: 3 }}>
          <Typography variant="h6" sx={{ fontWeight: 600, mb: 2 }}>
            Get Started
          </Typography>

          <Stack direction={{ md: 'row', xs: 'column' }} spacing={2} sx={{ mb: 3 }}>
            <TextField
              fullWidth
              label="URL"
              value={gateway.url || 'Awaiting connection'}
              slotProps={{ input: { readOnly: true } }}
            />
            <TextField
              fullWidth
              label="Associated Environment"
              value={environmentNames.get(gateway.environmentId) || gateway.environmentId || '—'}
              slotProps={{ input: { readOnly: true } }}
            />
          </Stack>

          <Box sx={{ borderBottom: '1px solid', borderColor: 'divider', mb: 3 }}>
            <Tabs value={installMethod} onChange={(_event, value) => setInstallMethod(value)}>
              {INSTALL_METHODS.map((method) => (
                <Tab key={method} label={method} value={method} />
              ))}
            </Tabs>
          </Box>

          {installMethod === 'Quick Start' ? (
            <Stack spacing={3} sx={{ maxWidth: 760 }}>
              <Box>
                <Typography variant="h6" sx={{ fontWeight: 600, mb: 1 }}>
                  Prerequisites
                </Typography>
                <Box component="ul" sx={{ m: 0, pl: 3 }}>
                  {PREREQUISITES.map((item) => (
                    <Typography key={item} component="li" variant="body2" color="text.secondary">
                      {item}
                    </Typography>
                  ))}
                </Box>
              </Box>

              {isGatewayV12OrAbove(distVersion) ? (
                <GatewaySetupStepsV1_2Plus {...setupStepsProps} />
              ) : (
                <GatewaySetupStepsPreV1_2 {...setupStepsProps} />
              )}
            </Stack>
          ) : (
            <Box sx={{ py: 6, textAlign: 'center' }}>
              <Typography variant="body2" color="text.secondary">
                {installMethod} setup instructions are coming soon — use Quick Start for now.
              </Typography>
            </Box>
          )}
        </Box>
      )}

      {isRegeneratingToken ? (
        <Box sx={{ position: 'fixed', bottom: 24, right: 24 }}>
          <CircularProgress size={24} />
        </Box>
      ) : null}
    </PageContent>
  );
};

export default GatewayOverview;

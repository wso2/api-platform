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

import { useState, type FC, type ReactNode } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Grid,
  IconButton,
  PageContent,
  Tab,
  Tabs,
  Typography,
} from '@wso2/oxygen-ui';
import { Check, ChevronLeft, Copy, Edit, Globe, Laptop, Menu } from '@wso2/oxygen-ui-icons-react';
import DockerIcon from '../assets/images/docker.svg';
import KubernetesIcon from '../assets/images/helm.svg';
import {
  CommandField,
  DISTRIBUTION_NAME,
  GatewaySetupStepsKubernetes,
  GatewaySetupStepsPreV1_2,
  GatewaySetupStepsV1_2Plus,
  isGatewayV12OrAbove,
  RELEASE_PATH,
} from './GatewaySetupSteps';
import { gatewayStatusColor, gatewayStatusLabel, gatewayTypeLabel } from '../utils/gateway';
import type { GatewayVersionEntry } from '../hostPort';
import type { Gateway } from '../types';

export type GatewayOverviewProps = {
  gateway: Gateway;
  environmentNames: Map<string, string>;
  /** `host:port` a self-hosted gateway's controller dials; shown in the setup commands. */
  controlPlaneHost: string;
  gatewayVersions: GatewayVersionEntry[];
  /** The token minted when the gateway was just registered, so its setup steps can use it straight away. */
  initialRegistrationToken?: string | null;
  onBack: () => void;
  onEditClick: (gatewayId: string) => void;
  /** Mints (and revokes the previous) registration token — see `selfHostedGatewaysApi.generateToken`. */
  onGenerateRegistrationToken: (gatewayId: string) => Promise<string>;
  notify: (message: string, severity?: 'success' | 'info' | 'warning' | 'error') => void;
};

const ACCENT = '#f7882f';

const INSTALL_METHODS = ['Quick Start', 'Virtual Machine', 'Docker', 'Kubernetes'] as const;
type InstallMethod = (typeof INSTALL_METHODS)[number];

const METHOD_ICON_SIZE = 20;

/** The logo shown beside each install method; Quick Start has none. */
const methodIcon = (method: InstallMethod) => {
  switch (method) {
    case 'Virtual Machine':
      return <Laptop size={METHOD_ICON_SIZE} />;
    case 'Docker':
      return <Box component="img" src={DockerIcon} alt="" sx={{ width: METHOD_ICON_SIZE, height: METHOD_ICON_SIZE }} />;
    case 'Kubernetes':
      return (
        <Box component="img" src={KubernetesIcon} alt="" sx={{ width: METHOD_ICON_SIZE, height: METHOD_ICON_SIZE }} />
      );
    default:
      return undefined;
  }
};

const DOWNLOAD_PREREQUISITES = ['cURL installed', 'unzip installed'];

/** What each install method needs on the machine, besides what its own steps install. */
const PREREQUISITES: Record<InstallMethod, string[]> = {
  'Quick Start': [...DOWNLOAD_PREREQUISITES, 'Docker installed and running'],
  'Virtual Machine': DOWNLOAD_PREREQUISITES,
  Docker: DOWNLOAD_PREREQUISITES,
  Kubernetes: [...DOWNLOAD_PREREQUISITES, 'Kubernetes 1.32+', 'Helm 3.18+'],
};

const CONTAINER_RUNTIMES = [
  'Docker Desktop (Windows / macOS)',
  'Rancher Desktop (Windows / macOS)',
  'Colima (macOS)',
  'Docker Engine + Compose plugin (Linux)',
];

const RUNTIME_CHECK_COMMAND = 'docker --version\ndocker compose version';

/** The checked requirements for an install method, with room for method-specific notes below them. */
const PrerequisitesCard: FC<{ items: string[]; children?: ReactNode }> = ({ items, children }) => (
  <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '16px', p: 2.5, mb: 4 }}>
    <Typography variant="overline" color="text.secondary" sx={{ letterSpacing: '0.12em', fontWeight: 600 }}>
      Prerequisites
    </Typography>
    <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.5, mt: 1.5 }}>
      {items.map((item) => (
        <Box
          key={item}
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            px: 2,
            py: 1,
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: '10px',
          }}
        >
          <Box sx={{ color: ACCENT, display: 'flex' }}>
            <Check size={16} />
          </Box>
          <Typography variant="body2">{item}</Typography>
        </Box>
      ))}
    </Box>
    {children}
  </Box>
);

const GatewayOverview: FC<GatewayOverviewProps> = ({
  gateway,
  environmentNames,
  controlPlaneHost,
  gatewayVersions,
  initialRegistrationToken,
  onBack,
  onEditClick,
  onGenerateRegistrationToken,
  notify,
}) => {
  const [tab, setTab] = useState<'configurations' | 'policies'>('configurations');
  const [installMethod, setInstallMethod] = useState<InstallMethod>('Quick Start');
  const [registrationToken, setRegistrationToken] = useState<string | null>(initialRegistrationToken ?? null);
  const [hasJustRegeneratedToken, setHasJustRegeneratedToken] = useState(false);
  const [isRegeneratingToken, setIsRegeneratingToken] = useState(false);

  // The catalog entry whose release this gateway was created against. A gateway that
  // reports a version is matched by it, and when the catalog does not list it the setup
  // commands use the reported version itself (below) — falling back to another release
  // would hand out a download for a version the gateway is not running. Only a legacy
  // gateway with no version yet takes its type's first entry, which the catalog's own
  // ordering treats as "Latest".
  const versionEntry: GatewayVersionEntry | undefined = gateway.version
    ? gatewayVersions.find((entry) => entry.type === gateway.type && entry.version === gateway.version)
    : gatewayVersions.find((entry) => entry.type === gateway.type);
  const resolvedVersion = versionEntry?.latestVersion ?? (gateway.version ? `v${gateway.version}` : 'v1.0.0');
  const distVersion = versionEntry?.distVersion ?? resolvedVersion.replace(/^v/, '');
  const releaseChannel = RELEASE_PATH[gateway.type];
  const gatewayZipName = `wso2apip-${DISTRIBUTION_NAME[gateway.type]}-${distVersion}`;

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
        'error',
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
    controlPlaneHost,
    registrationToken,
    hasJustRegeneratedToken,
    isRegeneratingToken,
    onRegenerateToken: () => void handleRegenerateToken(),
  };

  const isActive = gateway.status === 'active';
  const statusTitle = isActive ? 'Connected' : 'Waiting for connection';
  const statusHint = isActive
    ? 'This gateway is connected to the control plane.'
    : 'Status updates once the gateway is running.';
  const statusDot = isActive ? '#5cc98f' : ACCENT;

  const copyUrl = () => {
    if (gateway.url) void navigator.clipboard.writeText(gateway.url);
  };

  return (
    <PageContent fullWidth>
      <Button size="small" startIcon={<ChevronLeft size={18} />} onClick={onBack}>
        Back to list
      </Button>

      <Card sx={{ mt: 2 }}>
        <CardContent>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2.5 }}>
            <Box
              sx={{
                width: 64,
                height: 64,
                flexShrink: 0,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                borderRadius: '16px',
                border: `1.5px solid ${ACCENT}`,
                color: ACCENT,
                bgcolor: 'rgba(247,136,47,.12)',
              }}
            >
              <Menu size={30} />
            </Box>
            <Box sx={{ minWidth: 0, flex: 1 }}>
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  flexWrap: 'wrap',
                  gap: 1,
                }}
              >
                <Typography variant="h4" sx={{ fontWeight: 600 }}>
                  {gateway.name}
                </Typography>
                {gateway.version ? (
                  <Chip
                    label={`${gatewayTypeLabel(gateway.type)} v${gateway.version}`}
                    size="small"
                    variant="outlined"
                  />
                ) : null}
                <Chip
                  size="small"
                  variant="outlined"
                  color={gatewayStatusColor(gateway.status)}
                  label={gatewayStatusLabel(gateway.status)}
                  icon={
                    <Box
                      sx={{
                        width: 7,
                        height: 7,
                        borderRadius: '50%',
                        bgcolor: statusDot,
                        ml: '8px !important',
                      }}
                    />
                  }
                />
              </Box>
              <Typography variant="body1" color="text.secondary" sx={{ mt: 0.5 }}>
                {gateway.description || `This is a ${gatewayTypeLabel(gateway.type)}`}
              </Typography>
            </Box>
            <Button
              variant="outlined"
              startIcon={<Edit size={16} />}
              onClick={() => onEditClick(gateway.id)}
              sx={{
                borderRadius: 999,
                textTransform: 'none',
                flexShrink: 0,
                alignSelf: 'flex-start',
              }}
            >
              Edit
            </Button>
          </Box>
        </CardContent>
      </Card>

      <Card sx={{ mt: 3 }}>
        <Box sx={{ px: 1, borderBottom: '1px solid', borderColor: 'divider' }}>
          <Tabs value={tab} onChange={(_event, value) => setTab(value)}>
            <Tab label="Configurations" value="configurations" />
            <Tab label="Policies" value="policies" />
          </Tabs>
        </Box>
        <CardContent>
          {tab === 'policies' ? (
            <Box sx={{ py: 4, textAlign: 'center' }}>
              <Typography variant="body2" color="text.secondary">
                Gateway policies aren&apos;t available here yet.
              </Typography>
            </Box>
          ) : (
            <Grid container spacing={4} sx={{ width: '100%', m: 0 }}>
              <Grid size={{ xs: 12, md: 8 }}>
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    flexWrap: 'wrap',
                    gap: 2,
                    mb: 3,
                  }}
                >
                  <Typography variant="h5" sx={{ fontWeight: 600 }}>
                    Get Started
                  </Typography>
                  <Box
                    role="tablist"
                    sx={{
                      display: 'flex',
                      gap: 0.5,
                      p: 0.5,
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 999,
                    }}
                  >
                    {INSTALL_METHODS.map((method) => {
                      const selected = method === installMethod;
                      return (
                        <Button
                          key={method}
                          role="tab"
                          aria-selected={selected}
                          onClick={() => setInstallMethod(method)}
                          startIcon={methodIcon(method)}
                          sx={{
                            borderRadius: 999,
                            textTransform: 'none',
                            px: 2.5,
                            minWidth: 0,
                            color: selected ? '#fff' : 'text.primary',
                            background: selected ? 'linear-gradient(90deg, #f0903f, #e0553d)' : 'transparent',
                            '&:hover': selected
                              ? {
                                  background: 'linear-gradient(90deg, #f7a066, #ea6a4f)',
                                }
                              : undefined,
                          }}
                        >
                          {method}
                        </Button>
                      );
                    })}
                  </Box>
                </Box>

                <PrerequisitesCard items={PREREQUISITES[installMethod]}>
                  {installMethod === 'Virtual Machine' ? (
                    <Box sx={{ mt: 2.5 }}>
                      <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                        A Docker-compatible container runtime such as:
                      </Typography>
                      <Box component="ul" sx={{ m: 0, mb: 1.5, pl: 3 }}>
                        {CONTAINER_RUNTIMES.map((runtime) => (
                          <Typography key={runtime} component="li" variant="body2" color="text.secondary">
                            {runtime}
                          </Typography>
                        ))}
                      </Box>
                      <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                        Ensure docker and docker compose commands are available.
                      </Typography>
                      <CommandField value={RUNTIME_CHECK_COMMAND} copyLabel="Prerequisites command" />
                    </Box>
                  ) : null}
                </PrerequisitesCard>

                {installMethod === 'Kubernetes' ? (
                  <GatewaySetupStepsKubernetes
                    gatewayVersionHelm={resolvedVersion.replace(/^v/, '')}
                    releaseName={gateway.id}
                    controlPlaneHost={controlPlaneHost}
                    registrationToken={registrationToken}
                    hasJustRegeneratedToken={hasJustRegeneratedToken}
                    isRegeneratingToken={isRegeneratingToken}
                    onRegenerateToken={() => void handleRegenerateToken()}
                  />
                ) : isGatewayV12OrAbove(distVersion) ? (
                  <GatewaySetupStepsV1_2Plus {...setupStepsProps} />
                ) : (
                  <GatewaySetupStepsPreV1_2 {...setupStepsProps} />
                )}
              </Grid>

              <Grid size={{ xs: 12, md: 4 }}>
                <Box
                  sx={{
                    border: '1px solid',
                    borderColor: 'divider',
                    borderRadius: '16px',
                    overflow: 'hidden',
                  }}
                >
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 1.5,
                      p: 2.5,
                      borderBottom: '1px solid',
                      borderColor: 'divider',
                    }}
                  >
                    <Box
                      sx={{
                        width: 10,
                        height: 10,
                        flexShrink: 0,
                        borderRadius: '50%',
                        bgcolor: statusDot,
                      }}
                    />
                    <Box>
                      <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                        {statusTitle}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {statusHint}
                      </Typography>
                    </Box>
                  </Box>
                  <Box
                    sx={{
                      p: 2.5,
                      borderBottom: '1px solid',
                      borderColor: 'divider',
                    }}
                  >
                    <Typography variant="overline" color="text.secondary" sx={{ letterSpacing: '0.12em' }}>
                      URL
                    </Typography>
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        gap: 1,
                      }}
                    >
                      <Typography
                        sx={{
                          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
                          wordBreak: 'break-all',
                        }}
                      >
                        {gateway.url || 'Awaiting connection'}
                      </Typography>
                      {gateway.url ? (
                        <IconButton size="small" aria-label="Copy URL" onClick={copyUrl}>
                          <Copy size={16} />
                        </IconButton>
                      ) : null}
                    </Box>
                  </Box>
                  <Box sx={{ p: 2.5 }}>
                    <Typography variant="overline" color="text.secondary" sx={{ letterSpacing: '0.12em' }}>
                      Associated Environment
                    </Typography>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Box sx={{ color: ACCENT, display: 'flex' }}>
                        <Globe size={18} />
                      </Box>
                      <Typography variant="subtitle1">
                        {environmentNames.get(gateway.environmentId) || gateway.environmentId || '—'}
                      </Typography>
                    </Box>
                  </Box>
                </Box>
              </Grid>
            </Grid>
          )}
        </CardContent>
      </Card>

      {isRegeneratingToken ? (
        <Box sx={{ position: 'fixed', bottom: 24, right: 24 }}>
          <CircularProgress size={24} />
        </Box>
      ) : null}
    </PageContent>
  );
};

export default GatewayOverview;

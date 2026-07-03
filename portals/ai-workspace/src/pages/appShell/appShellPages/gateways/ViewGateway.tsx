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

import { useState, useEffect, useMemo, useRef } from "react";
import { useNavigate, useParams, Link as RouterLink } from "react-router-dom";
import {
  Box,
  Button,
  Typography,
  Card,
  CardContent,
  CircularProgress,
  Alert,
  Chip,
  TextField,
  IconButton,
  PageContent,
  Stack,
  Avatar,
  Tab,
  Tabs,
  Divider,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Drawer,
  FormControl,
  FormLabel,
  Grid,
  Tooltip,
} from "@wso2/oxygen-ui";
import {
  Copy,
  Download,
  Eye,
  EyeOff,
  ChevronLeft,
  Clock,
  Edit,
  Computer,
  X,
} from "@wso2/oxygen-ui-icons-react";
import DockerIcon from "../../../../assets/icons/docker.svg";
import HelmIcon from "../../../../assets/icons/helm.svg";
import { useAppShell } from "../../../../contexts/AppShellContext";
import { buildOrgPath } from "../../../../utils/projectRouting";
import { useAIWorkspaceSnackbar } from "../../../../hooks/aiWorkspaceSnackbar";
import {
  PLATFORM_GATEWAY_VERSIONS,
  PLATFORM_API_BASE_URL,
  CONTROLPLANE_HOST,
} from "../../../../config.env";
import {
  getGateways,
  getGatewayById,
  getGatewayConfigs,
  listGatewayTokens,
  revokeGatewayToken,
  rotateGatewayToken,
} from "../../../../apis/gateway/gatewayApi";
import type { GatewayConfigs } from "../../../../apis/gateway/gatewayApi";
import {
  getRegistrationToken,
  clearRegistrationToken,
  setRegistrationToken,
} from "./registrationTokenStore";
import { formatRelativeTime } from "../../../../contexts/llmProvider";
import {
  getActiveColorScheme,
  subscribeToColorSchemeChanges,
  getCommandTextFieldSx,
} from "../../../../utils/colorScheme";
import type { ColorScheme } from "../../../../utils/colorScheme";
import AIGatewayStepBanner from "../quickStart/AIGatewayStepBanner";
import ErrorAlert from "../../../../Components/common/ErrorAlert";

const resolveGatewayVersion = (gatewayVersion?: string): string => {
  const entry = gatewayVersion
    ? PLATFORM_GATEWAY_VERSIONS.find((v) => v.version === gatewayVersion)
    : PLATFORM_GATEWAY_VERSIONS[0];
  if (!entry) return gatewayVersion ? `v${gatewayVersion}` : 'v1.0.0';
  return entry.latestVersion ?? `v${entry.version}`;
};

const getPlatformApiBaseUrl = (): string => {
  return PLATFORM_API_BASE_URL;
};

const getDisplayUrl = (vhost: string): string => {
  if (!vhost || !vhost.trim()) return "";
  const v = vhost.trim();
  if (v.startsWith("http://") || v.startsWith("https://")) return v;
  return `https://${v}`;
};

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length === 0) return "";
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

type TabPanelProps = {
  value: number;
  index: number;
  children: React.ReactNode;
};
type GatewayConfigEntry = Record<string, unknown>;
type GatewayConfigEnvelope = {
  list?: GatewayConfigEntry[];
  configs?: GatewayConfigEntry[];
  data?: GatewayConfigEntry[];
  items?: GatewayConfigEntry[];
  keyManagerList?: GatewayConfigEntry[];
};

const GATEWAY_CONFIG_COLLECTION_KEYS: Array<keyof GatewayConfigEnvelope> = [
  "list",
  "configs",
  "data",
  "items",
];

function TabPanel({ value, index, children }: TabPanelProps) {
  return (
    <Box role="tabpanel" hidden={value !== index} sx={{ pt: 2 }}>
      {value === index ? children : null}
    </Box>
  );
}

function normalizeGatewayConfigList(
  configs: GatewayConfigs | null,
): GatewayConfigEntry[] {
  if (!configs) {
    return [];
  }

  if (Array.isArray(configs)) {
    return configs as GatewayConfigEntry[];
  }

  const wrappedConfigs = configs as GatewayConfigEnvelope;
  const nestedConfigList = GATEWAY_CONFIG_COLLECTION_KEYS.map(
    (key) => wrappedConfigs[key],
  ).find(Array.isArray);

  if (nestedConfigList) {
    return nestedConfigList;
  }

  return [configs as GatewayConfigEntry];
}

function withKeyManagerEntries(
  configs: GatewayConfigEntry[],
): GatewayConfigEntry[] {
  return [
    ...configs,
    ...configs.flatMap((configEntry) => {
      const keyManagerList = (configEntry as GatewayConfigEnvelope)
        .keyManagerList;
      return Array.isArray(keyManagerList) ? keyManagerList : [];
    }),
  ];
}

function getConfigStringValue(
  configEntry: GatewayConfigEntry,
  keys: string[],
): string {
  for (const key of keys) {
    const value = configEntry[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return "";
}

const tabs = ["Quick Start", "Virtual Machine", "Docker", "Kubernetes"];

export default function ViewGateway() {
  const navigate = useNavigate();
  const { gatewayName } = useParams<{ gatewayName: string }>();
  const { currentOrganization } = useAppShell();
  const showSnackbar = useAIWorkspaceSnackbar();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [gateway, setGateway] = useState<any>(null);
  const [showToken, setShowToken] = useState(false);
  const [showDrawerRegistrationToken, setShowDrawerRegistrationToken] =
    useState(false);
  const [tabIndex, setTabIndex] = useState(0);
  const [isRegenerateDialogOpen, setIsRegenerateDialogOpen] = useState(false);
  const [isRegeneratingToken, setIsRegeneratingToken] = useState(false);
  const [hasJustRegeneratedToken, setHasJustRegeneratedToken] = useState(false);
  const [activeColorScheme, setActiveColorScheme] = useState<ColorScheme>(() =>
    getActiveColorScheme(),
  );
  const [showSetupBanner, setShowSetupBanner] = useState(true);
  const [isConfigsDrawerOpen, setIsConfigsDrawerOpen] = useState(false);
  const [isConfigsLoading, setIsConfigsLoading] = useState(false);
  const [gatewayConfigs, setGatewayConfigs] = useState<GatewayConfigs | null>(
    null,
  );
  const [gatewayConfigsError, setGatewayConfigsError] = useState<string | null>(
    null,
  );

  // Get one-time registration token from memory (only available after gateway creation)
  const [registrationToken, setRegistrationTokenState] = useState<
    string | null
  >(() => getRegistrationToken());

  // Version vars derived from the gateway's stored version
  const gatewayVersion = resolveGatewayVersion(gateway?.version);
  const gatewayVersionHelm = gatewayVersion.startsWith("v")
    ? gatewayVersion.slice(1)
    : gatewayVersion;
  const gatewayZipName = `wso2apip-ai-gateway-${gatewayVersionHelm}`;
  const gatewayFolderName = `wso2apip-ai-gateway-${gatewayVersionHelm}`;
  const gatewayEnvFile = `${gatewayFolderName}/configs/keys.env`;

  const getSetupGatewayDisplayCommand = () =>
    `curl -sLO https://github.com/wso2/api-platform/releases/download/ai-gateway/${gatewayVersion}/${gatewayZipName}.zip && \\
unzip ${gatewayZipName}.zip`;

  const getSetupGatewayCopyCommand = () => getSetupGatewayDisplayCommand();

  const getConfigureGatewayDisplayCommand = (moesifKey: string | null) => {
    const controlPlaneHost = CONTROLPLANE_HOST;
    const moesifLine = moesifKey ? `MOESIF_KEY=<your-moesif-key>\n` : "";
    return `cat > ${gatewayEnvFile} << 'ENVFILE'
${moesifLine}GATEWAY_CONTROLPLANE_HOST=${controlPlaneHost}
GATEWAY_REGISTRATION_TOKEN=<your-gateway-token>
ENVFILE`;
  };

  const getConfigureGatewayCopyCommand = (
    token: string | null,
    moesifKey: string | null,
  ) => {
    const controlPlaneHost = CONTROLPLANE_HOST;
    const tokenValue = token || "<your-gateway-token>";
    const moesifLine = moesifKey ? `MOESIF_KEY=${moesifKey}\n` : "";
    return `cat > ${gatewayEnvFile} << 'ENVFILE'
${moesifLine}GATEWAY_CONTROLPLANE_HOST=${controlPlaneHost}
GATEWAY_REGISTRATION_TOKEN=${tokenValue}
ENVFILE`;
  };

  const getStep3NavigateCommand = () => `cd ${gatewayFolderName}`;

  const getStartGatewayDisplayCommand = () =>
    `docker compose --env-file configs/keys.env up`;

  const getStartGatewayCopyCommand = () => getStartGatewayDisplayCommand();

  const getK8sCustomHelmDisplayCommand = (moesifKey: string | null) => {
    const controlPlaneHost = CONTROLPLANE_HOST;
    const lines = [
      `helm install gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway --version ${gatewayVersionHelm} \\`,
      `  --set gateway.controller.controlPlane.host="${controlPlaneHost}" \\`,
      "  --set gateway.controller.controlPlane.port=443 \\",
      '  --set gateway.controller.controlPlane.token.value="your-gateway-token"',
    ];
    if (moesifKey) {
      lines[lines.length - 1] += " \\";
      lines.push(
        "  --set gateway.config.analytics.publishers.moesif.application_id=<your-moesif-key>",
      );
      lines[lines.length - 1] += " \\";
    } else {
      lines[lines.length - 1] += " \\";
    }
    lines.push("  --set gateway.config.analytics.enabled=true");
    return lines.join("\n");
  };

  const getK8sCustomHelmCopyCommand = (
    token: string | null,
    moesifKey: string | null,
  ) => {
    const tokenValue = token || "your-gateway-token";
    const controlPlaneHost = CONTROLPLANE_HOST;
    const lines = [
      `helm install gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway --version ${gatewayVersionHelm} \\`,
      `  --set gateway.controller.controlPlane.host="${controlPlaneHost}" \\`,
      "  --set gateway.controller.controlPlane.port=443 \\",
      `  --set gateway.controller.controlPlane.token.value="${tokenValue}"`,
    ];
    if (moesifKey) {
      lines[lines.length - 1] += " \\";
      lines.push(
        `  --set gateway.config.analytics.publishers.moesif.application_id="${moesifKey}"`,
      );
      lines[lines.length - 1] += " \\";
    } else {
      lines[lines.length - 1] += " \\";
    }
    lines.push("  --set gateway.config.analytics.enabled=true");
    return lines.join("\n");
  };

  // Clear the token when leaving the page
  useEffect(() => {
    return () => {
      clearRegistrationToken();
    };
  }, []);

  useEffect(() => {
    const unsubscribe = subscribeToColorSchemeChanges((nextScheme) => {
      setActiveColorScheme((prev) => (prev === nextScheme ? prev : nextScheme));
    });

    return unsubscribe;
  }, []);

  // Load gateway data
  useEffect(() => {
    const loadGateway = async () => {
      if (!gatewayName || !currentOrganization?.uuid) return;

      try {
        const response = await getGateways(currentOrganization.uuid);
        const foundGateway = response.data?.list?.find(
          (gw) => gw.id === gatewayName,
        );

        if (foundGateway) {
          setGateway(foundGateway);
        } else {
          setError("Gateway not found");
        }
      } catch (err: any) {
        console.error("Failed to load gateway:", err);
        setError(err?.message || "Failed to load gateway");
      } finally {
        setLoading(false);
      }
    };

    loadGateway();
  }, [gatewayName, currentOrganization?.uuid]);

  // Poll gateway status: 5s when inactive, 15s when active
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const pollingIntervalRef = useRef<number | null>(null);
  const hasAutoGeneratedTokenRef = useRef(false);

  useEffect(() => {
    if (!gateway?.id || !currentOrganization?.uuid) return;

    const desiredInterval = gateway.isActive ? 15000 : 5000;

    if (pollingRef.current && pollingIntervalRef.current === desiredInterval) {
      return;
    }

    if (pollingRef.current) {
      clearInterval(pollingRef.current);
    }

    const gatewayId = gateway.id;
    pollingIntervalRef.current = desiredInterval;
    pollingRef.current = setInterval(async () => {
      try {
        const response = await getGatewayById(
          gatewayId,
          currentOrganization.uuid,
        );
        if (response.data) {
          setGateway(response.data);
        }
      } catch (err) {
        console.error("Polling failed:", err);
      }
    }, desiredInterval);

    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
        pollingRef.current = null;
        pollingIntervalRef.current = null;
      }
    };
  }, [gateway?.isActive, gateway?.id, currentOrganization?.uuid]);

  // Show snackbar when gateway transitions to active
  const prevIsActiveRef = useRef<boolean | undefined>(undefined);

  useEffect(() => {
    if (prevIsActiveRef.current === false && gateway?.isActive) {
      showSnackbar("Your gateway is connected successfully.", "success");
    }
    prevIsActiveRef.current = gateway?.isActive;
  }, [gateway?.isActive, showSnackbar]);

  // Auto-generate a token when a non-active gateway has no token in memory
  useEffect(() => {
    if (
      !hasAutoGeneratedTokenRef.current &&
      !registrationToken &&
      gateway?.id &&
      !gateway.isActive &&
      currentOrganization?.uuid
    ) {
      hasAutoGeneratedTokenRef.current = true;
      regenerateGatewayToken()
        .then((newToken) => {
          setRegistrationTokenState(newToken);
          setRegistrationToken(newToken);
        })
        .catch(() => {
          hasAutoGeneratedTokenRef.current = false;
        });
    }
    // regenerateGatewayToken is stable within the same gateway/org context
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gateway?.id, gateway?.isActive, currentOrganization?.uuid]);

  const handleCopy = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    showSnackbar(`${label} copied to clipboard`, "success");
  };

  const handleDownloadKeysEnvFile = () => {
    const envContent = `GATEWAY_CONTROLPLANE_HOST=${CONTROLPLANE_HOST}
GATEWAY_REGISTRATION_TOKEN=${registrationToken || ""}`;
    const blob = new Blob([`${envContent}\n`], {
      type: "text/plain;charset=utf-8",
    });
    const objectUrl = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = objectUrl;
    link.download = "keys.env";
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(objectUrl);
    showSnackbar("keys.env file downloaded", "success");
  };

  const handleOpenGatewayConfigsDrawer = async () => {
    if (!gateway?.id || !currentOrganization?.uuid) {
      return;
    }

    setIsConfigsDrawerOpen(true);
    setIsConfigsLoading(true);
    setGatewayConfigsError(null);

    try {
      const response = await getGatewayConfigs(
        gateway.id,
        currentOrganization.uuid,
      );
      setGatewayConfigs(response.data || {});
    } catch (err: any) {
      console.error("Failed to load gateway configs:", err);
      setGatewayConfigs(null);
      setGatewayConfigsError(
        err?.message || "Failed to load gateway configurations",
      );
    } finally {
      setIsConfigsLoading(false);
    }
  };

  const handleBack = () => {
    const listPath = buildOrgPath(currentOrganization, "/gateways");
    navigate(listPath);
  };

  const regenerateGatewayToken = async (): Promise<string> => {
    if (!gateway?.id || !currentOrganization?.uuid) {
      throw new Error("Gateway ID or Organization ID is not available");
    }

    const gatewayId = gateway.id;
    const organizationId = currentOrganization.uuid;

    // 1) List existing tokens
    const existingTokens = await listGatewayTokens(gatewayId, organizationId);

    // 2) Revoke all existing tokens
    await Promise.all(
      existingTokens.map((token) =>
        revokeGatewayToken(gatewayId, token.id, organizationId).catch(() => {
          // Swallow individual revoke errors
          return;
        }),
      ),
    );

    // 3) Rotate to get a new token
    const newToken = await rotateGatewayToken(gatewayId, organizationId);
    return newToken;
  };

  const handleRegenerateToken = () => {
    setIsRegenerateDialogOpen(true);
  };

  const handleConfirmRegenerateToken = async () => {
    try {
      setIsRegeneratingToken(true);
      const newToken = await regenerateGatewayToken();
      setRegistrationTokenState(newToken);
      setRegistrationToken(newToken);
      setHasJustRegeneratedToken(true);
      showSnackbar("Successfully generated new registration token", "success");
    } catch (err: any) {
      console.error("Failed to regenerate token:", err);
      showSnackbar(err?.message || "Failed to regenerate token", "error");
    } finally {
      setIsRegeneratingToken(false);
      setIsRegenerateDialogOpen(false);
    }
  };

  if (loading) {
    return (
      <PageContent fullWidth>
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
            minHeight: 300,
          }}
        >
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  if (error) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2}>
          <Typography variant="h6" color="error">
            Error loading gateway
          </Typography>
          <Typography>{error}</Typography>
          <Button
            component={RouterLink}
            to={buildOrgPath(currentOrganization, "/gateways")}
          >
            Back to list
          </Button>
        </Stack>
      </PageContent>
    );
  }

  if (!gateway) {
    return (
      <PageContent fullWidth>
        <Stack spacing={2}>
          <Typography variant="h6">Gateway not found</Typography>
          <Button
            component={RouterLink}
            to={buildOrgPath(currentOrganization, "/gateways")}
          >
            Back to list
          </Button>
        </Stack>
      </PageContent>
    );
  }

  const gatewayType =
    gateway?.functionalityType === "ai"
      ? "AI Gateway"
      : gateway?.functionalityType === "event"
        ? "Event Gateway"
        : "API Gateway";

  const descriptionText = gateway?.description?.trim() || "No description";
  const truncatedDescription =
    descriptionText.length > 200
      ? `${descriptionText.slice(0, 200).trim()}…`
      : descriptionText;

  const lastUpdated = gateway?.createdAt ?? gateway?.lastUpdated;
  const gatewayConfigItemsWithKeyManagers = withKeyManagerEntries(
    normalizeGatewayConfigList(gatewayConfigs),
  );
  const keyManagerConfigs = gatewayConfigItemsWithKeyManagers
    .map((configEntry, index) => ({
      name:
        getConfigStringValue(configEntry, ["name"]) ||
        `Key Manager ${index + 1}`,
      issuerUrl: getConfigStringValue(configEntry, [
        "issuerUrl",
        "issuer_url",
        "issuerURL",
      ]),
      jwksUrl: getConfigStringValue(configEntry, [
        "jwksUrl",
        "jwks_url",
        "jwksURL",
      ]),
    }))
    .filter((configEntry) => configEntry.issuerUrl || configEntry.jwksUrl);
  return (
    <PageContent fullWidth>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 2,
        }}
      >
        <Button
          component={RouterLink}
          to={buildOrgPath(currentOrganization, "/gateways")}
          size="small"
          startIcon={<ChevronLeft size={24} />}
        >
          Back to list
        </Button>
      </Box>
      {showSetupBanner && (
        <Box mt={1} mb={2}>
          <AIGatewayStepBanner
            gatewayDisplayName={gateway.displayName || gateway.name}
            isActive={Boolean(gateway.isActive)}
            onFinish={() => setShowSetupBanner(false)}
            onDismiss={() => setShowSetupBanner(false)}
          />
        </Box>
      )}

      {/* Gateway Header Card */}
      <Card sx={{ mb: 2 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "space-between",
            flexWrap: "wrap",
            gap: 2,
            p: 2,
          }}
        >
          <Box sx={{ display: "flex", alignItems: "flex-start", gap: 2 }}>
            <Avatar
              sx={{
                width: 72,
                height: 72,
                fontWeight: 600,
                fontSize: 28,
                bgcolor: "primary.light",
                color: "primary.contrastText",
              }}
            >
              {getInitials(gateway?.displayName || gateway?.name || "GW")}
            </Avatar>
            <Stack spacing={0.75} sx={{ minWidth: 0 }}>
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                flexWrap="wrap"
              >
                <Typography variant="h3">
                  {gateway?.displayName || gateway?.name}
                </Typography>
                {gateway?.version && (
                  <Chip
                    label={`AI-Gateway v${gateway.version}`}
                    size="small"
                    variant="outlined"
                  />
                )}
                <Chip
                  label={gateway?.isActive ? "Active" : "Inactive"}
                  size="small"
                  color={gateway?.isActive ? "success" : "default"}
                />
                <Tooltip title="Edit Gateway">
                  <IconButton
                    component={RouterLink}
                    to={buildOrgPath(
                      currentOrganization,
                      `/gateways/edit/${gatewayName}`,
                    )}
                    size="small"
                  >
                    <Edit size={16} />
                  </IconButton>
                </Tooltip>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                {truncatedDescription}
              </Typography>
              {/* <Typography variant="body2" color="text.secondary">
                vhost: {gateway?.vhost || '—'}
              </Typography> */}
              <Stack direction="row" spacing={0.75} alignItems="center">
                <Typography variant="caption" color="text.secondary">
                  Created
                </Typography>
                <Clock size={14} />
                <Typography variant="caption" color="text.secondary">
                  {lastUpdated ? formatRelativeTime(lastUpdated) : "—"}
                </Typography>
              </Stack>
            </Stack>
          </Box>
        </Box>
      </Card>

      {/* Get Started Card */}
      <Card>
        <CardContent sx={{ p: 3 }}>
          <Box
            sx={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              gap: 2,
              mb: 3,
            }}
          >
            <Typography variant="h5">Get Started</Typography>
            <Button
              variant="outlined"
              size="small"
              startIcon={<Eye size={16} />}
              onClick={() => void handleOpenGatewayConfigsDrawer()}
            >
              View Gateway configurations
            </Button>
          </Box>
          {gateway?.isActive && (
            <Alert severity="success" sx={{ mb: 1 }}>
              Your gateway is connected successfully.
            </Alert>
          )}

          <Grid container spacing={2} sx={{ mt: 2, mb: 1 }}>
            <Grid size={{ xs: 12, md: 4, lg: 4 }}>
              <FormControl fullWidth>
                <FormLabel>URL</FormLabel>
                <TextField
                  value={getDisplayUrl(gateway.endpoints?.[0] || gateway.vhost || "")}
                  fullWidth
                  slotProps={{ input: { readOnly: true } }}
                />
              </FormControl>
            </Grid>
          </Grid>

          {/* Registration Token Alert */}
          {/* {registrationToken && (
            <Alert severity="info" sx={{ mb: 3 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 3 }}>
                <Box sx={{ flex: '0 0 auto' }}>
                  <Typography variant="body2" sx={{ fontWeight: 600, mb: 0.5 }}>
                    Gateway Registration Token
                  </Typography>
                  <Typography variant="body2">
                    This token is shown only once. Ensure it is securely saved
                    before leaving this page.
                  </Typography>
                </Box>
                <TextField
                  sx={{ flex: 1, minWidth: 500 }}
                  type={showToken ? 'text' : 'password'}
                  value={registrationToken}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <>
                          <IconButton
                            size="small"
                            onClick={() => setShowToken(!showToken)}
                          >
                            {showToken ? <EyeOff /> : <Eye />}
                          </IconButton>
                          <IconButton
                            size="small"
                            onClick={() =>
                              handleCopy(
                                registrationToken,
                                'Registration token'
                              )
                            }
                          >
                            <Copy />
                          </IconButton>
                        </>
                      ),
                    },
                  }}
                />
              </Box>
            </Alert>
          )} */}

          <Box sx={{ borderBottom: 1, borderColor: "divider", mb: 2 }}>
            <Tabs
              value={tabIndex}
              onChange={(_, newValue) => setTabIndex(newValue)}
            >
              <Tab label="Quick Start" />
              <Tab
                label="Virtual Machine"
                icon={<Computer />}
                iconPosition="start"
              />
              <Tab
                label="Docker"
                icon={
                  <Box
                    component="img"
                    src={DockerIcon}
                    sx={{ width: 24, height: 24 }}
                  />
                }
                iconPosition="start"
              />
              <Tab
                label="Kubernetes"
                icon={
                  <Box
                    component="img"
                    src={HelmIcon}
                    sx={{ width: 24, height: 24 }}
                  />
                }
                iconPosition="start"
              />
            </Tabs>
          </Box>

          {/* Quick Start Tab */}
          <TabPanel value={tabIndex} index={0}>
            <Stack spacing={3}>
              {/* Prerequisites - Quick Start */}
              <Box>
                <Typography variant="h6" sx={{ mb: 2 }}>
                  Prerequisites
                </Typography>
                <Stack component="ul" spacing={0.5} sx={{ pl: 3 }}>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    cURL installed
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    unzip installed
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    Docker installed and running
                  </Typography>
                </Stack>
              </Box>

              {/* Step 1: Download the Gateway */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
                  Step 1: Download the Gateway
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 2 }}
                >
                  Run this command in your terminal to download the gateway:
                </Typography>
                <TextField
                  fullWidth
                  multiline
                  minRows={2}
                  value={getSetupGatewayDisplayCommand()}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getSetupGatewayCopyCommand(),
                              "Download command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                  sx={getCommandTextFieldSx(activeColorScheme)}
                />
              </Box>

              {/* Step 2: Configure the Gateway */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
                  Step 2: Configure the Gateway
                </Typography>
                {registrationToken ? (
                  <>
                    {hasJustRegeneratedToken && (
                      <Alert severity="success" sx={{ mb: 2 }}>
                        Successfully generated new configurations. Use the
                        updated command below to reconfigure your gateway.
                      </Alert>
                    )}
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mb: 2 }}
                    >
                      Run this command to create {gatewayEnvFile} with the
                      required environment variables:
                    </Typography>
                    <TextField
                      fullWidth
                      multiline
                      minRows={4}
                      sx={getCommandTextFieldSx(activeColorScheme)}
                      value={getConfigureGatewayDisplayCommand(null)}
                      onCopy={(e) => {
                        e.preventDefault();
                        e.clipboardData.setData(
                          "text/plain",
                          getConfigureGatewayCopyCommand(
                            registrationToken,
                            null,
                          ),
                        );
                      }}
                      slotProps={{
                        input: {
                          readOnly: true,
                          endAdornment: (
                            <IconButton
                              size="small"
                              onClick={() =>
                                handleCopy(
                                  getConfigureGatewayCopyCommand(
                                    registrationToken,
                                    null,
                                  ),
                                  "Configure command",
                                )
                              }
                            >
                              <Copy />
                            </IconButton>
                          ),
                        },
                      }}
                    />
                    <Alert severity="info" sx={{ mt: 2 }}>
                      To gain gateway analytics, you can integrate with Moesif
                      by adding your Moesif application token with the key{" "}
                      <code>MOESIF_KEY</code> to your{" "}
                      <code>configs/keys.env</code>.
                    </Alert>
                  </>
                ) : (
                  <>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mb: 2 }}
                    >
                      The registration token is single-use. If you need to
                      reconfigure the gateway, generate a new token—this will
                      revoke the old token and disconnect the gateway from the
                      control plane.
                    </Typography>
                    <Button
                      variant="outlined"
                      color="primary"
                      onClick={handleRegenerateToken}
                      disabled={isRegeneratingToken}
                    >
                      Reconfigure
                    </Button>
                  </>
                )}
              </Box>

              {/* Step 3: Start the Gateway */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
                  Step 3: Start the Gateway
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 1.5 }}
                >
                  1. Navigate to the gateway folder.
                </Typography>
                <TextField
                  fullWidth
                  value={getStep3NavigateCommand()}
                  sx={getCommandTextFieldSx(activeColorScheme)}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getStep3NavigateCommand(),
                              "Navigate command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                />
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ my: 2 }}
                >
                  2. Run this command to start the gateway using the
                  configs/keys.env file created in Step 2:
                </Typography>
                <TextField
                  fullWidth
                  value={getStartGatewayDisplayCommand()}
                  sx={getCommandTextFieldSx(activeColorScheme)}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getStartGatewayCopyCommand(),
                              "Start command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                />
              </Box>
            </Stack>
          </TabPanel>

          {/* Virtual Machine Tab */}
          <TabPanel value={tabIndex} index={1}>
            <Stack spacing={3}>
              {/* Prerequisites - VM */}
              <Box>
                <Typography variant="h6" sx={{ mb: 2 }}>
                  Prerequisites
                </Typography>
                <Stack component="ul" spacing={0.5} sx={{ pl: 3 }}>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    cURL installed
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    unzip installed
                  </Typography>
                </Stack>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mt: 1.5, mb: 1 }}
                >
                  A Docker-compatible container runtime such as:
                </Typography>
                <Stack component="ul" spacing={0.5} sx={{ pl: 3 }}>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    Docker Desktop (Windows / macOS)
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    Rancher Desktop (Windows / macOS)
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    Colima (macOS)
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    Docker Engine + Compose plugin (Linux)
                  </Typography>
                </Stack>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mt: 1.5, mb: 1.5 }}
                >
                  Ensure docker and docker compose commands are available.
                </Typography>
                <TextField
                  fullWidth
                  multiline
                  minRows={2}
                  sx={getCommandTextFieldSx(activeColorScheme)}
                  value={`docker --version\ndocker compose version`}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              "docker --version\ndocker compose version",
                              "Prerequisites command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                />
              </Box>

              {/* Step 1: Download the Gateway */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
                  Step 1: Download the Gateway
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 2 }}
                >
                  Run this command in your terminal to download the gateway:
                </Typography>
                <TextField
                  fullWidth
                  multiline
                  minRows={2}
                  value={getSetupGatewayDisplayCommand()}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getSetupGatewayCopyCommand(),
                              "Download command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                  sx={getCommandTextFieldSx(activeColorScheme)}
                />
              </Box>

              {/* Step 2: Configure the Gateway */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
                  Step 2: Configure the Gateway
                </Typography>
                {registrationToken ? (
                  <>
                    {hasJustRegeneratedToken && (
                      <Alert severity="success" sx={{ mb: 2 }}>
                        Successfully generated new configurations. Use the
                        updated command below to reconfigure your gateway.
                      </Alert>
                    )}
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mb: 2 }}
                    >
                      Run this command to create {gatewayEnvFile} with the
                      required environment variables:
                    </Typography>
                    <TextField
                      fullWidth
                      multiline
                      minRows={4}
                      sx={getCommandTextFieldSx(activeColorScheme)}
                      value={getConfigureGatewayDisplayCommand(null)}
                      onCopy={(e) => {
                        e.preventDefault();
                        e.clipboardData.setData(
                          "text/plain",
                          getConfigureGatewayCopyCommand(
                            registrationToken,
                            null,
                          ),
                        );
                      }}
                      slotProps={{
                        input: {
                          readOnly: true,
                          endAdornment: (
                            <IconButton
                              size="small"
                              onClick={() =>
                                handleCopy(
                                  getConfigureGatewayCopyCommand(
                                    registrationToken,
                                    null,
                                  ),
                                  "Configure command",
                                )
                              }
                            >
                              <Copy />
                            </IconButton>
                          ),
                        },
                      }}
                    />
                    <Alert severity="info" sx={{ mt: 2 }}>
                      To gain gateway analytics, you can integrate with Moesif
                      by adding your Moesif application token with the key{" "}
                      <code>MOESIF_KEY</code> to your{" "}
                      <code>configs/keys.env</code>.
                    </Alert>
                  </>
                ) : (
                  <>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mb: 2 }}
                    >
                      The registration token is single-use. If you need to
                      reconfigure the gateway, generate a new token—this will
                      revoke the old token and disconnect the gateway from the
                      control plane.
                    </Typography>
                    <Button
                      variant="outlined"
                      color="primary"
                      onClick={handleRegenerateToken}
                      disabled={isRegeneratingToken}
                    >
                      Reconfigure
                    </Button>
                  </>
                )}
              </Box>

              {/* Step 3: Start the Gateway */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
                  Step 3: Start the Gateway
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 1.5 }}
                >
                  1. Navigate to the gateway folder.
                </Typography>
                <TextField
                  fullWidth
                  sx={getCommandTextFieldSx(activeColorScheme)}
                  value={getStep3NavigateCommand()}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getStep3NavigateCommand(),
                              "Navigate command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                />
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ my: 2 }}
                >
                  2. Run this command to start the gateway using the
                  configs/keys.env file created in Step 2:
                </Typography>
                <TextField
                  fullWidth
                  value={getStartGatewayDisplayCommand()}
                  sx={getCommandTextFieldSx(activeColorScheme)}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getStartGatewayCopyCommand(),
                              "Start command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                />
              </Box>
            </Stack>
          </TabPanel>

          {/* Docker Tab */}
          <TabPanel value={tabIndex} index={2}>
            <Stack spacing={3}>
              {/* Prerequisites - Docker */}
              <Box>
                <Typography variant="h6" sx={{ mb: 2 }}>
                  Prerequisites
                </Typography>
                <Stack component="ul" spacing={0.5} sx={{ pl: 3 }}>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    cURL installed
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    unzip installed
                  </Typography>
                </Stack>
              </Box>

              {/* Step 1: Download the Gateway */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
                  Step 1: Download the Gateway
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 2 }}
                >
                  Run this command in your terminal to download the gateway:
                </Typography>
                <TextField
                  fullWidth
                  multiline
                  minRows={2}
                  value={getSetupGatewayDisplayCommand()}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getSetupGatewayCopyCommand(),
                              "Download command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                  sx={getCommandTextFieldSx(activeColorScheme)}
                />
              </Box>

              {/* Step 2: Configure the Gateway */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
                  Step 2: Configure the Gateway
                </Typography>
                {registrationToken ? (
                  <>
                    {hasJustRegeneratedToken && (
                      <Alert severity="success" sx={{ mb: 2 }}>
                        Successfully generated new configurations. Use the
                        updated command below to reconfigure your gateway.
                      </Alert>
                    )}
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mb: 2 }}
                    >
                      Run this command to create {gatewayEnvFile} with the
                      required environment variables:
                    </Typography>
                    <TextField
                      fullWidth
                      multiline
                      minRows={4}
                      sx={getCommandTextFieldSx(activeColorScheme)}
                      value={getConfigureGatewayDisplayCommand(null)}
                      onCopy={(e) => {
                        e.preventDefault();
                        e.clipboardData.setData(
                          "text/plain",
                          getConfigureGatewayCopyCommand(
                            registrationToken,
                            null,
                          ),
                        );
                      }}
                      slotProps={{
                        input: {
                          readOnly: true,
                          endAdornment: (
                            <IconButton
                              size="small"
                              onClick={() =>
                                handleCopy(
                                  getConfigureGatewayCopyCommand(
                                    registrationToken,
                                    null,
                                  ),
                                  "Configure command",
                                )
                              }
                            >
                              <Copy />
                            </IconButton>
                          ),
                        },
                      }}
                    />
                    <Alert severity="info" sx={{ mt: 2 }}>
                      To gain gateway analytics, you can integrate with Moesif
                      by adding your Moesif application token with the key{" "}
                      <code>MOESIF_KEY</code> to your{" "}
                      <code>configs/keys.env</code>.
                    </Alert>
                  </>
                ) : (
                  <>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mb: 2 }}
                    >
                      The registration token is single-use. If you need to
                      reconfigure the gateway, generate a new token—this will
                      revoke the old token and disconnect the gateway from the
                      control plane.
                    </Typography>
                    <Button
                      variant="outlined"
                      color="primary"
                      onClick={handleRegenerateToken}
                      disabled={isRegeneratingToken}
                    >
                      Reconfigure
                    </Button>
                  </>
                )}
              </Box>

              {/* Step 3: Start the Gateway */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }} color="warning.main">
                  Step 3: Start the Gateway
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 1.5 }}
                >
                  1. Navigate to the gateway folder.
                </Typography>
                <TextField
                  fullWidth
                  value={getStep3NavigateCommand()}
                  sx={getCommandTextFieldSx(activeColorScheme)}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getStep3NavigateCommand(),
                              "Navigate command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                />
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ my: 2 }}
                >
                  2. Run this command to start the gateway using the
                  configs/keys.env file created in Step 2:
                </Typography>
                <TextField
                  fullWidth
                  value={getStartGatewayDisplayCommand()}
                  sx={getCommandTextFieldSx(activeColorScheme)}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getStartGatewayCopyCommand(),
                              "Start command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                />
              </Box>
            </Stack>
          </TabPanel>

          {/* Kubernetes Tab */}
          <TabPanel value={tabIndex} index={3}>
            <Stack spacing={3}>
              {/* Prerequisites - Kubernetes */}
              <Box>
                <Typography variant="h6" sx={{ mb: 1 }}>
                  Prerequisites
                </Typography>
                <Stack component="ul" spacing={0.5} sx={{ pl: 3 }}>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    cURL installed
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    unzip installed
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    Kubernetes 1.32+
                  </Typography>
                  <Typography
                    component="li"
                    variant="body2"
                    color="text.secondary"
                  >
                    Helm 3.18+
                  </Typography>
                </Stack>
              </Box>

              {/* Token notice / reconfigure */}
              <Box>
                {registrationToken ? (
                  hasJustRegeneratedToken && (
                    <Alert severity="success" sx={{ mb: 2 }}>
                      Successfully generated new configurations. Use the updated
                      command below to install the gateway chart.
                    </Alert>
                  )
                ) : (
                  <>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mb: 2 }}
                    >
                      The registration token is a one-time generated token for
                      this gateway. If you need to install or update the gateway
                      chart again, first reconfigure this gateway to generate a
                      new registration token. Reconfiguring will revoke the
                      previous token.
                    </Typography>
                    <Button
                      variant="outlined"
                      color="primary"
                      onClick={handleRegenerateToken}
                      disabled={isRegeneratingToken}
                    >
                      Reconfigure
                    </Button>
                  </>
                )}
              </Box>

              {/* Installing the Chart */}
              <Box>
                <Typography variant="h6" color="warning.main">
                  Installing the Chart
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 2 }}
                >
                  Run this command to install the gateway chart with control
                  plane configurations:
                </Typography>
                <TextField
                  fullWidth
                  sx={getCommandTextFieldSx(activeColorScheme)}
                  multiline
                  minRows={4}
                  value={getK8sCustomHelmDisplayCommand(null)}
                  slotProps={{
                    input: {
                      readOnly: true,
                      endAdornment: (
                        <IconButton
                          size="small"
                          onClick={() =>
                            handleCopy(
                              getK8sCustomHelmCopyCommand(
                                registrationToken,
                                null,
                              ),
                              "Helm install command",
                            )
                          }
                        >
                          <Copy />
                        </IconButton>
                      ),
                    },
                  }}
                />
              </Box>
            </Stack>
          </TabPanel>
        </CardContent>
      </Card>

      <Drawer
        anchor="right"
        open={isConfigsDrawerOpen}
        onClose={() => setIsConfigsDrawerOpen(false)}
      >
        <Box
          sx={{
            width: { xs: "100vw", sm: 560 },
            maxWidth: "100vw",
            p: 2,
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "flex-start",
              justifyContent: "space-between",
              gap: 1,
            }}
          >
            <Stack spacing={0.5}>
              <Typography variant="body1">
                {(gateway?.displayName || gateway?.name || "Gateway") +
                  " Configurations"}
              </Typography>
            </Stack>
            <IconButton
              size="small"
              aria-label="Close gateway configs drawer"
              onClick={() => setIsConfigsDrawerOpen(false)}
            >
              <X size={18} />
            </IconButton>
          </Box>

          <Divider sx={{ my: 2 }} />

          {isConfigsLoading ? (
            <Box
              sx={{ display: "flex", alignItems: "center", gap: 1.5, py: 1 }}
            >
              <CircularProgress size={20} />
              <Typography variant="body2" color="text.secondary">
                Loading gateway configurations...
              </Typography>
            </Box>
          ) : gatewayConfigsError ? (
            <ErrorAlert
              error={new Error(gatewayConfigsError)}
              onRetry={() => {
                void handleOpenGatewayConfigsDrawer();
              }}
            />
          ) : (
            <Stack spacing={2}>
              <Tooltip
                title="You must configure the Gateway before downloading the keys.env file."
                disableHoverListener={Boolean(registrationToken)}
              >
                <span
                  style={{ display: "inline-flex", alignSelf: "flex-start" }}
                >
                  <Button
                    variant="outlined"
                    size="small"
                    startIcon={<Download size={16} />}
                    onClick={handleDownloadKeysEnvFile}
                    disabled={!registrationToken}
                  >
                    Download keys.env file
                  </Button>
                </span>
              </Tooltip>

              {registrationToken && (
                <Box sx={{ display: "flex", alignItems: "flex-end", gap: 1 }}>
                  <FormControl fullWidth>
                    <FormLabel>Gateway Registration Token</FormLabel>
                    <TextField
                      fullWidth
                      type={showDrawerRegistrationToken ? "text" : "password"}
                      value={registrationToken}
                      placeholder="Not available"
                      slotProps={{
                        input: {
                          readOnly: true,
                          endAdornment: (
                            <IconButton
                              size="small"
                              onClick={() =>
                                setShowDrawerRegistrationToken((prev) => !prev)
                              }
                            >
                              {showDrawerRegistrationToken ? (
                                <EyeOff />
                              ) : (
                                <Eye />
                              )}
                            </IconButton>
                          ),
                        },
                      }}
                    />
                  </FormControl>
                  <IconButton
                    size="small"
                    onClick={() =>
                      handleCopy(
                        registrationToken,
                        "Gateway registration token",
                      )
                    }
                    sx={{ mb: 0.5 }}
                  >
                    <Copy size={16} />
                  </IconButton>
                </Box>
              )}

              <Divider />

              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Typography variant="body2">
                  Key Manager Configurations
                </Typography>
                <Tooltip title="You can change config.toml using these values.">
                  <Box
                    sx={{
                      width: 20,
                      height: 20,
                      border: "1px solid",
                      borderColor: "divider",
                      borderRadius: "50%",
                      display: "inline-flex",
                      alignItems: "center",
                      justifyContent: "center",
                      fontSize: 12,
                      color: "text.secondary",
                      cursor: "default",
                    }}
                  >
                    i
                  </Box>
                </Tooltip>
              </Box>

              {keyManagerConfigs.length === 0 ? (
                <Alert severity="warning">
                  No key manager configurations are available for this gateway.
                </Alert>
              ) : (
                keyManagerConfigs.map((keyManagerConfig, index) => (
                  <Card key={`${keyManagerConfig.name}-${index}`}>
                    <CardContent sx={{ p: 2 }}>
                      <Stack spacing={2}>
                        <Typography variant="body1">
                          {keyManagerConfig.name}
                        </Typography>

                        <FormControl fullWidth>
                          <FormLabel>Issuer URL</FormLabel>
                          <TextField
                            fullWidth
                            value={keyManagerConfig.issuerUrl}
                            placeholder="Not available"
                            slotProps={{
                              input: {
                                readOnly: true,
                                endAdornment: (
                                  <IconButton
                                    size="small"
                                    onClick={() =>
                                      handleCopy(
                                        keyManagerConfig.issuerUrl,
                                        "Issuer URL",
                                      )
                                    }
                                    disabled={!keyManagerConfig.issuerUrl}
                                  >
                                    <Copy size={16} />
                                  </IconButton>
                                ),
                              },
                            }}
                          />
                        </FormControl>

                        <FormControl fullWidth>
                          <FormLabel>JWKS URL</FormLabel>
                          <TextField
                            fullWidth
                            value={keyManagerConfig.jwksUrl}
                            placeholder="Not available"
                            slotProps={{
                              input: {
                                readOnly: true,
                                endAdornment: (
                                  <IconButton
                                    size="small"
                                    onClick={() =>
                                      handleCopy(
                                        keyManagerConfig.jwksUrl,
                                        "JWKS URL",
                                      )
                                    }
                                    disabled={!keyManagerConfig.jwksUrl}
                                  >
                                    <Copy size={16} />
                                  </IconButton>
                                ),
                              },
                            }}
                          />
                        </FormControl>
                      </Stack>
                    </CardContent>
                  </Card>
                ))
              )}
            </Stack>
          )}
        </Box>
      </Drawer>

      {/* Reconfigure Confirmation Dialog */}
      <Dialog
        open={isRegenerateDialogOpen}
        onClose={() => !isRegeneratingToken && setIsRegenerateDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Reconfigure gateway</DialogTitle>
        <DialogContent>
          <Typography>
            Regenerating the registration token will revoke the existing token
            for this gateway and disconnect the gateway from the control plane.
            Do you want to continue?
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setIsRegenerateDialogOpen(false)}
            disabled={isRegeneratingToken}
            variant="outlined"
            color="secondary"
          >
            Cancel
          </Button>
          <Button
            onClick={handleConfirmRegenerateToken}
            color="error"
            variant="contained"
            disabled={isRegeneratingToken}
          >
            {isRegeneratingToken ? (
              <CircularProgress size={20} />
            ) : (
              "Reconfigure"
            )}
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}

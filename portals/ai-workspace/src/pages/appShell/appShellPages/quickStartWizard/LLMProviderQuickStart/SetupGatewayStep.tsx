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

/**
 * The wizard's in-drawer view of a self-hosted gateway: identity header, live
 * connection state, and the download/configure/start instructions.
 *
 * The instructions themselves are NOT written here — this renders the same
 * `GatewaySetupSteps` the Gateways page uses, picked by gateway version the
 * same way `ViewGateway` picks it. That is deliberate: the commands differ
 * between gateway v1.2+ (`scripts/setup.sh` + `api-platform.env`) and older
 * releases (`configs/keys.env`), and a second copy of that logic here would
 * drift the moment a new gateway release changes it.
 */

import { useEffect, useRef, useState } from 'react';
import { Box, Chip, CircularProgress, Stack, Typography } from '@wso2/oxygen-ui';
import { HardDrive } from '@wso2/oxygen-ui-icons-react';
import {
  getActiveColorScheme,
  subscribeToColorSchemeChanges,
  type ColorScheme,
} from '../../../../../utils/colorScheme';
import { useAppShell } from '../../../../../contexts/AppShellContext';
import { useEnvironments } from '../../../../../hooks/useEnvironments';
import { useAIWorkspaceSnackbar } from '../../../../../hooks/aiWorkspaceSnackbar';
import {
  getGatewayById,
  listGatewayTokens,
  revokeGatewayToken,
  rotateGatewayToken,
  type HybridGateway,
} from '../../../../../apis/gateway/gatewayApi';
import {
  GatewaySetupStepsV1_2Plus,
  GatewaySetupStepsPreV1_2,
  isGatewayV12OrAbove,
} from '../../gateways/GatewaySetupSteps';
import {
  getGatewayFolderName,
  getGatewayVersionHelm,
  getGatewayZipName,
  resolveGatewayReleaseTag,
} from './utils';

type SetupGatewayStepProps = {
  gatewayId: string;
  gateway: HybridGateway | null;
  gatewayVersion?: string;
  registrationToken: string | null;
  onGatewayChange: (gateway: HybridGateway) => void;
  onRegistrationTokenChange: (token: string | null) => void;
  onGatewayReadyChange: (isReady: boolean) => void;
};

/** How often the wizard re-reads the gateway while waiting for it to connect. */
const CONNECTED_POLL_INTERVAL_MS = 15000;
const WAITING_POLL_INTERVAL_MS = 5000;

export default function SetupGatewayStep({
  gatewayId,
  gateway: initialGateway,
  gatewayVersion,
  registrationToken,
  onGatewayChange,
  onRegistrationTokenChange,
  onGatewayReadyChange,
}: SetupGatewayStepProps) {
  const { currentOrganization } = useAppShell();
  const { environments } = useEnvironments();
  const showSnackbar = useAIWorkspaceSnackbar();
  const [gateway, setGateway] = useState<HybridGateway | null>(initialGateway);
  const [loading, setLoading] = useState(!initialGateway);
  const [isRegeneratingToken, setIsRegeneratingToken] = useState(false);
  const [hasJustRegeneratedToken, setHasJustRegeneratedToken] = useState(false);
  const [activeColorScheme, setActiveColorScheme] = useState<ColorScheme>(() =>
    getActiveColorScheme()
  );
  const previousIsActiveRef = useRef(Boolean(initialGateway?.isActive));

  const getEnvironmentName = (envId: string): string =>
    environments.find((environment) => environment.id === envId)?.name || envId;

  useEffect(() => {
    const unsubscribe = subscribeToColorSchemeChanges((nextScheme) => {
      setActiveColorScheme((prev) => (prev === nextScheme ? prev : nextScheme));
    });

    return unsubscribe;
  }, []);

  useEffect(() => {
    onGatewayReadyChange(Boolean(gateway?.isActive));
  }, [gateway?.isActive, onGatewayReadyChange]);

  useEffect(() => {
    const isActiveNow = Boolean(gateway?.isActive);
    if (isActiveNow && !previousIsActiveRef.current) {
      showSnackbar('AI Gateway connected successfully', 'success');
    }
    previousIsActiveRef.current = isActiveNow;
  }, [gateway?.isActive, showSnackbar]);

  useEffect(() => {
    if (gateway?.isActive) {
      setHasJustRegeneratedToken(false);
    }
  }, [gateway?.isActive]);

  useEffect(() => {
    if (!gatewayId || !currentOrganization?.uuid) {
      return;
    }

    let isMounted = true;

    const loadGateway = async () => {
      try {
        setLoading(true);
        const response = await getGatewayById(gatewayId, currentOrganization.uuid);
        if (!isMounted || !response.data) {
          return;
        }

        const nextGateway: HybridGateway = {
          ...response.data,
          status: response.data.isActive ? 'connected' : 'disconnected',
        };
        setGateway(nextGateway);
        onGatewayChange(nextGateway);
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    };

    void loadGateway();

    return () => {
      isMounted = false;
    };
  }, [currentOrganization?.uuid, gatewayId, onGatewayChange]);

  // Keep polling after the gateway connects too: the wizard's Next button is
  // gated on `isActive`, so a gateway that drops has to ungate it again.
  useEffect(() => {
    if (!gateway?.id || !currentOrganization?.uuid) {
      return;
    }

    const intervalMs = gateway.isActive
      ? CONNECTED_POLL_INTERVAL_MS
      : WAITING_POLL_INTERVAL_MS;
    const interval = setInterval(async () => {
      try {
        const response = await getGatewayById(gateway.id, currentOrganization.uuid);
        if (!response.data) {
          return;
        }
        const nextGateway: HybridGateway = {
          ...response.data,
          status: response.data.isActive ? 'connected' : 'disconnected',
        };
        setGateway(nextGateway);
        onGatewayChange(nextGateway);
      } catch {
        // Ignore transient polling failures.
      }
    }, intervalMs);

    return () => {
      clearInterval(interval);
    };
  }, [currentOrganization?.uuid, gateway?.id, gateway?.isActive, onGatewayChange]);

  const handleCopy = async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      showSnackbar(`Failed to copy ${label.toLowerCase()}.`, 'error');
    }
  };

  /**
   * The registration token is single-use, so reconfiguring means minting a new
   * one — which first revokes every token the gateway still holds, otherwise a
   * previously issued token would keep working.
   */
  const regenerateGatewayToken = async (): Promise<string> => {
    if (!gateway?.id || !currentOrganization?.uuid) {
      throw new Error('Gateway ID or organization ID is not available.');
    }

    const gatewayTokens = await listGatewayTokens(
      gateway.id,
      currentOrganization.uuid
    );

    await Promise.all(
      gatewayTokens.map((token) =>
        revokeGatewayToken(gateway.id, token.id, currentOrganization.uuid).catch(
          () => undefined
        )
      )
    );

    return rotateGatewayToken(gateway.id, currentOrganization.uuid);
  };

  const handleRegenerateToken = async () => {
    try {
      setIsRegeneratingToken(true);
      const newToken = await regenerateGatewayToken();
      onRegistrationTokenChange(newToken);
      setHasJustRegeneratedToken(true);
      showSnackbar('Successfully generated new registration token', 'success');
    } catch (error: unknown) {
      const typedError = error as { message?: string };
      showSnackbar(
        typedError.message || 'Failed to regenerate registration token.',
        'error'
      );
    } finally {
      setIsRegeneratingToken(false);
    }
  };

  if (loading || !gateway) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  const configuredVersion = gateway.properties?.version || gatewayVersion;
  const releaseTag = resolveGatewayReleaseTag(configuredVersion);
  const SetupSteps = isGatewayV12OrAbove(getGatewayVersionHelm(releaseTag))
    ? GatewaySetupStepsV1_2Plus
    : GatewaySetupStepsPreV1_2;

  const renderConnectionStatus = () => (
    <Typography
      variant="body2"
      sx={{
        mt: 1.5,
        fontFamily: 'monospace',
        color: gateway.isActive ? 'success.main' : 'info.main',
        fontWeight: 600,
      }}
    >
      {gateway.isActive
        ? '✓ Gateway connected'
        : 'Waiting to connect to the gateway...'}
    </Typography>
  );

  return (
    <Stack spacing={3}>
      <Box sx={{ display: 'flex', gap: 2.5, alignItems: 'flex-start' }}>
        <Box
          sx={{
            width: 72,
            height: 72,
            borderRadius: 2,
            bgcolor: 'action.hover',
            color: 'text.disabled',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
          }}
        >
          <HardDrive size={28} />
        </Box>

        <Stack spacing={0.75}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
            <Typography variant="h3" sx={{ fontSize: '2rem', fontWeight: 700 }}>
              {gateway.displayName || gateway.name}
            </Typography>
            <Chip
              size="small"
              variant="outlined"
              color={gateway.isActive ? 'success' : 'default'}
              label={gateway.isActive ? 'Active' : 'Not Deployed'}
            />
          </Box>

          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
            <Typography variant="body2" color="text.secondary">
              {gateway.properties?.environment
                ? getEnvironmentName(gateway.properties.environment)
                : 'Environment'}
            </Typography>
            <Chip size="small" variant="outlined" label={releaseTag} />
            <Typography variant="body2" color="text.secondary">
              {gateway.vhost}
            </Typography>
          </Stack>
        </Stack>
      </Box>

      <SetupSteps
        gatewayVersion={releaseTag}
        gatewayZipName={getGatewayZipName(configuredVersion)}
        gatewayFolderName={getGatewayFolderName(configuredVersion)}
        registrationToken={registrationToken}
        hasJustRegeneratedToken={hasJustRegeneratedToken}
        isRegeneratingToken={isRegeneratingToken}
        onRegenerateToken={() => {
          void handleRegenerateToken();
        }}
        onCopy={(text, label) => {
          void handleCopy(text, label);
        }}
        colorScheme={activeColorScheme}
        renderConnectionStatus={renderConnectionStatus}
      />
    </Stack>
  );
}

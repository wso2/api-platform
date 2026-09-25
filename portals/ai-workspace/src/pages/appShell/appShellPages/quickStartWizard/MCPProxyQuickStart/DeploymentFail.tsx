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

import type { Dispatch, SetStateAction } from 'react';
import {
  Alert,
  Avatar,
  Box,
  Card,
  CardContent,
  Chip,
  FormControl,
  FormLabel,
  IconButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Clock, Pencil } from '@wso2/oxygen-ui-icons-react';
import { formatRelativeTime } from '../../../../../contexts/llmProvider';
import type { HybridGateway } from '../../../../../apis/gateway/gatewayApi';
import GatewayDeploySection from '../LLMProviderQuickStart/GatewayDeploySection';
import type { GatewayFormState } from '../LLMProviderQuickStart/types';

function getInitials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return '';
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}

type DeploymentFailProps = {
  serverName: string;
  serverVersion: string;
  serverDescription: string;
  serverContext: string;
  serverSavedAt: string | null;
  deploymentError: string | null;
  onEditDetails: () => void;
  gatewayFormState: GatewayFormState;
  setGatewayFormState: Dispatch<SetStateAction<GatewayFormState>>;
  preferredGatewayId: string | null;
  onPreferredGatewayChange: (gatewayId: string) => void;
  createdGateway: HybridGateway | null;
  onGatewayCreated: (gateway: HybridGateway) => void;
  onGatewayChange: (gateway: HybridGateway) => void;
  gatewayRegistrationToken: string | null;
  onRegistrationTokenChange: (token: string | null) => void;
  onGatewayReadyChange: (isReady: boolean) => void;
};

export default function DeploymentFail({
  serverName,
  serverVersion,
  serverDescription,
  serverContext,
  serverSavedAt,
  deploymentError,
  onEditDetails,
  gatewayFormState,
  setGatewayFormState,
  preferredGatewayId,
  onPreferredGatewayChange,
  createdGateway,
  onGatewayCreated,
  onGatewayChange,
  gatewayRegistrationToken,
  onRegistrationTokenChange,
  onGatewayReadyChange,
}: DeploymentFailProps) {
  return (
    <Stack spacing={3}>
      <Card variant="outlined">
        <CardContent>
          <Stack direction="row" spacing={2} alignItems="flex-start">
            <Avatar
              sx={{
                width: 56,
                height: 56,
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
                fontWeight: 700,
                fontSize: 18,
              }}
            >
              {getInitials(serverName)}
            </Avatar>

            <Stack spacing={0.5} sx={{ flex: 1, minWidth: 0 }}>
              <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
                <Typography variant="h5" sx={{ fontWeight: 700 }}>
                  {serverName}
                </Typography>
                <Chip label={serverVersion || 'v1.0'} size="small" variant="outlined" />
                <IconButton
                  size="small"
                  aria-label="Edit server details"
                  onClick={onEditDetails}
                >
                  <Pencil size={14} />
                </IconButton>
              </Stack>

              <Typography variant="body2" color="text.secondary">
                {serverDescription.trim() || 'No description'}
              </Typography>

              <Typography variant="caption" color="text.secondary">
                Context: {serverContext || '/'}
              </Typography>

              <Stack direction="row" spacing={0.75} alignItems="center">
                <Typography variant="caption" color="text.secondary">
                  Last updated :
                </Typography>
                <Clock size={14} />
                <Typography variant="caption" color="text.secondary">
                  {serverSavedAt ? formatRelativeTime(serverSavedAt) : '—'}
                </Typography>
              </Stack>
            </Stack>
          </Stack>
        </CardContent>
      </Card>

      <Box>
        <FormControl fullWidth>
          <FormLabel>Deploy to:</FormLabel>
          <GatewayDeploySection
            gatewayFormState={gatewayFormState}
            setGatewayFormState={setGatewayFormState}
            preferredGatewayId={preferredGatewayId}
            onPreferredGatewayChange={onPreferredGatewayChange}
            createdGateway={createdGateway}
            onGatewayCreated={onGatewayCreated}
            onGatewayChange={onGatewayChange}
            gatewayRegistrationToken={gatewayRegistrationToken}
            onRegistrationTokenChange={onRegistrationTokenChange}
            onGatewayReadyChange={onGatewayReadyChange}
          />
        </FormControl>

        {deploymentError ? (
          <Alert severity="error" sx={{ mt: 2 }}>
            {deploymentError}
          </Alert>
        ) : null}
      </Box>
    </Stack>
  );
}

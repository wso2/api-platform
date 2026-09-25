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

import React from 'react';
import type { Dispatch, SetStateAction } from 'react';
import {
  FormControl,
  FormLabel,
  Grid,
  MenuItem,
  Select,
  TextField,
} from '@wso2/oxygen-ui';
import type { ProjectBase } from '../../../../../utils/types';
import type { ParameterValues } from '../../../PolicyParameterEditor/types';
import type { EndpointValidationResponse } from '../../externalServers/externalServersValidationTypes';
import PolicyMapper, {
  type SelectedPolicy,
} from '../../externalServers/PolicyMapper';
import GatewayDeploySection from '../LLMProviderQuickStart/GatewayDeploySection';
import type { GatewayFormState } from '../LLMProviderQuickStart/types';
import type { HybridGateway } from '../../../../../apis/gateway/gatewayApi';

type ConfigureMCPStepProps = {
  projects: ProjectBase[];
  selectedProjectId: string;
  onProjectChange: (id: string) => void;
  serverName: string;
  serverVersion: string;
  serverDescription: string;
  serverContext: string;
  serverTarget: string;
  onNameChange: (v: string) => void;
  onVersionChange: (v: string) => void;
  onDescriptionChange: (v: string) => void;
  onContextChange: (v: string) => void;
  onTargetChange: (v: string) => void;
  selectedPolicies: SelectedPolicy[];
  onAddPolicy: (policy: Omit<SelectedPolicy, 'instanceId'>) => void;
  onUpdatePolicy: (instanceId: string, params: ParameterValues) => void;
  onRemovePolicy: (instanceId: string) => void;
  onReorderPolicies: (draggedId: string, targetId: string) => void;
  validationResult: EndpointValidationResponse | null;
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

export default function ConfigureMCPStep({
  projects,
  selectedProjectId,
  onProjectChange,
  serverName,
  serverVersion,
  serverDescription,
  serverContext,
  serverTarget,
  onNameChange,
  onVersionChange,
  onDescriptionChange,
  onContextChange,
  onTargetChange,
  selectedPolicies,
  onAddPolicy,
  onUpdatePolicy,
  onRemovePolicy,
  onReorderPolicies,
  validationResult,
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
}: ConfigureMCPStepProps) {
  return (
    <Grid container spacing={2}>
      <Grid size={{ xs: 12, md: 10 }}>
        <Grid container spacing={2}>

          <Grid size={{ xs: 12, md: 4 }}>
            <FormControl fullWidth>
              <FormLabel required>Project</FormLabel>
              <Select
                value={selectedProjectId}
                onChange={(event) => onProjectChange(String(event.target.value))}
                displayEmpty
                disabled={projects.length === 0}
              >
                {projects.length === 0 ? (
                  <MenuItem value="" disabled>
                    No projects available
                  </MenuItem>
                ) : (
                  projects.map((project) => (
                    <MenuItem key={project.id} value={project.id}>
                      {project.displayName}
                    </MenuItem>
                  ))
                )}
              </Select>
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 5 }}>
            <FormControl fullWidth>
              <FormLabel required>MCP Server Name</FormLabel>
              <TextField
                fullWidth
                placeholder="Acme MCP Server"
                value={serverName}
                onChange={(event) => onNameChange(event.target.value)}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12, md: 3 }}>
            <FormControl fullWidth>
              <FormLabel required>Version</FormLabel>
              <TextField
                fullWidth
                placeholder="v1.0"
                value={serverVersion}
                onChange={(event) => onVersionChange(event.target.value)}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Context (Optional)</FormLabel>
              <TextField
                fullWidth
                value={serverContext}
                onChange={(event) => onContextChange(event.target.value)}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel required>Target</FormLabel>
              <TextField
                fullWidth
                placeholder="https://example.com/mcp"
                value={serverTarget}
                onChange={(event) => onTargetChange(event.target.value)}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControl fullWidth>
              <FormLabel>Description (Optional)</FormLabel>
              <TextField
                fullWidth
                multiline
                minRows={2}
                placeholder="Primary MCP Proxy"
                value={serverDescription}
                onChange={(event) => onDescriptionChange(event.target.value)}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <PolicyMapper
              selectedPolicies={selectedPolicies}
              onAddPolicy={onAddPolicy}
              onUpdatePolicy={onUpdatePolicy}
              onRemovePolicy={onRemovePolicy}
              onReorderPolicies={onReorderPolicies}
              validationResult={validationResult ?? undefined}
            />
          </Grid>

          <Grid size={{ xs: 12 }}>
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
          </Grid>

        </Grid>
      </Grid>
    </Grid>
  );
}

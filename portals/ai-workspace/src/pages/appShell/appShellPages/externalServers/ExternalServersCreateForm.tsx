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
import {
  Button,
  Chip,
  FormControl,
  FormLabel,
  Grid,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { HelpCircle } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import { SimpleTagInput } from '../../PolicyParameterEditor/FieldRenderers';
import {
  mcpSpecVersionWarning,
  validateMCPSpecVersion,
} from './mcpSpecVersions';

type FieldErrors = {
  name?: string;
  version?: string;
  description?: string;
  context?: string;
  target?: string;
};

type Props = {
  isCreateDisabled: boolean;
  serverContext: string;
  serverDescription: string;
  serverName: string;
  serverSpecVersions: string[];
  serverTarget: string;
  serverVersion: string;
  /** Versions the probe reported, offered as one-click additions. */
  suggestedSpecVersions?: string[];
  fieldErrors?: FieldErrors;
  onCancel: () => void;
  onCreate: () => void;
  onContextChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onNameChange: (value: string) => void;
  onSpecVersionsChange: (value: string[]) => void;
  onTargetChange: (value: string) => void;
  onVersionChange: (value: string) => void;
};

export default function ExternalServersCreateForm({
  isCreateDisabled,
  serverContext,
  serverDescription,
  serverName,
  serverSpecVersions,
  serverTarget,
  serverVersion,
  suggestedSpecVersions = [],
  fieldErrors = {},
  onCancel,
  onCreate,
  onContextChange,
  onDescriptionChange,
  onNameChange,
  onSpecVersionsChange,
  onTargetChange,
  onVersionChange,
}: Props): JSX.Element {
  const unusedSuggestions = suggestedSpecVersions.filter(
    (version) => !serverSpecVersions.includes(version)
  );
  const specVersionWarning = mcpSpecVersionWarning(serverSpecVersions);

  return (
    <Stack spacing={2} sx={{ mt: 1, maxWidth: 920 }}>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 8 }}>
          <FormControl fullWidth>
            <FormLabel required>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.form.name"
                defaultMessage="Name"
              />
            </FormLabel>
            <TextField
              fullWidth
              placeholder="WSO2 MCP Proxy"
              value={serverName}
              onChange={(event) => onNameChange(event.target.value)}
              error={Boolean(fieldErrors.name)}
              helperText={fieldErrors.name}
            />
          </FormControl>
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <FormControl fullWidth>
            <FormLabel required>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.form.version"
                defaultMessage="Version"
              />
            </FormLabel>
            <TextField
              fullWidth
              placeholder="v1.0"
              value={serverVersion}
              onChange={(event) => onVersionChange(event.target.value)}
              error={Boolean(fieldErrors.version)}
              helperText={fieldErrors.version}
            />
          </FormControl>
        </Grid>
        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.form.description"
                defaultMessage="Description"
              />
            </FormLabel>
            <TextField
              fullWidth
              multiline
              minRows={3}
              placeholder="Primary MCP Proxy"
              value={serverDescription}
              onChange={(event) => onDescriptionChange(event.target.value)}
              error={Boolean(fieldErrors.description)}
              helperText={fieldErrors.description}
            />
          </FormControl>
        </Grid>
        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.form.context"
                defaultMessage="Context"
              />
            </FormLabel>
            <TextField
              fullWidth
              value={serverContext}
              onChange={(event) => onContextChange(event.target.value)}
              error={Boolean(fieldErrors.context)}
              helperText={fieldErrors.context}
            />
          </FormControl>
        </Grid>
        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel required>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.form.target"
                defaultMessage="Target"
              />
            </FormLabel>
            <TextField
              fullWidth
              placeholder="https://example.com/mcp"
              value={serverTarget}
              onChange={(event) => onTargetChange(event.target.value)}
              error={Boolean(fieldErrors.target)}
              helperText={fieldErrors.target}
            />
          </FormControl>
        </Grid>
        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth>
            <FormLabel>
              <Stack direction="row" spacing={0.5} alignItems="center">
                <span>
                  <FormattedMessage
                    id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create.form.mcpSpecVersions"
                    defaultMessage="MCP Versions"
                  />
                </span>
                <Tooltip title="MCP specification versions this proxy declares support for.">
                  <HelpCircle size={14} />
                </Tooltip>
              </Stack>
            </FormLabel>
            <SimpleTagInput
              testId="mcp-spec-versions"
              placeholder="2026-07-28"
              value={serverSpecVersions}
              onChange={onSpecVersionsChange}
              validate={validateMCPSpecVersion}
            />
            {specVersionWarning && (
              <Typography variant="body2" color="warning.main" sx={{ mt: 0.5 }}>
                {specVersionWarning}
              </Typography>
            )}
            {unusedSuggestions.length > 0 && (
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                flexWrap="wrap"
                useFlexGap
                sx={{ mt: 1 }}
              >
                <Typography variant="body2" color="text.secondary">
                  Reported by the upstream server
                </Typography>
                {unusedSuggestions.map((version) => (
                  <Chip
                    key={version}
                    label={version}
                    size="small"
                    variant="outlined"
                    onClick={() =>
                      onSpecVersionsChange([...serverSpecVersions, version])
                    }
                  />
                ))}
              </Stack>
            )}
          </FormControl>
        </Grid>
      </Grid>

      <Stack direction="row" spacing={1}>
        <Button variant="outlined" color="secondary" onClick={onCancel}>
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.cancel"
            defaultMessage="Cancel"
          />
        </Button>
        <Button
          variant="contained"
          disabled={isCreateDisabled}
          onClick={onCreate}
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.externalServers.Main.create"
            defaultMessage="Create"
          />
        </Button>
      </Stack>
    </Stack>
  );
}

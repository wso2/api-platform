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

import React, { useState } from 'react';
import {
  Alert,
  Button,
  Checkbox,
  FormControl,
  FormLabel,
  Grid,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Edit } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage } from 'react-intl';
import TransportPathField from './TransportPathField';
import type { A2ATransport } from '../../../../utils/types';

type FieldErrors = {
  name?: string;
  version?: string;
  description?: string;
  context?: string;
  target?: string;
  transports?: string;
};

export const AGENT_TRANSPORT_OPTIONS: ReadonlyArray<Required<A2ATransport>> = [
  { protocolBinding: 'JSONRPC', pathPrefix: '/rpc' },
  { protocolBinding: 'HTTP+JSON', pathPrefix: '/rest' },
];

type Props = {
  isCreateDisabled: boolean;
  /** Path each binding is served on, seeded from the card and editable here. */
  transportPaths: Record<string, string>;
  onTransportPathChange: (protocolBinding: string, path: string) => void;
  agentContext: string;
  agentDescription: string;
  agentName: string;
  agentTarget: string;
  agentVersion: string;
  selectedTransports: string[];
  fieldErrors?: FieldErrors;
  onCancel: () => void;
  onCreate: () => void;
  onContextChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onNameChange: (value: string) => void;
  onTargetChange: (value: string) => void;
  onVersionChange: (value: string) => void;
  onTransportToggle: (protocolBinding: string) => void;
};

export default function AgentProxiesCreateForm({
  isCreateDisabled,
  agentContext,
  agentDescription,
  agentName,
  agentTarget,
  agentVersion,
  selectedTransports,
  transportPaths,
  onTransportPathChange,
  fieldErrors = {},
  onCancel,
  onCreate,
  onContextChange,
  onDescriptionChange,
  onNameChange,
  onTargetChange,
  onVersionChange,
  onTransportToggle,
}: Props): React.JSX.Element {
  const [editingTransport, setEditingTransport] = useState<string | null>(null);

  return (
    <Stack spacing={2} sx={{ mt: 1, maxWidth: 920 }}>
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 8 }}>
          <FormControl fullWidth>
            <FormLabel required>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.form.name"
                defaultMessage="Name"
              />
            </FormLabel>
            <TextField
              fullWidth
              placeholder="Trip Planning Agent"
              value={agentName}
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
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.form.version"
                defaultMessage="Version"
              />
            </FormLabel>
            <TextField
              fullWidth
              placeholder="v1.0"
              value={agentVersion}
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
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.form.description"
                defaultMessage="Description"
              />
            </FormLabel>
            <TextField
              fullWidth
              multiline
              minRows={3}
              placeholder="Plans multi-city trips end to end"
              value={agentDescription}
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
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.form.context"
                defaultMessage="Context"
              />
            </FormLabel>
            <TextField
              fullWidth
              value={agentContext}
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
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.form.target"
                defaultMessage="Target"
              />
            </FormLabel>
            <TextField
              fullWidth
              placeholder="http://host.docker.internal:9000"
              value={agentTarget}
              onChange={(event) => onTargetChange(event.target.value)}
              error={Boolean(fieldErrors.target)}
              helperText={fieldErrors.target}
            />
          </FormControl>
        </Grid>
        <Grid size={{ xs: 12 }}>
          <FormControl fullWidth error={Boolean(fieldErrors.transports)}>
            <FormLabel
              required
              // Ticking a checkbox focuses the group; keep the label untinted.
              sx={{ '&.Mui-focused:not(.Mui-error)': { color: 'text.secondary' } }}
            >
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.form.transports"
                defaultMessage="Transports to Expose"
              />
            </FormLabel>
            <Grid container spacing={1.5} sx={{ mt: 0.5 }}>
              {AGENT_TRANSPORT_OPTIONS.map((transport) => (
                <Grid key={transport.protocolBinding} size={{ xs: 12, sm: 6 }}>
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="flex-start"
                    onClick={() => onTransportToggle(transport.protocolBinding)}
                    sx={{
                      p: 1.5,
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 1,
                      cursor: 'pointer',
                    }}
                  >
                    <Checkbox
                      size="small"
                      sx={{ p: 0, mt: 0.25 }}
                      checked={selectedTransports.includes(
                        transport.protocolBinding
                      )}
                      onChange={() =>
                        onTransportToggle(transport.protocolBinding)
                      }
                      onClick={(event) => event.stopPropagation()}
                    />
                    <Stack spacing={0.25} sx={{ flex: 1, minWidth: 0 }}>
                      <Typography variant="body2">
                        {transport.protocolBinding}
                      </Typography>
                      <TransportPathField
                        value={
                          transportPaths[transport.protocolBinding] ??
                          transport.pathPrefix
                        }
                        editing={editingTransport === transport.protocolBinding}
                        onEditingChange={(editing) =>
                          setEditingTransport(
                            editing ? transport.protocolBinding : null
                          )
                        }
                        onChange={(path) =>
                          onTransportPathChange(transport.protocolBinding, path)
                        }
                      />
                    </Stack>
                    <Tooltip title="Edit path">
                      <IconButton
                        size="small"
                        sx={{ mt: -0.5, mr: -0.5 }}
                        onClick={(event) => {
                          event.stopPropagation();
                          setEditingTransport(transport.protocolBinding);
                        }}
                        aria-label={`Edit ${transport.protocolBinding} path`}
                      >
                        <Edit size={14} />
                      </IconButton>
                    </Tooltip>
                  </Stack>
                </Grid>
              ))}
            </Grid>
            {fieldErrors.transports ? (
              <Typography variant="caption" color="error" sx={{ mt: 0.75 }}>
                {fieldErrors.transports}
              </Typography>
            ) : null}
          </FormControl>
        </Grid>
      </Grid>

      <Alert severity="info">
        <FormattedMessage
          id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create.form.passthrough.note"
          defaultMessage="The Agent Card is served in Passthrough mode and URLs inside it are rewritten to the gateway. You can change this after the proxy is created."
        />
      </Alert>

      <Stack direction="row" spacing={1}>
        <Button variant="outlined" color="secondary" onClick={onCancel}>
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.cancel"
            defaultMessage="Cancel"
          />
        </Button>
        <Button
          variant="contained"
          disabled={isCreateDisabled}
          onClick={onCreate}
        >
          <FormattedMessage
            id="aiWorkspace.pages.appShell.appShellPages.agentProxies.Main.create"
            defaultMessage="Create"
          />
        </Button>
      </Stack>
    </Stack>
  );
}

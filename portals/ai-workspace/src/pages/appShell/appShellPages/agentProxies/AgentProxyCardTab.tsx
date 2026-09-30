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
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  FormControlLabel,
  IconButton,
  Radio,
  RadioGroup,
  Stack,
  Switch,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { HelpCircle } from '@wso2/oxygen-ui-icons-react';
import Editor from '@monaco-editor/react';
import type { AgentCardMode } from '../../../../utils/types';

export type AgentCardTabState = {
  publicMode: AgentCardMode;
  publicRewriteUrls: boolean;
  publicContent: string;
  protectedMode: AgentCardMode;
  protectedRewriteUrls: boolean;
  protectedContent: string;
};

type Props = {
  state: AgentCardTabState;
  onChange: (patch: Partial<AgentCardTabState>) => void;
  disabled: boolean;
  /** Live passthrough fetch — transient, never stored. */
  fetchedCard: string | null;
  isFetching: boolean;
  fetchError: string | null;
  onRefetch: () => void;
};

const MODE_HELP =
  "Passthrough forwards the upstream agent's own card and stores nothing, so upstream changes flow through automatically. Managed stores a card you author and serves that instead. Set independently for each card.";

function HelpHint({ title }: { title: string }): React.JSX.Element {
  return (
    <Tooltip title={title}>
      <IconButton size="small">
        <HelpCircle size={16} />
      </IconButton>
    </Tooltip>
  );
}

function ModeOption({
  value,
  label,
  description,
  isDefault,
  disabled,
}: {
  value: AgentCardMode;
  label: string;
  description: string;
  isDefault?: boolean;
  disabled?: boolean;
}): React.JSX.Element {
  return (
    <FormControlLabel
      value={value}
      disabled={disabled}
      control={<Radio size="small" sx={{ alignSelf: 'flex-start', pt: 0.5 }} />}
      sx={{
        alignItems: 'flex-start',
        m: 0,
        p: 1.5,
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
      }}
      label={
        <Stack spacing={0.25}>
          <Stack direction="row" spacing={1} alignItems="center">
            <Typography variant="body2">{label}</Typography>
            {isDefault ? <Chip size="small" label="Default" /> : null}
          </Stack>
          <Typography variant="caption" color="text.secondary">
            {description}
          </Typography>
        </Stack>
      }
    />
  );
}

function CardEditor({
  value,
  onChange,
  readOnly,
  placeholder,
}: {
  value: string;
  onChange?: (next: string) => void;
  readOnly?: boolean;
  placeholder?: string;
}): React.JSX.Element {
  return (
    <Box
      sx={{
        position: 'relative',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        overflow: 'hidden',
      }}
    >
      <Editor
        height="300px"
        language="json"
        value={value}
        onChange={(next) => onChange?.(next || '')}
        options={{
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          fontSize: 12,
          lineHeight: 20,
          wordWrap: 'on',
          automaticLayout: true,
          readOnly: Boolean(readOnly),
        }}
        theme="vs-dark"
        loading={<Box sx={{ p: 2 }}>Loading editor...</Box>}
      />
      {/* Overlaid so the hint stays out of the editor's value. */}
      {!value && placeholder ? (
        <Typography
          variant="caption"
          sx={{
            position: 'absolute',
            top: 6,
            left: 62,
            color: 'text.disabled',
            fontFamily: 'monospace',
            pointerEvents: 'none',
          }}
        >
          {placeholder}
        </Typography>
      ) : null}
    </Box>
  );
}

export default function AgentProxyCardTab({
  state,
  onChange,
  disabled,
  fetchedCard,
  isFetching,
  fetchError,
  onRefetch,
}: Props): React.JSX.Element {
  const isPublicManaged = state.publicMode === 'managed';
  const isProtectedManaged = state.protectedMode === 'managed';

  const renderPassthroughCard = (): React.JSX.Element => {
    if (isFetching) {
      return (
        <Stack direction="row" spacing={1} alignItems="center" sx={{ py: 3 }}>
          <CircularProgress size={16} />
          <Typography variant="body2" color="text.secondary">
            Fetching the Agent Card...
          </Typography>
        </Stack>
      );
    }
    if (fetchError) {
      return (
        <Alert severity="warning">
          Could not fetch the card from the upstream agent.
        </Alert>
      );
    }
    return (
      <Stack spacing={1}>
        <Stack direction="row" spacing={1} alignItems="center">
          <Typography variant="caption" color="text.secondary">
            Card Content
          </Typography>
          <Chip size="small" variant="outlined" label="Read Only" />
        </Stack>
        <CardEditor value={fetchedCard ?? ''} readOnly />
        <Typography variant="caption" color="text.disabled">
          Fetched from the upstream. Switch to Managed to edit it.
        </Typography>
      </Stack>
    );
  };

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: { xs: 'column', md: 'row' },
        gap: 3,
        alignItems: 'stretch',
      }}
    >
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Stack spacing={2}>
          <Stack spacing={0.5}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                Public Card
              </Typography>
              <HelpHint title="Served unauthenticated at a discovery path — anyone who can reach the gateway can read it, and no auth policy runs on that route." />
            </Stack>
            <Typography variant="body2" color="text.secondary">
              Served unauthenticated at a discovery path.
            </Typography>
          </Stack>

          <Stack spacing={1}>
            <Stack direction="row" spacing={0.75} alignItems="center">
              <Typography variant="caption" color="text.secondary">
                Mode
              </Typography>
              <HelpHint title={MODE_HELP} />
            </Stack>
            <RadioGroup
              value={state.publicMode}
              onChange={(event) =>
                onChange({
                  publicMode: event.target.value as AgentCardMode,
                })
              }
              sx={{ gap: 1 }}
            >
              <ModeOption
                value="passthrough"
                disabled={disabled}
                label="Passthrough"
                description="Forwards the upstream's own card. Upstream changes flow through automatically."
                isDefault
              />
              <ModeOption
                value="managed"
                disabled={disabled}
                label="Managed"
                description="The platform stores and serves an authored card, edited below."
              />
            </RadioGroup>
          </Stack>

          {isPublicManaged ? null : (
            <Stack
              direction="row"
              spacing={1.5}
              alignItems="flex-start"
              sx={{
                p: 1.75,
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
              }}
            >
              <Switch
                size="small"
                checked={state.publicRewriteUrls}
                disabled={disabled}
                onChange={(event) =>
                  onChange({ publicRewriteUrls: event.target.checked })
                }
              />
              <Stack spacing={0.25}>
                <Typography variant="body2">Rewrite URLs</Typography>
                <Typography variant="caption" color="text.secondary">
                  Replaces the upstream URLs inside the card with gateway URLs,
                  so clients reach the gateway.
                </Typography>
              </Stack>
            </Stack>
          )}

          {isPublicManaged ? (
            <Stack spacing={1}>
              <Typography variant="caption" color="text.secondary">
                Card Content
              </Typography>
              <CardEditor
                value={state.publicContent}
                onChange={(next) => onChange({ publicContent: next })}
                readOnly={disabled}
              />
            </Stack>
          ) : (
            renderPassthroughCard()
          )}

            <Box>
              <Tooltip
                title={
                  isPublicManaged
                    ? 'The upstream card is shown in Passthrough mode. This card is authored here, so there is nothing to fetch.'
                    : ''
                }
              >
                <Box component="span">
                  <Button
                    variant="outlined"
                    size="small"
                    onClick={onRefetch}
                    disabled={isFetching || isPublicManaged}
                  >
                    Fetch Agent Info
                  </Button>
                </Box>
              </Tooltip>
            </Box>
        </Stack>
      </Box>

      <Divider
        orientation="vertical"
        flexItem
        sx={{ display: { xs: 'none', md: 'block' } }}
      />

      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Stack spacing={2}>
          <Stack spacing={0.5}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                Protected Card
              </Typography>
              <HelpHint title="Returned by the GetExtendedAgentCard operation rather than served on a discovery route, so it is reachable only on configured transports and carries no path or policies of its own." />
            </Stack>
            <Typography variant="body2" color="text.secondary">
              Returned by the GetExtendedAgentCard operation.
            </Typography>
          </Stack>

          <Stack spacing={1}>
            <Stack direction="row" spacing={0.75} alignItems="center">
              <Typography variant="caption" color="text.secondary">
                Mode
              </Typography>
              <HelpHint title={MODE_HELP} />
            </Stack>
            <RadioGroup
              value={state.protectedMode}
              onChange={(event) =>
                onChange({
                  protectedMode: event.target.value as AgentCardMode,
                })
              }
              sx={{ gap: 1 }}
            >
              <ModeOption
                value="passthrough"
                disabled={disabled}
                label="Passthrough"
                description="Forwards the upstream's own card. Upstream changes flow through automatically."
              />
              <ModeOption
                value="managed"
                disabled={disabled}
                label="Managed"
                description="The platform stores and serves an authored card, edited below."
              />
            </RadioGroup>
          </Stack>

          {isProtectedManaged ? null : (
            <Stack
              direction="row"
              spacing={1.5}
              alignItems="flex-start"
              sx={{
                p: 1.75,
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 1,
              }}
            >
              <Switch
                size="small"
                checked={state.protectedRewriteUrls}
                disabled={disabled}
                onChange={(event) =>
                  onChange({ protectedRewriteUrls: event.target.checked })
                }
              />
              <Stack spacing={0.25}>
                <Typography variant="body2">Rewrite URLs</Typography>
                <Typography variant="caption" color="text.secondary">
                  Replaces the upstream URLs inside the card with gateway URLs,
                  so clients reach the gateway.
                </Typography>
              </Stack>
            </Stack>
          )}

          {isProtectedManaged ? (
            <Stack spacing={1}>
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                justifyContent="space-between"
              >
                <Typography variant="caption" color="text.secondary">
                  Card Content
                </Typography>
                <Button
                  variant="outlined"
                  size="small"
                  disabled={disabled}
                  onClick={() =>
                    onChange({
                      protectedContent: isPublicManaged
                        ? state.publicContent
                        : (fetchedCard ?? ''),
                    })
                  }
                >
                  Start From Agent&apos;s Public Card
                </Button>
              </Stack>
              <CardEditor
                value={state.protectedContent}
                onChange={(next) => onChange({ protectedContent: next })}
                readOnly={disabled}
                placeholder="Paste your protected Agent Card JSON"
              />
            </Stack>
          ) : null}
        </Stack>
      </Box>
    </Box>
  );
}

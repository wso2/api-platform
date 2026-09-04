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

import { useCallback, useEffect, useMemo, useRef, useState, type FC } from 'react';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Drawer,
  IconButton,
  Typography,
} from '@wso2/oxygen-ui';
import { X } from '@wso2/oxygen-ui-icons-react';
import { readConfiguration, writeConfiguration } from '../config/api';
import {
  fieldForServerMessage,
  validateForm,
  withoutPathPrefix,
  type FieldErrors,
} from '../config/validate';
import type { AIWorkspaceHostPort } from '../hostPort';
import type { ConfigValues, Gateway, GatewayConfiguration } from '../types';
import ConfigStatusBar from './ConfigStatusBar';
import SettingField from './SettingField';
import TomlField from './TomlField';

export type GatewaySettingsDrawerProps = {
  open: boolean;
  onClose: () => void;
  gateway: Gateway | null;
  port: AIWorkspaceHostPort;
};

/**
 * The gateway's configuration form.
 *
 * Rendered entirely FROM THE RESPONSE: the platform reads its allowlist at
 * request time, so `editable[]` is the field list and `constraints[]` the
 * cross-field rules, and there is deliberately no client-side copy of either. A
 * setting the deployment adds or withdraws appears or disappears here without a
 * plugin release.
 *
 * The caller mounts this with `key={gateway.id}`, so its state belongs to one
 * gateway and cannot outlive it.
 */

const DRAWER_WIDTH = 520;

const GatewaySettingsDrawer: FC<GatewaySettingsDrawerProps> = ({
  open,
  onClose,
  gateway,
  port,
}) => {
  const { apiFetch, notify } = port;
  const gatewayId = gateway?.id;

  const [config, setConfig] = useState<GatewayConfiguration | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  /** Only the paths the user has touched. The request body is a sparse patch of exactly these. */
  const [drafts, setDrafts] = useState<ConfigValues>({});
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [serverErrors, setServerErrors] = useState<FieldErrors>({});

  /**
   * Bumped by every read AND every write, so a response can tell whether it is
   * still the newest thing in flight.
   *
   * A refresh started before a save can land after it. Both call `setConfig`,
   * so without this the older GET would replace the configuration the PUT just
   * confirmed -- leaving stale values on screen with no pending edits, which
   * reads as "saved" and is not.
   */
  const generation = useRef(0);

  /** Re-seeds `config` only. Pending edits survive, so Refresh cannot discard them. */
  const load = useCallback(
    async (id: string) => {
      const mine = ++generation.current;
      setLoading(true);
      setLoadError(null);
      try {
        const loaded = await readConfiguration(apiFetch, id);
        if (mine !== generation.current) return;
        setConfig(loaded);
      } catch (error) {
        if (mine !== generation.current) return;
        setLoadError(
          error instanceof Error
            ? error.message
            : 'The configuration could not be loaded.'
        );
      } finally {
        // Only the newest request owns the spinner; a superseded one must not
        // turn it off while its replacement is still running.
        if (mine === generation.current) setLoading(false);
      }
    },
    [apiFetch]
  );

  useEffect(() => {
    if (!open || !gatewayId) return;
    void load(gatewayId);
  }, [gatewayId, load, open]);

  /** Edits that actually differ from what is stored — editing a field back to its original un-dirties it. */
  const patch = useMemo<ConfigValues>(() => {
    if (!config) return {};
    return Object.fromEntries(
      Object.entries(drafts).filter(([path, value]) => {
        // Typing into a field the platform carries NO value for and then
        // clearing it again is not an edit. The stored value reads back
        // `undefined` while an emptied input reads `''`, so a plain `!==`
        // called that a change and left the form permanently dirty — with a
        // validation error under a field the user had just put back the way
        // they found it. There is no "unset" operation on the endpoint: an
        // empty input on an unset field means the chart default still stands,
        // which is exactly the state before the typing.
        if (!(path in config.values)) return value !== '' && value !== undefined;
        return value !== config.values[path];
      })
    );
  }, [config, drafts]);

  const clientErrors = useMemo<FieldErrors>(
    () => (config ? validateForm(config, patch) : {}),
    [config, patch]
  );
  const errors = { ...serverErrors, ...clientErrors };

  const dirtyCount = Object.keys(patch).length;
  const canSave =
    dirtyCount > 0 && Object.keys(clientErrors).length === 0 && !saving;

  const setDraft = (path: string, value: unknown) => {
    setDrafts((current) => ({ ...current, [path]: value }));
    // A server message is about the value that was sent, so it stops applying
    // the moment the field changes.
    setServerErrors(({ [path]: _sent, ...rest }) => rest);
  };

  const save = async () => {
    if (!config || !gatewayId) return;
    // Invalidates any read still in flight: what the write returns is newer
    // than anything a GET started before it can report.
    const mine = ++generation.current;
    setSaving(true);
    setSaveError(null);
    setServerErrors({});
    try {
      // The response is the WHOLE configuration after the write, in the GET's
      // shape — so it is both the confirmation and the new baseline. Re-seeding
      // from it is what makes a canonicalized quantity ("1000m" -> "1") stop
      // looking edited, and why there is no second GET here.
      const written = await writeConfiguration(apiFetch, gatewayId, patch);
      // The write happened, so it is confirmed and the edits are no longer
      // pending whatever else is in flight. Only the BASELINE is conditional:
      // a refresh started after this write owns the newer generation and its
      // response is about to arrive, so let it install the values rather than
      // fighting over them.
      if (mine === generation.current) setConfig(written);
      setDrafts({});
      notify('Configuration saved.', 'success');
    } catch (error) {
      const message =
        error instanceof Error && error.message
          ? error.message
          : 'The configuration could not be saved.';
      // A field-level message begins with the setting path it is about;
      // anything else is form-level. Either way the platform's own sentence is
      // user-presentable prose, so it is surfaced verbatim.
      const path = fieldForServerMessage(
        message,
        config.editable.map((field) => field.path)
      );
      // Under a field the path is dropped — the label already says which
      // setting this is. The banner keeps the full sentence, having no label
      // to lean on.
      if (path) setServerErrors({ [path]: withoutPathPrefix(message, path) });
      else setSaveError(message);
    } finally {
      setSaving(false);
    }
  };

  if (!gateway) return null;

  // `string` carries its own warnings and character budget, so it is
  // partitioned out of the flat list and rendered by `TomlField`.
  const listed = config?.editable.filter((field) => field.type !== 'string') ?? [];
  const freeText = config?.editable.filter((field) => field.type === 'string') ?? [];
  const currentValue = (path: string): unknown =>
    path in drafts ? drafts[path] : config?.values[path];

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Box
        sx={{
          width: DRAWER_WIDTH,
          display: 'flex',
          flexDirection: 'column',
          height: '100%',
        }}
      >
        <Box sx={{ px: 3, pt: 3 }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              mb: 0.5,
            }}
          >
            <Typography sx={{ fontSize: 16, fontWeight: 600 }}>
              Gateway Configuration
            </Typography>
            <IconButton size="small" onClick={onClose} aria-label="Close">
              <X size={18} />
            </IconButton>
          </Box>
          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              gap: 1,
              mb: 2,
              minHeight: 32,
            }}
          >
            <Typography
              color="text.secondary"
              sx={{ flex: 1, minWidth: 0 }}
              variant="body2"
            >
              {gateway.name}
            </Typography>
            {config ? (
              <ConfigStatusBar
                status={config.status}
                refreshing={loading}
                onRefresh={() => gatewayId && void load(gatewayId)}
              />
            ) : null}
          </Box>
        </Box>

        <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', px: 3 }}>
          {loading && !config ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
              <CircularProgress size={28} />
            </Box>
          ) : null}

          {loadError ? (
            <Alert
              severity="error"
              sx={{ my: 2 }}
              action={
                <Button
                  size="small"
                  onClick={() => gatewayId && void load(gatewayId)}
                >
                  Retry
                </Button>
              }
            >
              {loadError}
            </Alert>
          ) : null}

          {saveError ? (
            <Alert severity="error" sx={{ my: 2 }}>
              {saveError}
            </Alert>
          ) : null}

          {listed.map((field) => (
            <SettingField
              key={field.path}
              field={field}
              value={currentValue(field.path)}
              error={errors[field.path]}
              readOnly={saving}
              onChange={(value) => setDraft(field.path, value)}
            />
          ))}

          {freeText.map((field) => (
            <TomlField
              key={field.path}
              field={field}
              value={currentValue(field.path)}
              error={errors[field.path]}
              readOnly={saving}
              onChange={(value) => setDraft(field.path, value)}
            />
          ))}
        </Box>

        {config ? (
          <Box
            sx={{
              display: 'flex',
              justifyContent: 'flex-end',
              gap: 1,
              px: 3,
              py: 2,
              borderTop: 1,
              borderColor: 'divider',
            }}
          >
            {/* Restores the last loaded values — NOT platform defaults, which is not an operation the endpoint has. */}
            <Button
              variant="outlined"
              color="secondary"
              disabled={dirtyCount === 0 || saving}
              onClick={() => {
                setDrafts({});
                setServerErrors({});
                setSaveError(null);
              }}
            >
              Reset
            </Button>
            <Button variant="contained" disabled={!canSave} onClick={save}>
              {dirtyCount === 0
                ? 'Save'
                : `Save ${dirtyCount} ${dirtyCount === 1 ? 'change' : 'changes'}`}
            </Button>
          </Box>
        ) : null}
      </Box>
    </Drawer>
  );
};

export default GatewaySettingsDrawer;

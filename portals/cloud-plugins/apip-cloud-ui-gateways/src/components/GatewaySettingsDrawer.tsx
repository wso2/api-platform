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
  Chip,
  CircularProgress,
  Collapse,
  Divider,
  Drawer,
  Grid,
  IconButton,
  LinearProgress,
  Tooltip,
  Typography,
  alpha,
  type Theme,
} from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp, X } from '@wso2/oxygen-ui-icons-react';
import { discoverKeyManager, readConfiguration, writeConfiguration } from '../config/api';
import { sameKeyManagers } from '../config/keyManagers';
import { isChange, shownValue } from '../config/defaults';
import { isApplying } from '../config/status';
import {
  fieldForServerMessage,
  validateForm,
  withoutPathPrefix,
  type FieldErrors,
} from '../config/validate';
import type { AIWorkspaceHostPort } from '../hostPort';
import { NO_PERMISSION_TOOLTIP } from '../permissions';
import type {
  ConfigStatus,
  ConfigValues,
  EditableField,
  Gateway,
  GatewayConfiguration,
  GatewayKeyManager,
} from '../types';
import ConfigStatusBar from './ConfigStatusBar';
import KeyManagersSection from './KeyManagersSection';
import SettingField, { isTomlBlock } from './SettingField';
import SettingsSection, { SettingsCell } from './SettingsSection';
import { PROGRESS_ACCENT, secondaryTextSx } from './styles';

export type GatewaySettingsDrawerProps = {
  open: boolean;
  onClose: () => void;
  gateway: Gateway | null;
  port: AIWorkspaceHostPort;
  /**
   * Whether the user holds a scope that writes the configuration. When not, the
   * drawer still reads and polls — reading is its own scope — but every field is
   * read-only and Save says why.
   */
  canWrite: boolean;
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

const DRAWER_WIDTH = 640;

/** The trusted key managers belong to jwt-auth, so they sit in the section of its settings. */
const JWT_PATH_PREFIX = 'gateway.config.policy_configurations.jwtauth_v1.';

/** The phase as a tinted pill with a dot, beside the gateway's name. */
const PHASE_CHIP: Partial<Record<string, { label: string; color: (theme: Theme) => string }>> = {
  healthy: { label: 'Running', color: (theme) => theme.palette.success.main },
  applying: { label: 'Applying', color: () => PROGRESS_ACCENT },
  failed: { label: 'Failed', color: (theme) => theme.palette.error.main },
};

const PhaseChip: FC<{ status: ConfigStatus }> = ({ status }) => {
  const chip = PHASE_CHIP[status.phase];
  if (!chip) return null;
  return (
    <Chip
      size="small"
      label={chip.label}
      sx={(theme) => ({
        bgcolor: alpha(chip.color(theme), 0.16),
        color: 'text.primary',
        fontWeight: 500,
        pl: 0.5,
      })}
      icon={
        <Box
          sx={(theme) => ({
            width: 7,
            height: 7,
            borderRadius: '50%',
            bgcolor: chip.color(theme),
            ml: '8px !important',
          })}
        />
      }
    />
  );
};

/**
 * How often the drawer re-reads the configuration while it is open.
 *
 * The phase moves on the platform's clock, not the user's: a write comes back
 * `applying` and the data plane takes minutes to catch up (10m07s measured on
 * 2026-08-31), so nothing the user does will ever be the event that turns the
 * line to `healthy`. A read is cheap and idempotent, and it re-seeds `config`
 * only — pending edits are untouched — so the cost of polling is one GET and
 * the benefit is a status line that is true without anyone pressing anything.
 *
 * The PUT is never re-sent. Only the phase is being waited on, and it settles on
 * its own.
 */
const POLL_INTERVAL_MS = 20_000;

const GatewaySettingsDrawer: FC<GatewaySettingsDrawerProps> = ({
  open,
  onClose,
  gateway,
  port,
  canWrite,
}) => {
  const { apiFetch, notify } = port;
  const gatewayId = gateway?.id;

  const [config, setConfig] = useState<GatewayConfiguration | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  /** Whether the JWT section shows its settings below the key managers. */
  const [jwtAdvancedOpen, setJwtAdvancedOpen] = useState(false);
  /** Only the paths the user has touched. The request body is a sparse patch of exactly these. */
  const [drafts, setDrafts] = useState<ConfigValues>({});
  const [saving, setSaving] = useState(false);
  /**
   * The same flag the poll timer reads. `saving` itself cannot be: the interval
   * callback closes over the render that created it, so it would see `false`
   * forever unless the timer were torn down and rebuilt on every keystroke-
   * adjacent state change.
   */
  const savingRef = useRef(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [serverErrors, setServerErrors] = useState<FieldErrors>({});
  /** The user's edited key manager list, `null` until edited; the saved one comes with `config`. */
  const [keyManagerDraft, setKeyManagerDraft] = useState<GatewayKeyManager[] | null>(null);
  /**
   * The saved list the draft was started from. The draft replaces the whole
   * list, so if a refresh brings in a different one, saving would undo
   * someone else's change.
   */
  const [keyManagerBase, setKeyManagerBase] = useState<GatewayKeyManager[] | null>(null);

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

  /**
   * Re-seeds `config` only. Pending edits survive, so neither Refresh nor the
   * poll below can discard them.
   *
   * `background: true` is the poll's read: it owns no spinner, and a failure
   * leaves the form exactly as it is. A poll that lost the network must not
   * replace a working form with an error banner every twenty seconds — the
   * Refresh button is the path that reports a read failure, because someone
   * pressed it and is waiting for an answer.
   */
  const load = useCallback(
    async (id: string, { background = false }: { background?: boolean } = {}) => {
      const mine = ++generation.current;
      if (!background) {
        setLoading(true);
        setLoadError(null);
      }
      try {
        const loaded = await readConfiguration(apiFetch, id);
        if (mine !== generation.current) return;
        setConfig(loaded);
        // A read that worked settles any banner an earlier one left behind.
        setLoadError(null);
      } catch (error) {
        if (mine !== generation.current || background) return;
        setLoadError(
          error instanceof Error
            ? error.message
            : 'The configuration could not be loaded.'
        );
      } finally {
        // Only the newest request owns the spinner; a superseded one must not
        // turn it off while its replacement is still running.
        if (!background && mine === generation.current) setLoading(false);
      }
    },
    [apiFetch]
  );

  useEffect(() => {
    if (!open || !gatewayId) return;
    void load(gatewayId);
  }, [gatewayId, load, open]);

  const keyManagers = config?.keyManagers ?? null;

  /**
   * Poll while the drawer is open, so `Applying…` becomes a timestamp on its own.
   *
   * A tick is SKIPPED while a write is in flight. The generation counter settles
   * which response wins by which started last, and a read that started before
   * the PUT returned can legitimately own the newer generation while carrying
   * pre-write values — the one ordering that would leave stale values on screen
   * with no pending edits, which reads as "saved" and is not. Once the PUT has
   * returned, the binding is written and any read reflects it.
   */
  useEffect(() => {
    if (!open || !gatewayId) return;
    const timer = setInterval(() => {
      if (savingRef.current) return;
      void load(gatewayId, { background: true });
    }, POLL_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [gatewayId, load, open]);

  /**
   * Edits that actually differ from what is stored — editing a field back to its
   * original un-dirties it. On an unset field, clearing the input or picking the
   * default is not an edit either (see `isChange`).
   */
  const patch = useMemo<ConfigValues>(() => {
    if (!config) return {};
    const fields = new Map(config.editable.map((field) => [field.path, field]));
    return Object.fromEntries(
      Object.entries(drafts).filter(([path, value]) => {
        const field = fields.get(path);
        return field ? isChange(field, config.values, value) : false;
      })
    );
  }, [config, drafts]);

  const clientErrors = useMemo<FieldErrors>(
    () => (config ? validateForm(config, patch) : {}),
    [config, patch]
  );
  const errors = { ...serverErrors, ...clientErrors };

  /** The edited list, when it differs from what the gateway trusts. */
  const keyManagerChange =
    keyManagerDraft && keyManagers && !sameKeyManagers(keyManagerDraft, keyManagers)
      ? keyManagerDraft
      : null;
  const dirtyCount = Object.keys(patch).length + (keyManagerChange ? 1 : 0);
  /**
   * The previous change has not reached the data plane yet. Writing again on top
   * of it re-renders the release from a document the gateway has not finished
   * picking up, so Save waits — and this is the state EVERY write lands in, for
   * minutes, which is why the button explains itself rather than just greying out.
   */
  const applying = isApplying(config?.status);

  /**
   * Set when this drawer watched a change land: the phase went from `applying` to
   * `healthy` while it was open. Not on open, since a gateway that was already
   * healthy has nothing new to report.
   */
  const [applied, setApplied] = useState(false);
  const previousPhase = useRef<string | undefined>(undefined);
  const phase = config?.status.phase;
  useEffect(() => {
    if (phase === 'applying') setApplied(false);
    else if (phase === 'healthy' && previousPhase.current === 'applying') setApplied(true);
    previousPhase.current = phase;
  }, [phase]);
  /**
   * The organization may not write at all. Read directly rather than through
   * `canCreate('gateways')`: that answers whether there is room for one MORE
   * gateway, and a full quota must still leave an existing one configurable.
   * `false` when the host supplies no entitlements, which is every deployment
   * without a supplier mounted.
   */
  const readOnly = port.resourceLimits?.readOnly ?? false;
  const canSave =
    canWrite &&
    dirtyCount > 0 &&
    Object.keys(clientErrors).length === 0 &&
    !saving &&
    !applying &&
    !readOnly;

  const setDraft = (path: string, value: unknown) => {
    setDrafts((current) => ({ ...current, [path]: value }));
    // A server message is about the value that was sent, so it stops applying
    // the moment the field changes.
    setServerErrors(({ [path]: _sent, ...rest }) => rest);
  };

  /**
   * Both user-initiated reads. Refused while a write is in flight: such a read
   * carries pre-write values and yet owns the NEWER generation, so the PUT's
   * response loses the check below while the drafts clear anyway -- the form
   * then shows the pre-write configuration with nothing pending, which reads as
   * "saved" and is not. The poll skips its tick for the same reason.
   */
  const refresh = () => {
    if (savingRef.current || !gatewayId) return;
    void load(gatewayId);
  };

  const save = async () => {
    if (!config || !gatewayId) return;
    if (
      keyManagerChange &&
      keyManagerBase &&
      !sameKeyManagers(keyManagerBase, config.keyManagers ?? [])
    ) {
      setSaveError(
        'The key managers were changed elsewhere since you started editing them. Discard your changes and make them again.'
      );
      return;
    }
    // Invalidates any read still in flight: what the write returns is newer
    // than anything a GET started before it can report.
    const mine = ++generation.current;
    savingRef.current = true;
    setSaving(true);
    setSaveError(null);
    setServerErrors({});
    try {
      // The response is the WHOLE configuration after the write, in the GET's
      // shape — so it is both the confirmation and the new baseline. Re-seeding
      // from it is what makes a canonicalized quantity ("1000m" -> "1") stop
      // looking edited, and why there is no second GET here. Settings and key
      // managers go in one request, so the save is applied whole or not at all.
      const written = await writeConfiguration(
        apiFetch,
        gatewayId,
        patch,
        keyManagerChange ?? undefined
      );
      // The write happened, so it is confirmed and the edits are no longer
      // pending whatever else is in flight. Only the BASELINE is conditional:
      // a refresh started after this write owns the newer generation and its
      // response is about to arrive, so let it install the values rather than
      // fighting over them.
      if (mine === generation.current) setConfig(written);
      setDrafts({});
      setKeyManagerDraft(null);
      setKeyManagerBase(null);
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
      savingRef.current = false;
      setSaving(false);
    }
  };

  if (!gateway) return null;

  // The platform lists a group's fields together and in the order to show
  // them, so each run of one group is a section under its heading.
  const sections: { group?: string; fields: EditableField[] }[] = [];
  for (const field of config?.editable.filter((f) => !isTomlBlock(f)) ?? []) {
    const last = sections[sections.length - 1];
    if (last && last.group === field.group) last.fields.push(field);
    else sections.push({ group: field.group, fields: [field] });
  }
  const currentValue = (field: EditableField): unknown =>
    config ? shownValue(field, config.values, drafts) : undefined;

  const locked = saving || !canWrite || applying;
  const hasJwtSection = sections.some((section) =>
    section.fields.some((field) => field.path.startsWith(JWT_PATH_PREFIX))
  );
  const descriptionOf = (group?: string) =>
    config?.groups?.find((candidate) => candidate.name === group)?.description;
  const keyManagerField = config ? (
    <SettingsCell wide>
      <KeyManagersSection
        keyManagers={keyManagerDraft ?? keyManagers}
        saved={keyManagers ?? []}
        readOnly={locked}
        modified={keyManagerChange !== null}
        discover={(issuer, wellKnownUrl) =>
          discoverKeyManager(apiFetch, config.id, issuer, wellKnownUrl)
        }
        onChange={(next) => {
          if (keyManagerDraft === null) setKeyManagerBase(keyManagers);
          setKeyManagerDraft(next);
        }}
      />
    </SettingsCell>
  ) : null;

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Box
        sx={{
          width: DRAWER_WIDTH,
          maxWidth: '100vw',
          display: 'flex',
          flexDirection: 'column',
          height: '100%',
        }}
      >
        <Box sx={{ px: 3, pt: 3, pb: 2, borderBottom: 1, borderColor: 'divider' }}>
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              mb: 1.5,
            }}
          >
            <Typography component="h2" variant="h5" sx={{ fontWeight: 600 }}>
              Gateway configuration
            </Typography>
            <IconButton size="small" onClick={onClose} aria-label="Close">
              <X size={18} />
            </IconButton>
          </Box>
          <Box sx={{ alignItems: 'center', display: 'flex', gap: 1.5, minHeight: 32 }}>
            <Typography variant="body1" sx={{ fontWeight: 500 }}>
              {gateway.name}
            </Typography>
            {config ? <PhaseChip status={config.status} /> : null}
            <Box sx={{ flex: 1 }} />
            {config ? (
              <ConfigStatusBar
                status={config.status}
                refreshing={loading || saving}
                onRefresh={refresh}
              />
            ) : null}
          </Box>
        </Box>

        <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', px: 3 }}>
          {loading && !config ? (
            <Box sx={{ alignItems: 'center', display: 'flex', justifyContent: 'center', py: 6, width: '100%' }}>
              <CircularProgress size={28} />
            </Box>
          ) : null}

          {loadError ? (
            <Alert
              severity="error"
              sx={{ mt: 3 }}
              action={
                <Button
                  size="small"
                  disabled={saving}
                  onClick={refresh}
                >
                  Retry
                </Button>
              }
            >
              {loadError}
            </Alert>
          ) : null}

          {saveError ? (
            <Alert severity="error" sx={{ mt: 3 }}>
              {saveError}
            </Alert>
          ) : null}

          {config && !canWrite ? (
            <Alert severity="info" sx={{ mt: 3 }}>
              You can view this configuration, but you don&apos;t have permission to change it.
            </Alert>
          ) : null}

          {/* No percentage or steps: the platform reports only that the change has
              not landed yet, and a progress bar that guessed would be wrong for
              minutes at a time. */}
          {applying ? (
            <Box
              role="status"
              sx={{
                mt: 3,
                p: 2,
                border: 1,
                borderColor: alpha(PROGRESS_ACCENT, 0.45),
                bgcolor: alpha(PROGRESS_ACCENT, 0.08),
                borderRadius: 2,
              }}
            >
              <Box sx={{ alignItems: 'flex-start', display: 'flex', gap: 1.5, mb: 2 }}>
                <CircularProgress size={18} sx={{ color: PROGRESS_ACCENT, mt: 0.25 }} />
                <Box>
                  <Typography variant="body1" sx={{ fontWeight: 500 }}>
                    Applying changes
                  </Typography>
                  <Typography variant="body2" sx={secondaryTextSx}>
                    Settings are locked until the gateway is running the new
                    configuration. This can take a few minutes.
                  </Typography>
                </Box>
              </Box>
              <LinearProgress
                sx={{
                  bgcolor: alpha(PROGRESS_ACCENT, 0.2),
                  borderRadius: 1,
                  '& .MuiLinearProgress-bar': { bgcolor: PROGRESS_ACCENT },
                }}
              />
            </Box>
          ) : null}

          {applied && !applying ? (
            <Alert
              severity="success"
              variant="outlined"
              sx={{ mt: 3 }}
              action={
                <Button color="inherit" size="small" onClick={() => setApplied(false)}>
                  Dismiss
                </Button>
              }
            >
              Changes applied. The gateway is running the new configuration.
            </Alert>
          ) : null}

          {sections.map((section, index) => {
            const isJwt = section.fields.some((field) => field.path.startsWith(JWT_PATH_PREFIX));
            const cells = section.fields.map((field) => (
              <SettingsCell key={field.path}>
                <SettingField
                  field={field}
                  value={currentValue(field)}
                  error={errors[field.path]}
                  modified={field.path in patch}
                  readOnly={locked}
                  onChange={(value) => setDraft(field.path, value)}
                />
              </SettingsCell>
            ));
            // The key managers lead the JWT section; its settings wait under Advanced
            // settings, opened for the user when one of them is in error or unsaved.
            const advancedOpen =
              jwtAdvancedOpen ||
              section.fields.some((field) => Boolean(errors[field.path]) || field.path in patch);
            return (
              <Box key={`${section.group ?? ''}-${index}`}>
                {index > 0 ? <Divider /> : null}
                <SettingsSection title={section.group} description={descriptionOf(section.group)}>
                  {isJwt ? (
                    <>
                      {keyManagerField}
                      <SettingsCell wide>
                        <Button
                          size="small"
                          onClick={() => setJwtAdvancedOpen(!advancedOpen)}
                          endIcon={advancedOpen ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
                          aria-expanded={advancedOpen}
                          sx={{ px: 0 }}
                        >
                          Advanced settings
                        </Button>
                        <Collapse in={advancedOpen}>
                          <Grid container columnSpacing={2} rowSpacing={2.5} sx={{ pt: 1.5 }}>
                            {cells}
                          </Grid>
                        </Collapse>
                      </SettingsCell>
                    </>
                  ) : (
                    cells
                  )}
                </SettingsSection>
              </Box>
            );
          })}

          {/* A platform that offers no JWT settings still lists what the gateway trusts. */}
          {config && !hasJwtSection && keyManagerField ? (
            <Box>
              {sections.length > 0 ? <Divider /> : null}
              <SettingsSection title="Key managers">{keyManagerField}</SettingsSection>
            </Box>
          ) : null}
        </Box>

        {config ? (
          <Box
            sx={{
              alignItems: 'center',
              display: 'flex',
              gap: 1,
              px: 3,
              py: 2,
              borderTop: 1,
              borderColor: 'divider',
            }}
          >
            <Typography
              variant="body2"
              sx={{ ...(dirtyCount > 0 ? { color: 'primary.main' } : secondaryTextSx), flex: 1 }}
            >
              {dirtyCount === 0
                ? 'No changes'
                : `${dirtyCount} unsaved ${dirtyCount === 1 ? 'change' : 'changes'}`}
            </Typography>
            {/* Restores the last loaded values — NOT platform defaults, which is not an operation the endpoint has. */}
            <Button
              variant="outlined"
              color="secondary"
              disabled={dirtyCount === 0 || saving}
              onClick={() => {
                setDrafts({});
                setKeyManagerDraft(null);
                setKeyManagerBase(null);
                setServerErrors({});
                setSaveError(null);
              }}
            >
              Discard
            </Button>
            <Tooltip
              title={
                !canWrite
                  ? NO_PERMISSION_TOOLTIP
                  : readOnly
                    ? (port.resourceLimits?.readOnlyMessage ?? '')
                    : applying
                      ? 'The previous change is still being applied. Saving is available once it has landed.'
                      : ''
              }
            >
              {/* Wrapped: a disabled button fires no events, so the tooltip
                  explaining why it is disabled would never open. */}
              <Box component="span">
                <Button variant="contained" disabled={!canSave} onClick={save}>
                  Save and apply
                </Button>
              </Box>
            </Tooltip>
          </Box>
        ) : null}
      </Box>
    </Drawer>
  );
};

export default GatewaySettingsDrawer;

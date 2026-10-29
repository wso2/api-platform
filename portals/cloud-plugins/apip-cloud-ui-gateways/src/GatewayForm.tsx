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

import { useEffect, useState, type FC } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogContent,
  FormControl,
  FormControlLabel,
  FormLabel,
  Grid,
  IconButton,
  Link,
  MenuItem,
  PageContent,
  PageTitle,
  Select,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ArrowLeftRight, ChevronLeft, Cloud, Server, X } from '@wso2/oxygen-ui-icons-react';
import EnvironmentSelect from './components/EnvironmentSelect';
import GatewayTypeSelector from './components/GatewayTypeSelector';
import HostingCompareTable from './components/HostingCompareTable';
import HostingOptionCard from './components/HostingOptionCard';
import { gatewayHandleFromName, validateGatewayName } from './utils/name';
import { gatewayTypeLabel } from './utils/gateway';
import type { GatewayVersionEntry } from './hostPort';
import type { Environment, Gateway, GatewayHosting, GatewayInput, GatewayType, SelfHostedGatewayInput } from './types';

export type GatewayFormProps = {
  /** Editing an existing gateway pre-fills the form and changes the page's labels. Omit (or 'create') for a blank gateway. */
  mode?: 'create' | 'edit';
  /** Required when `mode` is 'edit' — the gateway to load and update. */
  gateway?: Gateway;
  /** The gateway types this host offers. A host with only one type gets no picker. */
  types: GatewayType[];
  environments: Environment[];
  /**
   * Every gateway already visible to this host, managed and self-hosted alike.
   * Used only to tell whether the one being created would be the first of its
   * kind — of its hosting, type AND environment — and so that bucket's default.
   */
  gateways: Gateway[];
  /** The self-hosted "Gateway Version" picker's choices, per gateway type. */
  gatewayVersions: GatewayVersionEntry[];
  /** Preselects the Hosting option when creating, e.g. from the empty state's buttons. */
  initialHosting?: GatewayHosting | null;
  onBack: () => void;
  /** Returning a promise lets the form keep its submit button busy until the save settles. Unchanged: still the `/managed-gateways` path. */
  onSubmit: (input: GatewayInput) => void | Promise<void>;
  /** The self-hosted path — a different call entirely (`/gateways`, see `selfHostedGatewaysApi`), so it gets its own callback rather than overloading `onSubmit`. */
  onSubmitSelfHosted: (input: SelfHostedGatewayInput) => void | Promise<void>;
};

/** Shown before a name is typed, so the naming rule is known up front. */
const NAME_HELPER_TEXT = 'The gateway handle is derived from this name and cannot be changed later.';

/** A plausible local endpoint, shown pre-filled so the field reads as "edit this" rather than "write one from scratch". */
const SAMPLE_SELF_HOSTED_URL = 'https://localhost:8443';

const HOSTING_OPTIONS: {
  value: GatewayHosting;
  icon: FC<{ size?: number }>;
  title: string;
  description: string;
}[] = [
  {
    value: 'managed',
    icon: Cloud,
    title: 'WSO2 managed (cloud)',
    description: 'WSO2 runs, scales, and upgrades the gateway for you. No infrastructure to maintain.',
  },
  {
    value: 'self-hosted',
    icon: Server,
    title: 'Self-hosted',
    description: 'Run the gateway on your own infrastructure and connect it to this control plane.',
  },
];

const GatewayForm: FC<GatewayFormProps> = ({
  mode = 'create',
  gateway,
  types,
  environments,
  gateways,
  gatewayVersions,
  initialHosting = null,
  onBack,
  onSubmit,
  onSubmitSelfHosted,
}) => {
  const isEdit = mode === 'edit' && !!gateway;

  // Hosting is fixed at creation, same as type and environment — an edit has
  // nothing to choose, so the picker and its comparison table are left out
  // entirely rather than shown read-only; the fields below just reflect the
  // gateway's own hosting from the start.
  const [hosting, setHosting] = useState<GatewayHosting | null>(
    isEdit ? gateway!.hosting : initialHosting
  );
  const [showCompare, setShowCompare] = useState(false);
  const isSelfHosted = hosting === 'self-hosted';

  // A host that offers a single type has nothing to pick, so the field is left
  // out entirely and that one type is used.
  const showTypeField = types.length > 1;

  const [type, setType] = useState<GatewayType>(gateway?.type ?? types[0]);
  // Marking is one-way: a default is handed over by marking another gateway, so
  // an existing default's switch stays on and disabled rather than offering an
  // "unset" that would leave the environment without one.
  const [isDefault, setIsDefault] = useState(gateway?.isDefault ?? false);
  const [name, setName] = useState(gateway?.name ?? '');
  const [description, setDescription] = useState(gateway?.description ?? '');
  const [environmentId, setEnvironmentId] = useState(gateway?.environmentId ?? '');

  // Self-hosted-only: which release to declare (there is no provisioning step
  // to discover it from, unlike managed) and the endpoint it will answer on.
  const typeVersions = gatewayVersions.filter((entry) => entry.type === type);
  const [version, setVersion] = useState(typeVersions[0]?.version ?? '');
  const [url, setUrl] = useState(gateway?.url || SAMPLE_SELF_HOSTED_URL);

  // The name is only constrained on create, where the backend derives the
  // gateway's handle from it. It is a plain display name from then on — the
  // handle is immutable — so an edit must not be blocked by rules that no longer
  // apply to what it changes.
  const nameError = isEdit ? undefined : validateGatewayName(name, environmentId);

  // The handle is what the gateway is addressed by and cannot be changed later,
  // so show what the name will become instead of leaving the user to guess.
  const derivedHandle = isEdit ? '' : gatewayHandleFromName(name);
  const nameHelperText =
    nameError ??
    (derivedHandle ? `Handle: ${derivedHandle}` : isEdit ? undefined : NAME_HELPER_TEXT);

  // "First of its kind" is scoped to the chosen hosting too: a managed and a
  // self-hosted gateway of the same type and environment are tracked by two
  // entirely separate mechanisms (the binding table vs. a client-side
  // property marker — see `selfHostedGatewaysApi`), so one becoming the
  // default says nothing about the other.
  const isFirstOfType =
    !isEdit &&
    hosting !== null &&
    environmentId.length > 0 &&
    !gateways.some(
      (candidate) =>
        candidate.hosting === hosting &&
        candidate.environmentId === environmentId &&
        candidate.type === type
    );

  // Re-derived whenever the chosen hosting, environment or type changes, so the
  // switch always describes the current selection. A manual toggle afterwards
  // sticks: `isFirstOfType` does not change when the switch does, so this does
  // not fire and undo it.
  useEffect(() => {
    if (isEdit) return;
    setIsDefault(isFirstOfType);
  }, [isEdit, isFirstOfType]);

  // Switching type (on a host that offers more than one) changes which
  // release catalog applies — re-pin to that type's own default rather than
  // leaving a stale version from the previous type selected.
  useEffect(() => {
    if (isEdit) return;
    setVersion(typeVersions[0]?.version ?? '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isEdit, type]);

  const [submitting, setSubmitting] = useState(false);

  const missingRequired =
    hosting === null || name.trim().length === 0 || environmentId.length === 0;
  const canSubmit = !missingRequired && !nameError && !submitting;

  const handleSubmit = async () => {
    // Provisioning a gateway takes seconds, so the button has to show the work is
    // under way — and a second click must not issue a second create.
    if (submitting || hosting === null) return;
    setSubmitting(true);
    try {
      const input = {
        name: name.trim(),
        description: description.trim() || undefined,
        type,
        environmentId,
        isDefault,
      };
      if (hosting === 'self-hosted') {
        await onSubmitSelfHosted({ ...input, url: url.trim() || undefined, version: version || undefined });
      } else {
        await onSubmit(input);
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Button size="small" startIcon={<ChevronLeft size={18} />} onClick={onBack}>
        Back to list
      </Button>

      <Stack spacing={2} mt={2}>
        <PageTitle>
          <PageTitle.Header>{isEdit ? 'Edit Gateway' : 'Add Gateway'}</PageTitle.Header>
        </PageTitle>
      </Stack>

      <Box sx={{ mt: 2, maxWidth: 820 }}>
        <Grid container spacing={2}>
          {!isEdit ? (
            <Grid size={{ xs: 12 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
                <FormLabel required>Hosting</FormLabel>
                <Link
                  component="button"
                  type="button"
                  underline="hover"
                  onClick={() => setShowCompare(true)}
                  sx={{ display: 'flex', alignItems: 'center', gap: 0.5, fontSize: '0.8125rem' }}
                >
                  <ArrowLeftRight size={14} />
                  Compare options
                </Link>
              </Box>
              <Box role="radiogroup" aria-label="Hosting" sx={{ display: 'flex', gap: 2 }}>
                {HOSTING_OPTIONS.map((option) => (
                  <HostingOptionCard
                    key={option.value}
                    icon={<option.icon size={20} />}
                    title={option.title}
                    description={option.description}
                    selected={hosting === option.value}
                    onClick={() => setHosting(option.value)}
                  />
                ))}
              </Box>

              <Dialog open={showCompare} onClose={() => setShowCompare(false)} maxWidth="md" fullWidth>
                <DialogContent sx={{ p: 4 }}>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 3 }}>
                    <Box>
                      <Typography variant="h5" sx={{ fontWeight: 700 }}>
                        Compare hosting options
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        See how WSO2 managed and self-hosted gateways differ.
                      </Typography>
                    </Box>
                    <IconButton onClick={() => setShowCompare(false)} aria-label="Close">
                      <X size={20} />
                    </IconButton>
                  </Box>
                  <HostingCompareTable
                    onChoose={(value) => {
                      setHosting(value);
                      setShowCompare(false);
                    }}
                  />
                </DialogContent>
              </Dialog>
            </Grid>
          ) : null}

          {hosting !== null ? (
            <>
              {showTypeField ? (
                <Grid size={{ xs: 12 }}>
                  <FormControl fullWidth>
                    <FormLabel required>Gateway Type</FormLabel>
                    <GatewayTypeSelector types={types} value={type} onChange={setType} readOnly={isEdit} />
                  </FormControl>
                </Grid>
              ) : null}

              <Grid size={{ xs: 12 }}>
                <FormControl fullWidth>
                  <FormLabel required>Name</FormLabel>
                  <TextField
                    fullWidth
                    required
                    placeholder="Enter gateway name"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    error={Boolean(nameError)}
                    helperText={nameHelperText}
                    autoFocus
                  />
                </FormControl>
              </Grid>

              <Grid size={{ xs: 12 }}>
                <FormControl fullWidth>
                  <FormLabel>Description (Optional)</FormLabel>
                  <TextField
                    fullWidth
                    multiline
                    minRows={3}
                    placeholder="Enter description"
                    value={description}
                    onChange={(event) => setDescription(event.target.value)}
                  />
                </FormControl>
              </Grid>

              <Grid size={{ xs: 12 }}>
                <FormControl fullWidth>
                  <FormLabel required>Environment</FormLabel>
                  {/* The environment is fixed at creation — a gateway lives in exactly one. */}
                  <EnvironmentSelect
                    environments={environments}
                    value={environmentId}
                    onChange={setEnvironmentId}
                    disabled={isEdit}
                  />
                </FormControl>
              </Grid>

              {isSelfHosted && !isEdit ? (
                <>
                  {typeVersions.length > 0 ? (
                    <Grid size={{ xs: 12 }}>
                      <FormControl fullWidth>
                        <FormLabel required>Gateway Version</FormLabel>
                        <Select
                          value={version}
                          onChange={(event) => setVersion(event.target.value)}
                          renderValue={(value) => {
                            const isLatest = value === typeVersions[0]?.version;
                            return (
                              <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                                <span>{gatewayTypeLabel(type).replace(' ', '-')} v{value}</span>
                                {isLatest ? (
                                  <Chip label="Latest" size="small" color="success" sx={{ ml: 1 }} />
                                ) : null}
                              </Box>
                            );
                          }}
                        >
                          {typeVersions.map((entry, index) => (
                            <MenuItem key={entry.version} value={entry.version}>
                              {gatewayTypeLabel(type).replace(' ', '-')} v{entry.version}
                              {index === 0 ? ' (Latest)' : ''}
                            </MenuItem>
                          ))}
                        </Select>
                      </FormControl>
                    </Grid>
                  ) : null}

                  <Grid size={{ xs: 12 }}>
                    <FormControl fullWidth>
                      <FormLabel required>URL</FormLabel>
                      <TextField
                        fullWidth
                        required
                        placeholder={SAMPLE_SELF_HOSTED_URL}
                        value={url}
                        onChange={(event) => setUrl(event.target.value)}
                        helperText="The endpoint this gateway will answer on. Change it to match your own infrastructure."
                      />
                    </FormControl>
                  </Grid>

                  <Grid size={{ xs: 12 }}>
                    <Alert severity="info">
                      After you add the gateway, you get a registration token and setup commands to run it
                      on your infrastructure. The gateway stays inactive until it connects to the control
                      plane.
                    </Alert>
                  </Grid>
                </>
              ) : null}

              <Grid size={{ xs: 12 }}>
                <FormControlLabel
                  control={
                    <Switch
                      checked={isDefault}
                      // Locked in three cases: nothing to be the default of until an
                      // environment is chosen; an existing default cannot be unset
                      // here (it is handed over by marking another gateway); and the
                      // first gateway of its kind BECOMES the default whatever this
                      // says, so offering to untick it would be a lie.
                      disabled={
                        (gateway?.isDefault ?? false) || environmentId.length === 0 || isFirstOfType
                      }
                      onChange={(event) => setIsDefault(event.target.checked)}
                    />
                  }
                  label="Default gateway for this environment"
                />
                {isSelfHosted ? (
                  <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: -1 }}>
                    Self-hosted gateways track their own default separately from WSO2-managed ones.
                  </Typography>
                ) : null}
              </Grid>
            </>
          ) : null}
        </Grid>

        <Box sx={{ mt: 3, display: 'flex', gap: 1 }}>
          <Button variant="outlined" color="secondary" disabled={submitting} onClick={onBack}>
            Cancel
          </Button>
          {/* An invalid name explains itself in the field's helper text; a missing
              hosting or other required field only needs the generic nudge. */}
          <Tooltip title={missingRequired ? 'Fill in the required fields to continue.' : ''}>
            <span>
              <Button
                variant="contained"
                disabled={!canSubmit}
                onClick={handleSubmit}
                startIcon={submitting ? <CircularProgress size={16} color="inherit" /> : undefined}
              >
                {submitting ? (isEdit ? 'Saving…' : 'Adding…') : isEdit ? 'Save Changes' : 'Add Gateway'}
              </Button>
            </span>
          </Tooltip>
        </Box>
      </Box>
    </PageContent>
  );
};

export default GatewayForm;

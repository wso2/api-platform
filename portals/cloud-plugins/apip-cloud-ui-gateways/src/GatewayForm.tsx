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
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormControlLabel,
  FormLabel,
  Grid,
  PageContent,
  PageTitle,
  Stack,
  Switch,
  TextField,
  Tooltip,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';
import EnvironmentSelect from './components/EnvironmentSelect';
import GatewayTypeSelector from './components/GatewayTypeSelector';
import { gatewayHandleFromName, validateGatewayName } from './utils/name';
import type { Environment, Gateway, GatewayInput, GatewayType } from './types';

export type GatewayFormProps = {
  /** Editing an existing gateway pre-fills the form and changes the page's labels. Omit (or 'create') for a blank gateway. */
  mode?: 'create' | 'edit';
  /** Required when `mode` is 'edit' — the gateway to load and update. */
  gateway?: Gateway;
  /** The gateway types this host offers. A host with only one type gets no picker. */
  types: GatewayType[];
  environments: Environment[];
  /**
   * The gateways already in this host's view. Used only to tell whether the one
   * being created would be the first of its type in the chosen environment, and
   * so the environment's default.
   */
  gateways: Gateway[];
  onBack: () => void;
  /** Returning a promise lets the form keep its submit button busy until the save settles. */
  onSubmit: (input: GatewayInput) => void | Promise<void>;
};

/** Shown before a name is typed, so the naming rule is known up front. */
const NAME_HELPER_TEXT = 'The gateway handle is derived from this name and cannot be changed later.';

const GatewayForm: FC<GatewayFormProps> = ({
  mode = 'create',
  gateway,
  types,
  environments,
  gateways,
  onBack,
  onSubmit,
}) => {
  const isEdit = mode === 'edit' && !!gateway;

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

  // The backend makes the first gateway of a type in an environment its default
  // whether or not it was asked to, so the switch shows that outcome rather than
  // letting the form claim otherwise. It needs an environment to be true of:
  // before one is chosen there is nothing to be the first of.
  const isFirstOfType =
    !isEdit &&
    environmentId.length > 0 &&
    !gateways.some(
      (candidate) => candidate.environmentId === environmentId && candidate.type === type
    );

  // Re-derived whenever the chosen environment or type changes, so the switch
  // always describes the current selection. A manual toggle afterwards sticks:
  // `isFirstOfType` does not change when the switch does, so this does not fire
  // and undo it.
  useEffect(() => {
    if (isEdit) return;
    setIsDefault(isFirstOfType);
  }, [isEdit, isFirstOfType]);

  const [submitting, setSubmitting] = useState(false);

  const missingRequired = name.trim().length === 0 || environmentId.length === 0;
  const canSubmit = !missingRequired && !nameError && !submitting;

  const handleSubmit = async () => {
    // Provisioning a gateway takes seconds, so the button has to show the work is
    // under way — and a second click must not issue a second create.
    if (submitting) return;
    setSubmitting(true);
    try {
      await onSubmit({
        name: name.trim(),
        description: description.trim() || undefined,
        type,
        environmentId,
        isDefault,
      });
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
              {/* The environment is fixed at creation — a managed gateway lives in exactly one. */}
              <EnvironmentSelect
                environments={environments}
                value={environmentId}
                onChange={setEnvironmentId}
                disabled={isEdit}
              />
            </FormControl>
          </Grid>

          <Grid size={{ xs: 12 }}>
            <FormControlLabel
              control={
                <Switch
                  checked={isDefault}
                  // Locked in three cases: nothing to be the default of until an
                  // environment is chosen; an existing default cannot be unset
                  // here (it is handed over by marking another gateway); and the
                  // first gateway of its type BECOMES the default whatever this
                  // says, so offering to untick it would be a lie.
                  disabled={
                    (gateway?.isDefault ?? false) || environmentId.length === 0 || isFirstOfType
                  }
                  onChange={(event) => setIsDefault(event.target.checked)}
                />
              }
              label="Default gateway for this environment"
            />
          </Grid>
        </Grid>

        <Box sx={{ mt: 3, display: 'flex', gap: 1 }}>
          <Button variant="outlined" color="secondary" disabled={submitting} onClick={onBack}>
            Cancel
          </Button>
          {/* An invalid name explains itself in the field's helper text, so the
              tooltip only covers the still-empty case. */}
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

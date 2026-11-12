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

import { useEffect, useRef, useState, type FC } from 'react';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormHelperText,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {
  normalizeKeyManager,
  sameKeyManagers,
  validateKeyManager,
  wellKnownUrlError,
  wellKnownUrlOf,
  type KeyManagerErrors,
} from '../config/keyManagers';
import type { GatewayKeyManager, KeyManagerDiscovery } from '../types';
import { inputSx, secondaryTextSx } from './styles';

export type KeyManagerDialogProps = {
  open: boolean;
  /** The entry being edited, or `null` to add one. */
  editing: GatewayKeyManager | null;
  /** The rest of the list, which the entry must not collide with. */
  others: GatewayKeyManager[];
  /** JWKS URLs the gateway already trusts, which keep their scheme. */
  knownUris: string[];
  /** Reads an issuer's discovery document, at `wellKnownUrl` when given. */
  discover: (issuer: string, wellKnownUrl?: string) => Promise<KeyManagerDiscovery>;
  onCancel: () => void;
  onSubmit: (keyManager: GatewayKeyManager) => void;
};

type Draft = {
  name: string;
  issuer: string;
  wellKnownUrl: string;
  jwksUri: string;
};

type FieldKey = keyof Draft;

/**
 * Adding one starts at its name and issuer; the URLs follow from the issuer's
 * discovery document, or from the user when it cannot be read. An edit opens
 * on every field.
 */
type Step = 'identify' | 'details';

const toDraft = (km: GatewayKeyManager | null): Draft => ({
  name: km?.name ?? '',
  issuer: km?.issuer ?? '',
  wellKnownUrl: km?.issuer ? wellKnownUrlOf(km.issuer) : '',
  jwksUri: km?.jwksUri ?? '',
});

const fromDraft = (draft: Draft): GatewayKeyManager => ({
  name: draft.name,
  issuer: draft.issuer,
  jwksUri: draft.jwksUri,
});

/**
 * Adds or edits one key manager. Nothing is sent from here: the entry joins the
 * drawer's unsaved changes and goes out with "Save and apply", so several edits
 * cost one gateway restart. The well-known URL only finds the JWKS URL; it is
 * not saved.
 */
const KeyManagerDialog: FC<KeyManagerDialogProps> = ({
  open,
  editing,
  others,
  knownUris,
  discover,
  onCancel,
  onSubmit,
}) => {
  const [draft, setDraft] = useState<Draft>(() => toDraft(editing));
  const [step, setStep] = useState<Step>(editing ? 'details' : 'identify');
  const [fetching, setFetching] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  /** Fields left once: a required field says so only after the user has been there. */
  const [touched, setTouched] = useState<Set<FieldKey>>(new Set());
  /** Drops the answer to a lookup the dialog has moved on from. */
  const lookup = useRef(0);

  useEffect(() => {
    if (!open) return;
    lookup.current += 1;
    setDraft(toDraft(editing));
    setStep(editing ? 'details' : 'identify');
    setFetching(false);
    setFetchError(null);
    setTouched(new Set());
  }, [editing, open]);

  const candidate = fromDraft(draft);
  const errors: KeyManagerErrors & { wellKnownUrl?: string } = {
    ...validateKeyManager(candidate, others, knownUris),
    wellKnownUrl: wellKnownUrlError(draft.wellKnownUrl),
  };
  const errorOf = (key: FieldKey): string | undefined => {
    const error = errors[key as keyof typeof errors];
    return error && (touched.has(key) || draft[key].trim() !== '') ? error : undefined;
  };
  const identified = !errors.name && !errors.issuer;
  const valid = !fetching && Object.values(errors).every((error) => !error);
  /** An edit that changes nothing has nothing to save. */
  const changed = !editing || !sameKeyManagers([normalizeKeyManager(candidate)], [editing]);
  const complete = valid && changed;

  const set = (key: keyof Draft) => (value: string) =>
    setDraft((current) => ({ ...current, [key]: value }));

  const fetchDetails = async (wellKnownUrl?: string) => {
    const id = ++lookup.current;
    setFetching(true);
    setFetchError(null);
    try {
      const found = await discover(draft.issuer.trim(), wellKnownUrl?.trim() || undefined);
      if (id !== lookup.current) return;
      setDraft((current) => ({
        ...current,
        issuer: found.issuer,
        wellKnownUrl: found.wellKnownUrl,
        jwksUri: found.jwksUri,
      }));
    } catch (error) {
      if (id !== lookup.current) return;
      setFetchError(
        error instanceof Error && error.message ? error.message : 'The request failed.'
      );
    }
    setFetching(false);
    setStep('details');
  };

  const submit = () => {
    if (!complete) return;
    onSubmit(normalizeKeyManager(candidate));
  };

  const field = (
    key: FieldKey,
    label: string,
    help: string,
    extra: { placeholder?: string } = {}
  ) => {
    const error = errorOf(key);
    return (
      <Box>
        <Typography component="label" htmlFor={`km-${key}`} variant="body2" sx={{ display: 'block', mb: 0.75 }}>
          {label}
        </Typography>
        <TextField
          id={`km-${key}`}
          fullWidth
          size="small"
          disabled={fetching}
          error={Boolean(error)}
          placeholder={extra.placeholder}
          sx={inputSx(false)}
          value={draft[key]}
          onBlur={() => setTouched((current) => new Set(current).add(key))}
          onChange={(event) => set(key)(event.target.value)}
        />
        {error ? (
          <FormHelperText error sx={{ mx: 0 }}>
            {error}
          </FormHelperText>
        ) : (
          <FormHelperText sx={{ ...secondaryTextSx, mx: 0 }}>{help}</FormHelperText>
        )}
      </Box>
    );
  };

  return (
    <Dialog open={open} onClose={onCancel} fullWidth maxWidth="sm">
      <DialogTitle>{editing ? `Edit ${editing.name}` : 'Add key manager'}</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2.5, pt: '8px !important' }}>
        {field('name', 'Name', 'Identifier used to reference this key manager in API policies.')}
        {field('issuer', 'Issuer', 'Must exactly match the iss claim in issued tokens.', {
          placeholder: 'https://idp.example.com',
        })}
        {step === 'details' ? (
          <>
            {fetchError ? (
              <Alert severity="warning">
                Couldn&apos;t fetch the identity provider&apos;s details: {fetchError} Enter the URLs
                below, or correct the well-known URL and fetch again.
              </Alert>
            ) : null}
            <Box>
              {field('wellKnownUrl', 'Well-known URL', 'OpenID Connect discovery endpoint of the identity provider.', {
                placeholder: 'https://idp.example.com/.well-known/openid-configuration',
              })}
              <Button
                size="small"
                disabled={!identified || Boolean(errors.wellKnownUrl) || fetching}
                onClick={() => void fetchDetails(draft.wellKnownUrl)}
                startIcon={fetching ? <CircularProgress size={16} color="inherit" /> : undefined}
                sx={{ px: 0 }}
              >
                {fetching ? 'Fetching…' : 'Fetch again'}
              </Button>
            </Box>
            {field('jwksUri', 'JWKS URL', 'HTTPS endpoint from which signing keys are retrieved.', {
              placeholder: 'https://idp.example.com/.well-known/jwks.json',
            })}
          </>
        ) : null}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" color="secondary" onClick={onCancel}>
          Cancel
        </Button>
        {step === 'details' ? (
          <Button variant="contained" disabled={!complete} onClick={submit}>
            {editing ? 'Done' : 'Add'}
          </Button>
        ) : (
          <Button
            variant="contained"
            disabled={!identified || fetching}
            onClick={() => void fetchDetails()}
            startIcon={fetching ? <CircularProgress size={16} color="inherit" /> : undefined}
          >
            {fetching ? 'Fetching…' : 'Continue'}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
};

export default KeyManagerDialog;

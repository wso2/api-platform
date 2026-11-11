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
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  FormHelperText,
  Radio,
  RadioGroup,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {
  normalizeKeyManager,
  validateKeyManager,
  type KeyManagerErrors,
} from '../config/keyManagers';
import type { GatewayKeyManager } from '../types';
import { inputSx, secondaryTextSx } from './styles';

export type KeyManagerDialogProps = {
  open: boolean;
  /** The entry being edited, or `null` to add one. */
  editing: GatewayKeyManager | null;
  /** The rest of the list, which the entry must not collide with. */
  others: GatewayKeyManager[];
  /** JWKS URLs the gateway already trusts, which keep their scheme. */
  knownUris: string[];
  onCancel: () => void;
  onSubmit: (keyManager: GatewayKeyManager) => void;
};

type Draft = {
  name: string;
  issuer: string;
  source: 'jwks' | 'publicKey';
  jwksUri: string;
  publicKey: string;
  scopeClaim: string;
  scopeClaimSeparator: string;
};

const toDraft = (km: GatewayKeyManager | null): Draft => ({
  name: km?.name ?? '',
  issuer: km?.issuer ?? '',
  source: km?.publicKey ? 'publicKey' : 'jwks',
  jwksUri: km?.jwksUri ?? '',
  publicKey: km?.publicKey ?? '',
  scopeClaim: km?.scopeClaim ?? '',
  scopeClaimSeparator: km?.scopeClaimSeparator ?? '',
});

const fromDraft = (draft: Draft): GatewayKeyManager => ({
  name: draft.name,
  issuer: draft.issuer,
  ...(draft.source === 'jwks' ? { jwksUri: draft.jwksUri } : { publicKey: draft.publicKey }),
  scopeClaim: draft.scopeClaim,
  scopeClaimSeparator: draft.scopeClaimSeparator,
});

/**
 * Adds or edits one key manager. Nothing is sent from here: the entry joins the
 * drawer's unsaved changes and goes out with "Save and apply", so several edits
 * cost one gateway restart.
 */
const KeyManagerDialog: FC<KeyManagerDialogProps> = ({
  open,
  editing,
  others,
  knownUris,
  onCancel,
  onSubmit,
}) => {
  const [draft, setDraft] = useState<Draft>(() => toDraft(editing));
  const [advanced, setAdvanced] = useState(false);
  /** Errors show once the user has tried to submit, not while they type the first character. */
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    if (!open) return;
    setDraft(toDraft(editing));
    setAdvanced(Boolean(editing?.scopeClaim || editing?.scopeClaimSeparator));
    setSubmitted(false);
  }, [editing, open]);

  const candidate = fromDraft(draft);
  const errors: KeyManagerErrors = validateKeyManager(candidate, others, knownUris);
  const shown = submitted ? errors : {};
  const set = (key: keyof Draft) => (value: string) =>
    setDraft((current) => ({ ...current, [key]: value }));

  const submit = () => {
    setSubmitted(true);
    if (Object.keys(errors).length > 0) return;
    onSubmit(normalizeKeyManager(candidate));
  };

  const field = (
    key: 'name' | 'issuer' | 'jwksUri' | 'publicKey' | 'scopeClaim' | 'scopeClaimSeparator',
    label: string,
    help: string,
    extra: { multiline?: boolean; placeholder?: string } = {}
  ) => {
    const error = key in shown ? shown[key as keyof KeyManagerErrors] : undefined;
    return (
      <Box>
        <Typography component="label" htmlFor={`km-${key}`} variant="body2" sx={{ display: 'block', mb: 0.75 }}>
          {label}
        </Typography>
        <TextField
          id={`km-${key}`}
          fullWidth
          size="small"
          error={Boolean(error)}
          multiline={extra.multiline}
          minRows={extra.multiline ? 4 : undefined}
          placeholder={extra.placeholder}
          sx={inputSx(false)}
          value={draft[key]}
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
        {field('name', 'Name', "What an API's JWT policy names it by.")}
        {field('issuer', 'Issuer', "Must match the tokens' iss claim exactly.")}
        <Box>
          <Typography variant="body2" sx={{ mb: 0.5 }}>
            Signing keys
          </Typography>
          <RadioGroup
            row
            value={draft.source}
            onChange={(event) => set('source')(event.target.value)}
          >
            <FormControlLabel value="jwks" control={<Radio size="small" />} label="JWKS URL" />
            <FormControlLabel value="publicKey" control={<Radio size="small" />} label="Public key" />
          </RadioGroup>
        </Box>
        {draft.source === 'jwks'
          ? field('jwksUri', 'JWKS URL', 'Where the gateway fetches the signing keys. https.', {
              placeholder: 'https://idp.example.com/.well-known/jwks.json',
            })
          : field('publicKey', 'Public key', 'A PEM public key or certificate.', {
              multiline: true,
              placeholder: '-----BEGIN PUBLIC KEY-----',
            })}
        <Box>
          <Button size="small" onClick={() => setAdvanced((value) => !value)} sx={{ px: 0 }}>
            {advanced ? 'Hide advanced' : 'Advanced'}
          </Button>
          <Collapse in={advanced}>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2.5, pt: 1 }}>
              {field(
                'scopeClaim',
                'Scope claim',
                'The claim scopes are read from. Default: the scope and scp claims.'
              )}
              {field(
                'scopeClaimSeparator',
                'Scope separator',
                'Splits a string scope claim. Default: a space.'
              )}
            </Box>
          </Collapse>
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button variant="outlined" color="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="contained" onClick={submit}>
          {editing ? 'Done' : 'Add'}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default KeyManagerDialog;

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

import { useState, type FC } from 'react';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormHelperText,
  IconButton,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { Edit, Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { MAX_KEY_MANAGERS } from '../config/keyManagers';
import type { GatewayKeyManager } from '../types';
import KeyManagerDialog, { type KeyManagerDialogProps } from './KeyManagerDialog';
import { secondaryTextSx } from './styles';

export type KeyManagersSectionProps = {
  /** What the form shows: the pending list if there is one, else the saved one. */
  keyManagers: GatewayKeyManager[] | null;
  /** The list as saved, whose JWKS URLs keep their scheme. */
  saved: GatewayKeyManager[];
  loadError?: string | null;
  readOnly?: boolean;
  /** Whether the shown list is an unsaved edit. */
  modified?: boolean;
  /** Reads an issuer's discovery document for the add and edit dialog. */
  discover: KeyManagerDialogProps['discover'];
  onChange: (keyManagers: GatewayKeyManager[]) => void;
};

/**
 * The identity providers the gateway accepts JWTs from, as a field of the JWT
 * authentication section: they are jwt-auth's key managers. The environment's
 * ThunderID is the first one a gateway trusts and is an entry like any other.
 *
 * Edits are unsaved changes of the drawer, saved with everything else by "Save
 * and apply", so several of them cost one gateway restart.
 */
const KeyManagersSection: FC<KeyManagersSectionProps> = ({
  keyManagers,
  saved,
  loadError,
  readOnly = false,
  modified = false,
  discover,
  onChange,
}) => {
  /** The index being edited, `'new'` while adding, `null` with the dialog closed. */
  const [dialog, setDialog] = useState<number | 'new' | null>(null);
  /**
   * The name awaiting delete confirmation, so a stray click removes nothing. A
   * name, not an index: a refresh can reorder the list under the open dialog.
   */
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);
  const list = keyManagers ?? [];
  const editing = typeof dialog === 'number' ? list[dialog] : null;
  const others = list.filter((_, index) => index !== dialog);
  const knownUris = saved.flatMap((km) => (km.jwksUri ? [km.jwksUri] : []));
  const atCap = list.length >= MAX_KEY_MANAGERS;

  const submit = (keyManager: GatewayKeyManager) => {
    if (dialog === 'new') onChange([...list, keyManager]);
    else if (typeof dialog === 'number') onChange(list.map((km, i) => (i === dialog ? keyManager : km)));
    setDialog(null);
  };

  return (
    <Box>
      <Box sx={{ alignItems: 'center', display: 'flex', gap: 0.75, mb: 0.75 }}>
        <Typography variant="body2">Key managers</Typography>
        {modified ? (
          <Box
            aria-label="Unsaved change"
            role="img"
            sx={{ bgcolor: 'primary.main', borderRadius: '50%', height: 7, width: 7 }}
          />
        ) : null}
        <Box sx={{ flex: 1 }} />
        <Tooltip title={atCap ? `A gateway can trust at most ${MAX_KEY_MANAGERS} key managers.` : ''}>
          <Box component="span">
            <Button
              size="small"
              startIcon={<Plus size={14} />}
              disabled={readOnly || keyManagers === null || atCap}
              onClick={() => setDialog('new')}
            >
              Add key manager
            </Button>
          </Box>
        </Tooltip>
      </Box>

      {loadError ? (
        <FormHelperText error sx={{ mx: 0 }}>
          {loadError}
        </FormHelperText>
      ) : null}

      {keyManagers ? (
        <Box
          role="list"
          aria-label="Key managers"
          sx={{
            border: 1,
            borderColor: modified ? 'primary.main' : 'divider',
            borderRadius: 1,
          }}
        >
          {list.length === 0 ? (
            <Typography variant="body2" sx={{ ...secondaryTextSx, px: 1.5, py: 1.25 }}>
              No key managers configured. Add an identity provider to accept its tokens.
            </Typography>
          ) : (
            list.map((keyManager, index) => (
              <KeyManagerRow
                key={keyManager.name}
                keyManager={keyManager}
                divided={index > 0}
                readOnly={readOnly}
                onEdit={() => setDialog(index)}
                onDelete={() => setPendingDelete(keyManager.name)}
              />
            ))
          )}
        </Box>
      ) : null}
      <FormHelperText sx={{ ...secondaryTextSx, mx: 0 }}>
        Tokens are validated against the issuer of each key manager.
      </FormHelperText>

      <KeyManagerDialog
        open={dialog !== null}
        editing={editing}
        others={others}
        knownUris={knownUris}
        discover={discover}
        onCancel={() => setDialog(null)}
        onSubmit={submit}
      />

      <Dialog open={pendingDelete !== null} onClose={() => setPendingDelete(null)}>
        <DialogTitle>Delete Key Manager</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to delete {pendingDelete ?? ''}? The
            gateway stops accepting its tokens once you save and apply.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingDelete(null)} variant="outlined" color="secondary">
            Cancel
          </Button>
          <Button
            color="error"
            onClick={() => {
              if (pendingDelete !== null) onChange(list.filter((km) => km.name !== pendingDelete));
              setPendingDelete(null);
            }}
          >
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

/** One key manager by name, with Edit and Delete; Edit shows its issuer and URLs. */
const KeyManagerRow: FC<{
  keyManager: GatewayKeyManager;
  divided: boolean;
  readOnly: boolean;
  onEdit: () => void;
  onDelete: () => void;
}> = ({ keyManager, divided, readOnly, onEdit, onDelete }) => (
  <Box
    role="listitem"
    sx={{
      alignItems: 'center',
      borderTop: divided ? 1 : 0,
      borderColor: 'divider',
      display: 'flex',
      gap: 1,
      px: 1.5,
      py: 1.25,
    }}
  >
    <Typography variant="body2" sx={{ flex: 1, fontWeight: 500, minWidth: 0, overflowWrap: 'anywhere' }}>
      {keyManager.name}
    </Typography>
    <Box sx={{ display: 'flex', alignItems: 'center', flexShrink: 0 }}>
      {/* The span keeps each tooltip working while its button is disabled. */}
      <Tooltip title="Edit">
        <span style={{ display: 'inline-flex' }}>
          <IconButton size="small" disabled={readOnly} onClick={onEdit} aria-label={`Edit ${keyManager.name}`}>
            <Edit size={16} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Delete">
        <span style={{ display: 'inline-flex' }}>
          <IconButton
            size="small"
            color="error"
            disabled={readOnly}
            onClick={onDelete}
            aria-label={`Delete ${keyManager.name}`}
          >
            <Trash2 size={16} />
          </IconButton>
        </span>
      </Tooltip>
    </Box>
  </Box>
);

export default KeyManagersSection;

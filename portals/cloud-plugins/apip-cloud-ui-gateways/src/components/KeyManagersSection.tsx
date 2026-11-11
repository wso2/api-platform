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
import { Box, Button, FormHelperText, Tooltip, Typography } from '@wso2/oxygen-ui';
import { Edit, Plus, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { MAX_KEY_MANAGERS } from '../config/keyManagers';
import type { GatewayKeyManager } from '../types';
import KeyManagerDialog from './KeyManagerDialog';
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
  onChange: (keyManagers: GatewayKeyManager[]) => void;
};

const LAST_ONE =
  'Keep at least one key manager. Without one, every API that uses JWT authentication on this gateway refuses all tokens.';

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
  onChange,
}) => {
  /** The index being edited, `'new'` while adding, `null` with the dialog closed. */
  const [dialog, setDialog] = useState<number | 'new' | null>(null);
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
              None. Add the identity provider whose tokens this gateway should accept.
            </Typography>
          ) : (
            list.map((keyManager, index) => (
              <KeyManagerRow
                key={keyManager.name}
                keyManager={keyManager}
                divided={index > 0}
                readOnly={readOnly}
                lastOne={list.length === 1}
                onEdit={() => setDialog(index)}
                onDelete={() => onChange(list.filter((_, i) => i !== index))}
              />
            ))
          )}
        </Box>
      ) : null}
      <FormHelperText sx={{ ...secondaryTextSx, mx: 0 }}>
        Checked against the issuer of each token.
      </FormHelperText>

      <KeyManagerDialog
        open={dialog !== null}
        editing={editing}
        others={others}
        knownUris={knownUris}
        onCancel={() => setDialog(null)}
        onSubmit={submit}
      />
    </Box>
  );
};

/** One key manager: its name, issuer and key source, with Edit and Delete. */
const KeyManagerRow: FC<{
  keyManager: GatewayKeyManager;
  divided: boolean;
  readOnly: boolean;
  lastOne: boolean;
  onEdit: () => void;
  onDelete: () => void;
}> = ({ keyManager, divided, readOnly, lastOne, onEdit, onDelete }) => (
  <Box
    role="listitem"
    sx={{
      alignItems: 'flex-start',
      borderTop: divided ? 1 : 0,
      borderColor: 'divider',
      display: 'flex',
      gap: 1,
      px: 1.5,
      py: 1.25,
    }}
  >
    <Box sx={{ flex: 1, minWidth: 0 }}>
      <Typography variant="body2" sx={{ fontWeight: 500 }}>
        {keyManager.name}
      </Typography>
      <Detail label="Issuer" value={keyManager.issuer} />
      {keyManager.jwksUri ? (
        <Detail label="JWKS URL" value={keyManager.jwksUri} />
      ) : (
        <Detail label="Public key" value="PEM" />
      )}
    </Box>
    <Box sx={{ display: 'flex', gap: 0.5, flexShrink: 0 }}>
      <Button size="small" startIcon={<Edit size={14} />} disabled={readOnly} onClick={onEdit}>
        Edit
      </Button>
      <Tooltip title={lastOne ? LAST_ONE : ''}>
        <Box component="span">
          <Button
            size="small"
            color="error"
            startIcon={<Trash2 size={14} />}
            disabled={readOnly || lastOne}
            onClick={onDelete}
            aria-label={`Delete ${keyManager.name}`}
          >
            Delete
          </Button>
        </Box>
      </Tooltip>
    </Box>
  </Box>
);

const Detail: FC<{ label: string; value: string }> = ({ label, value }) => (
  <Box sx={{ display: 'flex', gap: 1 }}>
    <Typography variant="caption" sx={{ ...secondaryTextSx, flex: '0 0 64px' }}>
      {label}
    </Typography>
    <Typography variant="caption" sx={{ ...secondaryTextSx, minWidth: 0, overflowWrap: 'anywhere' }}>
      {value}
    </Typography>
  </Box>
);

export default KeyManagersSection;

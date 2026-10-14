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

import React, { useEffect, useState } from 'react';
import { TextField, Typography } from '@wso2/oxygen-ui';

/** Empty when the path is usable, otherwise why it is not. */
export function validateTransportPath(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return 'Path is required.';
  if (!trimmed.startsWith('/')) return 'Path must start with "/".';
  if (/\s/.test(trimmed)) return 'Path cannot contain spaces.';
  return '';
}

type TransportPathFieldProps = {
  value: string;
  /** Owned by the parent, since the pencil that opens it sits outside. */
  editing: boolean;
  onEditingChange: (editing: boolean) => void;
  onChange: (path: string) => void;
};

/** The path a transport is served on, edited in place. */
export default function TransportPathField({
  value,
  editing,
  onEditingChange,
  onChange,
}: TransportPathFieldProps): React.JSX.Element {
  const [draft, setDraft] = useState(value);

  useEffect(() => {
    if (editing) setDraft(value);
  }, [editing, value]);

  const error = editing ? validateTransportPath(draft) : '';

  const commit = () => {
    if (validateTransportPath(draft)) return;
    onChange(draft.trim());
    onEditingChange(false);
  };

  if (!editing) {
    return (
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ fontFamily: 'monospace' }}
        noWrap
      >
        {value}
      </Typography>
    );
  }

  return (
    <TextField
      autoFocus
      size="small"
      value={draft}
      error={Boolean(error)}
      helperText={error || undefined}
      // The surrounding box toggles the transport, so edits must not reach it.
      onClick={(event) => event.stopPropagation()}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={() => (validateTransportPath(draft) ? onEditingChange(false) : commit())}
      onKeyDown={(event) => {
        event.stopPropagation();
        if (event.key === 'Enter') commit();
        if (event.key === 'Escape') onEditingChange(false);
      }}
      slotProps={{
        input: { sx: { fontFamily: 'monospace', fontSize: '0.75rem' } },
      }}
      sx={{ mt: 0.25, maxWidth: 200 }}
    />
  );
}

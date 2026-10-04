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

import { useId, useMemo, useRef } from 'react';
import {
  Autocomplete,
  Box,
  Checkbox,
  Chip,
  CircularProgress,
  IconButton,
  Input,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, KeyRound, RefreshCw, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { SecretValue } from '../../components/SecretValue';
import type { KeyValueRow } from '../../utils/types';

const messages = defineMessages({
  auto: {
    id: 'apiControlPlane.pages.test.console.KeyValueEditor.auto',
    defaultMessage: 'Auto',
    description:
      'Badge on a row the console filled in itself rather than the user, such as the test key header.',
  },
  closeOptions: {
    id: 'apiControlPlane.pages.test.console.KeyValueEditor.closeOptions',
    defaultMessage: 'Close the list of standard names',
    description: 'Accessible label for the button that closes the name suggestion list. A command.',
  },
  exclude: {
    id: 'apiControlPlane.pages.test.console.KeyValueEditor.exclude',
    defaultMessage: 'Include this row',
    description: 'Accessible label for the checkbox that includes or excludes a row.',
  },
  keyColumn: {
    id: 'apiControlPlane.pages.test.console.KeyValueEditor.keyColumn',
    defaultMessage: 'Key',
    description: 'Column heading for the name of a query parameter or header.',
  },
  noOptions: {
    id: 'apiControlPlane.pages.test.console.KeyValueEditor.noOptions',
    defaultMessage: 'No standard name matches. Typing one of your own is fine.',
    description:
      'Shown in the name suggestion list when what has been typed matches none of the standard names.',
  },
  openOptions: {
    id: 'apiControlPlane.pages.test.console.KeyValueEditor.openOptions',
    defaultMessage: 'Pick a standard name',
    description: 'Accessible label for the button that opens the name suggestion list. A command.',
  },
  regenerate: {
    id: 'apiControlPlane.pages.test.console.KeyValueEditor.regenerate',
    defaultMessage: 'Issue a new key',
    description: 'Accessible label for the button replacing the test key from within the row.',
  },
  remove: {
    id: 'apiControlPlane.pages.test.console.KeyValueEditor.remove',
    defaultMessage: 'Remove row',
    description: 'Accessible label for the button deleting a query parameter or header row.',
  },
  valueColumn: {
    id: 'apiControlPlane.pages.test.console.KeyValueEditor.valueColumn',
    defaultMessage: 'Value',
    description: 'Column heading for the value of a query parameter or header.',
  },
});

type KeyValueEditorProps = {
  helperText: React.ReactNode;
  /**
   * Names offered in a picker on the key field, still free-text.
   *
   * Supplied for headers, whose names are a fixed vocabulary nobody should have
   * to recall exactly; omitted for query parameters and body fields, whose
   * names are the API's own and unknowable from here.
   */
  nameOptions?: readonly string[];
  namePlaceholder: string;
  onChange: (rows: KeyValueRow[]) => void;
  /** Offered on the secret row, so the key can be replaced without leaving. */
  onRegenerateSecret?: () => void;
  regenerating?: boolean;
  rows: KeyValueRow[];
  valuePlaceholder: string;
};

/** A row id that cannot collide with a synced one. */
const newRowId = (): string =>
  `local-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;

/**
 * Shared editor for query parameters and headers in the cURL builder.
 *
 * - Editing the trailing blank row promotes it and adds a new blank row.
 *   Stable IDs preserve focus while names are being entered.
 * - Excluded rows remain visible and can be restored without re-entering them.
 * - When `nameOptions` is provided, names can be selected or entered freely.
 */
export function KeyValueEditor({
  helperText,
  nameOptions,
  namePlaceholder,
  onChange,
  onRegenerateSecret,
  regenerating,
  rows,
  valuePlaceholder,
}: KeyValueEditorProps) {
  const intl = useIntl();
  const headingId = useId();

  /** Stable ID for the trailing blank row, preserving focus while editing. */
  const draftId = useRef(newRowId());

  // Rotate once the previous draft has been committed to `rows`, so the next
  // blank row cannot reuse an id that is already on screen.
  if (rows.some((row) => row.id === draftId.current)) draftId.current = newRowId();

  /** A row the user has not put anything into yet. */
  const isBlank = (row: KeyValueRow) => row.name === '' && row.value === '';

  /** The caller's rows plus one blank row for input. */
  const rendered = useMemo(() => {
    const last = rows[rows.length - 1];
    if (last && isBlank(last)) return rows;
    return [
      ...rows,
      { enabled: true, id: draftId.current, name: '', value: '' } satisfies KeyValueRow,
    ];
    // `draftId` is a ref, deliberately excluded: rotating it must not rebuild
    // this list mid-keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows]);

  /**
   * Applies an edit, committing the draft row if that is what was edited.
   *
   * `enabled` is already true on the draft: someone who has just typed a header
   * name means to send it, and making them also tick a box would be a trap.
   */
  const update = (id: string, patch: Partial<KeyValueRow>) => {
    const next = rendered.map((row) => (row.id === id ? { ...row, ...patch } : row));

    // Keep blank existing rows during edits; only discard an untouched draft.
    onChange(next.filter((row) => !(row.id === draftId.current && isBlank(row))));
  };

  const remove = (id: string) => onChange(rows.filter((row) => row.id !== id));

  return (
    <Box>
      <Stack
        direction="row"
        spacing={1}
        sx={{ borderBottom: '1px solid', borderColor: 'divider', px: 2, py: 1 }}
      >
        <Box sx={{ width: 40 }} />
        <Typography color="text.secondary" id={headingId} sx={{ flex: 1 }} variant="caption">
          <FormattedMessage {...messages.keyColumn} />
        </Typography>
        <Typography color="text.secondary" sx={{ flex: 1.6 }} variant="caption">
          <FormattedMessage {...messages.valueColumn} />
        </Typography>
        <Box sx={{ width: 40 }} />
      </Stack>

      {rendered.map((row) => (
        <Stack
          alignItems="center"
          direction="row"
          key={row.id}
          spacing={1}
          sx={{
            // The auto row is tinted so it reads as supplied rather than typed.
            bgcolor: row.auto ? 'action.hover' : undefined,
            borderBottom: '1px solid',
            borderColor: 'divider',
            px: 2,
            py: 0.5,
          }}
        >
          <Box sx={{ width: 40 }}>
            <Checkbox
              checked={row.enabled}
              inputProps={{ 'aria-label': intl.formatMessage(messages.exclude) }}
              onChange={(event) => update(row.id, { enabled: event.target.checked })}
              size="small"
            />
          </Box>

          <Stack alignItems="center" direction="row" spacing={0.5} sx={{ flex: 1, minWidth: 0 }}>
            {row.auto && <KeyRound size={14} />}
            {nameOptions ? (
              <Autocomplete
                closeText={intl.formatMessage(messages.closeOptions)}
                disableClearable
                // Off by default for `freeSolo`, but the arrow is the only hint
                // that a list exists at all — which is the point of it.
                forcePopupIcon
                freeSolo
                fullWidth
                inputValue={row.name}
                noOptionsText={intl.formatMessage(messages.noOptions)}
                onInputChange={(_event, next) => update(row.id, { name: next })}
                openText={intl.formatMessage(messages.openOptions)}
                options={nameOptions}
                popupIcon={<ChevronDown size={16} />}
                renderInput={(params) => (
                  <Input
                    disableUnderline
                    fullWidth
                    {...params.InputProps}
                    inputProps={{
                      ...params.inputProps,
                      'aria-label': intl.formatMessage(messages.keyColumn),
                    }}
                    placeholder={namePlaceholder}
                  />
                )}
                sx={{ minWidth: 0 }}
              />
            ) : (
              <Input
                disableUnderline
                fullWidth
                inputProps={{ 'aria-label': intl.formatMessage(messages.keyColumn) }}
                onChange={(event) => update(row.id, { name: event.target.value })}
                placeholder={namePlaceholder}
                value={row.name}
              />
            )}
            {row.auto && (
              <Chip label={intl.formatMessage(messages.auto)} size="small" variant="outlined" />
            )}
          </Stack>

          <Box sx={{ flex: 1.6, minWidth: 0 }}>
            {row.secret ? (
              <SecretValue
                actions={
                  onRegenerateSecret && (
                    <Tooltip title={intl.formatMessage(messages.regenerate)}>
                      <IconButton
                        aria-label={intl.formatMessage(messages.regenerate)}
                        disabled={regenerating}
                        onClick={onRegenerateSecret}
                        size="small"
                      >
                        {regenerating ? <CircularProgress size={14} /> : <RefreshCw size={16} />}
                      </IconButton>
                    </Tooltip>
                  )
                }
                value={row.value}
              />
            ) : (
              <Input
                disableUnderline
                fullWidth
                inputProps={{ 'aria-label': intl.formatMessage(messages.valueColumn) }}
                onChange={(event) => update(row.id, { value: event.target.value })}
                placeholder={valuePlaceholder}
                value={row.value}
              />
            )}
          </Box>

          <Box sx={{ width: 40 }}>
            {!row.auto && !isBlank(row) && (
              <Tooltip title={intl.formatMessage(messages.remove)}>
                <IconButton
                  aria-label={intl.formatMessage(messages.remove)}
                  onClick={() => remove(row.id)}
                  size="small"
                >
                  <Trash2 size={16} />
                </IconButton>
              </Tooltip>
            )}
          </Box>
        </Stack>
      ))}

      <Typography
        color="text.secondary"
        sx={{ display: 'block', px: 2, py: 1.5 }}
        variant="caption"
      >
        {helperText}
      </Typography>
    </Box>
  );
}

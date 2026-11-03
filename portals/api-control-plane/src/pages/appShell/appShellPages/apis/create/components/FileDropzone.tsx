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

import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  FormControl,
  FormHelperText,
  IconButton,
  Stack,
  Typography,
  alpha,
  type Theme,
} from '@wso2/oxygen-ui';
import { FileText, Trash2, Upload } from '@wso2/oxygen-ui-icons-react';
import { useRef, useState, type DragEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl, type IntlShape } from 'react-intl';

import { hairline } from '@/theme/receipes';

const messages = defineMessages({
  uploadAction: {
    id: 'api.create.fromContract.upload.action',
    defaultMessage: 'Upload',
  },
  uploadHint: {
    id: 'api.create.fromContract.upload.hint',
    defaultMessage: 'Drag & drop your file or click to select · Accepted file types: {extensions}',
    description: 'Sits under the drop-zone heading; {extensions} is a list such as ".json, .yaml".',
  },
  uploadRemove: {
    id: 'api.create.fromContract.upload.remove',
    defaultMessage: 'Remove {fileName}',
    description: 'Accessible name for the button that discards the chosen file.',
  },
  uploadedFile: {
    id: 'api.create.fromContract.upload.uploadedFile',
    defaultMessage: 'Uploaded file',
    description: 'Heading above the selected file — an API contract, or a GraphQL schema.',
  },
  uploadRequired: {
    id: 'api.create.fromContract.upload.required',
    defaultMessage: 'Select an API contract file to continue',
  },
  uploadTitle: {
    id: 'api.create.fromContract.upload.title',
    defaultMessage: 'Upload API Contract',
  },
  uploadUnsupported: {
    id: 'api.create.fromContract.upload.unsupported',
    defaultMessage: 'That file type is not supported. Accepted types: {extensions}',
  },
});

/** Why the field is rejected; `null` while it is acceptable. */
export type FileFieldError = 'required' | 'unsupported' | null;

/** Why a file was rejected: removed or unsupported. */
export type FileDropzoneRejection = 'removed' | 'unsupported';

export type FileDropzoneProps = {
  /** Extensions this dropzone accepts, e.g. `['.json', '.yaml']`. */
  extensions: string[];
  error: FileFieldError;
  file: File | null;
  onReject: (reason: FileDropzoneRejection) => void;
  onSelect: (file: File) => void;
  /** The empty dropzone's heading; defaults to "Upload API Contract". */
  title?: string;
};

/** Bytes rendered as a locale-aware "13 kB" / "1.4 MB". */
const formatFileSize = (intl: IntlShape, bytes: number): string => {
  const asUnit = (value: number, unit: 'kilobyte' | 'megabyte', fractionDigits: number) =>
    intl.formatNumber(value, {
      maximumFractionDigits: fractionDigits,
      style: 'unit',
      unit,
      unitDisplay: 'short',
    });
  if (bytes >= 1024 * 1024) {
    return asUnit(bytes / (1024 * 1024), 'megabyte', 1);
  }
  // Anything under a kilobyte still reads as "1 kB" rather than a bare "0".
  return asUnit(Math.max(1, Math.round(bytes / 1024)), 'kilobyte', 0);
};

/**
 * A filename's actual last extension, lowercased and dot-included (e.g.
 * `.graphql`), or `''` when it has none — read off the last path segment, so
 * a name carrying directory separators is judged on its own final component
 * rather than accidentally matching on something earlier in the string.
 */
const fileExtension = (fileName: string): string => {
  const base = fileName.split(/[\\/]/).pop() ?? fileName;
  const dot = base.lastIndexOf('.');
  return dot === -1 ? '' : base.slice(dot).toLowerCase();
};

/** The extension badge on a chosen file, e.g. `YML`. Empty when there is none. */
const fileExtensionLabel = (fileName: string): string => {
  const extension = fileExtension(fileName);
  return extension === '' ? '' : extension.slice(1).toUpperCase();
};

/** The lowest code point of a printable, non-control character (space). */
const MIN_PRINTABLE_CODE_POINT = 32;

/**
 * Path separators and C0 control characters (code points below a plain
 * space) — never legitimate in a filename, and rejected outright rather
 * than silently stripped. A browser-picked `File` can't actually carry a
 * path that escapes anywhere — this is defense-in-depth, not a gap this
 * component would otherwise be exposed through.
 *
 * Deliberately does *not* reject a `..` substring: unlike `/`/`\`/control
 * characters, `..` is only meaningful as a path-traversal segment, and
 * `File.name` (per the same reasoning above) can't carry a path at all —
 * so the check has no attacker-reachable case to catch here, only
 * legitimate names it rejects by accident (`api..yaml`, a spec's own
 * double-dot version suffix, etc.). The backend's `validateSDLFileName`
 * (`graphql_multipart.go`) keeps its own `..` rule — there it guards a
 * genuinely attacker-controlled multipart filename, not a browser `File`.
 *
 * Written as a character-code scan rather than a regex control-character
 * range: a `\x00`-style escape is easy to mangle into a literal raw byte
 * when a regex literal gets edited, which is a much worse failure mode than
 * this being slightly more verbose.
 */
const isUnsafeFileName = (name: string): boolean => {
  if (name.includes('/') || name.includes('\\')) return true;
  for (let index = 0; index < name.length; index += 1) {
    if (name.charCodeAt(index) < MIN_PRINTABLE_CODE_POINT) return true;
  }
  return false;
};

/** The soft tinted square a drop-zone icon sits in. */
const iconTileSx = (size: number) => (theme: Theme) => ({
  alignItems: 'center',
  bgcolor: alpha(theme.palette.primary.main, 0.12),
  borderRadius: 2,
  color: 'primary.main',
  display: 'flex',
  flexShrink: 0,
  height: theme.spacing(size),
  justifyContent: 'center',
  width: theme.spacing(size),
});

/**
 * Drop area and file picker for a single file — shared by every creation
 * wizard's "Upload" source tab (REST/WebSocket contracts, GraphQL schemas) so
 * they read as the same control rather than each wizard growing its own.
 *
 * The chosen file is summarised inside the drop area, so the area itself
 * cannot be a `<label>`: the remove and replace controls sitting within it
 * would reopen the picker on the very click meant to clear or swap the
 * selection. The hidden input is opened through a ref instead, and the
 * buttons around it stay real buttons for keyboard and screen-reader users.
 */
export const FileDropzone = ({ extensions, error, file, onReject, onSelect, title }: FileDropzoneProps) => {
  const intl = useIntl();
  const [draggedOver, setDraggedOver] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const helperId = 'fileDropzone-helper';
  const extensionList = extensions.join(', ');

  const accepts = (candidate: File): boolean =>
    !isUnsafeFileName(candidate.name) && extensions.includes(fileExtension(candidate.name));

  const take = (candidate: File | undefined) => {
    if (candidate === undefined) {
      return;
    }
    if (accepts(candidate)) {
      onSelect(candidate);
      return;
    }
    onReject('unsupported');
  };

  const openPicker = () => inputRef.current?.click();

  const handleDrop = (event: DragEvent<HTMLElement>) => {
    event.preventDefault();
    setDraggedOver(false);
    take(event.dataTransfer.files[0]);
  };

  const handleDragOver = (event: DragEvent<HTMLElement>) => {
    // Without this the browser navigates to the dropped file instead of
    // handing it to `onDrop`.
    event.preventDefault();
    setDraggedOver(true);
  };

  const helperText = (() => {
    if (error === 'required') {
      return <FormattedMessage {...messages.uploadRequired} />;
    }
    if (error === 'unsupported') {
      return (
        <FormattedMessage {...messages.uploadUnsupported} values={{ extensions: extensionList }} />
      );
    }
    return undefined;
  })();

  return (
    <FormControl error={error !== null} fullWidth required>
      <Box
        accept={extensions.join(',')}
        aria-describedby={helperId}
        component="input"
        onChange={(event) => {
          const input = event.target as HTMLInputElement;
          take(input.files?.[0]);
          // Lets the same file be picked again after it was removed.
          input.value = '';
        }}
        ref={inputRef}
        sx={{ display: 'none' }}
        type="file"
      />

      <Card
        onClick={file === null ? openPicker : undefined}
        onDragLeave={() => setDraggedOver(false)}
        onDragOver={handleDragOver}
        onDrop={handleDrop}
        sx={(theme) => ({
          bgcolor: draggedOver ? 'action.hover' : 'background.default',
          border: hairline(theme),
          borderColor: draggedOver ? 'primary.main' : 'divider',
          borderRadius: 2,
          borderStyle: 'dashed',
          cursor: file === null ? 'pointer' : 'default',
          minHeight: 300,
        })}
        variant="outlined"
      >
        <CardContent
          sx={{
            alignItems: 'center',
            display: 'flex',
            justifyContent: 'center',
            minHeight: 300,
            p: 3,
            '&:last-child': { pb: 3 },
          }}
        >
          {file === null ? (
            <Stack spacing={1} sx={{ alignItems: 'center', textAlign: 'center' }}>
              <Box sx={iconTileSx(7)}>
                <Upload size={24} />
              </Box>
              <Typography sx={{ fontWeight: 700, pt: 1 }} variant="h6">
                {title ?? <FormattedMessage {...messages.uploadTitle} />}
              </Typography>
              <Typography color="text.secondary" variant="body2">
                <FormattedMessage {...messages.uploadHint} values={{ extensions: extensionList }} />
              </Typography>
              <Button onClick={openPicker} sx={{ mt: 2 }} type="button" variant="contained">
                <FormattedMessage {...messages.uploadAction} />
              </Button>
            </Stack>
          ) : (
            <Stack spacing={2} sx={{ alignItems: 'center', width: '100%' }}>
              <Typography sx={{ fontWeight: 700 }} variant="h6">
                <FormattedMessage {...messages.uploadedFile} />
              </Typography>
              <Card sx={{ maxWidth: 480, width: '100%' }}>
                <CardContent sx={{ p: 2, '&:last-child': { pb: 2 } }}>
                  <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
                    <Box sx={iconTileSx(5)}>
                      <FileText size={20} />
                    </Box>
                    <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', minWidth: 0 }}>
                        <Typography noWrap sx={{ fontWeight: 600 }} variant="body1">
                          {file.name}
                        </Typography>
                        {fileExtensionLabel(file.name) === '' ? null : (
                          <Chip label={fileExtensionLabel(file.name)} size="small" />
                        )}
                      </Stack>
                      <Typography color="text.secondary" variant="caption">
                        {formatFileSize(intl, file.size)}
                      </Typography>
                    </Box>
                    <IconButton
                      aria-label={intl.formatMessage(messages.uploadRemove, {
                        fileName: file.name,
                      })}
                      color="error"
                      onClick={() => onReject('removed')}
                      size="small"
                      type="button"
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </Stack>
                </CardContent>
              </Card>
            </Stack>
          )}
        </CardContent>
      </Card>

      {helperText === undefined ? null : (
        <FormHelperText id={helperId}>{helperText}</FormHelperText>
      )}
    </FormControl>
  );
};

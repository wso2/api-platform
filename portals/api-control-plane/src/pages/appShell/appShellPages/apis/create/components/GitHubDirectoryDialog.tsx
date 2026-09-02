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
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  OutlinedInput,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ChevronDown,
  ChevronRight,
  FileCode2,
  Folder,
  Search,
  X,
} from '@wso2/oxygen-ui-icons-react';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { hairline } from '@/theme/receipes';
import { contractFilesIn, type GitHubTree } from '../utils/github';

const messages = defineMessages({
  cancel: {
    id: 'api.create.gitHubDirectory.action.cancel',
    defaultMessage: 'Cancel',
  },
  close: {
    id: 'api.create.gitHubDirectory.action.close',
    defaultMessage: 'Close',
  },
  collapse: {
    id: 'api.create.gitHubDirectory.action.collapse',
    defaultMessage: 'Collapse {name}',
    description: 'Accessible name for the chevron that folds a directory shut.',
  },
  confirm: {
    id: 'api.create.gitHubDirectory.action.confirm',
    defaultMessage: 'Continue',
  },
  expand: {
    id: 'api.create.gitHubDirectory.action.expand',
    defaultMessage: 'Expand {name}',
    description: 'Accessible name for the chevron that opens a directory.',
  },
  filesHeading: {
    id: 'api.create.gitHubDirectory.files.heading',
    defaultMessage: 'API contract in this directory',
  },
  loading: {
    id: 'api.create.gitHubDirectory.loading',
    defaultMessage: 'Reading the repository…',
  },
  noDirectories: {
    id: 'api.create.gitHubDirectory.noDirectories',
    defaultMessage: 'No directory matches that search.',
  },
  noFiles: {
    id: 'api.create.gitHubDirectory.files.none',
    defaultMessage: 'No YAML or JSON file here. Pick a directory that holds the API contract.',
  },
  searchPlaceholder: {
    id: 'api.create.gitHubDirectory.search.placeholder',
    defaultMessage: 'Search directories',
  },
  title: {
    id: 'api.create.gitHubDirectory.title',
    defaultMessage: 'API directory',
  },
  truncated: {
    id: 'api.create.gitHubDirectory.truncated',
    defaultMessage:
      'This repository is too large to list in full, so some directories are missing.',
  },
});

/** What the dialog hands back: a directory, and the file to read inside it. */
export type GitHubDirectorySelection = {
  directory: string;
  file: string;
};

export type GitHubDirectoryDialogProps = {
  /** Shown in place of the listing when the tree could not be read. */
  error?: ReactNode;
  /** Directory the dialog opens on. */
  initialDirectory: string;
  /** File selected in that directory, if one already was. */
  initialFile: string;
  loading?: boolean;
  onCancel: () => void;
  onConfirm: (selection: GitHubDirectorySelection) => void;
  open: boolean;
  tree: GitHubTree | null;
};

/** How deep a directory sits, so the list can show the shape of the repo. */
const depthOf = (directory: string): number =>
  directory === '/' ? 0 : directory.split('/').length - 1;

/** The last segment — what the row is called. */
const nameOf = (directory: string): string =>
  directory === '/' ? '/' : directory.slice(directory.lastIndexOf('/') + 1);

/** The directory one level up: "/apis/orders" → "/apis", "/apis" → "/". */
const parentOf = (directory: string): string => {
  const cut = directory.lastIndexOf('/');
  return cut <= 0 ? '/' : directory.slice(0, cut);
};

/** Every directory on the way down to one, so it can be revealed. */
const ancestorsOf = (directory: string): string[] => {
  const ancestors: string[] = ['/'];
  let current = directory;
  while (current !== '/' && current !== '') {
    current = parentOf(current);
    ancestors.push(current);
  }
  return ancestors;
};

/**
 * Browser for a repository's directories, and the contract file inside the one
 * that is picked.
 *
 * The whole tree arrives in one request (see `github.ts`), so the list is flat
 * and indented by depth rather than expanded a level at a time: it makes the
 * search box cover every directory in the repository instead of only the
 * folders someone has already opened.
 */
export const GitHubDirectoryDialog = ({
  error,
  initialDirectory,
  initialFile,
  loading = false,
  onCancel,
  onConfirm,
  open,
  tree,
}: GitHubDirectoryDialogProps) => {
  const intl = useIntl();
  const [directory, setDirectory] = useState(initialDirectory);
  const [file, setFile] = useState(initialFile);
  const [search, setSearch] = useState('');
  /** Open folders; root starts open so the list begins one level deep. */
  const [expanded, setExpanded] = useState<string[]>(['/']);

  // Opening is what resets the dialog: it is a fresh decision each time, and
  // leaving last time's search in place would hide most of the repository.
  useEffect(() => {
    if (open) {
      setDirectory(initialDirectory);
      setFile(initialFile);
      setSearch('');
      // Whatever the form was already pointing at has to be visible, so the
      // folders leading down to it open with the dialog.
      setExpanded(ancestorsOf(initialDirectory));
    }
  }, [initialDirectory, initialFile, open]);

  const childrenByParent = useMemo(() => {
    const children: Record<string, string[]> = {};
    for (const candidate of tree?.directories ?? []) {
      if (candidate === '/') {
        continue;
      }
      const parent = parentOf(candidate);
      children[parent] = [...(children[parent] ?? []), candidate];
    }
    return children;
  }, [tree]);

  // Rows to draw: root, then contents of open folders in depth-first order.
  // Search shows flat list of matches instead.
  const directories = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (term !== '') {
      return (tree?.directories ?? []).filter((candidate) =>
        candidate.toLowerCase().includes(term),
      );
    }

    const rows: string[] = [];
    const walk = (parent: string) => {
      rows.push(parent);
      if (!expanded.includes(parent)) {
        return;
      }
      for (const child of childrenByParent[parent] ?? []) {
        walk(child);
      }
    };
    if (tree !== null) {
      walk('/');
    }
    return rows;
  }, [childrenByParent, expanded, search, tree]);

  const searching = search.trim() !== '';

  const toggle = (candidate: string) =>
    setExpanded((current) =>
      current.includes(candidate)
        ? current.filter((entry) => entry !== candidate)
        : [...current, candidate],
    );

  const files = contractFilesIn(tree, directory);

  const handleDirectorySelect = (next: string) => {
    setDirectory(next);
    // Best match first, so the common case needs no second click; the rest of
    // the list is right there to override it.
    setFile(contractFilesIn(tree, next)[0] ?? '');
  };

  return (
    <Dialog fullWidth maxWidth="sm" onClose={onCancel} open={open}>
      <DialogTitle>
        <Stack
          direction="row"
          spacing={2}
          sx={{ alignItems: 'center', justifyContent: 'space-between' }}
        >
          <Typography sx={{ fontWeight: 700 }} variant="h6">
            <FormattedMessage {...messages.title} />
          </Typography>
          <IconButton
            aria-label={intl.formatMessage(messages.close)}
            onClick={onCancel}
            size="small"
          >
            <X size={18} />
          </IconButton>
        </Stack>
      </DialogTitle>

      <DialogContent dividers>
        <Stack spacing={2}>
          <OutlinedInput
            fullWidth
            inputProps={{ 'aria-label': intl.formatMessage(messages.title) }}
            readOnly
            sx={{ fontFamily: 'monospace' }}
            value={directory}
          />

          <OutlinedInput
            fullWidth
            inputProps={{
              'aria-label': intl.formatMessage(messages.searchPlaceholder),
            }}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={intl.formatMessage(messages.searchPlaceholder)}
            startAdornment={
              <InputAdornment position="start">
                <Search size={18} />
              </InputAdornment>
            }
            value={search}
          />

          {error === undefined ? null : <Alert severity="error">{error}</Alert>}

          {tree?.truncated === true ? (
            <Alert severity="warning">
              <FormattedMessage {...messages.truncated} />
            </Alert>
          ) : null}

          {loading ? (
            <Stack sx={{ alignItems: 'center', justifyContent: 'center', py: 6 }}>
              <CircularProgress aria-label={intl.formatMessage(messages.loading)} size={24} />
            </Stack>
          ) : (
            <Box
              sx={(theme) => ({
                border: hairline(theme),
                borderColor: 'divider',
                borderRadius: 1,
                maxHeight: 280,
                overflow: 'auto',
              })}
            >
              {directories.length === 0 ? (
                <Typography color="text.secondary" sx={{ p: 2 }} variant="body2">
                  <FormattedMessage {...messages.noDirectories} />
                </Typography>
              ) : (
                <List dense disablePadding>
                  {directories.map((candidate) => {
                    const hasChildren = (childrenByParent[candidate] ?? []).length > 0;
                    const isOpen = expanded.includes(candidate);

                    return (
                      <ListItemButton
                        key={candidate}
                        onClick={() => {
                          handleDirectorySelect(candidate);
                          // Picking a folder opens it too; closing one again is
                          // the chevron's job, so a second click on the folder
                          // you just chose doesn't hide its contents.
                          if (hasChildren && !isOpen && !searching) {
                            toggle(candidate);
                          }
                        }}
                        selected={candidate === directory}
                        sx={{
                          pl: searching ? 1 : 1 + depthOf(candidate) * 2,
                        }}
                      >
                        <ListItemIcon sx={{ minWidth: 28 }}>
                          {hasChildren && !searching ? (
                            <IconButton
                              aria-label={intl.formatMessage(
                                isOpen ? messages.collapse : messages.expand,
                                { name: nameOf(candidate) },
                              )}
                              onClick={(event) => {
                                // The row underneath would otherwise select the
                                // folder as well as fold it.
                                event.stopPropagation();
                                toggle(candidate);
                              }}
                              size="small"
                            >
                              {isOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                            </IconButton>
                          ) : null}
                        </ListItemIcon>
                        <ListItemIcon sx={{ minWidth: 32 }}>
                          <Folder size={18} />
                        </ListItemIcon>
                        <ListItemText
                          primary={searching ? candidate : nameOf(candidate)}
                          slotProps={{ primary: { noWrap: true } }}
                        />
                      </ListItemButton>
                    );
                  })}
                </List>
              )}
            </Box>
          )}

          {loading ? null : (
            <Box>
              <Typography color="text.secondary" sx={{ fontWeight: 700 }} variant="overline">
                <FormattedMessage {...messages.filesHeading} />
              </Typography>
              {files.length === 0 ? (
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.noFiles} />
                </Typography>
              ) : (
                <List dense disablePadding>
                  {files.map((candidate) => (
                    <ListItemButton
                      key={candidate}
                      onClick={() => setFile(candidate)}
                      selected={candidate === file}
                    >
                      <ListItemIcon sx={{ minWidth: 32 }}>
                        <FileCode2 size={18} />
                      </ListItemIcon>
                      <ListItemText primary={candidate} />
                    </ListItemButton>
                  ))}
                </List>
              )}
            </Box>
          )}
        </Stack>
      </DialogContent>

      <DialogActions>
        <Button onClick={onCancel} variant="outlined">
          <FormattedMessage {...messages.cancel} />
        </Button>
        <Button
          // Nothing to import from a directory that holds no contract.
          disabled={file === ''}
          onClick={() => onConfirm({ directory, file })}
          variant="contained"
        >
          <FormattedMessage {...messages.confirm} />
        </Button>
      </DialogActions>
    </Dialog>
  );
};

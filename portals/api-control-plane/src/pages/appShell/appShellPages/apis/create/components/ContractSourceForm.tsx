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
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Form,
  FormControl,
  FormHelperText,
  Grid,
  IconButton,
  InputAdornment,
  InputLabel,
  MenuItem,
  OutlinedInput,
  Select,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
  alpha,
  type Theme,
} from '@wso2/oxygen-ui';
import { FileText, GitHub, Pencil, RefreshCw, Upload, X } from '@wso2/oxygen-ui-icons-react';
import yaml from 'js-yaml';
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type DragEvent,
  type FormEvent,
  type ReactNode,
} from 'react';
import {
  defineMessages,
  FormattedMessage,
  useIntl,
  type IntlShape,
  type MessageDescriptor,
} from 'react-intl';

import { hairline } from '@/theme/receipes';
import { GitHubDirectoryDialog, type GitHubDirectorySelection } from './GitHubDirectoryDialog';
import {
  isGitHubRepositoryUrl,
  contractFilesIn,
  fetchGitHubTree,
  lookupGitHubRepository,
  parseGitHubRepoUrl,
  rawFileUrl,
  resolveGitHubRef,
  type GitHubTree,
} from '../utils/github';
import {
  lookupSwaggerHubOrganization,
  swaggerHubSpecUrl,
  type SwaggerHubApi,
} from '../utils/swaggerHub';
import { isValidUrl } from '../../utils/developEdit';
import { validateApiSpec, type SpecDialect, type SpecIssue } from '../utils/specValidation';
import { SpecIssueList } from './SpecIssueList';
import { type ApiType } from '../types';
import { API_TYPES } from '../uiConfig';

/** Where the contract is read from. */
export type ContractSourceKey = 'url' | 'file' | 'github' | 'swaggerhub';

/**
 * What this step hands to the wizard once the chosen source is accepted.
 *
 * Deliberately flat and loose: nothing fetches a contract yet, so only the one
 * field belonging to `sourceKey` is ever populated. Tighten it into a
 * discriminated union once the import logic decides what it actually needs.
 */
export type ContractValues = {
  apiTypeKey: string;
  sourceKey: ContractSourceKey;
  /** `url` source. */
  url?: string;
  /** `file` source. */
  file?: File;
  /** `github` source — a public repository, or one reached after authorizing. */
  repositoryUrl?: string;
  /** Branch the contract is read from. */
  repositoryBranch?: string;
  /** Directory inside the repository, "/" for the root. */
  repositoryDirectory?: string;
  /** The contract file itself, inside that directory. */
  repositoryFile?: string;
  /** `swaggerhub` source — organization, then the API and version chosen in it. */
  swaggerHubOrganization?: string;
  swaggerHubApi?: string;
  swaggerHubVersion?: string;
};

export type ContractSourceFormProps = {
  /** Types offered in the first tier. Defaults to the three that carry a contract. */
  apiTypes?: ApiType[];
  /**
   * Whether the definition was edited in the preview source view. If so,
   * warnings from this form are cleared because they no longer match.
   */
  definitionEdited?: boolean;
  /** Type selected on first render. Defaults to the first entry in `apiTypes`. */
  initialApiTypeKey?: string;
  /**
   * Starts the GitHub OAuth flow. Unused while the GitHub source is withheld
   * from `CONTRACT_SOURCES_BY_API_TYPE`; kept for when it is offered again.
   */
  onAuthorizeGitHub?: () => void;
  /**
   * The contract this form currently stands behind: what the last fetch
   * returned, or `null` once the inputs have moved on from it. The panel around
   * the form owns Back and Next, so it needs to know when there is something to
   * proceed with.
   */
  onContractChange?: (contract: FetchedContract | null) => void;
  /**
   * Re-fetches the SwaggerHub organizations. Unused while the SwaggerHub
   * source is withheld, on the same terms as `onAuthorizeGitHub`.
   */
  onRefreshSwaggerHubOrganizations?: () => void;
};

/**
 * Which sources each API type can be imported from. Key order is the order the
 * type toggles render in, so the map doubles as the first tier's catalog: a
 * type absent here is not offered by this step at all.
 */
const CONTRACT_SOURCES_BY_API_TYPE: Record<string, ContractSourceKey[]> = {
  // `github` and `swaggerhub` are withheld from every type until their import
  // flow is settled. Everything behind them; the lookups, the pickers, and
  // their branches of `fetchContractForPreview`; is left in place, so
  // offering one again is a matter of listing it here and do necessary changes.
  rest: ['url', 'file'],
  websocket: ['url', 'file'],
  graphql: ['url', 'file'],
};

/** Sources every type supports, for a type the map above doesn't cover. */
const UNIVERSAL_CONTRACT_SOURCES: ContractSourceKey[] = ['url', 'file'];

/** File extensions the upload tab accepts, per API type. */
const CONTRACT_FILE_EXTENSIONS: Record<string, string[]> = {
  rest: ['.json', '.yaml', '.yml'],
  websocket: ['.json', '.yaml', '.yml'],
  graphql: ['.graphql', '.gql', '.json'],
};

/** Contract the "Try with Sample URL" link fills the URL field with. */
const SAMPLE_CONTRACT_URLS: Record<string, string> = {
  rest: 'https://petstore3.swagger.io/api/v3/openapi.json',
  websocket:
    'https://raw.githubusercontent.com/asyncapi/spec/master/examples/streetlights-kafka-asyncapi.yml',
  graphql:
    'https://raw.githubusercontent.com/graphql/graphql-js/main/src/__testUtils__/kitchenSinkSDL.ts',
};

/** Repository the GitHub tab's own sample link fills in. */
const SAMPLE_REPOSITORY_URL = 'https://github.com/wso2/bijira-samples';

const messages = defineMessages({
  fetching: {
    id: 'api.create.fromContract.status.fetching',
    defaultMessage: 'Reading the contract…',
    description:
      'Shown while the chosen contract is being read and checked, which starts on its own.',
  },
  gitHubAuthorize: {
    id: 'api.create.fromContract.gitHub.authorize',
    defaultMessage: 'Authorize With GitHub',
  },
  gitHubConnectTitle: {
    id: 'api.create.fromContract.gitHub.connectTitle',
    defaultMessage: 'Connect Your Repository',
  },
  gitHubOr: {
    id: 'api.create.fromContract.gitHub.or',
    defaultMessage: 'OR',
    description:
      'Separates the two ways of reaching a repository: a public URL, or authorizing with GitHub.',
  },
  gitHubBranchLabel: {
    id: 'api.create.fromContract.gitHub.branchLabel',
    defaultMessage: 'Branch',
  },
  gitHubDirectoryEdit: {
    id: 'api.create.fromContract.gitHub.directoryEdit',
    defaultMessage: 'Choose the API directory',
    description: 'Accessible name for the button that opens the directory browser.',
  },
  gitHubDirectoryLabel: {
    id: 'api.create.fromContract.gitHub.directoryLabel',
    defaultMessage: 'API directory',
  },
  gitHubNoContract: {
    id: 'api.create.fromContract.gitHub.noContract',
    defaultMessage: 'No YAML or JSON contract in this directory. Choose another one.',
  },
  gitHubRateLimited: {
    id: 'api.create.fromContract.gitHub.rateLimited',
    defaultMessage: 'GitHub is rate limiting anonymous requests. Try again in a little while.',
  },
  gitHubRepoNotFound: {
    id: 'api.create.fromContract.gitHub.repoNotFound',
    defaultMessage: 'No public repository at that URL.',
  },
  gitHubRepoUnreachable: {
    id: 'api.create.fromContract.gitHub.repoUnreachable',
    defaultMessage: 'GitHub could not be reached. Try again.',
  },
  gitHubSearching: {
    id: 'api.create.fromContract.gitHub.searching',
    defaultMessage: 'Looking up repository…',
  },
  gitHubSelectedFile: {
    id: 'api.create.fromContract.gitHub.selectedFile',
    defaultMessage: 'Importing {file}',
  },
  gitHubUrlInvalid: {
    id: 'api.create.fromContract.gitHub.invalid',
    defaultMessage: 'Enter a valid GitHub repository URL.',
  },
  gitHubUrlLabel: {
    id: 'api.create.fromContract.gitHub.label',
    defaultMessage: 'Public Repository URL',
  },
  gitHubUrlPlaceholder: {
    id: 'api.create.fromContract.gitHub.placeholder',
    defaultMessage: 'https://github.com/org/repo',
  },
  gitHubUrlRequired: {
    id: 'api.create.fromContract.gitHub.required',
    defaultMessage: 'The repository URL cannot be empty',
  },
  sampleUrl: {
    id: 'api.create.fromContract.action.sampleUrl',
    defaultMessage: 'Try with Sample URL',
    description: 'Fills the field with a ready-made example to try the import with.',
  },
  sourceFile: {
    id: 'api.create.fromContract.source.file',
    defaultMessage: 'Upload',
  },
  sourceGitHub: {
    id: 'api.create.fromContract.source.gitHub',
    defaultMessage: 'GitHub',
  },
  sourceSwaggerHub: {
    id: 'api.create.fromContract.source.swaggerHub',
    defaultMessage: 'SwaggerHub',
  },
  sourceLabel: {
    id: 'api.create.fromContract.source.label',
    defaultMessage: 'Import the contract from',
    description: 'Label over the picker that chooses where the API contract is read from.',
  },
  sourceUrl: {
    id: 'api.create.fromContract.source.url',
    defaultMessage: 'URL',
  },
  specOversized: {
    id: 'api.create.fromContract.spec.oversized',
    defaultMessage: 'That file is too large to validate in the browser.',
  },
  specUnsupportedSource: {
    id: 'api.create.fromContract.spec.unsupportedSource',
    defaultMessage: 'Importing from this source is not available yet.',
  },
  specUnreachable: {
    id: 'api.create.fromContract.spec.unreachable',
    defaultMessage:
      'That contract could not be downloaded. Check the URL, and that the host allows cross-origin requests.',
  },
  specUnreadable: {
    id: 'api.create.fromContract.spec.unreadable',
    defaultMessage: 'That contract could not be read as YAML or JSON.',
  },
  swaggerHubApiLabel: {
    id: 'api.create.fromContract.swaggerHub.apiLabel',
    defaultMessage: 'API',
  },
  swaggerHubApiPlaceholder: {
    id: 'api.create.fromContract.swaggerHub.apiPlaceholder',
    defaultMessage: 'Select an API',
  },
  swaggerHubLookupFailed: {
    id: 'api.create.fromContract.swaggerHub.lookupFailed',
    defaultMessage: 'SwaggerHub could not be reached. Try again.',
  },
  swaggerHubNotFound: {
    id: 'api.create.fromContract.swaggerHub.notFound',
    defaultMessage: 'No SwaggerHub organization with public APIs under that name.',
  },
  swaggerHubPartialListing: {
    id: 'api.create.fromContract.swaggerHub.partialListing',
    defaultMessage: 'Showing {shown} of {total} APIs in this organization.',
    description: 'The registry pages long listings; only the first page is offered.',
  },
  swaggerHubSearching: {
    id: 'api.create.fromContract.swaggerHub.searching',
    defaultMessage: 'Looking up organization…',
  },
  swaggerHubVersionLabel: {
    id: 'api.create.fromContract.swaggerHub.versionLabel',
    defaultMessage: 'Version',
  },
  swaggerHubVersionPlaceholder: {
    id: 'api.create.fromContract.swaggerHub.versionPlaceholder',
    defaultMessage: 'Select a version',
  },
  swaggerHubAuthorized: {
    id: 'api.create.fromContract.swaggerHub.authorized',
    defaultMessage: 'Authorized',
    description: 'SwaggerHub access mode that reads private definitions. Not released yet.',
  },
  swaggerHubAuthorizedHint: {
    id: 'api.create.fromContract.swaggerHub.authorizedHint',
    defaultMessage: 'Not available yet.',
  },
  swaggerHubOrganizationLabel: {
    id: 'api.create.fromContract.swaggerHub.organizationLabel',
    defaultMessage: 'SwaggerHub Organization',
  },
  swaggerHubOrganizationPlaceholder: {
    id: 'api.create.fromContract.swaggerHub.organizationPlaceholder',
    defaultMessage: 'Enter SwaggerHub Organization here',
  },
  swaggerHubOrganizationRequired: {
    id: 'api.create.fromContract.swaggerHub.organizationRequired',
    defaultMessage: 'The SwaggerHub organization cannot be empty',
  },
  swaggerHubPublic: {
    id: 'api.create.fromContract.swaggerHub.public',
    defaultMessage: 'Public',
    description: 'SwaggerHub access mode that reads publicly listed definitions.',
  },
  swaggerHubRefresh: {
    id: 'api.create.fromContract.swaggerHub.refresh',
    defaultMessage: 'Refresh SwaggerHub organizations',
    description: 'Accessible name for the reload button on the organization field.',
  },
  title: {
    id: 'api.create.fromContract.title',
    defaultMessage: 'Create API Proxy from Contract',
  },
  uploadAccepted: {
    id: 'api.create.fromContract.upload.accepted',
    defaultMessage: 'Accepted: {extensions} \u00b7 up to {maxSize}',
    description:
      'Neutral helper line under the drop zone. {maxSize} is a formatted size such as "10 MB".',
  },
  uploadAction: {
    id: 'api.create.fromContract.upload.action',
    defaultMessage: 'Select file',
  },
  uploadHint: {
    id: 'api.create.fromContract.upload.hint',
    defaultMessage: 'One file \u00b7 {extensions}',
    description: 'Sits under the drop-zone heading; {extensions} is a list such as ".json, .yaml".',
  },
  uploadRemove: {
    id: 'api.create.fromContract.upload.remove',
    defaultMessage: 'Remove {fileName}',
    description: 'Accessible name for the button that discards the chosen file.',
  },
  uploadReplace: {
    id: 'api.create.fromContract.upload.replace',
    defaultMessage: 'Replace file',
    description: 'Reopens the file picker so the chosen contract can be swapped for another.',
  },
  uploadRequired: {
    id: 'api.create.fromContract.upload.required',
    defaultMessage: 'Select an API contract file to continue',
  },
  uploadTitle: {
    id: 'api.create.fromContract.upload.title',
    defaultMessage: 'Drag and drop your contract here',
  },
  uploadUnsupported: {
    id: 'api.create.fromContract.upload.unsupported',
    defaultMessage: 'That file type is not supported. Accepted types: {extensions}',
  },
  urlInvalid: {
    id: 'api.create.fromContract.url.invalid',
    defaultMessage: 'Enter a valid HTTP or HTTPS URL.',
  },
  urlLabel: {
    id: 'api.create.fromContract.url.label',
    defaultMessage: 'URL for API Contract',
  },
  urlPlaceholder: {
    id: 'api.create.fromContract.url.placeholder',
    defaultMessage: 'Enter URL for API Contract here',
  },
  urlRequired: {
    id: 'api.create.fromContract.url.required',
    defaultMessage: 'The URL for the API contract cannot be empty',
  },
});

/** Copy shown on each source tab, keyed the same way as the availability map. */
const SOURCE_LABELS: Record<ContractSourceKey, MessageDescriptor> = {
  file: messages.sourceFile,
  github: messages.sourceGitHub,
  swaggerhub: messages.sourceSwaggerHub,
  url: messages.sourceUrl,
};

/** Why a field is rejected; `null` while it is acceptable. */
type TextFieldError = 'required' | 'invalid' | null;

type ContractTextField = ReturnType<typeof useContractTextField>;

/**
 * One required text field's value and validity.
 *
 * Validation runs on blur and on submit, never per keystroke: a URL passes
 * through many invalid prefixes while it is being typed, so checking on every
 * change would mark the field wrong before the user has finished writing it.
 */
const useContractTextField = ({
  isValid,
}: {
  /** Extra check applied to a non-empty value; absent means "anything goes". */
  isValid?: (value: string) => boolean;
} = {}) => {
  const [value, setValue] = useState('');
  const [error, setError] = useState<TextFieldError>(null);

  /** Replaces the value and drops the verdict the old one earned. */
  const change = (next: string) => {
    setError(null);
    setValue(next);
  };

  /** Re-runs validation and reports whether the field may be submitted. */
  const commit = (): boolean => {
    const trimmed = value.trim();
    const nextError: TextFieldError = (() => {
      if (trimmed === '') {
        return 'required';
      }
      return isValid === undefined || isValid(trimmed) ? null : 'invalid';
    })();

    setError(nextError);
    return nextError === null;
  };

  return {
    commit,
    error,
    handleChange: change,
    setValue: change,
    value,
  };
};

type ContractTextControlProps = {
  /** Sits inside the field, at its end, a lookup spinner, say. */
  endAdornment?: ReactNode;
  /** An error the field itself doesn't know about, e.g. a failed lookup. */
  externalError?: ReactNode;
  field: ContractTextField;
  /** Sits under the field whenever there is no error to report. */
  helper?: ReactNode;
  id: string;
  invalidMessage?: MessageDescriptor;
  /** Plain string, not a node: it also sizes the notch it sits in. */
  label: string;
  /**
   * The field passed validation on blur, with the trimmed value it holds. This
   * is where a source acts on a finished value; the URL source fetches from
   * it; so nothing has to be triggered by hand.
   */
  onCommitted?: (value: string) => void;
  placeholder: string;
  requiredMessage: MessageDescriptor;
};

/** Labelled, required text input carrying its own validation message. */
const ContractTextControl = ({
  endAdornment,
  externalError,
  field,
  helper,
  id,
  invalidMessage,
  label,
  onCommitted,
  placeholder,
  requiredMessage,
}: ContractTextControlProps) => {
  const helperId = `${id}-helper`;

  const helperText = (() => {
    if (field.error === 'required') {
      return <FormattedMessage {...requiredMessage} />;
    }
    if (field.error === 'invalid' && invalidMessage !== undefined) {
      return <FormattedMessage {...invalidMessage} />;
    }
    return externalError ?? helper;
  })();

  return (
    <FormControl error={field.error !== null || externalError !== undefined} fullWidth required>
      <InputLabel htmlFor={id}>{label}</InputLabel>
      <OutlinedInput
        aria-describedby={helperId}
        endAdornment={endAdornment}
        id={id}
        // Cuts the gap in the border the InputLabel floats into; must match
        // that label exactly or the notch is the wrong width.
        label={label}
        name={id}
        onBlur={() => {
          if (field.commit()) {
            onCommitted?.(field.value.trim());
          }
        }}
        onChange={(event) => field.handleChange(event.target.value)}
        placeholder={placeholder}
        value={field.value}
      />
      {helperText === undefined ? null : (
        <FormHelperText id={helperId}>{helperText}</FormHelperText>
      )}
    </FormControl>
  );
};

/** Text button that reads as a link, for the "try a sample" affordances. */
const SampleLink = ({ onClick }: { onClick: () => void }) => (
  <Button
    onClick={onClick}
    size="small"
    sx={{ alignSelf: 'flex-start', px: 0, textTransform: 'none' }}
    type="button"
    variant="text"
  >
    <FormattedMessage {...messages.sampleUrl} />
  </Button>
);

/** Why the upload field is rejected; `null` while it is acceptable. */
type FileFieldError = 'required' | 'unsupported' | null;

/** Why a file was rejected: removed or unsupported. */
export type ContractFileRejection = 'removed' | 'unsupported';

type ContractFileControlProps = {
  /** Extensions this API type's contract may use, e.g. `['.json', '.yaml']`. */
  extensions: string[];
  error: FileFieldError;
  file: File | null;
  onReject: (reason: ContractFileRejection) => void;
  onSelect: (file: File) => void;
};

/** Ceiling for in-browser parsing — a huge document would freeze the tab. */
const MAX_CONTRACT_BYTES = 10 * 1024 * 1024;

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

/** The extension badge on a chosen file, e.g. `YML`. Empty when there is none. */
const fileExtensionLabel = (fileName: string): string => {
  const dot = fileName.lastIndexOf('.');
  return dot === -1 ? '' : fileName.slice(dot + 1).toUpperCase();
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
 * Drop area and file picker for a single contract file.
 *
 * The chosen file is summarised inside the drop area, so the area itself
 * cannot be a `<label>`: the remove and replace controls sitting within it
 * would reopen the picker on the very click meant to clear or swap the
 * selection. The hidden input is opened through a ref instead, and the
 * buttons around it stay real buttons for keyboard and screen-reader users.
 */
const ContractFileControl = ({
  extensions,
  error,
  file,
  onReject,
  onSelect,
}: ContractFileControlProps) => {
  const intl = useIntl();
  const [draggedOver, setDraggedOver] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const helperId = 'contractFile-helper';
  const extensionList = extensions.join(', ');

  const accepts = (candidate: File): boolean =>
    extensions.some((extension) => candidate.name.toLowerCase().endsWith(extension));

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
    return (
      <FormattedMessage
        {...messages.uploadAccepted}
        values={{
          extensions: extensionList,
          maxSize: formatFileSize(intl, MAX_CONTRACT_BYTES),
        }}
      />
    );
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

      <Box
        onClick={file === null ? openPicker : undefined}
        onDragLeave={() => setDraggedOver(false)}
        onDragOver={handleDragOver}
        onDrop={handleDrop}
        sx={(theme) => ({
          alignItems: 'center',
          bgcolor: draggedOver ? 'action.hover' : 'background.default',
          border: hairline(theme),
          borderColor: draggedOver ? 'primary.main' : 'divider',
          borderRadius: 2,
          borderStyle: 'dashed',
          cursor: file === null ? 'pointer' : 'default',
          display: 'flex',
          justifyContent: 'center',
          px: 3,
          py: file === null ? 5 : 3,
        })}
      >
        {file === null ? (
          <Stack spacing={1} sx={{ alignItems: 'center', textAlign: 'center' }}>
            <Box sx={iconTileSx(7)}>
              <Upload size={24} />
            </Box>
            <Typography sx={{ fontWeight: 700, pt: 1 }} variant="h6">
              <FormattedMessage {...messages.uploadTitle} />
            </Typography>
            <Typography color="text.secondary" variant="body2">
              <FormattedMessage {...messages.uploadHint} values={{ extensions: extensionList }} />
            </Typography>
            <Button onClick={openPicker} sx={{ mt: 2 }} variant="contained">
              <FormattedMessage {...messages.uploadAction} />
            </Button>
          </Stack>
        ) : (
          <Stack spacing={1} sx={{ alignItems: 'center', width: '100%' }}>
            <Stack
              direction="row"
              spacing={2}
              sx={(theme) => ({
                alignItems: 'center',
                bgcolor: 'background.paper',
                border: hairline(theme),
                borderColor: 'divider',
                borderRadius: 2,
                maxWidth: theme.spacing(60),
                px: 2,
                py: 1.5,
                width: '100%',
              })}
            >
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
                onClick={() => onReject('removed')}
                size="small"
              >
                <X size={16} />
              </IconButton>
            </Stack>
            <Button onClick={openPicker} sx={{ mt: 1 }} variant="text">
              <FormattedMessage {...messages.uploadReplace} />
            </Button>
          </Stack>
        )}
      </Box>

      <FormHelperText id={helperId}>{helperText}</FormHelperText>
    </FormControl>
  );
};

/** A parsed contract, as loose as it arrives — nothing here inspects it. */
type SpecDocument = Record<string, unknown>;

/** How long the SwaggerHub organization field settles before it is looked up. */
const LOOKUP_DEBOUNCE_MS = 450;

/** How long a contract download may take before it is abandoned. */
const CONTRACT_FETCH_TIMEOUT_MS = 20_000;

/** What the preview pane needs in order to render a fetched contract. */
export type FetchedContract = {
  /** Which flavour of definition it turned out to be. */
  dialect: SpecDialect;
  /**
   * The parsed document. Every source resolves to one, so the preview renders
   * the same thing whether it came off disk or off a URL, and the source view
   * has something to show in both cases.
   */
  spec: SpecDocument;
  /** The source it came from, for whoever consumes this step. */
  values: ContractValues;
  /** Things worth saying about it that didn't stop the import. */
  warnings: SpecIssue[];
};

/** Why a fetch produced nothing to preview. */
export type ContractFetchFailure =
  | 'oversized'
  | 'unreachable'
  | 'unreadable'
  /** The source has no fetching behind it yet, GitHub, SwaggerHub. */
  | 'unsupportedSource';

export type ContractFetchResult =
  | { contract: FetchedContract; status: 'fetched' }
  /** Read and parsed, but not a definition this step can use. */
  | { issues: SpecIssue[]; status: 'invalidSpec' }
  | { status: ContractFetchFailure };

/**
 * The file's text. `Blob.text()` where it exists, `FileReader` otherwise —
 * jsdom (and Safari before 14) ships the reader but not the method, and
 * without the fallback every upload fails as unreadable there.
 */
const readContractText = (file: File): Promise<string> => {
  if (typeof file.text === 'function') {
    return file.text();
  }
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error('The file could not be read.'));
    reader.onload = () => resolve(String(reader.result ?? ''));
    reader.readAsText(file);
  });
};

/** Parses contract text, or `null` when it isn't a document at all. */
const parseContractText = (text: string): SpecDocument | null => {
  // js-yaml reads JSON as well, JSON is a subset of YAML, so one path covers
  // both the .json and the .yaml sources. `load` uses the default schema,
  // which builds plain data only, never arbitrary JS types.
  const parsed: unknown = yaml.load(text);
  if (typeof parsed !== 'object' || parsed === null) {
    return null;
  }
  return parsed as SpecDocument;
};

/**
 * The last gate a parsed document passes: is it an OpenAPI definition this
 * step can preview and create from?
 *
 * Applied to every source, so a file dropped on the upload tab is held to the
 * same standard as one downloaded from a URL.
 */
const acceptSpec = (spec: SpecDocument, values: ContractValues): ContractFetchResult => {
  const validation = validateApiSpec(spec);
  return validation.status === 'valid'
    ? {
        contract: {
          dialect: validation.dialect,
          spec,
          values,
          warnings: validation.warnings,
        },
        status: 'fetched',
      }
    : { issues: validation.issues, status: 'invalidSpec' };
};

/**
 * Downloads and parses a definition. Shared by the URL source and the
 * SwaggerHub source, which differ only in how the location is arrived at.
 */
const fetchDocumentFrom = async (
  url: string,
  values: ContractValues,
): Promise<ContractFetchResult> => {
  let text: string;
  try {
    const response = await fetch(url, {
      headers: {
        Accept: 'application/json, application/yaml, text/yaml, text/plain',
      },
      // Timeouts also read as `unreachable`.
      signal: AbortSignal.timeout(CONTRACT_FETCH_TIMEOUT_MS),
    });
    if (!response.ok) {
      return { status: 'unreachable' };
    }
    // Check length before reading; chunked responses are backstopped later.
    const declaredBytes = Number(response.headers.get('content-length'));
    if (Number.isFinite(declaredBytes) && declaredBytes > MAX_CONTRACT_BYTES) {
      return { status: 'oversized' };
    }
    text = await response.text();
  } catch {
    // Network failure, a timeout, or the host refused the cross-origin read.
    // The reason is developer-facing, so it stays in the console, not the UI.
    return { status: 'unreachable' };
  }
  // Backstop for responses with no declared length.
  if (text.length > MAX_CONTRACT_BYTES) {
    return { status: 'oversized' };
  }
  const spec = parseContractText(text);
  return spec === null ? { status: 'unreadable' } : acceptSpec(spec, values);
};

/**
 * Loads the contract behind `values` into a document the preview can render.
 *
 * A URL is downloaded here rather than left to Swagger UI, so one request
 * serves both the resources and the source view. That does put the browser's
 * CORS rules in the path: a spec host that sends no `Access-Control-Allow-Origin`
 * comes back as `unreachable`.
 */
export const fetchContractForPreview = async (
  values: ContractValues,
): Promise<ContractFetchResult> => {
  switch (values.sourceKey) {
    case 'url': {
      if (values.url === undefined || values.url === '') {
        return { status: 'unreadable' };
      }
      return fetchDocumentFrom(values.url, values);
    }
    case 'file': {
      if (values.file === undefined) {
        return { status: 'unreadable' };
      }
      if (values.file.size > MAX_CONTRACT_BYTES) {
        return { status: 'oversized' };
      }
      try {
        const spec = parseContractText(await readContractText(values.file));
        return spec === null ? { status: 'unreadable' } : acceptSpec(spec, values);
      } catch {
        // Malformed YAML/JSON — js-yaml's own message is developer-facing.
        return { status: 'unreadable' };
      }
    }
    case 'swaggerhub': {
      const { swaggerHubApi, swaggerHubOrganization, swaggerHubVersion } = values;
      if (
        swaggerHubOrganization === undefined ||
        swaggerHubApi === undefined ||
        swaggerHubVersion === undefined
      ) {
        return { status: 'unreadable' };
      }
      return fetchDocumentFrom(
        swaggerHubSpecUrl(swaggerHubOrganization, swaggerHubApi, swaggerHubVersion),
        values,
      );
    }
    case 'github': {
      const { repositoryBranch, repositoryDirectory, repositoryFile, repositoryUrl } = values;
      const ref = repositoryUrl === undefined ? null : parseGitHubRepoUrl(repositoryUrl);
      if (
        ref === null ||
        repositoryBranch === undefined ||
        repositoryDirectory === undefined ||
        repositoryFile === undefined
      ) {
        return { status: 'unreadable' };
      }
      return fetchDocumentFrom(
        rawFileUrl(ref.owner, ref.repo, repositoryBranch, repositoryDirectory, repositoryFile),
        values,
      );
    }
    default: {
      return { status: 'unsupportedSource' };
    }
  }
};

/**
 * Whether two selections name the same contract, comparing only the fields
 * that belong to their shared source.
 *
 * Used both ways round: to tell whether a fetched contract still describes the
 * form, and to tell whether a fetch about to be queued would return what is
 * already in hand.
 */
const isSameContractSource = (
  left: ContractValues | undefined,
  right: ContractValues | undefined,
): boolean => {
  if (left === undefined || right === undefined || left.sourceKey !== right.sourceKey) {
    return false;
  }
  switch (left.sourceKey) {
    case 'url':
      return left.url === right.url;
    case 'file':
      return left.file === right.file;
    case 'github':
      return (
        left.repositoryUrl === right.repositoryUrl &&
        left.repositoryBranch === right.repositoryBranch &&
        left.repositoryDirectory === right.repositoryDirectory &&
        left.repositoryFile === right.repositoryFile
      );
    case 'swaggerhub':
      return (
        left.swaggerHubOrganization === right.swaggerHubOrganization &&
        left.swaggerHubApi === right.swaggerHubApi &&
        left.swaggerHubVersion === right.swaggerHubVersion
      );
  }
};

/** API types this step offers, in the order the map declares them. */
const CONTRACT_API_TYPES: ApiType[] = Object.keys(CONTRACT_SOURCES_BY_API_TYPE)
  .map((key) => API_TYPES.find((apiType) => apiType.key === key))
  .filter((apiType): apiType is ApiType => apiType !== undefined);

const sourcesFor = (apiTypeKey: string): ContractSourceKey[] =>
  CONTRACT_SOURCES_BY_API_TYPE[apiTypeKey] ?? UNIVERSAL_CONTRACT_SOURCES;

const extensionsFor = (apiTypeKey: string): string[] =>
  CONTRACT_FILE_EXTENSIONS[apiTypeKey] ?? CONTRACT_FILE_EXTENSIONS.rest;

export const ContractSourceForm = ({
  apiTypes = CONTRACT_API_TYPES,
  definitionEdited = false,
  initialApiTypeKey = CONTRACT_API_TYPES[0]?.key,
  onAuthorizeGitHub,
  onContractChange,
  onRefreshSwaggerHubOrganizations,
}: ContractSourceFormProps) => {
  const intl = useIntl();

  const [apiTypeKey] = useState(() => initialApiTypeKey ?? apiTypes[0]?.key ?? '');
  const [sourceKey, setSourceKey] = useState<ContractSourceKey>(
    () => sourcesFor(initialApiTypeKey ?? apiTypes[0]?.key ?? '')[0],
  );

  const contractUrl = useContractTextField({ isValid: isValidUrl });
  const repositoryUrl = useContractTextField({ isValid: isGitHubRepositoryUrl });
  const swaggerHubOrganization = useContractTextField();

  /** What GitHub said about the repository currently typed. */
  const [repoLookup, setRepoLookup] = useState<
    | { status: 'idle' }
    | { status: 'searching' }
    | { branches: string[]; defaultBranch: string; status: 'found' }
    | { status: 'notFound' }
    | { status: 'rateLimited' }
    | { status: 'failed' }
  >({ status: 'idle' });
  const [branch, setBranch] = useState('');
  /** Directory from a pasted deep link, resolved after the branch is known. */
  const [linkedPath, setLinkedPath] = useState('');
  const [directory, setDirectory] = useState('/');
  const [contractFile, setContractFile] = useState('');
  const [tree, setTree] = useState<GitHubTree | null>(null);
  const [treeLoading, setTreeLoading] = useState(false);
  const [treeError, setTreeError] = useState<'rateLimited' | 'failed' | null>(null);
  const [directoryDialogOpen, setDirectoryDialogOpen] = useState(false);

  /**
   * What the registry said about the SwaggerHub organization currently typed. `idle`
   * until there is something to look up; the two selects appear only on
   * `found`.
   */
  const [lookup, setLookup] = useState<
    | { status: 'idle' }
    | { status: 'searching' }
    | { apis: SwaggerHubApi[]; status: 'found'; total: number }
    | { status: 'notFound' }
    | { status: 'failed' }
  >({ status: 'idle' });
  const [swaggerHubApiSlug, setSwaggerHubApiSlug] = useState('');
  const [swaggerHubVersion, setSwaggerHubVersion] = useState('');

  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<FileFieldError>(null);

  /**
   * Why the last fetch produced nothing; `null` while nothing has failed. The
   * whole verdict rather than a code, because an invalid definition carries the
   * list of what is wrong with it.
   */
  const [fetchError, setFetchError] = useState<Exclude<
    ContractFetchResult,
    { status: 'fetched' }
  > | null>(null);
  const [fetching, setFetching] = useState(false);
  /**
   * The source a fetch has been asked for, or `null` while none has. Held as
   * state so the request is made by an effect rather than inside the handler
   * that raised it, which keeps a reply the form has since moved on from out
   * of the preview.
   */
  const [request, setRequest] = useState<ContractValues | null>(null);
  /**
   * What the last successful fetch returned. Reported upward only while it
   * still describes what the form holds, editing the URL or swapping the file
   * makes it stale, and the panel takes Next away again until it is re-fetched.
   */
  const [fetched, setFetched] = useState<FetchedContract | null>(null);

  const repositoryRef = useMemo(
    () => parseGitHubRepoUrl(repositoryUrl.value),
    [repositoryUrl.value],
  );
  // Identity of the repository, so the effects below re-run when it changes but
  // not when an unrelated character is typed after a valid URL.
  const repositoryKey =
    repositoryRef === null ? '' : `${repositoryRef.owner}/${repositoryRef.repo}`;

  /** Clears repo-derived state when the repository changes. */
  useEffect(() => {
    setBranch('');
    setLinkedPath('');
    setTree(null);
    setTreeError(null);
    setDirectory('/');
    setContractFile('');
  }, [repositoryKey]);

  /**
   * Looks the repository up as the URL is typed. Same debounce-and-abort shape
   * as the SwaggerHub lookup: one request per pause, and a reply that a later
   * keystroke has already replaced is dropped rather than allowed to land.
   */
  useEffect(() => {
    if (sourceKey !== 'github' || repositoryRef === null) {
      setRepoLookup({ status: 'idle' });
      return;
    }

    const controller = new AbortController();
    setRepoLookup({ status: 'searching' });
    const timer = setTimeout(() => {
      void lookupGitHubRepository(repositoryRef, controller.signal).then((result) => {
        if (controller.signal.aborted) {
          return;
        }
        if (result.status !== 'found') {
          setRepoLookup({ status: result.status });
          return;
        }
        setRepoLookup({
          branches: result.repository.branches,
          defaultBranch: result.repository.defaultBranch,
          status: 'found',
        });
        // A pasted branch link wins over the repo default.
        const linked = resolveGitHubRef(repositoryRef.refSegments, result.repository.branches);
        setBranch(linked?.branch ?? result.repository.defaultBranch);
        setLinkedPath(linked?.path ?? '');
      });
    }, LOOKUP_DEBOUNCE_MS);

    return () => {
      clearTimeout(timer);
      controller.abort();
    };
    // `repositoryKey` rather than the ref object, which is new on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repositoryKey, sourceKey]);

  /**
   * The branch's tree, in one request. Loaded as soon as a branch is known
   * rather than when the dialog opens, so the root directory can be checked for
   * a contract — and its file pre-selected — without anyone having to browse.
   */
  useEffect(() => {
    if (sourceKey !== 'github' || repositoryRef === null || branch === '') {
      // A previous run whose `.then` was cut short by `abort()` never got to
      // clear this, so the guard has to.
      setTreeLoading(false);
      return;
    }

    const controller = new AbortController();
    setTreeLoading(true);
    setTreeError(null);
    void fetchGitHubTree(repositoryRef.owner, repositoryRef.repo, branch, controller.signal).then(
      (result) => {
        if (controller.signal.aborted) {
          return;
        }
        setTreeLoading(false);
        if (result.status !== 'loaded') {
          setTree(null);
          setTreeError(result.status === 'rateLimited' ? 'rateLimited' : 'failed');
          return;
        }
        setTree(result.tree);
        // A pasted deep link decides where to start; otherwise the root.
        const linked = linkedPath === '' ? '/' : `/${linkedPath}`;
        const startAt = result.tree.directories.includes(linked) ? linked : '/';
        setDirectory(startAt);
        setContractFile(contractFilesIn(result.tree, startAt)[0] ?? '');
      },
    );

    return () => controller.abort();
    // `repositoryKey` rather than the ref object. `linkedPath` is always set in
    // the same update as `branch`, so it never fires this on its own.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [branch, linkedPath, repositoryKey, sourceKey]);

  const handleDirectorySelected = (selection: GitHubDirectorySelection) => {
    setDirectory(selection.directory);
    setContractFile(selection.file);
    setDirectoryDialogOpen(false);
  };

  const organizationName = swaggerHubOrganization.value.trim();

  /**
   * Looks the organization up as it is typed, rather than on blur: the two
   * selects are the feedback that the name was right, so waiting for a blur
   * would leave the user staring at a field that looks unfinished.
   *
   * The debounce keeps one keystroke from firing one request, and the abort
   * drops an answer that a later keystroke has already replaced, without it a
   * slow early request can land after a fast later one and overwrite it.
   */
  useEffect(() => {
    if (sourceKey !== 'swaggerhub' || organizationName === '') {
      setLookup({ status: 'idle' });
      return;
    }

    const controller = new AbortController();
    setLookup({ status: 'searching' });
    const timer = setTimeout(() => {
      void lookupSwaggerHubOrganization(organizationName, controller.signal).then((result) => {
        if (controller.signal.aborted) {
          return;
        }
        setLookup(
          result.status === 'found'
            ? {
                apis: result.organization.apis,
                status: 'found',
                total: result.organization.total,
              }
            : { status: result.status },
        );
      });
    }, LOOKUP_DEBOUNCE_MS);

    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [organizationName, sourceKey]);

  const swaggerHubApis = lookup.status === 'found' ? lookup.apis : [];

  // The chosen API decides which versions exist; a selection that no longer
  // appears in the listing is treated as no selection at all.
  const selectedSwaggerHubApi = useMemo(
    () => swaggerHubApis.find((api) => api.slug === swaggerHubApiSlug),
    [swaggerHubApis, swaggerHubApiSlug],
  );

  // A listing that has moved on leaves stale choices behind; drop them so the
  // selects never show a value the user cannot see in the menu.
  useEffect(() => {
    if (swaggerHubApiSlug !== '' && selectedSwaggerHubApi === undefined) {
      setSwaggerHubApiSlug('');
      setSwaggerHubVersion('');
    }
  }, [selectedSwaggerHubApi, swaggerHubApiSlug]);

  const availableSources = sourcesFor(apiTypeKey);
  const extensions = extensionsFor(apiTypeKey);

  const handleSourceChange = (next: ContractSourceKey) => {
    setSourceKey(next);
    setFetchError(null);
  };

  /** An accepted file is a finished selection, so it is read straight away. */
  const handleFileSelect = (next: File) => {
    setFileError(null);
    setFetchError(null);
    setFile(next);
    requestFetch({ apiTypeKey, file: next, sourceKey: 'file' });
  };

  /** Rejects the current file using the provided reason. */
  const handleFileReject = (reason: ContractFileRejection) => {
    setFile(null);
    setFileError(reason === 'unsupported' ? 'unsupported' : null);
  };

  /** What the form holds right now, uncommitted and unvalidated. */
  const currentValues: ContractValues = {
    apiTypeKey,
    file: file ?? undefined,
    repositoryBranch: branch,
    repositoryDirectory: directory,
    repositoryFile: contractFile,
    repositoryUrl: repositoryUrl.value.trim(),
    sourceKey,
    swaggerHubApi: selectedSwaggerHubApi?.slug,
    swaggerHubOrganization: organizationName,
    swaggerHubVersion,
    url: contractUrl.value.trim(),
  };

  /**
   * Asks for `values` to be fetched, unless there is nothing to gain: one is
   * already running, or the contract in hand came from exactly these values.
   * That second case is what keeps a blur on a field nobody edited from
   * re-reading the document; and from discarding an edit made in the preview
   * since it was read.
   */
  const requestFetch = (values: ContractValues) => {
    if (isSameContractSource(fetched?.values, values)) {
      return;
    }
    setRequest(values);
  };

  /**
   * Commits the active source's own fields and reports what it holds, or
   * `null` when they don't pass, the field controls carry those errors, so
   * nothing needs to be raised here.
   */
  const collectValues = (): ContractValues | null => {
    switch (sourceKey) {
      case 'url': {
        return contractUrl.commit()
          ? { apiTypeKey, sourceKey, url: contractUrl.value.trim() }
          : null;
      }
      case 'file': {
        if (file === null) {
          setFileError('required');
          return null;
        }
        return { apiTypeKey, file, sourceKey };
      }
      case 'github': {
        if (!repositoryUrl.commit() || contractFile === '') {
          return null;
        }
        return {
          apiTypeKey,
          repositoryBranch: branch,
          repositoryDirectory: directory,
          repositoryFile: contractFile,
          repositoryUrl: repositoryUrl.value.trim(),
          sourceKey,
        };
      }
      case 'swaggerhub': {
        if (!swaggerHubOrganization.commit()) {
          return null;
        }
        if (selectedSwaggerHubApi === undefined || swaggerHubVersion === '') {
          return null;
        }
        return {
          apiTypeKey,
          sourceKey,
          swaggerHubApi: selectedSwaggerHubApi.slug,
          swaggerHubOrganization: organizationName,
          swaggerHubVersion,
        };
      }
    }
    return null;
  };

  /**
   * Enter in a text field. There is no fetch button to press, so this is the
   * keyboard's way of asking for one without leaving the field first.
   */
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const values = collectValues();
    if (values !== null) {
      requestFetch(values);
    }
  };

  /**
   * Reads whatever was last asked for. An effect rather than an `await` in the
   * handler that asked: a request the form has already moved past is dropped
   * on arrival instead of landing in the preview behind the current one.
   */
  useEffect(() => {
    if (request === null) {
      return;
    }

    let current = true;
    setFetchError(null);
    setFetching(true);
    void fetchContractForPreview(request).then((result) => {
      if (!current) {
        return;
      }
      setFetching(false);
      if (result.status !== 'fetched') {
        setFetchError(result);
        return;
      }
      // Fetched: the effect further down hands it to the panel, which renders
      // it in the preview and unlocks Next.
      setFetched(result.contract);
    });

    return () => {
      current = false;
    };
  }, [request]);

  /**
   * Fills the URL field with the sample and reads it straight away. Clicking
   * the link blurs the field before the value lands, so the blur alone would
   * fetch the value being replaced rather than the sample.
   */
  const fillWithSampleUrl = () => {
    const sample = SAMPLE_CONTRACT_URLS[apiTypeKey] ?? '';
    contractUrl.setValue(sample);
    if (sample !== '') {
      requestFetch({ apiTypeKey, sourceKey: 'url', url: sample });
    }
  };

  /**
   * Authorizing is only worth offering while there is no usable repository:
   * once one resolves, the branch and directory pickers are the work, and a
   * second way in beside them is just noise.
   */
  const repositorySettled = repoLookup.status === 'found';

  /** Whether the GitHub authorize card renders; hidden if handler is missing. */
  const showGitHubAuthorize = !repositorySettled && onAuthorizeGitHub !== undefined;

  /** Why the repository lookup failed, as a sentence; `undefined` when it didn't. */
  const gitHubLookupError = (() => {
    switch (repoLookup.status) {
      case 'notFound':
        return <FormattedMessage {...messages.gitHubRepoNotFound} />;
      case 'rateLimited':
        return <FormattedMessage {...messages.gitHubRateLimited} />;
      case 'failed':
        return <FormattedMessage {...messages.gitHubRepoUnreachable} />;
      default:
        return undefined;
    }
  })();

  /**
   * A failure describes the inputs it was raised against, so touching any of
   * them retires it; otherwise a corrected URL sits under the complaint about
   * the old one.
   */
  useEffect(() => {
    setFetchError(null);
  }, [
    branch,
    contractFile,
    contractUrl.value,
    directory,
    file,
    organizationName,
    repositoryUrl.value,
    swaggerHubApiSlug,
    swaggerHubVersion,
  ]);

  /** Why the last fetch came back empty, as a sentence; `null` when it didn't. */
  const fetchErrorText = (() => {
    if (fetchError?.status === 'invalidSpec') {
      return <SpecIssueList issues={fetchError.issues} />;
    }
    switch (fetchError?.status) {
      case 'oversized':
        return <FormattedMessage {...messages.specOversized} />;
      case 'unreachable':
        return <FormattedMessage {...messages.specUnreachable} />;
      case 'unreadable':
        return <FormattedMessage {...messages.specUnreadable} />;
      case 'unsupportedSource':
        return <FormattedMessage {...messages.specUnsupportedSource} />;
      default:
        return null;
    }
  })();

  /**
   * Whether the fetched contract still matches the form. Compared rather than
   * invalidated on every keystroke: one derived answer beats resetting a flag
   * from four separate change handlers.
   */
  const fetchedIsCurrent = isSameContractSource(fetched?.values, currentValues);

  // Lifted rather than pushed from each change handler: staleness is derived
  // from four inputs, and one effect over the derived answer beats invalidating
  // the same flag from every place a field can change.
  useEffect(() => {
    onContractChange?.(fetchedIsCurrent ? fetched : null);
  }, [fetched, fetchedIsCurrent, onContractChange]);

  return (
    // Still a form, with no button to submit it: Enter in a text field reads
    // the contract without having to leave the field first. Back and Next
    // belong to the panel around this one.
    <Stack component="form" noValidate onSubmit={handleSubmit} spacing={3}>
      <FormControl>
        <ToggleButtonGroup
          aria-label={intl.formatMessage(messages.sourceLabel)}
          exclusive
          onChange={(_event, next: ContractSourceKey | null) => {
            // `exclusive` reports null when the active button is clicked
            // again; keep the current source rather than clearing it.
            if (next !== null) {
              handleSourceChange(next);
            }
          }}
          sx={(theme) => ({
            alignSelf: 'flex-start',
            bgcolor: 'action.hover',
            border: hairline(theme),
            borderColor: 'divider',
            borderRadius: 2,
            gap: 0.5,
            p: 0.5,
            // The grouped class carries MUI's own radius and collapsing
            // margins, which would square off the inner buttons.
            '& .MuiToggleButtonGroup-grouped': {
              border: 0,
              borderRadius: 1.5,
              color: 'text.secondary',
              fontWeight: 600,
              m: 0,
              px: 2,
              py: 0.75,
              textTransform: 'none',
              '&.Mui-selected': {
                bgcolor: 'primary.main',
                color: 'primary.contrastText',
                '&:hover': { bgcolor: 'primary.dark' },
              },
            },
          })}
          value={sourceKey}
        >
          {availableSources.map((candidate) => (
            <ToggleButton key={candidate} value={candidate}>
              {intl.formatMessage(SOURCE_LABELS[candidate])}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
      </FormControl>

      {sourceKey === 'url' ? (
        <Form.Stack spacing={1}>
          <ContractTextControl
            field={contractUrl}
            id="contractUrl"
            invalidMessage={messages.urlInvalid}
            label={intl.formatMessage(messages.urlLabel)}
            // Leaving a valid URL is the whole gesture: the contract is read
            // then, rather than on a button afterwards.
            onCommitted={(url) => requestFetch({ apiTypeKey, sourceKey: 'url', url })}
            placeholder={intl.formatMessage(messages.urlPlaceholder)}
            requiredMessage={messages.urlRequired}
          />
          <SampleLink onClick={fillWithSampleUrl} />
        </Form.Stack>
      ) : null}

      {sourceKey === 'file' ? (
        <ContractFileControl
          error={fileError}
          extensions={extensions}
          file={file}
          onReject={handleFileReject}
          onSelect={handleFileSelect}
        />
      ) : null}

      {sourceKey === 'github' ? (
        <Grid container spacing={2} sx={{ alignItems: 'stretch' }}>
          <Grid size={{ xs: 12, md: showGitHubAuthorize ? 5.5 : 12 }}>
            <Card sx={{ height: '100%' }} variant="outlined">
              <CardContent sx={{ p: 3 }}>
                <Form.Stack spacing={2}>
                  <Stack
                    direction={{ sm: 'row', xs: 'column' }}
                    spacing={2}
                    sx={{ alignItems: 'flex-start' }}
                  >
                    <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                      <ContractTextControl
                        endAdornment={
                          repoLookup.status === 'searching' ? (
                            <InputAdornment position="end">
                              <CircularProgress
                                aria-label={intl.formatMessage(messages.gitHubSearching)}
                                size={16}
                              />
                            </InputAdornment>
                          ) : undefined
                        }
                        externalError={gitHubLookupError}
                        field={repositoryUrl}
                        id="contractRepositoryUrl"
                        invalidMessage={messages.gitHubUrlInvalid}
                        label={intl.formatMessage(messages.gitHubUrlLabel)}
                        placeholder={intl.formatMessage(messages.gitHubUrlPlaceholder)}
                        requiredMessage={messages.gitHubUrlRequired}
                      />
                    </Box>

                    {/* Only once the repository resolved: until then there
                          are no branches to choose between. */}
                    {repoLookup.status === 'found' ? (
                      <FormControl sx={{ minWidth: 180 }}>
                        <InputLabel id="contractRepositoryBranch-label">
                          {intl.formatMessage(messages.gitHubBranchLabel)}
                        </InputLabel>
                        <Select
                          label={intl.formatMessage(messages.gitHubBranchLabel)}
                          labelId="contractRepositoryBranch-label"
                          onChange={(event) => setBranch(event.target.value)}
                          value={branch}
                        >
                          {repoLookup.branches.map((candidate) => (
                            <MenuItem key={candidate} value={candidate}>
                              {candidate}
                            </MenuItem>
                          ))}
                        </Select>
                      </FormControl>
                    ) : null}
                  </Stack>

                  {repoLookup.status === 'found' ? (
                    <FormControl fullWidth>
                      <InputLabel htmlFor="contractRepositoryDirectory">
                        {intl.formatMessage(messages.gitHubDirectoryLabel)}
                      </InputLabel>
                      <OutlinedInput
                        aria-describedby="contractRepositoryDirectory-helper"
                        endAdornment={
                          <InputAdornment position="end">
                            {treeLoading ? (
                              <CircularProgress size={16} />
                            ) : (
                              <IconButton
                                aria-label={intl.formatMessage(messages.gitHubDirectoryEdit)}
                                edge="end"
                                onClick={() => setDirectoryDialogOpen(true)}
                                size="small"
                              >
                                <Pencil size={16} />
                              </IconButton>
                            )}
                          </InputAdornment>
                        }
                        id="contractRepositoryDirectory"
                        label={intl.formatMessage(messages.gitHubDirectoryLabel)}
                        // Chosen in the dialog, never typed: a path that does
                        // not exist in the tree would only fail at fetch.
                        readOnly
                        sx={{ fontFamily: 'monospace' }}
                        value={directory}
                      />
                      <FormHelperText id="contractRepositoryDirectory-helper">
                        {contractFile === '' ? (
                          <FormattedMessage {...messages.gitHubNoContract} />
                        ) : (
                          <FormattedMessage
                            {...messages.gitHubSelectedFile}
                            values={{ file: contractFile }}
                          />
                        )}
                      </FormHelperText>
                    </FormControl>
                  ) : null}

                  <SampleLink onClick={() => repositoryUrl.setValue(SAMPLE_REPOSITORY_URL)} />
                </Form.Stack>
              </CardContent>
            </Card>
          </Grid>

          {showGitHubAuthorize ? (
            <>
              <Grid
                size={{ xs: 12, md: 1 }}
                sx={{
                  alignItems: 'center',
                  display: 'flex',
                  justifyContent: 'center',
                }}
              >
                <Typography color="text.secondary" variant="body2">
                  <FormattedMessage {...messages.gitHubOr} />
                </Typography>
              </Grid>

              <Grid size={{ xs: 12, md: 5.5 }}>
                <Card sx={{ height: '100%' }} variant="outlined">
                  <CardContent sx={{ p: 3 }}>
                    <Stack spacing={2}>
                      <Typography sx={{ fontWeight: 600 }} variant="body1">
                        <FormattedMessage {...messages.gitHubConnectTitle} />
                      </Typography>
                      <Button
                        fullWidth
                        onClick={onAuthorizeGitHub}
                        startIcon={<GitHub size={22} />}
                        sx={{ py: 2, textTransform: 'none' }}
                        type="button"
                        variant="outlined"
                      >
                        <FormattedMessage {...messages.gitHubAuthorize} />
                      </Button>
                    </Stack>
                  </CardContent>
                </Card>
              </Grid>
            </>
          ) : null}
        </Grid>
      ) : null}

      {sourceKey === 'github' ? (
        <GitHubDirectoryDialog
          error={
            treeError === null ? undefined : (
              <FormattedMessage
                {...(treeError === 'rateLimited'
                  ? messages.gitHubRateLimited
                  : messages.gitHubRepoUnreachable)}
              />
            )
          }
          initialDirectory={directory}
          initialFile={contractFile}
          loading={treeLoading}
          onCancel={() => setDirectoryDialogOpen(false)}
          onConfirm={handleDirectorySelected}
          open={directoryDialogOpen}
          tree={tree}
        />
      ) : null}

      {sourceKey === 'swaggerhub' ? (
        <Stack spacing={2}>
          <ToggleButtonGroup
            aria-label={intl.formatMessage(messages.sourceSwaggerHub)}
            exclusive
            onChange={() => undefined} // the handler exists for when `Authorized` is enabled.
            sx={{ alignSelf: 'flex-start' }}
            value="public"
          >
            <ToggleButton sx={{ px: 3, textTransform: 'none' }} value="public">
              <FormattedMessage {...messages.swaggerHubPublic} />
            </ToggleButton>
            <ToggleButton disabled sx={{ px: 3, textTransform: 'none' }} value="authorized">
              <Tooltip title={intl.formatMessage(messages.swaggerHubAuthorizedHint)}>
                <Box component="span">
                  <FormattedMessage {...messages.swaggerHubAuthorized} />
                </Box>
              </Tooltip>
            </ToggleButton>
          </ToggleButtonGroup>

          <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
            <Box sx={{ flexGrow: 1, minWidth: 0 }}>
              <ContractTextControl
                endAdornment={
                  lookup.status === 'searching' ? (
                    <InputAdornment position="end">
                      <CircularProgress
                        aria-label={intl.formatMessage(messages.swaggerHubSearching)}
                        size={16}
                      />
                    </InputAdornment>
                  ) : undefined
                }
                externalError={
                  lookup.status === 'notFound' ? (
                    <FormattedMessage {...messages.swaggerHubNotFound} />
                  ) : lookup.status === 'failed' ? (
                    <FormattedMessage {...messages.swaggerHubLookupFailed} />
                  ) : undefined
                }
                field={swaggerHubOrganization}
                helper={
                  lookup.status === 'found' && lookup.total > lookup.apis.length ? (
                    <FormattedMessage
                      {...messages.swaggerHubPartialListing}
                      values={{
                        shown: lookup.apis.length,
                        total: lookup.total,
                      }}
                    />
                  ) : undefined
                }
                id="contractSwaggerHubOrganization"
                label={intl.formatMessage(messages.swaggerHubOrganizationLabel)}
                placeholder={intl.formatMessage(messages.swaggerHubOrganizationPlaceholder)}
                requiredMessage={messages.swaggerHubOrganizationRequired}
              />
            </Box>
            {onRefreshSwaggerHubOrganizations === undefined ? null : (
              <Tooltip title={intl.formatMessage(messages.swaggerHubRefresh)}>
                <IconButton
                  aria-label={intl.formatMessage(messages.swaggerHubRefresh)}
                  onClick={onRefreshSwaggerHubOrganizations}
                  size="small"
                >
                  <RefreshCw size={18} />
                </IconButton>
              </Tooltip>
            )}
          </Stack>

          {lookup.status === 'found' ? (
            <Stack direction={{ sm: 'row', xs: 'column' }} spacing={2}>
              <FormControl fullWidth required>
                <InputLabel id="contractSwaggerHubApi-label">
                  {intl.formatMessage(messages.swaggerHubApiLabel)}
                </InputLabel>
                <Select
                  label={intl.formatMessage(messages.swaggerHubApiLabel)}
                  labelId="contractSwaggerHubApi-label"
                  onChange={(event) => {
                    setSwaggerHubApiSlug(event.target.value);
                    // The versions belong to the API that was replaced.
                    setSwaggerHubVersion('');
                  }}
                  value={swaggerHubApiSlug}
                >
                  {swaggerHubApis.map((api) => (
                    <MenuItem key={api.slug} value={api.slug}>
                      {api.name}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>

              <FormControl
                // Nothing to choose from until an API names its versions.
                disabled={selectedSwaggerHubApi === undefined}
                fullWidth
                required
              >
                <InputLabel id="contractSwaggerHubVersion-label">
                  {intl.formatMessage(messages.swaggerHubVersionLabel)}
                </InputLabel>
                <Select
                  label={intl.formatMessage(messages.swaggerHubVersionLabel)}
                  labelId="contractSwaggerHubVersion-label"
                  onChange={(event) => setSwaggerHubVersion(event.target.value)}
                  value={swaggerHubVersion}
                >
                  {(selectedSwaggerHubApi?.versions ?? []).map((version) => (
                    <MenuItem key={version} value={version}>
                      {version}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Stack>
          ) : null}
        </Stack>
      ) : null}

      {/* The read starts on its own; on leaving a finished URL, or on
          choosing a file; so this line is the only sign that it is running. */}
      {fetching ? (
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <CircularProgress size={16} />
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.fetching} />
          </Typography>
        </Stack>
      ) : null}

      {/* Fetch errors appear under the active source panel. */}
      {fetchErrorText === null ? null : <Alert severity="error">{fetchErrorText}</Alert>}

      {/* Definition warnings; cleared with the contract, and withdrawn once
          the definition has been edited past the one they were raised on. */}
      {fetchedIsCurrent && !definitionEdited && (fetched?.warnings.length ?? 0) > 0 ? (
        <Alert severity="warning">
          <SpecIssueList issues={fetched?.warnings ?? []} />
        </Alert>
      ) : null}
    </Stack>
  );
};

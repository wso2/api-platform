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
  Button,
  CircularProgress,
  FormControl,
  FormHelperText,
  FormLabel,
  OutlinedInput,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { Zap } from '@wso2/oxygen-ui-icons-react';
import { useRef, useState, type FormEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { pillToggleGroupSx } from '@/theme/receipes';
import { useValidateGraphQLSchema } from '@/api/resources/graphqlApis';
import { isValidUrl } from '../../../utils/developEdit';
import { FileDropzone, type FileDropzoneRejection, type FileFieldError } from '../FileDropzone';
import type { GraphqlResolutionFailure, GraphqlResolvedSchema } from './graphqlSourceTypes';

/** Extensions the upload tab accepts. */
const FILE_EXTENSIONS = ['.graphql', '.gql', '.json'];

/** SDL sample the "Try with Sample Schema" link fills the URL field with. */
const SAMPLE_SDL_URL = 'https://raw.githubusercontent.com/graphql/swapi-graphql/master/schema.graphql';

const messages = defineMessages({
  sampleUrl: {
    id: 'api.create.graphql.urlUpload.action.sampleUrl',
    defaultMessage: 'Try with Sample Schema',
  },
  sourceFile: {
    id: 'api.create.fromContract.source.file',
    defaultMessage: 'Upload',
  },
  sourceLabel: {
    id: 'api.create.graphql.urlUpload.source.label',
    defaultMessage: 'Import the schema from',
  },
  sourceUrl: {
    id: 'api.create.fromContract.source.url',
    defaultMessage: 'URL',
  },
  unresolved: {
    id: 'api.create.graphql.urlUpload.unresolved',
    defaultMessage: 'That schema could not be resolved. Check it is valid GraphQL SDL.',
  },
  urlInvalid: {
    id: 'api.create.fromContract.url.invalid',
    defaultMessage: 'Enter a valid HTTP or HTTPS URL.',
  },
  urlLabel: {
    id: 'api.create.graphql.urlUpload.url.label',
    defaultMessage: 'Schema URL',
  },
  urlPlaceholder: {
    id: 'api.create.graphql.urlUpload.url.placeholder',
    defaultMessage: 'Enter URL for GraphQL Schema here',
  },
  urlRequired: {
    id: 'api.create.graphql.urlUpload.url.required',
    defaultMessage: 'Enter the URL of the SDL file.',
  },
  validating: {
    id: 'api.create.graphql.urlUpload.status.validating',
    defaultMessage: 'Validating schema…',
    description: 'Shown while a URL/file just supplied is being checked, which starts on its own.',
  },
});

type SourceKey = 'url' | 'file';

export type GraphqlUrlUploadFormProps = {
  /** Called with the resolved schema, or `null` once the inputs move on from it. */
  onResolved: (resolved: GraphqlResolvedSchema | null) => void;
  /**
   * Called with the last validation failure's detail, or `null` once cleared —
   * lets `GraphqlSchemaExplorer` show the actual reason instead of its
   * generic empty state. Optional so a caller with no explorer to feed
   * (there is currently only one) isn't forced to wire it.
   */
  onValidationFailed?: (failure: GraphqlResolutionFailure | null) => void;
};

/**
 * "Start with a schema" side of the source step: import SDL from a URL (the
 * backend fetches it, SSRF-guarded) or upload a schema file, checked via the
 * dry-run `/graphql-apis/validate-schema` endpoint.
 *
 * Shares its source toggle styling (`pillToggleGroupSx`) and its upload
 * dropzone (`FileDropzone`) with REST/WebSocket's `ContractSourceForm`, so
 * every creation wizard's source step reads as the same control.
 *
 * Neither source needs a fetch button: leaving a valid URL field checks it
 * (mirroring `ContractSourceForm`'s own URL source), and choosing a file is
 * itself a finished selection, so it is checked the moment it is picked.
 */
export const GraphqlUrlUploadForm = ({
  onResolved,
  onValidationFailed,
}: GraphqlUrlUploadFormProps) => {
  const intl = useIntl();
  const [source, setSource] = useState<SourceKey>('url');
  const [url, setUrl] = useState('');
  const [urlTouched, setUrlTouched] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<FileFieldError>(null);
  const validate = useValidateGraphQLSchema();
  /**
   * The URL a check has already been run for, so leaving and re-entering an
   * unedited field doesn't re-check it. Cleared by `reset()` so a genuinely
   * new attempt (a tab switch, an edit) always re-checks even an unchanged
   * value.
   */
  const checkedUrlRef = useRef<string | null>(null);

  const trimmedUrl = url.trim();
  const urlInvalid = urlTouched && (trimmedUrl === '' || !isValidUrl(trimmedUrl));

  const reset = () => {
    checkedUrlRef.current = null;
    validate.reset();
    onResolved(null);
    onValidationFailed?.(null);
  };

  const handleSourceChange = (next: SourceKey | null) => {
    if (next === null) return;
    setSource(next);
    reset();
  };

  const checkUrl = (target: string) => {
    checkedUrlRef.current = target;
    validate.mutate(
      { metadata: { schemaSource: 'url', sdlUrl: target } },
      {
        onSuccess: (result) => {
          if (result.resolved) {
            onResolved({ schemaSource: 'url', sdl: result.sdl, sdlUrl: target });
            onValidationFailed?.(null);
          } else {
            onResolved(null);
            onValidationFailed?.({ message: result.message, sdlErrors: result.sdlErrors });
          }
        },
        onError: () => {
          onResolved(null);
          onValidationFailed?.(null);
        },
      },
    );
  };

  /** Leaving a valid URL is the whole gesture: it is checked then, rather than on a button afterwards. */
  const commitUrl = () => {
    setUrlTouched(true);
    if (trimmedUrl === '' || !isValidUrl(trimmedUrl) || checkedUrlRef.current === trimmedUrl) return;
    checkUrl(trimmedUrl);
  };

  /** Enter in the URL field checks it without having to leave the field first. */
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (source === 'url') commitUrl();
  };

  /** An accepted file is a finished selection, so it is checked straight away. */
  const handleFileSelect = (next: File) => {
    setFileError(null);
    setFile(next);
    reset();

    validate.mutate(
      { metadata: { schemaSource: 'file' }, sdlFile: next },
      {
        onSuccess: (result) => {
          if (result.resolved) {
            onResolved({ schemaSource: 'file', sdl: result.sdl, sdlFile: next });
            onValidationFailed?.(null);
          } else {
            onResolved(null);
            onValidationFailed?.({ message: result.message, sdlErrors: result.sdlErrors });
          }
        },
        onError: () => {
          onResolved(null);
          onValidationFailed?.(null);
        },
      },
    );
  };

  /** Rejects the current file using the provided reason. */
  const handleFileReject = (reason: FileDropzoneRejection) => {
    setFile(null);
    setFileError(reason === 'unsupported' ? 'unsupported' : null);
    reset();
  };

  const failedToResolve = validate.isSuccess && validate.data.resolved === false;

  return (
    // No fetch button: leaving a valid URL, or choosing a file, checks it —
    // Enter in the URL field is this form's only other way to trigger that.
    <Stack component="form" noValidate onSubmit={handleSubmit} spacing={2}>
      <FormControl>
        <ToggleButtonGroup
          aria-label={intl.formatMessage(messages.sourceLabel)}
          exclusive
          onChange={(_event, next: SourceKey | null) => handleSourceChange(next)}
          sx={pillToggleGroupSx}
          value={source}
        >
          <ToggleButton type="button" value="url">
            <FormattedMessage {...messages.sourceUrl} />
          </ToggleButton>
          <ToggleButton type="button" value="file">
            <FormattedMessage {...messages.sourceFile} />
          </ToggleButton>
        </ToggleButtonGroup>
      </FormControl>

      {source === 'url' ? (
        <Stack spacing={1}>
          <FormControl error={urlInvalid} fullWidth required>
            <FormLabel htmlFor="graphqlSchemaUrl">{intl.formatMessage(messages.urlLabel)}</FormLabel>
            <OutlinedInput
              aria-describedby="graphqlSchemaUrl-helper"
              id="graphqlSchemaUrl"
              onBlur={commitUrl}
              onChange={(event) => {
                setUrl(event.target.value);
                reset();
              }}
              placeholder={intl.formatMessage(messages.urlPlaceholder)}
              sx={{ mt: 0.75 }}
              value={url}
            />
            {urlInvalid ? (
              <FormHelperText id="graphqlSchemaUrl-helper">
                <FormattedMessage
                  {...(trimmedUrl === '' ? messages.urlRequired : messages.urlInvalid)}
                />
              </FormHelperText>
            ) : null}
          </FormControl>
          <Button
            onClick={() => {
              setUrl(SAMPLE_SDL_URL);
              setUrlTouched(false);
              reset();
              checkUrl(SAMPLE_SDL_URL);
            }}
            // Without this, the mousedown that starts the click first blurs
            // the URL field — committing whatever it held before this button
            // replaces it — and the sample would be checked twice.
            onMouseDown={(event) => event.preventDefault()}
            size="small"
            startIcon={<Zap size={16} />}
            sx={{ alignSelf: 'flex-start', textTransform: 'none' }}
            type="button"
            variant="text"
          >
            <FormattedMessage {...messages.sampleUrl} />
          </Button>
        </Stack>
      ) : (
        <FileDropzone
          error={fileError}
          extensions={FILE_EXTENSIONS}
          file={file}
          onReject={handleFileReject}
          onSelect={handleFileSelect}
        />
      )}

      {validate.isPending ? (
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <CircularProgress size={16} />
          <Typography color="text.secondary" variant="body2">
            <FormattedMessage {...messages.validating} />
          </Typography>
        </Stack>
      ) : failedToResolve ? (
        <Typography color="error.main" variant="body2">
          <FormattedMessage {...messages.unresolved} />
        </Typography>
      ) : null}
    </Stack>
  );
};

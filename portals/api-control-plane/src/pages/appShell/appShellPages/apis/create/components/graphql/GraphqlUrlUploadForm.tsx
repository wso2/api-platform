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
  FormControl,
  FormHelperText,
  IconButton,
  InputLabel,
  OutlinedInput,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { Upload, X, Zap } from '@wso2/oxygen-ui-icons-react';
import { useState, type ChangeEvent, type FormEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useValidateGraphQLSchema } from '@/api/resources/graphqlApis';
import { isValidUrl } from '../../../utils/developEdit';
import type { GraphqlResolvedSchema } from './graphqlSourceTypes';

/** Extensions the upload tab accepts. */
const FILE_EXTENSIONS = ['.graphql', '.gql', '.json'];

/** SDL sample the "Try with Sample Endpoint" link fills the URL field with. */
const SAMPLE_SDL_URL = 'https://raw.githubusercontent.com/graphql/swapi-graphql/master/schema.graphql';

const messages = defineMessages({
  fetch: {
    id: 'api.create.graphql.urlUpload.action.fetch',
    defaultMessage: 'Fetch Schema',
  },
  fileRequired: {
    id: 'api.create.graphql.urlUpload.file.required',
    defaultMessage: 'Select a schema file to continue.',
  },
  sampleUrl: {
    id: 'api.create.graphql.urlUpload.action.sampleUrl',
    defaultMessage: 'Try with Sample Endpoint',
  },
  sourceFile: {
    id: 'api.create.graphql.urlUpload.source.file',
    defaultMessage: 'Upload',
  },
  sourceUrl: {
    id: 'api.create.graphql.urlUpload.source.url',
    defaultMessage: 'URL',
  },
  unresolved: {
    id: 'api.create.graphql.urlUpload.unresolved',
    defaultMessage: 'That schema could not be resolved. Check it is valid GraphQL SDL.',
  },
  urlInvalid: {
    id: 'api.create.graphql.urlUpload.url.invalid',
    defaultMessage: 'Enter a valid HTTP or HTTPS URL.',
  },
  urlLabel: {
    id: 'api.create.graphql.urlUpload.url.label',
    defaultMessage: 'Schema URL',
  },
  urlRequired: {
    id: 'api.create.graphql.urlUpload.url.required',
    defaultMessage: 'Enter the URL of the SDL file.',
  },
  urlUploadRemove: {
    id: 'api.create.graphql.urlUpload.file.remove',
    defaultMessage: 'Remove {fileName}',
    description: 'Accessible name for the button that discards the chosen file.',
  },
  urlUploadTitle: {
    id: 'api.create.graphql.urlUpload.file.title',
    defaultMessage: 'Upload a schema file',
  },
});

type SourceKey = 'url' | 'file';

export type GraphqlUrlUploadFormProps = {
  /** Called with the resolved schema, or `null` once the inputs move on from it. */
  onResolved: (resolved: GraphqlResolvedSchema | null) => void;
};

/**
 * "Start with a schema" side of the source step: import SDL from a URL (the
 * backend fetches it, SSRF-guarded) or upload a schema file, checked without
 * leaving the step via the dry-run `/graphql-apis/validate-schema` endpoint.
 */
export const GraphqlUrlUploadForm = ({ onResolved }: GraphqlUrlUploadFormProps) => {
  const intl = useIntl();
  const [source, setSource] = useState<SourceKey>('url');
  const [url, setUrl] = useState('');
  const [urlTouched, setUrlTouched] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<'required' | null>(null);
  const validate = useValidateGraphQLSchema();

  const trimmedUrl = url.trim();
  const urlInvalid = urlTouched && (trimmedUrl === '' || !isValidUrl(trimmedUrl));

  const reset = () => {
    validate.reset();
    onResolved(null);
  };

  const handleSourceChange = (next: SourceKey | null) => {
    if (next === null) return;
    setSource(next);
    reset();
  };

  const handleFileSelect = (event: ChangeEvent<HTMLInputElement>) => {
    const next = event.target.files?.[0] ?? null;
    event.target.value = ''; // lets the same file be picked again after removal
    setFile(next);
    setFileError(null);
    reset();
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (source === 'url') {
      setUrlTouched(true);
      if (trimmedUrl === '' || !isValidUrl(trimmedUrl)) return;

      validate.mutate(
        { metadata: { schemaSource: 'url', sdlUrl: trimmedUrl } },
        {
          onSuccess: (result) =>
            onResolved(
              result.resolved
                ? { schemaSource: 'url', sdl: result.sdl, sdlUrl: trimmedUrl }
                : null,
            ),
          onError: () => onResolved(null),
        },
      );
      return;
    }

    if (file === null) {
      setFileError('required');
      return;
    }
    validate.mutate(
      { metadata: { schemaSource: 'file' }, sdlFile: file },
      {
        onSuccess: (result) =>
          onResolved(result.resolved ? { schemaSource: 'file', sdl: result.sdl, sdlFile: file } : null),
        onError: () => onResolved(null),
      },
    );
  };

  const resolved = validate.data?.resolved === true;
  const failedToResolve = validate.isSuccess && validate.data.resolved === false;

  return (
    <Stack component="form" noValidate onSubmit={handleSubmit} spacing={2}>
      <ToggleButtonGroup
        exclusive
        onChange={(_event, next: SourceKey | null) => handleSourceChange(next)}
        size="small"
        value={source}
      >
        <ToggleButton sx={{ textTransform: 'none' }} value="url">
          <FormattedMessage {...messages.sourceUrl} />
        </ToggleButton>
        <ToggleButton sx={{ textTransform: 'none' }} value="file">
          <FormattedMessage {...messages.sourceFile} />
        </ToggleButton>
      </ToggleButtonGroup>

      {source === 'url' ? (
        <Stack spacing={1}>
          <FormControl error={urlInvalid} fullWidth required>
            <InputLabel htmlFor="graphqlSchemaUrl">{intl.formatMessage(messages.urlLabel)}</InputLabel>
            <OutlinedInput
              id="graphqlSchemaUrl"
              label={intl.formatMessage(messages.urlLabel)}
              onBlur={() => setUrlTouched(true)}
              onChange={(event) => {
                setUrl(event.target.value);
                reset();
              }}
              value={url}
            />
            {urlInvalid ? (
              <FormHelperText>
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
            }}
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
        <FormControl error={fileError !== null} fullWidth>
          <Box
            sx={(theme) => ({
              alignItems: 'center',
              border: `1px dashed ${theme.palette.divider}`,
              borderRadius: 1,
              display: 'flex',
              justifyContent: 'space-between',
              px: 2,
              py: 1.5,
            })}
          >
            <Typography color="text.secondary" noWrap sx={{ minWidth: 0 }} variant="body2">
              {file?.name ?? intl.formatMessage(messages.urlUploadTitle)}
            </Typography>
            {file ? (
              <IconButton
                aria-label={intl.formatMessage(messages.urlUploadRemove, { fileName: file.name })}
                onClick={() => setFile(null)}
                size="small"
              >
                <X size={16} />
              </IconButton>
            ) : (
              <Button component="label" size="small" startIcon={<Upload size={16} />}>
                <FormattedMessage {...messages.sourceFile} />
                <Box
                  accept={FILE_EXTENSIONS.join(',')}
                  component="input"
                  onChange={handleFileSelect}
                  sx={{ display: 'none' }}
                  type="file"
                />
              </Button>
            )}
          </Box>
          {fileError === 'required' ? (
            <FormHelperText>
              <FormattedMessage {...messages.fileRequired} />
            </FormHelperText>
          ) : null}
        </FormControl>
      )}

      {resolved ? null : failedToResolve ? (
        <Typography color="error.main" variant="body2">
          <FormattedMessage {...messages.unresolved} />
        </Typography>
      ) : null}

      <Button
        disabled={resolved}
        loading={validate.isPending}
        sx={{ alignSelf: 'flex-start' }}
        type="submit"
        variant="contained"
      >
        <FormattedMessage {...messages.fetch} />
      </Button>
    </Stack>
  );
};

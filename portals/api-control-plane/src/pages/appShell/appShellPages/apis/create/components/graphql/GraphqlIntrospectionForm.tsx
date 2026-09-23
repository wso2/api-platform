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
  InputLabel,
  OutlinedInput,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Info } from '@wso2/oxygen-ui-icons-react';
import { useState, type FormEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useValidateGraphQLSchema } from '@/api/resources/graphqlApis';
import { isValidUrl } from '../../../utils/developEdit';
import { countNamedTypes, parseGraphQLSdl } from '../../utils/graphqlSchema';
import type { GraphqlResolutionFailure, GraphqlResolvedSchema } from './graphqlSourceTypes';

const messages = defineMessages({
  check: {
    id: 'api.create.graphql.introspection.action.check',
    defaultMessage: 'Fetch',
  },
  disabledHint: {
    id: 'api.create.graphql.introspection.disabledHint',
    defaultMessage: 'If introspection is disabled on the endpoint, the schema stays empty.',
  },
  endpointLabel: {
    id: 'api.create.defineApi.scratch.endpoint.heading',
    defaultMessage: 'Backend endpoint',
  },
  endpointRequired: {
    id: 'api.create.graphql.introspection.endpoint.required',
    defaultMessage: 'Enter the GraphQL endpoint to introspect.',
  },
  endpointInvalid: {
    id: 'api.create.fromContract.url.invalid',
    defaultMessage: 'Enter a valid HTTP or HTTPS URL.',
  },
  status: {
    id: 'api.create.graphql.introspection.status',
    defaultMessage: 'Introspection enabled · SDL fetched · {typeCount, plural, one {# type} other {# types}}',
  },
  unresolved: {
    id: 'api.create.graphql.introspection.unresolved',
    defaultMessage:
      'Could not derive a schema from that endpoint. Check the URL, and that introspection is enabled.',
  },
});

export type GraphqlIntrospectionFormProps = {
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
 * "Design from scratch" side of the source step: a backend endpoint the
 * gateway introspects to derive its starting schema, checked without leaving
 * the step via the dry-run `/graphql-apis/validate-schema` endpoint.
 */
export const GraphqlIntrospectionForm = ({
  onResolved,
  onValidationFailed,
}: GraphqlIntrospectionFormProps) => {
  const intl = useIntl();
  const [endpoint, setEndpoint] = useState('');
  const [touched, setTouched] = useState(false);
  const validate = useValidateGraphQLSchema();

  const trimmed = endpoint.trim();
  const invalid = touched && (trimmed === '' || !isValidUrl(trimmed));

  const handleChange = (next: string) => {
    setEndpoint(next);
    validate.reset();
    onResolved(null);
    onValidationFailed?.(null);
  };

  const handleCheck = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setTouched(true);
    if (trimmed === '' || !isValidUrl(trimmed)) return;

    validate.mutate(
      { metadata: { schemaSource: 'introspection', upstream: { main: { url: trimmed } } } },
      {
        onSuccess: (result) => {
          if (result.resolved) {
            onResolved({ endpointUrl: trimmed, schemaSource: 'introspection', sdl: result.sdl });
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

  const resolved = validate.data?.resolved === true;
  const failedToResolve = validate.isSuccess && validate.data.resolved === false;

  return (
    <Stack component="form" noValidate onSubmit={handleCheck} spacing={2}>
      <Stack direction="row" spacing={1.5}>
        <FormControl error={invalid} fullWidth required>
          <InputLabel htmlFor="graphqlIntrospectionEndpoint">
            {intl.formatMessage(messages.endpointLabel)}
          </InputLabel>
          <OutlinedInput
            id="graphqlIntrospectionEndpoint"
            label={intl.formatMessage(messages.endpointLabel)}
            onBlur={() => setTouched(true)}
            onChange={(event) => handleChange(event.target.value)}
            value={endpoint}
          />
          {invalid ? (
            <FormHelperText>
              <FormattedMessage
                {...(trimmed === '' ? messages.endpointRequired : messages.endpointInvalid)}
              />
            </FormHelperText>
          ) : null}
        </FormControl>
        <Button
          disabled={resolved}
          loading={validate.isPending}
          sx={{ flexShrink: 0 }}
          type="submit"
          variant="outlined"
        >
          <FormattedMessage {...messages.check} />
        </Button>
      </Stack>

      {resolved && validate.data ? (
        <Typography color="success.main" sx={{ alignItems: 'center', display: 'flex', gap: 1 }} variant="body2">
          <FormattedMessage {...messages.status} values={{ typeCount: typeCountOf(validate.data.sdl) }} />
        </Typography>
      ) : failedToResolve ? (
        <Typography color="error.main" variant="body2">
          <FormattedMessage {...messages.unresolved} />
        </Typography>
      ) : null}

      <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-start', pt: 1.5 }}>
        <Box sx={{ color: 'text.disabled', display: 'flex' }}>
          <Info size={18} />
        </Box>
        <Typography color="text.secondary" variant="caption">
          <FormattedMessage {...messages.disabledHint} />
        </Typography>
      </Stack>
    </Stack>
  );
};

/** Type count for the status line, parsed with `graphql-js` rather than a regex heuristic. */
function typeCountOf(sdl: string): number {
  const parsed = parseGraphQLSdl(sdl);
  return 'schema' in parsed ? countNamedTypes(parsed.schema) : 0;
}

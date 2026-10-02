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
  FormLabel,
  OutlinedInput,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Info, Zap } from '@wso2/oxygen-ui-icons-react';
import { useEffect, useRef, useState } from 'react';
import { defineMessages, FormattedMessage } from 'react-intl';

import { useValidateGraphQLSchema } from '@/api/resources/graphqlApis';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { isValidUrl } from '../../../utils/developEdit';
import { countNamedTypes, parseGraphQLSdl } from '../../utils/graphqlSchema';
import type { GraphqlResolvedSchema } from './graphqlSourceTypes';

const messages = defineMessages({
  checking: {
    id: 'api.create.graphql.introspection.checking',
    defaultMessage: 'Checking the endpoint for a schema…',
    description: 'Status line while the endpoint is being introspected in the background.',
  },
  disabledHint: {
    id: 'api.create.graphql.introspection.disabledHint',
    defaultMessage: 'If introspection is disabled on the endpoint, the schema stays empty.',
  },
  // Heading and label text/ids are shared verbatim with DefineApiPanel's own
  // (REST) "Backend endpoint" block — same copy, so same id, so translations
  // aren't duplicated across the two wizards. The description differs: REST's
  // "route every resource" doesn't hold for GraphQL, which has no per-resource
  // routing at all (see GraphqlPolicyPanel's own "a GraphQL API has a single
  // endpoint" copy) — every operation goes through this one endpoint, so this
  // gets its own id rather than reusing REST's.
  endpointHeading: {
    id: 'api.create.defineApi.scratch.endpoint.heading',
    defaultMessage: 'Backend endpoint',
  },
  endpointDescription: {
    id: 'api.create.graphql.introspection.endpoint.description',
    defaultMessage:
      'Every operation goes through this single endpoint to a running service. Calls are proxied through as soon as you deploy.',
  },
  endpointLabel: {
    id: 'api.create.defineApi.scratch.endpoint.label',
    defaultMessage: 'Endpoint URL',
  },
  endpointRequired: {
    id: 'api.create.graphql.introspection.endpoint.required',
    defaultMessage: 'Enter the GraphQL endpoint to introspect.',
  },
  endpointInvalid: {
    id: 'api.create.fromContract.url.invalid',
    defaultMessage: 'Enter a valid HTTP or HTTPS URL.',
  },
  // Same id/text as REST's own sample-endpoint link — GraphqlUrlUploadForm's
  // sibling "Try with Sample Schema" link is for SDL, a different sample kind,
  // so it keeps its own separate id/text.
  sampleUrl: {
    id: 'api.create.defineApi.scratch.endpoint.sampleUrl',
    defaultMessage: 'Try with Sample URL',
  },
  status: {
    id: 'api.create.graphql.introspection.status',
    defaultMessage: 'Introspection enabled · SDL fetched · {typeCount, plural, one {# type} other {# types}}',
  },
  unresolved: {
    id: 'api.create.graphql.introspection.unresolved',
    defaultMessage:
      'Could not derive a schema from that endpoint — introspection may be disabled. You can still continue; the API starts with an empty schema.',
  },
});

/**
 * A public, introspectable GraphQL endpoint the "Try with Sample URL" link
 * fills the field with — Platform API's own generated-example endpoint for
 * `DeployGraphQLAPI`/`ValidateGraphQLSchema` (see `platform.d.ts`), so it's
 * already the documented reference example rather than a one-off pick.
 */
const SAMPLE_ENDPOINT_URL = 'https://countries.trevorblades.com/graphql';

/** How long typing must settle before the endpoint is introspected. */
const CHECK_DEBOUNCE_MS = 500;

/** The background introspection check, tied to the endpoint it was run for. */
type CheckState =
  | { target: string; status: 'checking' | 'failed' | 'unresolved' }
  | { target: string; status: 'resolved'; sdl: string; typeCount: number };

export type GraphqlIntrospectionFormProps = {
  /** Called with the resolved schema, or `null` once the inputs move on from it. */
  onResolved: (resolved: GraphqlResolvedSchema | null) => void;
};

/**
 * "Start from scratch" side of the source step — mirrors REST's own: a valid
 * backend endpoint is all Continue needs. The gateway derives the schema by
 * introspecting it at create time, best-effort, so an endpoint with
 * introspection disabled still creates an API, just with an empty schema.
 *
 * Meanwhile the endpoint is introspected in the background through the dry-run
 * `/graphql-apis/validate-schema` call once typing settles, purely to preview
 * the schema in the explorer: a schema it finds is added to what's reported.
 * Finding none — usually just introspection being disabled — is explained in
 * the status line under the field, and the explorer keeps its initial empty
 * state rather than showing an error for a perfectly usable endpoint. Neither
 * outcome gates Continue.
 */
export const GraphqlIntrospectionForm = ({ onResolved }: GraphqlIntrospectionFormProps) => {
  const [endpoint, setEndpoint] = useState('');
  const [touched, setTouched] = useState(false);
  const [check, setCheck] = useState<CheckState | null>(null);
  const { mutate } = useValidateGraphQLSchema();

  const trimmed = endpoint.trim();
  const valid = trimmed !== '' && isValidUrl(trimmed);
  const invalid = touched && !valid;

  // The endpoint the latest result must belong to; a check still in flight
  // when the field changes resolves for an endpoint no longer on screen.
  const latestEndpoint = useRef(trimmed);
  latestEndpoint.current = trimmed;

  // Only a check for the endpoint currently in the field says anything about it.
  const current = check?.target === trimmed ? check : null;
  const currentSdl = current?.status === 'resolved' ? current.sdl : undefined;

  // What the wizard gets is derived from the field and the check for exactly
  // that endpoint: a valid endpoint is a usable source on its own, before (and
  // whatever) the background check finds; a schema it finds is added.
  useEffect(() => {
    onResolved(
      valid
        ? {
            endpointUrl: trimmed,
            schemaSource: 'introspection',
            ...(currentSdl === undefined ? {} : { sdl: currentSdl }),
          }
        : null,
    );
  }, [currentSdl, onResolved, trimmed, valid]);

  const checkTarget = useDebouncedValue(valid ? trimmed : '', CHECK_DEBOUNCE_MS);

  useEffect(() => {
    if (checkTarget === '') return;
    const target = checkTarget;
    const stale = () => latestEndpoint.current !== target;

    setCheck({ status: 'checking', target });
    mutate(
      { metadata: { schemaSource: 'introspection', upstream: { main: { url: target } } } },
      {
        onSuccess: (result) => {
          if (stale()) return;
          setCheck(
            result.resolved
              ? { sdl: result.sdl, status: 'resolved', target, typeCount: typeCountOf(result.sdl) }
              : { status: 'unresolved', target },
          );
        },
        onError: () => {
          if (stale()) return;
          setCheck({ status: 'failed', target });
        },
      },
    );
  }, [checkTarget, mutate]);

  /** Fills the field with a known-good endpoint; the background check picks it up. */
  const handleSample = () => {
    setEndpoint(SAMPLE_ENDPOINT_URL);
    setTouched(false);
  };

  return (
    <Stack spacing={2}>
      <Box>
        <Typography sx={{ fontWeight: 700 }} variant="h3">
          <FormattedMessage {...messages.endpointHeading} />
        </Typography>
        <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
          <FormattedMessage {...messages.endpointDescription} />
        </Typography>
      </Box>
      <FormControl error={invalid} fullWidth required>
        <FormLabel htmlFor="graphqlIntrospectionEndpoint">
          <FormattedMessage {...messages.endpointLabel} />
        </FormLabel>
        <OutlinedInput
          id="graphqlIntrospectionEndpoint"
          onBlur={() => setTouched(true)}
          onChange={(event) => setEndpoint(event.target.value)}
          sx={{ mt: 0.75 }}
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
        onClick={handleSample}
        size="small"
        startIcon={<Zap size={16} />}
        sx={{ alignSelf: 'flex-start', px: 0, textTransform: 'none' }}
        type="button"
        variant="text"
      >
        <FormattedMessage {...messages.sampleUrl} />
      </Button>

      {current?.status === 'checking' ? (
        <Typography color="text.secondary" variant="body2">
          <FormattedMessage {...messages.checking} />
        </Typography>
      ) : current?.status === 'resolved' ? (
        <Typography color="success.main" sx={{ alignItems: 'center', display: 'flex', gap: 1 }} variant="body2">
          <FormattedMessage {...messages.status} values={{ typeCount: current.typeCount }} />
        </Typography>
      ) : current?.status === 'unresolved' ? (
        <Typography color="warning.main" variant="body2">
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

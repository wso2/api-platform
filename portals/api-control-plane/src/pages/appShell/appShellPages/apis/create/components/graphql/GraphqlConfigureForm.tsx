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
  Divider,
  Form,
  FormControl,
  FormHelperText,
  FormLabel,
  Grid,
  InputAdornment,
  OutlinedInput,
  Paper,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { CircleCheck } from '@wso2/oxygen-ui-icons-react';
import type { FormEvent, ReactNode } from 'react';
import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { useGraphQLApiIdAvailability } from '@/api/resources/graphqlApis';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { versionLabel as toVersionSegment } from '@/utils/versionLabel';
import {
  CONTEXT_PATTERN,
  HANDLE_MAX_LENGTH,
  HANDLE_PATTERN,
  isHttpUrl,
  VERSION_PATTERN,
} from '../../../utils/basicInfoRules';
import type { CreateApiFormErrors, CreateApiFormField } from '../../utils/serverFieldErrors';
import type { GraphqlApiCreationFormState, GraphqlCreationWizardDraftState } from '../../types';

export type GraphqlConfigureFormProps = {
  formId?: string;
  hideActions?: boolean;
  initialValues?: GraphqlCreationWizardDraftState;
  onSubmit: (values: GraphqlApiCreationFormState) => void;
  onBack: () => void;
  /** Why the last submission was rejected, when there was one. See `GeneralCreateApiForm`. */
  serverErrors?: CreateApiFormErrors;
};

const messages = defineMessages({
  back: {
    id: 'api.create.graphql.configureForm.action.back',
    defaultMessage: 'Back',
  },
  basicInformation: {
    id: 'api.create.graphql.configureForm.section.basicInformation',
    defaultMessage: 'Basic information',
  },
  contextErrorPattern: {
    id: 'api.create.graphql.configureForm.context.error.pattern',
    defaultMessage: 'Start with / and use only letters, numbers, hyphens, dots and slashes.',
  },
  contextErrorRequired: {
    id: 'api.create.graphql.configureForm.context.error.required',
    defaultMessage: 'Enter a context.',
  },
  contextHelper: {
    id: 'api.create.graphql.configureForm.context.helper',
    defaultMessage:
      'Built from the identifier and version. All operations are served from this single path.',
  },
  contextLabel: {
    id: 'api.create.graphql.configureForm.context.label',
    defaultMessage: 'Context',
  },
  create: {
    id: 'api.create.graphql.configureForm.action.create',
    defaultMessage: 'Create',
  },
  descriptionLabel: {
    id: 'api.create.graphql.configureForm.description.label',
    defaultMessage: 'Description',
  },
  endpointHelper: {
    id: 'api.create.graphql.configureForm.endpoint.helper',
    defaultMessage: 'The GraphQL backend the gateway routes POST requests to.',
  },
  endpointLabel: {
    id: 'api.create.graphql.configureForm.endpoint.label',
    defaultMessage: 'Query and Mutation URL',
  },
  endpointSection: {
    id: 'api.create.graphql.configureForm.section.endpoint',
    defaultMessage: 'Endpoint',
  },
  endpointErrorInvalid: {
    id: 'api.create.graphql.configureForm.endpoint.error.invalid',
    defaultMessage: 'Enter a full URL, for example https://api.example.com/graphql.',
  },
  endpointErrorRequired: {
    id: 'api.create.graphql.configureForm.endpoint.error.required',
    defaultMessage: 'Enter the GraphQL endpoint URL.',
  },
  identifierErrorPattern: {
    id: 'api.create.graphql.configureForm.identifier.error.pattern',
    defaultMessage: 'Use lowercase letters and numbers, separated by single hyphens.',
  },
  identifierErrorRequired: {
    id: 'api.create.graphql.configureForm.identifier.error.required',
    defaultMessage: 'Enter an identifier.',
  },
  identifierErrorTooLong: {
    id: 'api.create.graphql.configureForm.identifier.error.tooLong',
    defaultMessage: 'Use {max} characters or fewer.',
  },
  identifierHelper: {
    id: 'api.create.graphql.configureForm.identifier.helper',
    defaultMessage: 'URL-friendly. Generated from the name until you change it.',
  },
  identifierLabel: {
    id: 'api.create.graphql.configureForm.identifier.label',
    defaultMessage: 'Identifier',
  },
  identifierStatusAvailable: {
    id: 'api.create.graphql.configureForm.identifier.status.available',
    defaultMessage: 'Available.',
  },
  identifierStatusAvailableIcon: {
    id: 'api.create.graphql.configureForm.identifier.status.availableIcon',
    defaultMessage: 'Identifier is available',
    description: 'Accessible label for the tick shown beside a free identifier.',
  },
  identifierStatusChecking: {
    id: 'api.create.graphql.configureForm.identifier.status.checking',
    defaultMessage: 'Checking whether this identifier is free…',
  },
  nameErrorRequired: {
    id: 'api.create.graphql.configureForm.name.error.required',
    defaultMessage: 'Enter a name.',
  },
  nameLabel: {
    id: 'api.create.graphql.configureForm.name.label',
    defaultMessage: 'Name',
  },
  rejectedTitle: {
    id: 'api.create.graphql.configureForm.rejected.title',
    defaultMessage: 'We could not create this API proxy',
  },
  versionErrorPattern: {
    id: 'api.create.graphql.configureForm.version.error.pattern',
    defaultMessage: 'Use letters, numbers, dots, hyphens and underscores — no spaces or slashes.',
  },
  versionErrorRequired: {
    id: 'api.create.graphql.configureForm.version.error.required',
    defaultMessage: 'Enter a version.',
  },
  versionHelper: {
    id: 'api.create.graphql.configureForm.version.helper',
    defaultMessage: 'e.g. 1.0',
  },
  versionLabel: {
    id: 'api.create.graphql.configureForm.version.label',
    defaultMessage: 'Version',
  },
});

const DEFAULT_FORM_STATE: GraphqlApiCreationFormState = {
  id: '',
  displayName: '',
  description: '',
  version: '1.0',
  context: '',
  endpointUrl: '',
  schemaSource: 'introspection',
};

/** Display name → URL-friendly handle. Identical to the REST configure form's. */
const toHandle = (displayName: string): string =>
  displayName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, HANDLE_MAX_LENGTH);

/**
 * The routing context this platform gives a GraphQL API: all operations are
 * served from one path (`/{api-handler}/v{version}/graphql`), unlike REST's
 * project-prefixed base path — a GraphQL API's project scope comes from
 * `projectId`, not the URL. The trailing `graphql` segment matches the
 * platform-api spec's own documented (if unenforced) convention for a
 * GraphQL endpoint's context, the same way the gateway always reserves
 * `/mcp` for an MCP proxy's resource path.
 */
const toContext = (apiHandle: string, version: string): string => {
  const segments = [
    apiHandle.trim(),
    version.trim() === '' ? undefined : toVersionSegment(version.trim()),
    'graphql',
  ].filter((segment): segment is string => Boolean(segment));

  return `/${segments.join('/')}`;
};

const getInitialValues = (
  draftData: GraphqlCreationWizardDraftState,
): GraphqlApiCreationFormState => {
  const merged = { ...DEFAULT_FORM_STATE, ...draftData };
  const id = merged.id.trim() === '' ? toHandle(merged.displayName) : merged.id;

  return {
    ...merged,
    id,
    context: merged.context.trim() === '' ? toContext(id, merged.version) : merged.context,
  };
};

/**
 * Reuses the REST form's field-name union: the server's `upstream.main.url`
 * rejection maps onto `targetUrl` regardless of API kind, and pinning this
 * form's endpoint field to that same key is what lets it reuse
 * `toCreateApiFormErrors` without a GraphQL-specific mapper.
 */
type ValidatedField = CreateApiFormField;

const INPUT_ID: Record<ValidatedField, string> = {
  context: 'graphqlContext',
  displayName: 'graphqlDisplayName',
  id: 'graphqlIdentifier',
  targetUrl: 'graphqlEndpointUrl',
  version: 'graphqlVersion',
};

const FIELD_ORDER: readonly ValidatedField[] = [
  'displayName',
  'id',
  'version',
  'context',
  'targetUrl',
];

const valueOf: Record<ValidatedField, (state: GraphqlApiCreationFormState) => string> = {
  context: (state) => state.context,
  displayName: (state) => state.displayName,
  id: (state) => state.id,
  targetUrl: (state) => state.endpointUrl,
  version: (state) => state.version,
};

type FieldErrors = Partial<Record<ValidatedField, MessageDescriptor>>;

const validate = (state: GraphqlApiCreationFormState): FieldErrors => {
  const errors: FieldErrors = {};

  if (state.displayName.trim() === '') {
    errors.displayName = messages.nameErrorRequired;
  }

  const id = state.id.trim();
  if (id === '') {
    errors.id = messages.identifierErrorRequired;
  } else if (id.length > HANDLE_MAX_LENGTH) {
    errors.id = messages.identifierErrorTooLong;
  } else if (!HANDLE_PATTERN.test(id)) {
    errors.id = messages.identifierErrorPattern;
  }

  const version = state.version.trim();
  if (version === '') {
    errors.version = messages.versionErrorRequired;
  } else if (!VERSION_PATTERN.test(version)) {
    errors.version = messages.versionErrorPattern;
  }

  const context = state.context.trim();
  if (context === '' || context === '/') {
    errors.context = messages.contextErrorRequired;
  } else if (!CONTEXT_PATTERN.test(context)) {
    errors.context = messages.contextErrorPattern;
  }

  const endpointUrl = state.endpointUrl.trim();
  if (endpointUrl === '') {
    errors.targetUrl = messages.endpointErrorRequired;
  } else if (!isHttpUrl(endpointUrl)) {
    errors.targetUrl = messages.endpointErrorInvalid;
  }

  return errors;
};

/** The GraphQL wizard's "Configure and create" step — screen 05 of the design. */
export const GraphqlConfigureForm = (props: GraphqlConfigureFormProps) => {
  const intl = useIntl();

  const [submittedState] = useState<GraphqlApiCreationFormState>(() =>
    getInitialValues(props.initialValues ?? {}),
  );
  const [formState, setFormState] = useState<GraphqlApiCreationFormState>(submittedState);

  const [identifierEdited, setIdentifierEdited] = useState(
    () => (props.initialValues?.id ?? '').trim() !== '',
  );
  const [contextEdited, setContextEdited] = useState(
    () => (props.initialValues?.context ?? '').trim() !== '',
  );
  const [touched, setTouched] = useState<Partial<Record<ValidatedField, boolean>>>({});

  const errors = validate(formState);

  const handle = formState.id.trim().toLowerCase();
  const probeCandidate = errors.id ? '' : handle;
  const debouncedCandidate = useDebouncedValue(probeCandidate, 400);
  const availability = useGraphQLApiIdAvailability(debouncedCandidate);

  const probeSettled = debouncedCandidate === probeCandidate && !availability.isFetching;
  const isChecking = probeCandidate !== '' && !probeSettled;
  const availabilityAnswered =
    probeCandidate !== '' && probeSettled && availability.data !== undefined;
  const isAvailable = availabilityAnswered && availability.data === true;

  const errorFor = (field: ValidatedField): MessageDescriptor | undefined =>
    touched[field] ? errors[field] : undefined;

  const serverErrorFor = (field: ValidatedField): string | undefined => {
    if (errors[field]) return undefined;
    if (valueOf[field](formState) !== valueOf[field](submittedState)) return undefined;
    return props.serverErrors?.fields[field];
  };

  const fieldErrors = FIELD_ORDER.reduce<Record<ValidatedField, ReactNode | undefined>>(
    (resolved, field) => {
      const descriptor = errorFor(field);
      resolved[field] = descriptor ? <FormattedMessage {...descriptor} /> : serverErrorFor(field);
      return resolved;
    },
    {} as Record<ValidatedField, ReactNode | undefined>,
  );

  const rejectedFields = props.serverErrors?.fields;
  useEffect(() => {
    if (!rejectedFields) return;
    const first = FIELD_ORDER.find((field) => rejectedFields[field] !== undefined);
    if (first) document.getElementById(INPUT_ID[first])?.focus();
  }, [rejectedFields]);

  const markTouched = (field: ValidatedField) =>
    setTouched((current) => ({ ...current, [field]: true }));

  const setField = <K extends keyof GraphqlApiCreationFormState>(
    key: K,
    value: GraphqlApiCreationFormState[K],
  ) => setFormState((current) => ({ ...current, [key]: value }));

  const handleDisplayNameChange = (displayName: string) => {
    setFormState((current) => {
      const id = identifierEdited ? current.id : toHandle(displayName);
      return {
        ...current,
        displayName,
        id,
        context: contextEdited ? current.context : toContext(id, current.version),
      };
    });
  };

  const handleIdentifierChange = (id: string) => {
    setIdentifierEdited(id.trim() !== '');
    setFormState((current) => ({
      ...current,
      id,
      context: contextEdited ? current.context : toContext(id, current.version),
    }));
  };

  const handleVersionChange = (version: string) => {
    setFormState((current) => ({
      ...current,
      version,
      context: contextEdited ? current.context : toContext(current.id, version),
    }));
  };

  const handleContextChange = (context: string) => {
    setContextEdited(context.trim() !== '');
    setField('context', context);
  };

  const onFormSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (Object.keys(errors).length > 0) {
      setTouched({ context: true, displayName: true, id: true, targetUrl: true, version: true });
      return;
    }

    props.onSubmit(formState);
  };

  const pinnedFieldCount = Object.keys(props.serverErrors?.fields ?? {}).length;
  const showRejection =
    props.serverErrors !== undefined &&
    (pinnedFieldCount === 0 ||
      props.serverErrors.unmapped.length > 0 ||
      FIELD_ORDER.some((field) => serverErrorFor(field) !== undefined));

  const nameLabel = intl.formatMessage(messages.nameLabel);
  const identifierLabel = intl.formatMessage(messages.identifierLabel);
  const versionLabel = intl.formatMessage(messages.versionLabel);
  const contextLabel = intl.formatMessage(messages.contextLabel);
  const descriptionLabel = intl.formatMessage(messages.descriptionLabel);
  const endpointLabel = intl.formatMessage(messages.endpointLabel);

  return (
    <Stack component="form" id={props.formId} noValidate spacing={3} onSubmit={onFormSubmit}>
      {showRejection && (
        <Alert severity="error">
          <Typography sx={{ fontWeight: 600 }} variant="body2">
            <FormattedMessage {...messages.rejectedTitle} />
          </Typography>
          {props.serverErrors?.message && (
            <Typography variant="body2">{props.serverErrors.message}</Typography>
          )}
          {props.serverErrors && props.serverErrors.unmapped.length > 0 && (
            <Box component="ul" sx={{ m: 0, mt: 1, pl: 2.5 }}>
              {props.serverErrors.unmapped.map((message) => (
                <Typography component="li" key={message} variant="body2">
                  {message}
                </Typography>
              ))}
            </Box>
          )}
        </Alert>
      )}

      <Paper component="section" sx={{ p: 3 }}>
        <Typography sx={{ fontWeight: 600 }} variant="body2">
          <FormattedMessage {...messages.basicInformation} />
        </Typography>

        <Form.Stack spacing={2} sx={{ mt: 1.5 }}>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl error={Boolean(fieldErrors.displayName)} fullWidth required>
                <FormLabel htmlFor={INPUT_ID.displayName}>{nameLabel}</FormLabel>
                <OutlinedInput
                  aria-describedby="graphqlDisplayName-error"
                  id={INPUT_ID.displayName}
                  onBlur={() => markTouched('displayName')}
                  onChange={(event) => handleDisplayNameChange(event.target.value)}
                  sx={{ mt: 0.75 }}
                  value={formState.displayName}
                />
                <FormHelperText id="graphqlDisplayName-error">
                  {fieldErrors.displayName}
                </FormHelperText>
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl error={Boolean(fieldErrors.id)} fullWidth required>
                <FormLabel htmlFor={INPUT_ID.id}>{identifierLabel}</FormLabel>
                <OutlinedInput
                  aria-describedby="graphqlIdentifier-error"
                  endAdornment={
                    <InputAdornment position="end">
                      {isChecking ? <CircularProgress size={16} /> : null}
                      {isAvailable ? (
                        <Box
                          aria-label={intl.formatMessage(messages.identifierStatusAvailableIcon)}
                          role="img"
                          sx={{ color: 'success.main', display: 'flex' }}
                        >
                          <CircleCheck size={18} />
                        </Box>
                      ) : null}
                    </InputAdornment>
                  }
                  id={INPUT_ID.id}
                  onBlur={() => markTouched('id')}
                  onChange={(event) => handleIdentifierChange(event.target.value)}
                  sx={{ mt: 0.75 }}
                  value={formState.id}
                />
                <FormHelperText
                  id="graphqlIdentifier-error"
                  sx={isAvailable ? { color: 'success.main' } : undefined}
                >
                  {fieldErrors.id ??
                    (isChecking ? (
                      <FormattedMessage {...messages.identifierStatusChecking} />
                    ) : isAvailable ? (
                      <FormattedMessage {...messages.identifierStatusAvailable} />
                    ) : (
                      <FormattedMessage {...messages.identifierHelper} />
                    ))}
                </FormHelperText>
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl error={Boolean(fieldErrors.version)} fullWidth required>
                <FormLabel htmlFor={INPUT_ID.version}>{versionLabel}</FormLabel>
                <OutlinedInput
                  aria-describedby="graphqlVersion-error"
                  id={INPUT_ID.version}
                  onBlur={() => markTouched('version')}
                  onChange={(event) => handleVersionChange(event.target.value)}
                  sx={{ mt: 0.75 }}
                  value={formState.version}
                />
                <FormHelperText id="graphqlVersion-error">
                  {fieldErrors.version ?? <FormattedMessage {...messages.versionHelper} />}
                </FormHelperText>
              </FormControl>
            </Grid>
          </Grid>

          <FormControl error={Boolean(fieldErrors.context)} fullWidth required>
            <FormLabel htmlFor={INPUT_ID.context}>{contextLabel}</FormLabel>
            <OutlinedInput
              aria-describedby="graphqlContext-error"
              id={INPUT_ID.context}
              onBlur={() => markTouched('context')}
              onChange={(event) => handleContextChange(event.target.value)}
              sx={{ mt: 0.75 }}
              value={formState.context}
            />
            <FormHelperText id="graphqlContext-error">
              {fieldErrors.context ?? <FormattedMessage {...messages.contextHelper} />}
            </FormHelperText>
          </FormControl>

          <FormControl fullWidth>
            <FormLabel htmlFor="graphqlDescription">{descriptionLabel}</FormLabel>
            <OutlinedInput
              id="graphqlDescription"
              multiline
              onChange={(event) => setField('description', event.target.value)}
              rows={3}
              sx={{ mt: 0.75 }}
              value={formState.description ?? ''}
            />
          </FormControl>
        </Form.Stack>
      </Paper>

      <Paper component="section" sx={{ p: 3, mt: 1 }}>
        <Typography sx={{ fontWeight: 600 }} variant="body2">
          <FormattedMessage {...messages.endpointSection} />
        </Typography>

        <Form.Stack spacing={2} sx={{ mt: 1.5 }}>
          <FormControl error={Boolean(fieldErrors.targetUrl)} fullWidth required>
            <FormLabel htmlFor={INPUT_ID.targetUrl}>{endpointLabel}</FormLabel>
            <OutlinedInput
              aria-describedby="graphqlEndpointUrl-error"
              id={INPUT_ID.targetUrl}
              onBlur={() => markTouched('targetUrl')}
              onChange={(event) => setField('endpointUrl', event.target.value)}
              sx={{ mt: 0.75 }}
              value={formState.endpointUrl}
            />
            <FormHelperText id="graphqlEndpointUrl-error">
              {fieldErrors.targetUrl ?? <FormattedMessage {...messages.endpointHelper} />}
            </FormHelperText>
          </FormControl>
        </Form.Stack>
      </Paper>

      {!props.hideActions && <Divider />}

      {!props.hideActions && (
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'flex-end' }}>
          <Button onClick={props.onBack} type="button" variant="text">
            <FormattedMessage {...messages.back} />
          </Button>
          <Button type="submit" variant="contained">
            <FormattedMessage {...messages.create} />
          </Button>
        </Stack>
      )}
    </Stack>
  );
};

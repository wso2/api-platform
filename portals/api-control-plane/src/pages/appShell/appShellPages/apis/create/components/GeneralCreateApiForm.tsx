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
  Divider,
  Form,
  FormControl,
  FormHelperText,
  Grid,
  InputLabel,
  OutlinedInput,
  Paper,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import type { FormEvent, ReactNode } from 'react';
import { useEffect, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import {
  CONTEXT_PATTERN,
  HANDLE_MAX_LENGTH,
  HANDLE_PATTERN,
  isHttpUrl,
  VERSION_PATTERN,
} from '../../utils/basicInfoRules';
import type { CreateApiFormErrors, CreateApiFormField } from '../utils/serverFieldErrors';
import { ApiCreationWizardDraftState, GeneralApiCreationFormState } from '../types';

export type GeneralCreateApiFormProps = {
  initialValues?: ApiCreationWizardDraftState;
  onSubmit: (values: GeneralApiCreationFormState) => void;
  onBack: () => void;
  /**
   * Why the last submission was rejected, when there was one. Rendered as a
   * summary and pinned to the inputs it names, so the user fixes the problem
   * where they made it rather than reading about it in a toast that has
   * already gone.
   */
  serverErrors?: CreateApiFormErrors;
};

const messages = defineMessages({
  back: {
    id: 'api.create.generalForm.action.back',
    defaultMessage: 'Back',
  },
  rejectedTitle: {
    id: 'api.create.generalForm.rejected.title',
    defaultMessage: 'We could not create this API proxy',
    description: 'Heading of the summary shown when the server rejected the submitted form.',
  },
  contextErrorPattern: {
    id: 'api.create.generalForm.context.error.pattern',
    defaultMessage: 'Start with / and use only letters, numbers, hyphens, dots and slashes.',
  },
  contextErrorRequired: {
    id: 'api.create.generalForm.context.error.required',
    defaultMessage: 'Enter a context.',
  },
  contextHelper: {
    id: 'api.create.generalForm.context.helper',
    defaultMessage:
      'Built from the project, identifier and version. Edit it to route this API somewhere else.',
  },
  contextLabel: {
    id: 'api.create.generalForm.context.label',
    defaultMessage: 'Context',
  },
  basicInformation: {
    id: 'api.create.generalForm.section.basicInformation',
    defaultMessage: 'Basic information',
  },
  create: {
    id: 'api.create.generalForm.action.create',
    defaultMessage: 'Create',
  },
  descriptionLabel: {
    id: 'api.create.generalForm.description.label',
    defaultMessage: 'Description',
  },
  nameErrorRequired: {
    id: 'api.create.generalForm.name.error.required',
    defaultMessage: 'Enter a name.',
  },
  nameLabel: {
    id: 'api.create.generalForm.name.label',
    defaultMessage: 'Name',
  },
  endpointSection: {
    id: 'api.create.generalForm.section.backendEndpoint',
    defaultMessage: 'Backend endpoint',
  },
  identifierErrorPattern: {
    id: 'api.create.generalForm.identifier.error.pattern',
    defaultMessage: 'Use lowercase letters and numbers, separated by single hyphens.',
  },
  identifierErrorRequired: {
    id: 'api.create.generalForm.identifier.error.required',
    defaultMessage: 'Enter an identifier.',
  },
  identifierErrorTaken: {
    id: 'api.create.generalForm.identifier.error.taken',
    defaultMessage: 'This project already has an API with that identifier.',
  },
  identifierErrorTooLong: {
    id: 'api.create.generalForm.identifier.error.tooLong',
    defaultMessage: 'Use {max} characters or fewer.',
  },
  identifierHelper: {
    id: 'api.create.generalForm.identifier.helper',
    defaultMessage: 'URL-friendly. Generated from the display name until you change it.',
  },
  identifierLabel: {
    id: 'api.create.generalForm.identifier.label',
    defaultMessage: 'Identifier',
  },
  identifierStatusAvailable: {
    id: 'api.create.generalForm.identifier.status.available',
    defaultMessage: 'Available. this identifier is free to use.',
  },
  identifierStatusAvailableIcon: {
    id: 'api.create.generalForm.identifier.status.availableIcon',
    defaultMessage: 'Identifier is available',
    description: 'Accessible label for the tick shown beside a free identifier.',
  },
  identifierStatusChecking: {
    id: 'api.create.generalForm.identifier.status.checking',
    defaultMessage: 'Checking whether this identifier is free…',
  },
  subtitle: {
    id: 'api.create.generalForm.subtitle',
    defaultMessage: 'Provide the details to configure and expose your API proxy.',
  },
  targetUrlErrorInvalid: {
    id: 'api.create.generalForm.targetUrl.error.invalid',
    defaultMessage: 'Enter a full URL, for example https://api.example.com.',
  },
  targetUrlErrorRequired: {
    id: 'api.create.generalForm.targetUrl.error.required',
    defaultMessage: 'Enter a target URL.',
  },
  targetUrlHelper: {
    id: 'api.create.generalForm.targetUrl.helper',
    defaultMessage: 'The backend the gateway routes to.',
  },
  targetUrlLabel: {
    id: 'api.create.generalForm.targetUrl.label',
    defaultMessage: 'Target URL',
  },
  title: {
    id: 'api.create.generalForm.title',
    defaultMessage: 'Create an API Proxy',
  },
  versionErrorPattern: {
    id: 'api.create.generalForm.version.error.pattern',
    defaultMessage: 'Use letters, numbers, dots, hyphens and underscores — no spaces or slashes.',
  },
  versionErrorRequired: {
    id: 'api.create.generalForm.version.error.required',
    defaultMessage: 'Enter a version.',
  },
  versionHelper: {
    id: 'api.create.generalForm.version.helper',
    defaultMessage: 'e.g. 1.0.0',
  },
  versionLabel: {
    id: 'api.create.generalForm.version.label',
    defaultMessage: 'Version',
  },
});

/**
 * Small uppercase rule above a group of fields. `Form.Header` is fixed at `h4`,
 * so the size comes from the theme's `overline` typography rather than a
 * font-size literal.
 */
const SECTION_LABEL_SX = {
  color: 'text.secondary',
  typography: 'overline',
} as const;

export const DEFAULT_FORM_STATE: GeneralApiCreationFormState = {
  id: '',
  displayName: '',
  description: '',
  version: '1.0.0',
  context: '',
  readOnly: false,
  kind: 'RestApis',
  lifeCycleStatus: 'CREATED',
  transports: ['http', 'https'],
  upstream: {
    main: { url: '' },
  },
  operations: [],
};

/** Display name → URL-friendly handle. */
const toHandle = (displayName: string): string =>
  displayName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, HANDLE_MAX_LENGTH);

/**
 * The routing base path this platform gives an API:
 * `/{project-handler}/{api-handler}/v{version}`.
 *
 * A segment that isn't known yet is left out rather than rendered as an empty
 * one, so a half-filled form shows `/retail/orders` instead of `/retail//v`.
 */
const toBasePath = (
  projectHandler: string | undefined,
  apiHandle: string,
  version: string,
): string => {
  const segments = [
    projectHandler?.trim(),
    apiHandle.trim(),
    version.trim() === '' ? undefined : `v${version.trim()}`,
  ].filter((segment): segment is string => Boolean(segment));

  return `/${segments.join('/')}`;
};

/**
 * Where the form starts: the wizard's draft over the defaults, with the two
 * derived fields filled in so the first render already shows the handle and
 * base path the API would get.
 *
 * A draft that already carries an identifier or a base path keeps it. That is
 * what makes returning from a failed create restore the user's own edits
 * rather than the values read off the imported document; a fresh import names
 * neither, so it still derives both.
 */
const getInitialValues = (
  draftData: ApiCreationWizardDraftState,
  projectHandler: string | undefined,
): GeneralApiCreationFormState => {
  const merged = {
    ...DEFAULT_FORM_STATE,
    ...draftData,
    upstream: {
      ...DEFAULT_FORM_STATE.upstream,
      ...(draftData.upstream || {}),
    },
  };

  const id = merged.id.trim() === '' ? toHandle(merged.displayName) : merged.id;

  return {
    ...merged,
    id,
    context:
      merged.context.trim() === ''
        ? toBasePath(projectHandler, id, merged.version)
        : merged.context,
  };
};

/**
 * The fields that carry a validation rule. Shared with the server-error mapper
 * so a rejection can only ever name an input that exists.
 */
type ValidatedField = CreateApiFormField;

/** The input each field is rendered as, for focus and `aria-describedby`. */
const INPUT_ID: Record<ValidatedField, string> = {
  context: 'context',
  displayName: 'displayName',
  id: 'identifier',
  targetUrl: 'targetUrl',
  version: 'version',
};

/**
 * Reading order, which is also the order a rejection is announced in — the
 * first field the server complained about is the one that takes focus.
 */
const FIELD_ORDER: readonly ValidatedField[] = [
  'displayName',
  'id',
  'version',
  'context',
  'targetUrl',
];

/**
 * The value each field submits. A server error describes the values that were
 * *sent*, so this is what decides whether one is still worth showing.
 */
const valueOf: Record<ValidatedField, (state: GeneralApiCreationFormState) => string> = {
  context: (state) => state.context,
  displayName: (state) => state.displayName,
  id: (state) => state.id,
  targetUrl: (state) => state.upstream.main.url,
  version: (state) => state.version,
};

type FieldErrors = Partial<Record<ValidatedField, MessageDescriptor>>;

/**
 * Every rule in one pure pass, so the same answer drives the field errors and
 * the submit gate — there is no second, drifting copy of the rules.
 */
const validate = (state: GeneralApiCreationFormState): FieldErrors => {
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

  const targetUrl = state.upstream.main.url.trim();
  if (targetUrl === '') {
    errors.targetUrl = messages.targetUrlErrorRequired;
  } else if (!isHttpUrl(targetUrl)) {
    errors.targetUrl = messages.targetUrlErrorInvalid;
  }

  return errors;
};

export const GeneralCreateApiForm = (props: GeneralCreateApiFormProps) => {
  const intl = useIntl();
  // The base path opens with the project this wizard is running in.
  const { activeScope } = useConsoleScope();
  const projectHandler = activeScope.projectHandler;

  // Lazy initialiser: `getInitialValues` runs once, on mount, instead of on
  // every render only to have its result thrown away.
  const [submittedState] = useState<GeneralApiCreationFormState>(() =>
    getInitialValues(props.initialValues || {}, projectHandler),
  );
  // The form remounts from what was last submitted, so `submittedState` is
  // exactly the payload any `serverErrors` were raised against — which is what
  // lets an edited field drop its server error without tracking dismissals.
  const [formState, setFormState] = useState<GeneralApiCreationFormState>(submittedState);

  // Both fields are generated until the user takes them over. Clearing one
  // hands it back, so there is always a way to return to the default. A draft
  // that arrives with either already filled in is a restored submission, so
  // the field starts out taken over — otherwise the next keystroke in the
  // display name would generate over the top of what was restored.
  const [identifierEdited, setIdentifierEdited] = useState(
    () => (props.initialValues?.id ?? '').trim() !== '',
  );
  const [basePathEdited, setBasePathEdited] = useState(
    () => (props.initialValues?.context ?? '').trim() !== '',
  );

  // Errors are recomputed from state on every render; `touched` decides which
  // of them the user is ready to see, so nothing shouts before it is typed in.
  const [touched, setTouched] = useState<Partial<Record<ValidatedField, boolean>>>({});

  const errors = validate(formState);

  const errorFor = (field: ValidatedField): MessageDescriptor | undefined => {
    return touched[field] ? errors[field] : undefined;
  };

  /**
   * The server's complaint about this field, while it still stands. Editing
   * the value retracts it: the server judged what was sent, and it has no
   * opinion on what the user is typing now. Client rules win where both have
   * something to say.
   */
  const serverErrorFor = (field: ValidatedField): string | undefined => {
    if (errors[field]) return undefined;
    if (valueOf[field](formState) !== valueOf[field](submittedState)) return undefined;
    return props.serverErrors?.fields[field];
  };

  /** One resolved message per field, so each is decided once per render. */
  const fieldErrors = FIELD_ORDER.reduce<Record<ValidatedField, ReactNode | undefined>>(
    (resolved, field) => {
      const descriptor = errorFor(field);
      resolved[field] = descriptor ? <FormattedMessage {...descriptor} /> : serverErrorFor(field);
      return resolved;
    },
    {} as Record<ValidatedField, ReactNode | undefined>,
  );

  // A rejection arrives with the form already rendered and the user's eyes
  // wherever they left them, so move focus to the first field it names.
  const rejectedFields = props.serverErrors?.fields;
  useEffect(() => {
    if (!rejectedFields) return;
    const first = FIELD_ORDER.find((field) => rejectedFields[field] !== undefined);
    if (first) document.getElementById(INPUT_ID[first])?.focus();
  }, [rejectedFields]);

  const markTouched = (field: ValidatedField) =>
    setTouched((current) => ({ ...current, [field]: true }));

  /** Scalar top-level field with nothing derived from it. */
  const setField = <K extends keyof GeneralApiCreationFormState>(
    key: K,
    value: GeneralApiCreationFormState[K],
  ) => {
    setFormState((current) => ({ ...current, [key]: value }));
  };

  const handleDisplayNameChange = (displayName: string) => {
    setFormState((current) => {
      const id = identifierEdited ? current.id : toHandle(displayName);

      return {
        ...current,
        displayName,
        id,
        context: basePathEdited ? current.context : toBasePath(projectHandler, id, current.version),
      };
    });
  };

  const handleIdentifierChange = (id: string) => {
    // An emptied field goes back to following the display name.
    setIdentifierEdited(id.trim() !== '');
    setFormState((current) => ({
      ...current,
      id,
      context: basePathEdited ? current.context : toBasePath(projectHandler, id, current.version),
    }));
  };

  const handleVersionChange = (version: string) => {
    setFormState((current) => ({
      ...current,
      version,
      context: basePathEdited ? current.context : toBasePath(projectHandler, current.id, version),
    }));
  };

  const handleBasePathChange = (context: string) => {
    setBasePathEdited(context.trim() !== '');
    setField('context', context);
  };

  const setMainUpstreamUrl = (url: string) => {
    setFormState((current) => ({
      ...current,
      upstream: {
        ...current.upstream,
        main: { ...current.upstream.main, url },
      },
    }));
  };

  const onFormSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (Object.keys(errors).length > 0) {
      // Reveal every rule at once rather than one field per attempt.
      setTouched({
        context: true,
        displayName: true,
        id: true,
        targetUrl: true,
        version: true,
      });
      return;
    }

    props.onSubmit(formState);
  };

  /**
   * Whether the last rejection is still worth showing. A rejection that named
   * specific fields has served its purpose once every one of them has been
   * edited; one that named none (a conflict the server described in prose)
   * stands until the next attempt, because nothing else carries it.
   */
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
  const targetUrlLabel = intl.formatMessage(messages.targetUrlLabel);

  return (
    <Stack component="form" noValidate spacing={3} onSubmit={onFormSubmit}>
      <Box>
        <Typography sx={{ fontWeight: 700 }} variant="h5">
          <FormattedMessage {...messages.title} />
        </Typography>
        <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
          <FormattedMessage {...messages.subtitle} />
        </Typography>
      </Box>

      {/* `Alert` carries `role="alert"`, so this is announced when it appears
        ,the inputs themselves say which values to change. */}
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
        <Form.Header sx={SECTION_LABEL_SX}>
          <FormattedMessage {...messages.basicInformation} />
        </Form.Header>

        <Form.Stack spacing={2} sx={{ mt: 1.5 }}>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl error={Boolean(fieldErrors.displayName)} fullWidth required>
                <InputLabel htmlFor="displayName">{nameLabel}</InputLabel>
                <OutlinedInput
                  aria-describedby="displayName-error"
                  id="displayName"
                  label={nameLabel}
                  name="displayName"
                  onBlur={() => markTouched('displayName')}
                  onChange={(event) => handleDisplayNameChange(event.target.value)}
                  value={formState.displayName}
                />
                <FormHelperText id="displayName-error">{fieldErrors.displayName}</FormHelperText>
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl error={Boolean(fieldErrors.id)} fullWidth required>
                <InputLabel htmlFor="identifier">{identifierLabel}</InputLabel>
                <OutlinedInput
                  aria-describedby="identifier-error"
                  id="identifier"
                  label={identifierLabel}
                  name="identifier"
                  onBlur={() => markTouched('id')}
                  onChange={(event) => handleIdentifierChange(event.target.value)}
                  value={formState.id}
                />
                {/* The identifier is the field a duplicate-handle rejection
                    lands on, so it needs somewhere to say so. */}
                <FormHelperText id="identifier-error">{fieldErrors.id}</FormHelperText>
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl error={Boolean(fieldErrors.version)} fullWidth required>
                <InputLabel htmlFor="version">{versionLabel}</InputLabel>
                <OutlinedInput
                  aria-describedby="version-error"
                  id="version"
                  label={versionLabel}
                  name="version"
                  onBlur={() => markTouched('version')}
                  onChange={(event) => handleVersionChange(event.target.value)}
                  value={formState.version}
                />
                <FormHelperText id="version-error">{fieldErrors.version}</FormHelperText>
              </FormControl>
            </Grid>
          </Grid>

          <FormControl error={Boolean(fieldErrors.context)} fullWidth required>
            <InputLabel htmlFor="context">{contextLabel}</InputLabel>
            <OutlinedInput
              aria-describedby="context-error"
              id="context"
              label={contextLabel}
              name="context"
              onBlur={() => markTouched('context')}
              onChange={(event) => handleBasePathChange(event.target.value)}
              value={formState.context}
            />
            <FormHelperText id="context-error">{fieldErrors.context}</FormHelperText>
          </FormControl>

          <FormControl fullWidth>
            <InputLabel htmlFor="description">{descriptionLabel}</InputLabel>
            <OutlinedInput
              id="description"
              label={descriptionLabel}
              multiline
              name="description"
              onChange={(event) => setField('description', event.target.value)}
              rows={3}
              value={formState.description ?? ''}
            />
          </FormControl>
        </Form.Stack>
      </Paper>

      <Paper component="section" sx={{ p: 3, mt: 1 }}>
        <Form.Header sx={SECTION_LABEL_SX}>
          <FormattedMessage {...messages.endpointSection} />
        </Form.Header>

        <Form.Stack spacing={2} sx={{ mt: 1.5 }}>
          <FormControl error={Boolean(fieldErrors.targetUrl)} fullWidth required>
            <InputLabel htmlFor="targetUrl">{targetUrlLabel}</InputLabel>
            <OutlinedInput
              aria-describedby="targetUrl-error"
              id="targetUrl"
              label={targetUrlLabel}
              name="targetUrl"
              onBlur={() => markTouched('targetUrl')}
              onChange={(event) => setMainUpstreamUrl(event.target.value)}
              value={formState.upstream.main.url}
            />
            <FormHelperText id="targetUrl-error">{fieldErrors.targetUrl}</FormHelperText>
          </FormControl>
        </Form.Stack>
      </Paper>

      <Divider />

      {/* Both buttons on the trailing edge, the same pairing as the step
          before this one. */}
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'flex-end' }}>
        <Button variant="text" onClick={props.onBack}>
          <FormattedMessage {...messages.back} />
        </Button>
        <Button type="submit" variant="contained">
          <FormattedMessage {...messages.create} />
        </Button>
      </Stack>
    </Stack>
  );
};

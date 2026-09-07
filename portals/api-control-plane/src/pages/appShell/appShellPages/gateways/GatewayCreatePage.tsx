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

import type { FormEvent } from 'react';
import { useState } from 'react';
import {
  Button,
  Chip,
  Divider,
  Form,
  FormControl,
  FormHelperText,
  FormLabel,
  Grid,
  InputLabel,
  MenuItem,
  OutlinedInput,
  PageTitle,
  Select,
  Stack,
} from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { useCreateGateway, type CreateGatewayBody } from '@/api/resources/gateways';
import { useNotifications } from '@/components/Notifications';
import { routes } from '@/routes/paths';
import { GatewayTypeSelector } from './components/GatewayTypeSelector';
import type { GatewayFunctionality } from './utils/gatewayDisplay';
import { MOCK_ENVIRONMENTS } from './utils/gatewayEnvironments';

const messages = defineMessages({
  back: {
    id: 'gateways.create.action.back',
    defaultMessage: 'Back to gateways',
  },
  cancel: {
    id: 'gateways.create.action.cancel',
    defaultMessage: 'Cancel',
  },
  configurationsSection: {
    id: 'gateways.create.section.configurations',
    defaultMessage: 'Configurations',
  },
  createdNotification: {
    id: 'gateways.create.notification.created',
    defaultMessage: 'Gateway “{name}” provisioned.',
    description: '{name} is the gateway display name the user just entered.',
  },
  descriptionLabel: {
    id: 'gateways.create.description.label',
    defaultMessage: 'Description (Optional)',
  },
  endpointErrorInvalid: {
    id: 'gateways.create.endpoint.error.invalid',
    defaultMessage: 'Enter a full URL, for example https://localhost:8443.',
  },
  endpointErrorRequired: {
    id: 'gateways.create.endpoint.error.required',
    defaultMessage: 'Enter a URL.',
  },
  endpointLabel: {
    id: 'gateways.create.endpoint.label',
    defaultMessage: 'URL',
  },
  endpointPlaceholder: {
    id: 'gateways.create.endpoint.placeholder',
    defaultMessage: 'https://localhost:8443',
    description: 'Example gateway address shown in the empty URL field.',
  },
  environmentLabel: {
    id: 'gateways.create.environment.label',
    defaultMessage: 'Associated Environment',
  },
  generalSection: {
    id: 'gateways.create.section.general',
    defaultMessage: 'General Details',
  },
  lts: {
    id: 'gateways.create.version.badge.lts',
    defaultMessage: 'LTS',
    description: 'Badge marking a long-term-support gateway build.',
  },
  nameErrorRequired: {
    id: 'gateways.create.name.error.required',
    defaultMessage: 'Enter a gateway name.',
  },
  nameLabel: {
    id: 'gateways.create.name.label',
    defaultMessage: 'Name',
  },
  submit: {
    id: 'gateways.create.action.submit',
    defaultMessage: 'Provision gateway',
  },
  submitting: {
    id: 'gateways.create.action.submitting',
    defaultMessage: 'Provisioning…',
  },
  subtitle: {
    id: 'gateways.create.subtitle',
    defaultMessage: 'Register a self-hosted gateway, then connect it to the platform.',
  },
  title: {
    id: 'gateways.create.title',
    defaultMessage: 'Provision a gateway',
  },
  typeLabel: {
    id: 'gateways.create.type.label',
    defaultMessage: 'Type',
  },
  versionErrorRequired: {
    id: 'gateways.create.version.error.required',
    defaultMessage: 'Choose a gateway version.',
  },
  versionLabel: {
    id: 'gateways.create.version.label',
    defaultMessage: 'Gateway Version',
  },
  versionOption: {
    id: 'gateways.create.version.option',
    defaultMessage: 'api-gateway v{version}',
    description: 'One entry in the gateway version list; {version} is a number such as 1.0.',
  },
});

/** Section heading. `Form.Header` is fixed at `h4`, which outsizes the page. */
const SECTION_LABEL_SX = { typography: 'h6' } as const;

/** Adds spacing under the type row label. */
const TYPE_LABEL_SX = { mb: 0.75 } as const;

type GatewayVersionOption = {
  /** Long-term support build — the only kind offered today. */
  lts?: boolean;
  value: string;
};

/** Supported gateway versions. Currently only 1.0 LTS. */
const GATEWAY_VERSIONS: GatewayVersionOption[] = [{ lts: true, value: '1.0' }];

const DEFAULT_VERSION = GATEWAY_VERSIONS[0].value;

/** Schemes a gateway can serve on: HTTP APIs, and WebSocket for event traffic. */
const ENDPOINT_PROTOCOLS = ['http:', 'https:', 'ws:', 'wss:'];

const HANDLE_MAX_LENGTH = 64;

type GatewayFormState = {
  description: string;
  displayName: string;
  endpoint: string;
  environment: string;
  functionalityType: GatewayFunctionality;
  version: string;
};

const INITIAL_STATE: GatewayFormState = {
  description: '',
  displayName: '',
  endpoint: '',
  environment: MOCK_ENVIRONMENTS[0].id,
  functionalityType: 'regular',
  version: DEFAULT_VERSION,
};

/** The fields that carry a validation rule. */
type ValidatedField = 'displayName' | 'endpoint' | 'version';

type FieldErrors = Partial<Record<ValidatedField, MessageDescriptor>>;

/**
 * Name → URL-friendly handle.
 *
 * The handle is the gateway's immutable `id`, and the form no longer asks for
 * it: every name a user types produces a valid one, and the spec accepts the
 * request without an `id` when it doesn't (a name of only punctuation), leaving
 * the server to assign one.
 */
const toHandle = (displayName: string): string =>
  displayName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, HANDLE_MAX_LENGTH);

const isEndpointUrl = (value: string): boolean => {
  try {
    return ENDPOINT_PROTOCOLS.includes(new URL(value).protocol);
  } catch {
    return false;
  }
};

/**
 * Every rule in one pure pass, so the same answer drives the field errors and
 * the submit gate.
 */
const validate = (state: GatewayFormState): FieldErrors => {
  const errors: FieldErrors = {};

  if (state.displayName.trim() === '') {
    errors.displayName = messages.nameErrorRequired;
  }

  // A select cannot produce an unlisted value, but an empty one still has to
  // stop the submit rather than post a gateway with no build behind it.
  if (state.version.trim() === '') {
    errors.version = messages.versionErrorRequired;
  }

  const endpoint = state.endpoint.trim();
  if (endpoint === '') {
    errors.endpoint = messages.endpointErrorRequired;
  } else if (!isEndpointUrl(endpoint)) {
    errors.endpoint = messages.endpointErrorInvalid;
  }

  return errors;
};

/** Maps server field names to editable form fields. */
const SERVER_FIELD_MAP: Record<string, ValidatedField> = {
  displayName: 'displayName',
  endpoints: 'endpoint',
  id: 'displayName',
  version: 'version',
};

export function GatewayCreatePage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();
  const { notify } = useNotifications();
  const createGateway = useCreateGateway();

  const [formState, setFormState] = useState<GatewayFormState>(INITIAL_STATE);

  // Errors are recomputed every render; `touched` controls visibility.
  const [touched, setTouched] = useState<Partial<Record<ValidatedField, boolean>>>({});

  // Server-side field errors; cleared on the next edit.
  const [serverErrors, setServerErrors] = useState<Partial<Record<ValidatedField, string>>>({});

  const errors = validate(formState);

  const markTouched = (field: ValidatedField) =>
    setTouched((current) => ({ ...current, [field]: true }));

  const clearServerError = (field: ValidatedField) =>
    setServerErrors((current) => {
      if (current[field] === undefined) return current;
      const next = { ...current };
      delete next[field];
      return next;
    });

  const setField = <K extends keyof GatewayFormState>(key: K, value: GatewayFormState[K]) =>
    setFormState((current) => ({ ...current, [key]: value }));

  const onFormSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (Object.keys(errors).length > 0) {
      // Reveal every rule at once rather than one field per attempt.
      setTouched({ displayName: true, endpoint: true, version: true });
      return;
    }

    const handle = toHandle(formState.displayName);

    const body: CreateGatewayBody = {
      description: formState.description.trim() || undefined,
      displayName: formState.displayName.trim(),
      endpoints: [formState.endpoint.trim()],
      functionalityType: formState.functionalityType,
      id: handle || undefined,
      isCritical: false,
      // This flow only registers gateways the customer runs, and `gatewayMode`
      // is what the listing reads to tell self-hosted from WSO2-managed. The
      // environment rides along the same way until a real environment service
      // exists to associate it with.
      properties: { environment: formState.environment, gatewayMode: 'self-hosted' },
      version: formState.version,
    };

    createGateway.mutate(body, {
      onSuccess: (gateway) => {
        notify(
          intl.formatMessage(messages.createdNotification, {
            name: gateway.displayName,
          }),
          'success',
        );
        navigate(routes.gateway(orgHandle, gateway.id ?? ''));
      },
      onError: (error) => {
        // The global mutation handler already surfaces the failure; this only
        // moves what the server said about a field onto that field.
        const fieldErrors = error.fieldErrorMap();
        const mapped: Partial<Record<ValidatedField, string>> = {};
        for (const [serverField, message] of Object.entries(fieldErrors)) {
          const field = SERVER_FIELD_MAP[serverField];
          if (field) mapped[field] = message;
        }
        setServerErrors(mapped);
      },
    });
  };

  /** A server rejection outranks the local rules: those already passed. */
  const errorFor = (field: ValidatedField): string | undefined => {
    if (serverErrors[field]) return serverErrors[field];
    const descriptor = touched[field] ? errors[field] : undefined;
    return descriptor ? intl.formatMessage(descriptor) : undefined;
  };

  const nameLabel = intl.formatMessage(messages.nameLabel);
  const descriptionLabel = intl.formatMessage(messages.descriptionLabel);
  const versionLabel = intl.formatMessage(messages.versionLabel);
  const endpointLabel = intl.formatMessage(messages.endpointLabel);
  const environmentLabel = intl.formatMessage(messages.environmentLabel);

  return (
    <>
      <PageTitle>
        <Link to={routes.gateways(orgHandle)}>
          <PageTitle.BackButton>
            <FormattedMessage {...messages.back} />
          </PageTitle.BackButton>
        </Link>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...messages.subtitle} />
        </PageTitle.SubHeader>
      </PageTitle>

      <Stack
        component="form"
        noValidate
        onSubmit={onFormSubmit}
        spacing={4}
        sx={{ width: { lg: '70%', md: '85%', xs: '100%' } }}
      >
        <Stack component="section" spacing={2}>
          <Form.Header sx={SECTION_LABEL_SX}>
            <FormattedMessage {...messages.generalSection} />
          </Form.Header>

          <FormControl fullWidth>
            <FormLabel id="gatewayTypeLabel" sx={TYPE_LABEL_SX}>
              <FormattedMessage {...messages.typeLabel} />
            </FormLabel>
            <GatewayTypeSelector
              onChange={(value) => setField('functionalityType', value)}
              value={formState.functionalityType}
            />
          </FormControl>

          <Grid container spacing={2}>
            <Grid size={{ md: 6, xs: 12 }}>
              <FormControl error={Boolean(errorFor('version'))} fullWidth required>
                <InputLabel id="gatewayVersionLabel">{versionLabel}</InputLabel>
                <Select
                  id="gatewayVersion"
                  label={versionLabel}
                  labelId="gatewayVersionLabel"
                  name="version"
                  onBlur={() => markTouched('version')}
                  onChange={(event) => {
                    clearServerError('version');
                    setField('version', event.target.value);
                  }}
                  renderValue={(selected) =>
                    intl.formatMessage(messages.versionOption, { version: selected })
                  }
                  value={formState.version}
                >
                  {GATEWAY_VERSIONS.map((option) => (
                    <MenuItem key={option.value} value={option.value}>
                      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
                        <span>
                          <FormattedMessage
                            {...messages.versionOption}
                            values={{ version: option.value }}
                          />
                        </span>
                        {option.lts ? (
                          <Chip
                            label={<FormattedMessage {...messages.lts} />}
                            size="small"
                            variant="outlined"
                          />
                        ) : null}
                      </Stack>
                    </MenuItem>
                  ))}
                </Select>
                {errorFor('version') ? (
                  <FormHelperText>{errorFor('version')}</FormHelperText>
                ) : null}
              </FormControl>
            </Grid>
          </Grid>

          <Grid container spacing={2}>
            <Grid size={{ md: 6, xs: 12 }}>
              <FormControl error={Boolean(errorFor('displayName'))} fullWidth required>
                <InputLabel htmlFor="gatewayName">{nameLabel}</InputLabel>
                <OutlinedInput
                  id="gatewayName"
                  label={nameLabel}
                  name="displayName"
                  onBlur={() => markTouched('displayName')}
                  onChange={(event) => {
                    clearServerError('displayName');
                    setField('displayName', event.target.value);
                  }}
                  value={formState.displayName}
                />
                {errorFor('displayName') ? (
                  <FormHelperText>{errorFor('displayName')}</FormHelperText>
                ) : null}
              </FormControl>
            </Grid>

            <Grid size={{ md: 6, xs: 12 }}>
              <FormControl fullWidth>
                <InputLabel htmlFor="gatewayDescription">{descriptionLabel}</InputLabel>
                <OutlinedInput
                  id="gatewayDescription"
                  label={descriptionLabel}
                  multiline
                  name="description"
                  onChange={(event) => setField('description', event.target.value)}
                  value={formState.description}
                />
              </FormControl>
            </Grid>
          </Grid>
        </Stack>

        <Stack component="section" spacing={2}>
          <Form.Header sx={SECTION_LABEL_SX}>
            <FormattedMessage {...messages.configurationsSection} />
          </Form.Header>

          <Grid container spacing={2}>
            <Grid size={{ md: 6, xs: 12 }}>
              <FormControl error={Boolean(errorFor('endpoint'))} fullWidth required>
                <InputLabel htmlFor="gatewayEndpoint">{endpointLabel}</InputLabel>
                <OutlinedInput
                  id="gatewayEndpoint"
                  label={endpointLabel}
                  name="endpoint"
                  onBlur={() => markTouched('endpoint')}
                  onChange={(event) => {
                    clearServerError('endpoint');
                    setField('endpoint', event.target.value);
                  }}
                  placeholder={intl.formatMessage(messages.endpointPlaceholder)}
                  value={formState.endpoint}
                />
                {errorFor('endpoint') ? (
                  <FormHelperText>{errorFor('endpoint')}</FormHelperText>
                ) : null}
              </FormControl>
            </Grid>

            <Grid size={{ md: 6, xs: 12 }}>
              <FormControl fullWidth>
                <InputLabel id="gatewayEnvironmentLabel">{environmentLabel}</InputLabel>
                <Select
                  id="gatewayEnvironment"
                  label={environmentLabel}
                  labelId="gatewayEnvironmentLabel"
                  name="environment"
                  onChange={(event) => setField('environment', event.target.value)}
                  value={formState.environment}
                >
                  {/* Environment names are data, not copy — they are passed
                      through rather than translated. */}
                  {MOCK_ENVIRONMENTS.map((environment) => (
                    <MenuItem key={environment.id} value={environment.id}>
                      {environment.name}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          </Grid>
        </Stack>

        <Divider />

        <Stack
          direction="row"
          spacing={2}
          sx={{ alignItems: 'center', justifyContent: 'flex-end' }}
        >
          <Button component={Link} to={routes.gateways(orgHandle)} variant="text">
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button disabled={createGateway.isPending} type="submit" variant="contained">
            <FormattedMessage
              {...(createGateway.isPending ? messages.submitting : messages.submit)}
            />
          </Button>
        </Stack>
      </Stack>
    </>
  );
}

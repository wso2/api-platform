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
  Divider,
  Form,
  FormControl,
  FormHelperText,
  Grid,
  InputLabel,
  OutlinedInput,
  PageTitle,
  Stack,
} from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { useCreateApiPortal, type CreateApiPortalBody } from '@/api/resources/apiPortals';
import { AppPage } from '@/components/AppPage';
import { useNotifications } from '@/components/Notifications';
import { routes } from '@/routes/paths';
import type { ApiPortalDetailLocationState } from './ApiPortalDetailPage';
import {
  generateSharedKey,
  HANDLE_MAX_LENGTH,
  HANDLE_PATTERN,
  isHttpsUrl,
  sharedKeyHash,
  toHandle,
} from './utils/apiPortalSetup';

const messages = defineMessages({
  back: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.back',
    defaultMessage: 'Back to API Portals',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.cancel',
    defaultMessage: 'Cancel',
  },
  createdNotification: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.created',
    defaultMessage: 'API Portal “{name}” registered.',
    description: '{name} is the portal name the user just entered.',
  },
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.description.label',
    defaultMessage: 'Description (Optional)',
  },
  generalSection: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.section.general',
    defaultMessage: 'General Details',
  },
  handleErrorInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.handle.error.invalid',
    defaultMessage: 'Use lowercase letters, numbers and hyphens only.',
  },
  handleErrorRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.handle.error.required',
    defaultMessage: 'Enter a handle.',
  },
  handleHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.handle.helper',
    defaultMessage: 'Identifies the portal in URLs and the API. It cannot be changed later.',
  },
  handleLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.handle.label',
    defaultMessage: 'Handle',
  },
  nameErrorRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.name.error.required',
    defaultMessage: 'Enter a portal name.',
  },
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.name.label',
    defaultMessage: 'Name',
  },
  submit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.submit',
    defaultMessage: 'Register portal',
  },
  submitting: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.submitting',
    defaultMessage: 'Registering…',
  },
  subtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.subtitle',
    defaultMessage: 'Register an API Portal you run, then connect it to the control plane.',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.title',
    defaultMessage: 'Register an API Portal',
  },
  urlErrorInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.url.error.invalid',
    defaultMessage: 'Enter a full https URL, for example https://portal.example.com.',
  },
  urlErrorRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.url.error.required',
    defaultMessage: 'Enter the portal URL.',
  },
  urlHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.url.helper',
    defaultMessage: 'Where the control plane reaches the portal to publish APIs.',
  },
  urlLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.url.label',
    defaultMessage: 'URL',
  },
  urlPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.ApiPortalCreatePage.url.placeholder',
    defaultMessage: 'https://portal.example.com',
    description: 'Example portal address shown in the empty URL field.',
  },
});

/** Section heading. `Form.Header` is fixed at `h4`, which outsizes the page. */
const SECTION_LABEL_SX = { typography: 'h6' } as const;

/** Spec limits, so the field stops where the server would reject. */
const NAME_MAX_LENGTH = 255;
const URL_MAX_LENGTH = 500;
const DESCRIPTION_MAX_LENGTH = 1023;

type FormState = {
  description: string;
  handle: string;
  name: string;
  url: string;
};

const INITIAL_STATE: FormState = { description: '', handle: '', name: '', url: '' };

type ValidatedField = 'handle' | 'name' | 'url';

type FieldErrors = Partial<Record<ValidatedField, MessageDescriptor>>;

const validate = (state: FormState): FieldErrors => {
  const errors: FieldErrors = {};
  if (state.name.trim() === '') errors.name = messages.nameErrorRequired;

  if (state.handle === '') errors.handle = messages.handleErrorRequired;
  else if (!HANDLE_PATTERN.test(state.handle)) errors.handle = messages.handleErrorInvalid;

  const url = state.url.trim();
  if (url === '') errors.url = messages.urlErrorRequired;
  else if (!isHttpsUrl(url)) errors.url = messages.urlErrorInvalid;

  return errors;
};

/** Maps server field names onto the form's own. */
const SERVER_FIELD_MAP: Record<string, ValidatedField> = {
  handle: 'handle',
  name: 'name',
  url: 'url',
};

export function ApiPortalCreatePage() {
  return (
    <AppPage>
      <ApiPortalCreatePageContent />
    </AppPage>
  );
}

function ApiPortalCreatePageContent() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();
  const { notify } = useNotifications();
  const createPortal = useCreateApiPortal();

  const [formState, setFormState] = useState<FormState>(INITIAL_STATE);
  // The handle follows the name until the user edits it directly.
  const [handleEdited, setHandleEdited] = useState(false);
  const [touched, setTouched] = useState<Partial<Record<ValidatedField, boolean>>>({});
  const [serverErrors, setServerErrors] = useState<Partial<Record<ValidatedField, string>>>({});
  // Hashing is async, so the submit is busy before the mutation starts.
  const [preparing, setPreparing] = useState(false);

  const errors = validate(formState);
  const busy = preparing || createPortal.isPending;

  const markTouched = (field: ValidatedField) =>
    setTouched((current) => ({ ...current, [field]: true }));

  const clearServerError = (field: ValidatedField) =>
    setServerErrors((current) => {
      if (current[field] === undefined) return current;
      const next = { ...current };
      delete next[field];
      return next;
    });

  const setField = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setFormState((current) => ({ ...current, [key]: value }));

  const onNameChange = (name: string) => {
    clearServerError('name');
    setFormState((current) => ({
      ...current,
      name,
      handle: handleEdited ? current.handle : toHandle(name),
    }));
  };

  const onFormSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (Object.keys(errors).length > 0) {
      setTouched({ handle: true, name: true, url: true });
      return;
    }

    // The key leaves the browser once, in this request, and is not kept: the
    // portal is configured with its hash, which is all the detail page shows.
    const sharedKey = generateSharedKey();
    setPreparing(true);
    let hash: string;
    try {
      hash = await sharedKeyHash(sharedKey);
    } finally {
      setPreparing(false);
    }

    const body: CreateApiPortalBody = {
      description: formState.description.trim() || undefined,
      handle: formState.handle,
      name: formState.name.trim(),
      sharedKey,
      url: formState.url.trim(),
    };

    createPortal.mutate(body, {
      onSuccess: (portal) => {
        notify(intl.formatMessage(messages.createdNotification, { name: portal.name }), 'success');
        const state: ApiPortalDetailLocationState = { sharedKeyHash: hash, justRegistered: true };
        navigate(routes.orgApiPortal(orgHandle, portal.id), { state });
      },
      onError: (error) => {
        // The global mutation handler already surfaces the failure; this only
        // moves what the server said about a field onto that field.
        const mapped: Partial<Record<ValidatedField, string>> = {};
        for (const [serverField, message] of Object.entries(error.fieldErrorMap())) {
          const field = SERVER_FIELD_MAP[serverField];
          if (field) mapped[field] = message;
        }
        setServerErrors(mapped);
      },
    });
  };

  const errorFor = (field: ValidatedField): string | undefined => {
    if (serverErrors[field]) return serverErrors[field];
    const descriptor = touched[field] ? errors[field] : undefined;
    return descriptor ? intl.formatMessage(descriptor) : undefined;
  };

  const nameLabel = intl.formatMessage(messages.nameLabel);
  const handleLabel = intl.formatMessage(messages.handleLabel);
  const urlLabel = intl.formatMessage(messages.urlLabel);
  const descriptionLabel = intl.formatMessage(messages.descriptionLabel);

  return (
    <>
      <PageTitle>
        <Link to={routes.orgApiPortals(orgHandle)}>
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

          <Grid container spacing={2}>
            <Grid size={{ md: 6, xs: 12 }}>
              <FormControl error={Boolean(errorFor('name'))} fullWidth required>
                <InputLabel htmlFor="apiPortalName">{nameLabel}</InputLabel>
                <OutlinedInput
                  id="apiPortalName"
                  inputProps={{ maxLength: NAME_MAX_LENGTH }}
                  label={nameLabel}
                  name="name"
                  onBlur={() => markTouched('name')}
                  onChange={(event) => onNameChange(event.target.value)}
                  value={formState.name}
                />
                {errorFor('name') ? <FormHelperText>{errorFor('name')}</FormHelperText> : null}
              </FormControl>
            </Grid>

            <Grid size={{ md: 6, xs: 12 }}>
              <FormControl error={Boolean(errorFor('handle'))} fullWidth required>
                <InputLabel htmlFor="apiPortalHandle">{handleLabel}</InputLabel>
                <OutlinedInput
                  id="apiPortalHandle"
                  inputProps={{ maxLength: HANDLE_MAX_LENGTH }}
                  label={handleLabel}
                  name="handle"
                  onBlur={() => markTouched('handle')}
                  onChange={(event) => {
                    clearServerError('handle');
                    setHandleEdited(true);
                    setField('handle', event.target.value);
                  }}
                  value={formState.handle}
                />
                <FormHelperText>
                  {errorFor('handle') ?? intl.formatMessage(messages.handleHelper)}
                </FormHelperText>
              </FormControl>
            </Grid>
          </Grid>

          <Grid container spacing={2}>
            <Grid size={{ md: 6, xs: 12 }}>
              <FormControl error={Boolean(errorFor('url'))} fullWidth required>
                <InputLabel htmlFor="apiPortalUrl">{urlLabel}</InputLabel>
                <OutlinedInput
                  id="apiPortalUrl"
                  inputProps={{ maxLength: URL_MAX_LENGTH }}
                  label={urlLabel}
                  name="url"
                  onBlur={() => markTouched('url')}
                  onChange={(event) => {
                    clearServerError('url');
                    setField('url', event.target.value);
                  }}
                  placeholder={intl.formatMessage(messages.urlPlaceholder)}
                  value={formState.url}
                />
                <FormHelperText>
                  {errorFor('url') ?? intl.formatMessage(messages.urlHelper)}
                </FormHelperText>
              </FormControl>
            </Grid>

            <Grid size={{ md: 6, xs: 12 }}>
              <FormControl fullWidth>
                <InputLabel htmlFor="apiPortalDescription">{descriptionLabel}</InputLabel>
                <OutlinedInput
                  id="apiPortalDescription"
                  inputProps={{ maxLength: DESCRIPTION_MAX_LENGTH }}
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

        <Divider />

        <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'flex-end' }}>
          <Button component={Link} to={routes.orgApiPortals(orgHandle)} variant="text">
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button disabled={busy} type="submit" variant="contained">
            <FormattedMessage {...(busy ? messages.submitting : messages.submit)} />
          </Button>
        </Stack>
      </Stack>
    </>
  );
}

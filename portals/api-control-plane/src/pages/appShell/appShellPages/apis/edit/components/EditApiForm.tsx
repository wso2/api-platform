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
  Divider,
  Form,
  FormControl,
  FormHelperText,
  Grid,
  InputLabel,
  OutlinedInput,
  Paper,
  Stack,
} from '@wso2/oxygen-ui';
import type { FormEvent, ReactNode } from 'react';
import { useState } from 'react';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import type { RestApi } from '@/api/resources/restApis';
import { CONTEXT_PATTERN, isHttpUrl, VERSION_PATTERN } from '../../utils/basicInfoRules';

/**
 * The five fields this form edits, flattened. `targetUrl` is
 * `upstream.main.url` — the page puts it back on the API object, because the
 * spec's update body is the whole `RESTAPI` and only the page holds the
 * fetched original to merge into.
 */
export type ApiBasicInfoFormValues = {
  context: string;
  description: string;
  displayName: string;
  targetUrl: string;
  version: string;
};

export type EditApiFormProps = {
  api: RestApi;
  /**
   * Per-field messages the server rejected the last save with, keyed by the
   * spec's own field names (`ApiError.fieldErrorMap()`). Shown as-is: this is
   * backend copy, not something the catalog can translate.
   */
  fieldErrors?: Record<string, string>;
  isSaving: boolean;
  onCancel: () => void;
  onSubmit: (values: ApiBasicInfoFormValues) => void;
};

const messages = defineMessages({
  basicInformation: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.section.basicInformation',
    defaultMessage: 'Basic information',
    description: 'Heading of the field group holding name, identifier, version, context.',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.action.cancel',
    defaultMessage: 'Cancel',
    description: 'Discards the unsaved edits and returns to the API overview.',
  },
  contextErrorPattern: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.context.error.pattern',
    defaultMessage: 'Start with / and use only letters, numbers, hyphens, dots and slashes.',
  },
  contextErrorRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.context.error.required',
    defaultMessage: 'Enter a context.',
  },
  contextLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.context.label',
    defaultMessage: 'Context',
  },
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.description.label',
    defaultMessage: 'Description',
  },
  endpointSection: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.section.backendEndpoint',
    defaultMessage: 'Backend endpoint',
    description: 'Heading of the field group holding the target URL.',
  },
  identifierHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.identifier.helper',
    defaultMessage: 'Fixed once the API is created.',
    description: 'Helper text under the disabled identifier field on the API edit form.',
  },
  identifierLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.identifier.label',
    defaultMessage: 'Identifier',
  },
  nameErrorRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.name.error.required',
    defaultMessage: 'Enter a name.',
  },
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.name.label',
    defaultMessage: 'Name',
  },
  save: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.action.save',
    defaultMessage: 'Save changes',
    description: 'Commits the edits to the API.',
  },
  targetUrlErrorInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.targetUrl.error.invalid',
    defaultMessage: 'Enter a full URL, for example https://api.example.com.',
  },
  targetUrlErrorRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.targetUrl.error.required',
    defaultMessage: 'Enter a target URL.',
  },
  targetUrlLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.targetUrl.label',
    defaultMessage: 'Target URL',
  },
  targetUrlSharedUpstream: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.targetUrl.sharedUpstream',
    defaultMessage: 'Routed through the shared upstream “{ref}”, so there is no URL to edit here.',
    description:
      'Helper text shown instead of an editable target URL when the API points at a named upstream definition. {ref} is that definition’s name.',
  },
  versionErrorPattern: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.version.error.pattern',
    defaultMessage: 'Use letters, numbers, dots, hyphens and underscores — no spaces or slashes.',
  },
  versionErrorRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.version.error.required',
    defaultMessage: 'Enter a version.',
  },
  versionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.edit.EditApiForm.version.label',
    defaultMessage: 'Version',
  },
});

/**
 * Small uppercase rule above a group of fields, matching the create form.
 * `Form.Header` is fixed at `h4`, so the size comes from the theme's `overline`
 * typography rather than a font-size literal.
 */
const SECTION_LABEL_SX = {
  color: 'text.secondary',
  typography: 'overline',
} as const;

/** The fields that carry a validation rule — description has none. */
type ValidatedField = 'context' | 'displayName' | 'targetUrl' | 'version';

type FieldErrors = Partial<Record<ValidatedField, MessageDescriptor>>;

/**
 * Which spec field name a server-side field error binds to. The server names
 * the wire field, which is nested for the upstream URL, so the mapping is
 * spelled out rather than guessed from the input's own name.
 */
const SERVER_FIELD_NAMES: Record<ValidatedField, string[]> = {
  context: ['context'],
  displayName: ['displayName'],
  targetUrl: ['upstream.main.url', 'upstream'],
  version: ['version'],
};

/**
 * Every rule in one pure pass, so the same answer drives the field errors and
 * the submit gate — there is no second, drifting copy of the rules.
 *
 * `targetUrl` is only validated when the API carries a URL to begin with: an
 * API routed through a shared upstream `ref` has no URL, and requiring one
 * would make its form unsubmittable.
 */
const validate = (values: ApiBasicInfoFormValues, hasEditableUrl: boolean): FieldErrors => {
  const errors: FieldErrors = {};

  if (values.displayName.trim() === '') {
    errors.displayName = messages.nameErrorRequired;
  }

  const version = values.version.trim();
  if (version === '') {
    errors.version = messages.versionErrorRequired;
  } else if (!VERSION_PATTERN.test(version)) {
    errors.version = messages.versionErrorPattern;
  }

  const context = values.context.trim();
  if (context === '' || context === '/') {
    errors.context = messages.contextErrorRequired;
  } else if (!CONTEXT_PATTERN.test(context)) {
    errors.context = messages.contextErrorPattern;
  }

  if (hasEditableUrl) {
    const targetUrl = values.targetUrl.trim();
    if (targetUrl === '') {
      errors.targetUrl = messages.targetUrlErrorRequired;
    } else if (!isHttpUrl(targetUrl)) {
      errors.targetUrl = messages.targetUrlErrorInvalid;
    }
  }

  return errors;
};

/** The form's opening values, read off the fetched API. */
const toFormValues = (api: RestApi): ApiBasicInfoFormValues => ({
  context: api.context ?? '',
  description: api.description ?? '',
  displayName: api.displayName ?? '',
  targetUrl: api.upstream?.main?.url ?? '',
  version: api.version ?? '',
});

/**
 * Edit form for an API's basic information: name, description, context, version
 * and target URL.
 *
 * It owns the draft and the validation only. The mutation, the merge back onto
 * the fetched `RESTAPI` and the navigation belong to `ApiEditPage`, so this
 * component renders from props alone and can be exercised without a network.
 */
export const EditApiForm = (props: EditApiFormProps) => {
  const intl = useIntl();

  // Lazy initialiser: the API is already loaded when this mounts, so the draft
  // is seeded once rather than recomputed on every render.
  const [values, setValues] = useState<ApiBasicInfoFormValues>(() => toFormValues(props.api));

  // Errors are recomputed from state on every render; `touched` decides which
  // of them the user is ready to see, so nothing shouts before it is typed in.
  const [touched, setTouched] = useState<Partial<Record<ValidatedField, boolean>>>({});

  // An upstream carries `url` or `ref`, never both. A `ref` names a shared
  // upstream definition this form does not own, so the URL field goes
  // read-only rather than silently converting the API to a direct URL.
  const upstreamRef = props.api.upstream?.main?.ref;
  const hasEditableUrl = !upstreamRef;

  const errors = validate(values, hasEditableUrl);

  /** Whatever the server said about this field on the last rejected save. */
  const serverErrorFor = (field: ValidatedField): string | undefined => {
    const map = props.fieldErrors;
    if (!map) return undefined;
    return SERVER_FIELD_NAMES[field].map((name) => map[name]).find(Boolean);
  };

  const errorFor = (field: ValidatedField): MessageDescriptor | undefined =>
    touched[field] ? errors[field] : undefined;

  /** Whether the field shows an error at all, from either source. */
  const hasError = (field: ValidatedField): boolean =>
    Boolean(errorFor(field)) || Boolean(serverErrorFor(field));

  const markTouched = (field: ValidatedField) =>
    setTouched((current) => ({ ...current, [field]: true }));

  const setField = <K extends keyof ApiBasicInfoFormValues>(
    key: K,
    value: ApiBasicInfoFormValues[K],
  ) => setValues((current) => ({ ...current, [key]: value }));

  const onFormSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (Object.keys(errors).length > 0) {
      // Reveal every rule at once rather than one field per attempt.
      setTouched({ context: true, displayName: true, targetUrl: true, version: true });
      return;
    }

    props.onSubmit({
      context: values.context.trim(),
      description: values.description.trim(),
      displayName: values.displayName.trim(),
      targetUrl: values.targetUrl.trim(),
      version: values.version.trim(),
    });
  };

  /**
   * The helper line under one field: the local rule first, then whatever the
   * server rejected it with, then whatever standing note the field passes in.
   *
   * It returns the `FormHelperText` itself rather than its contents, so a field
   * with nothing to say renders no helper row — and, more importantly, so a
   * field can never end up marked `error` with no message beside it.
   */
  const helperFor = (field: ValidatedField, note?: ReactNode): ReactNode => {
    const local = errorFor(field);
    if (local) {
      return (
        <FormHelperText>
          <FormattedMessage {...local} />
        </FormHelperText>
      );
    }

    const fromServer = serverErrorFor(field);
    if (fromServer) return <FormHelperText>{fromServer}</FormHelperText>;

    return note ? <FormHelperText>{note}</FormHelperText> : null;
  };

  const contextLabel = intl.formatMessage(messages.contextLabel);
  const descriptionLabel = intl.formatMessage(messages.descriptionLabel);
  const identifierLabel = intl.formatMessage(messages.identifierLabel);
  const nameLabel = intl.formatMessage(messages.nameLabel);
  const targetUrlLabel = intl.formatMessage(messages.targetUrlLabel);
  const versionLabel = intl.formatMessage(messages.versionLabel);

  return (
    <Stack component="form" noValidate spacing={3} onSubmit={onFormSubmit}>
      <Paper component="section" sx={{ p: 3 }}>
        <Form.Header sx={SECTION_LABEL_SX}>
          <FormattedMessage {...messages.basicInformation} />
        </Form.Header>

        <Form.Stack spacing={2} sx={{ mt: 1.5 }}>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl error={hasError('displayName')} fullWidth required>
                <InputLabel htmlFor="displayName">{nameLabel}</InputLabel>
                <OutlinedInput
                  autoFocus
                  disabled={props.isSaving}
                  id="displayName"
                  label={nameLabel}
                  name="displayName"
                  onBlur={() => markTouched('displayName')}
                  onChange={(event) => setField('displayName', event.target.value)}
                  value={values.displayName}
                />
                {helperFor('displayName')}
              </FormControl>
            </Grid>

            {/* The handle addresses the resource: `PUT /rest-apis/{restApiId}`
                rejects a body whose `id` differs from the path, and there is no
                rename operation — so it is shown for reference, not for edit. */}
            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl disabled fullWidth>
                <InputLabel htmlFor="identifier">{identifierLabel}</InputLabel>
                <OutlinedInput
                  id="identifier"
                  label={identifierLabel}
                  name="identifier"
                  readOnly
                  value={props.api.id ?? ''}
                />
                <FormHelperText>
                  <FormattedMessage {...messages.identifierHelper} />
                </FormHelperText>
              </FormControl>
            </Grid>

            <Grid size={{ xs: 12, md: 4 }}>
              <FormControl error={hasError('version')} fullWidth required>
                <InputLabel htmlFor="version">{versionLabel}</InputLabel>
                <OutlinedInput
                  disabled={props.isSaving}
                  id="version"
                  label={versionLabel}
                  name="version"
                  onBlur={() => markTouched('version')}
                  onChange={(event) => setField('version', event.target.value)}
                  value={values.version}
                />
                {helperFor('version')}
              </FormControl>
            </Grid>
          </Grid>

          <FormControl error={hasError('context')} fullWidth required>
            <InputLabel htmlFor="context">{contextLabel}</InputLabel>
            <OutlinedInput
              disabled={props.isSaving}
              id="context"
              label={contextLabel}
              name="context"
              onBlur={() => markTouched('context')}
              onChange={(event) => setField('context', event.target.value)}
              value={values.context}
            />
            {helperFor('context')}
          </FormControl>

          <FormControl fullWidth>
            <InputLabel htmlFor="description">{descriptionLabel}</InputLabel>
            <OutlinedInput
              disabled={props.isSaving}
              id="description"
              label={descriptionLabel}
              multiline
              name="description"
              onChange={(event) => setField('description', event.target.value)}
              rows={3}
              value={values.description}
            />
          </FormControl>
        </Form.Stack>
      </Paper>

      <Paper component="section" sx={{ mt: 1, p: 3 }}>
        <Form.Header sx={SECTION_LABEL_SX}>
          <FormattedMessage {...messages.endpointSection} />
        </Form.Header>

        <Form.Stack spacing={2} sx={{ mt: 1.5 }}>
          <FormControl
            disabled={!hasEditableUrl}
            error={hasError('targetUrl')}
            fullWidth
            required={hasEditableUrl}
          >
            <InputLabel htmlFor="targetUrl">{targetUrlLabel}</InputLabel>
            <OutlinedInput
              disabled={props.isSaving || !hasEditableUrl}
              id="targetUrl"
              label={targetUrlLabel}
              name="targetUrl"
              onBlur={() => markTouched('targetUrl')}
              onChange={(event) => setField('targetUrl', event.target.value)}
              readOnly={!hasEditableUrl}
              value={values.targetUrl}
            />
            {helperFor(
              'targetUrl',
              hasEditableUrl ? null : (
                <FormattedMessage
                  {...messages.targetUrlSharedUpstream}
                  values={{ ref: upstreamRef }}
                />
              ),
            )}
          </FormControl>
        </Form.Stack>
      </Paper>

      <Divider />

      {/* Both buttons on the trailing edge, the same pairing as the create
          form's last step. */}
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center', justifyContent: 'flex-end' }}>
        <Button disabled={props.isSaving} variant="text" onClick={props.onCancel}>
          <FormattedMessage {...messages.cancel} />
        </Button>
        <Button disabled={props.isSaving} type="submit" variant="contained">
          <FormattedMessage {...messages.save} />
        </Button>
      </Stack>
    </Stack>
  );
};

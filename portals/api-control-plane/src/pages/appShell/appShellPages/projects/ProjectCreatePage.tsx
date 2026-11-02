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
  Form,
  FormControl,
  FormHelperText,
  FormLabel,
  Grid,
  IconButton,
  InputAdornment,
  OutlinedInput,
  PageTitle,
  Stack,
} from '@wso2/oxygen-ui';
import { Edit } from '@wso2/oxygen-ui-icons-react';
import { useState, type FormEvent, type ReactNode } from 'react';
import {
  defineMessages,
  FormattedMessage,
  useIntl,
  type MessageDescriptor,
} from 'react-intl';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { useCreateProject } from '@/api/resources/projects';
import { AppPage } from '@/components/AppPage';
import { useNotifications } from '@/components/Notifications';
import { routes } from '@/routes/paths';

const NAME_MAX = 128;
/** Bounds the API enforces on a project handle (`CreateProjectRequest.id`). */
const ID_MIN = 3;
const ID_MAX = 40;

/** Shared field ids for labels, inputs, and helper text. */
const NAME_FIELD = 'new-project-name';
const ID_FIELD = 'new-project-id';
const DESCRIPTION_FIELD = 'new-project-description';

/** Name → URL-friendly handle. */
const toHandle = (displayName: string): string =>
  displayName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, ID_MAX)
    .replace(/-+$/g, '');

/** Keeps hand-typed handles URL-friendly without fighting the cursor. */
const sanitizeHandle = (value: string): string =>
  value
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, '-')
    .slice(0, ID_MAX);

const messages = defineMessages({
  back: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.action.back',
    defaultMessage: 'Back to projects',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.action.cancel',
    defaultMessage: 'Cancel',
  },
  create: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.action.create',
    defaultMessage: 'Create',
  },
  createFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.notification.createFailed',
    defaultMessage: 'Failed to create project',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.notification.created',
    defaultMessage: 'Project created',
  },
  creating: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.action.creating',
    defaultMessage: 'Creating…',
    description: 'Label on the submit button while the project is being created.',
  },
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.description.label',
    defaultMessage: 'Description',
    description: 'Label for the project description field. A noun, not a command.',
  },
  descriptionPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.description.placeholder',
    defaultMessage: 'What is this project for?',
  },
  nameErrorRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.name.error.required',
    defaultMessage: 'Enter a project name.',
  },
  nameErrorTooLong: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.name.error.tooLong',
    defaultMessage: 'Name must be {max} characters or fewer.',
  },
  idEdit: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.id.edit',
    defaultMessage: 'Edit project ID',
  },
  idErrorLength: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.id.error.length',
    defaultMessage: 'Project ID must be {min}–{max} characters.',
  },
  idLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.id.label',
    defaultMessage: 'Project ID',
    description: 'Label for the project handle field. A noun, not a command.',
  },
  optional: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.optional',
    defaultMessage: '(optional)',
    description: 'Suffix after a field label marking the field as not required.',
  },
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.name.label',
    defaultMessage: 'Name',
    description: 'Label for the project name field. A noun, not a command.',
  },
  namePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.name.placeholder',
    defaultMessage: 'e.g. Customer Support Platform',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.ProjectCreatePage.title',
    defaultMessage: 'Create a project',
  },
});

/** A field's validation message, with the values its ICU placeholders need. */
type FieldMessage = { message: MessageDescriptor; values?: Record<string, number> };

/** A field's validation message; takes no space while the field is valid, so rows stay evenly spaced. */
function FieldError({ error, id }: { error?: FieldMessage; id: string }) {
  if (!error) return null;
  return (
    <FormHelperText id={`${id}-error`}>
      <FormattedMessage {...error.message} values={error.values} />
    </FormHelperText>
  );
}

type ProjectFieldProps = {
  disabled?: boolean;
  endAdornment?: ReactNode;
  error?: FieldMessage;
  id: string;
  label: MessageDescriptor;
  /** Text after the label, e.g. "(optional)". */
  labelSuffix?: MessageDescriptor;
  name: string;
  onBlur?: () => void;
  onChange: (value: string) => void;
  placeholder?: MessageDescriptor;
  readOnly?: boolean;
  required?: boolean;
  value: string;
};

/** A text field with its label and validation message. */
function ProjectField({
  disabled,
  endAdornment,
  error,
  id,
  label,
  labelSuffix,
  name,
  onBlur,
  onChange,
  placeholder,
  readOnly,
  required,
  value,
}: ProjectFieldProps) {
  const intl = useIntl();
  return (
    <FormControl disabled={disabled} error={Boolean(error)} fullWidth required={required}>
      <FormLabel htmlFor={id}>
        <FormattedMessage {...label} />
        {labelSuffix ? (
          <>
            {' '}
            <FormattedMessage {...labelSuffix} />
          </>
        ) : null}
      </FormLabel>
      <OutlinedInput
        aria-describedby={error ? `${id}-error` : undefined}
        endAdornment={endAdornment}
        id={id}
        name={name}
        onBlur={onBlur}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder ? intl.formatMessage(placeholder) : undefined}
        readOnly={readOnly}
        value={value}
      />
      <FieldError error={error} id={id} />
    </FormControl>
  );
}

export function ProjectCreatePage() {
  return (
    <AppPage>
      <ProjectCreatePageContent />
    </AppPage>
  );
}

/**
 * Create a project (platform-api `POST /api/v1/projects`). On success it navigates to the new
 * project's home.
 */
function ProjectCreatePageContent() {
  const { orgHandle = '' } = useParams();
  const intl = useIntl();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const mutation = useCreateProject();
  const [name, setName] = useState('');
  const [nameTouched, setNameTouched] = useState(false);
  const [description, setDescription] = useState('');
  // The handle follows the name until the user takes it over with the edit icon.
  const [idEditing, setIdEditing] = useState(false);
  const [customId, setCustomId] = useState('');

  const trimmedName = name.trim();
  const projectId = idEditing ? customId : toHandle(name);
  // A derived handle that is too short (e.g. a one-letter name) is left to the server.
  const idInvalid = idEditing && (projectId.length < ID_MIN || projectId.length > ID_MAX);

  const nameError: FieldMessage | undefined =
    trimmedName.length > NAME_MAX
      ? { message: messages.nameErrorTooLong, values: { max: NAME_MAX } }
      : nameTouched && trimmedName.length === 0
        ? { message: messages.nameErrorRequired }
        : undefined;
  const idError: FieldMessage | undefined = idInvalid
    ? { message: messages.idErrorLength, values: { max: ID_MAX, min: ID_MIN } }
    : undefined;

  const canSubmit =
    trimmedName.length > 0 && trimmedName.length <= NAME_MAX && !idInvalid && !mutation.isPending;

  const handleSubmit = async (event: FormEvent) => {
    // Real form submit; Enter in the name field works as expected.
    event.preventDefault();
    if (!canSubmit) return;
    try {
      const project = await mutation.mutateAsync({
        displayName: trimmedName,
        id: projectId.length >= ID_MIN ? projectId : undefined,
        description: description.trim() || undefined,
      });
      notify(intl.formatMessage(messages.created), 'success');
      navigate(routes.projectHome(orgHandle, project.id));
    } catch (error) {
      notify(
        error instanceof Error ? error.message : intl.formatMessage(messages.createFailed),
        'error',
      );
    }
  };

  return (
    <>
      <PageTitle>
        <Link style={{ textDecoration: 'none' }} to={routes.projects(orgHandle)}>
          <PageTitle.BackButton>
            <FormattedMessage {...messages.back} />
          </PageTitle.BackButton>
        </Link>
        <PageTitle.Header>
          <FormattedMessage {...messages.title} />
        </PageTitle.Header>
      </PageTitle>

      <Stack
        component="form"
        noValidate
        onSubmit={handleSubmit}
        spacing={3}
        sx={{ width: { lg: '70%', md: '85%', xs: '100%' } }}
      >
        <Form.Stack spacing={1}>
          <Grid container spacing={2}>
            <Grid size={{ md: 6, xs: 12 }}>
              <ProjectField
                error={nameError}
                id={NAME_FIELD}
                label={messages.nameLabel}
                name="name"
                onBlur={() => setNameTouched(true)}
                onChange={setName}
                placeholder={messages.namePlaceholder}
                value={name}
              />
            </Grid>

            <Grid size={{ md: 6, xs: 12 }}>
              <ProjectField
                disabled={!idEditing}
                endAdornment={
                  idEditing ? undefined : (
                    <InputAdornment position="end">
                      <IconButton
                        aria-label={intl.formatMessage(messages.idEdit)}
                        disabled={projectId.length === 0}
                        edge="end"
                        onClick={() => {
                          setCustomId(projectId);
                          setIdEditing(true);
                        }}
                        size="small"
                      >
                        <Edit size={16} />
                      </IconButton>
                    </InputAdornment>
                  )
                }
                error={idError}
                id={ID_FIELD}
                label={messages.idLabel}
                name="id"
                onChange={(value) => setCustomId(sanitizeHandle(value))}
                value={projectId}
              />
            </Grid>

            <Grid size={12}>
              <ProjectField
                id={DESCRIPTION_FIELD}
                label={messages.descriptionLabel}
                labelSuffix={messages.optional}
                name="description"
                onChange={setDescription}
                placeholder={messages.descriptionPlaceholder}
                value={description}
              />
            </Grid>
          </Grid>
        </Form.Stack>

        <Stack direction="row" spacing={2}>
          <Button
            color="primary"
            component={Link}
            disabled={mutation.isPending}
            to={routes.projects(orgHandle)}
            variant="outlined"
          >
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button disabled={!canSubmit} sx={{ borderRadius: 5 }} type="submit" variant="contained">
            <FormattedMessage {...(mutation.isPending ? messages.creating : messages.create)} />
          </Button>
        </Stack>
      </Stack>
    </>
  );
}

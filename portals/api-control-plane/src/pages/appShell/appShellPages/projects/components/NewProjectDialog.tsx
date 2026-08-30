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
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Form,
  FormControl,
  FormHelperText,
  InputLabel,
  OutlinedInput,
} from '@wso2/oxygen-ui';
import { useEffect, useState, type FormEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { useCreateProject } from '@/api/resources/projects';
import { useNotifications } from '@/components/Notifications';
import { routes } from '@/routes/paths';

const NAME_MAX = 120;

/** Shared field ids for labels, inputs, and helper text. */
const NAME_FIELD = 'new-project-name';
const DESCRIPTION_FIELD = 'new-project-description';

const messages = defineMessages({
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.action.cancel',
    defaultMessage: 'Cancel',
  },
  create: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.action.create',
    defaultMessage: 'Create',
  },
  createFailed: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.notification.createFailed',
    defaultMessage: 'Failed to create project',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.notification.created',
    defaultMessage: 'Project created',
  },
  creating: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.action.creating',
    defaultMessage: 'Creating…',
    description: 'Label on the submit button while the project is being created.',
  },
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.description.label',
    defaultMessage: 'Description',
    description: 'Label for the project description field. A noun, not a command.',
  },
  descriptionPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.description.placeholder',
    defaultMessage: 'What this project is for (optional)',
  },
  nameErrorTooLong: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.name.error.tooLong',
    defaultMessage: 'Name must be {max} characters or fewer.',
  },
  nameHelper: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.name.helper',
    defaultMessage: 'A unique name for the project within this organization.',
  },
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.name.label',
    defaultMessage: 'Name',
    description: 'Label for the project name field. A noun, not a command.',
  },
  namePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.name.placeholder',
    defaultMessage: 'e.g. Retail APIs',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.projects.components.NewProjectDialog.title',
    defaultMessage: 'Create project',
  },
});

export type NewProjectDialogProps = {
  open: boolean;
  orgHandle: string;
  onClose: () => void;
};

/**
 * Create a project (platform-api `POST /api/v1/projects`). On success it navigates to the new
 * project's home.
 */
export function NewProjectDialog({ open, orgHandle, onClose }: NewProjectDialogProps) {
  const intl = useIntl();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  // Scope comes from the route via `ApiScopeProvider`; `orgHandle` is only
  // needed to build the redirect once the project exists.
  const mutation = useCreateProject();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  // Reset fields each time the dialog opens.
  useEffect(() => {
    if (open) {
      setName('');
      setDescription('');
    }
  }, [open]);

  const trimmedName = name.trim();
  const isTooLong = trimmedName.length > NAME_MAX;
  const canSubmit = trimmedName.length > 0 && !isTooLong && !mutation.isPending;

  const handleSubmit = async (event: FormEvent) => {
    // Real form submit; Enter in the name field works as expected.
    event.preventDefault();
    if (!canSubmit) return;
    try {
      const project = await mutation.mutateAsync({
        displayName: trimmedName,
        description: description.trim() || undefined,
      });
      notify(intl.formatMessage(messages.created), 'success');
      onClose();
      navigate(routes.projectHome(orgHandle, project.id));
    } catch (error) {
      notify(
        error instanceof Error ? error.message : intl.formatMessage(messages.createFailed),
        'error',
      );
    }
  };

  const nameLabel = intl.formatMessage(messages.nameLabel);
  const descriptionLabel = intl.formatMessage(messages.descriptionLabel);

  return (
    <Dialog fullWidth maxWidth="sm" onClose={mutation.isPending ? undefined : onClose} open={open}>
      <DialogTitle>
        <FormattedMessage {...messages.title} />
      </DialogTitle>
      <Box component="form" noValidate onSubmit={handleSubmit}>
        <DialogContent>
          <Form.Stack spacing={2.5} sx={{ mt: 1 }}>
            <FormControl error={isTooLong} fullWidth required>
              <InputLabel htmlFor={NAME_FIELD}>{nameLabel}</InputLabel>
              <OutlinedInput
                aria-describedby={`${NAME_FIELD}-helper-text`}
                autoFocus
                id={NAME_FIELD}
                label={nameLabel}
                name="name"
                onChange={(event) => setName(event.target.value)}
                placeholder={intl.formatMessage(messages.namePlaceholder)}
                value={name}
              />
              <FormHelperText id={`${NAME_FIELD}-helper-text`}>
                {isTooLong ? (
                  <FormattedMessage {...messages.nameErrorTooLong} values={{ max: NAME_MAX }} />
                ) : (
                  <FormattedMessage {...messages.nameHelper} />
                )}
              </FormHelperText>
            </FormControl>

            <FormControl fullWidth>
              <InputLabel htmlFor={DESCRIPTION_FIELD}>{descriptionLabel}</InputLabel>
              <OutlinedInput
                id={DESCRIPTION_FIELD}
                label={descriptionLabel}
                multiline
                name="description"
                onChange={(event) => setDescription(event.target.value)}
                placeholder={intl.formatMessage(messages.descriptionPlaceholder)}
                rows={3}
                value={description}
              />
            </FormControl>
          </Form.Stack>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" disabled={mutation.isPending} onClick={onClose}>
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button disabled={!canSubmit} type="submit" variant="contained">
            {mutation.isPending ? (
              <FormattedMessage {...messages.creating} />
            ) : (
              <FormattedMessage {...messages.create} />
            )}
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}

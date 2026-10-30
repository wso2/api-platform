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

import { useState, type FormEvent } from 'react';
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
  FormLabel,
  OutlinedInput,
} from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { useUpdateApiPortal, type ApiPortal } from '@/api/resources/apiPortals';
import { useNotifications } from '@/components/Notifications';
import { isHttpsUrl } from '../utils/apiPortalSetup';

const messages = defineMessages({
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.cancel',
    defaultMessage: 'Cancel',
  },
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.description.label',
    defaultMessage: 'Description',
  },
  nameErrorRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.name.error.required',
    defaultMessage: 'Enter a portal name.',
  },
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.name.label',
    defaultMessage: 'Name',
  },
  save: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.save',
    defaultMessage: 'Save',
  },
  saved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.saved',
    defaultMessage: 'API Portal “{name}” updated.',
    description: '{name} is the portal name.',
  },
  saving: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.saving',
    defaultMessage: 'Saving…',
  },
  title: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.title',
    defaultMessage: 'Edit API Portal',
  },
  urlErrorInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.url.error.invalid',
    defaultMessage: 'Enter a full https URL, for example https://portal.example.com.',
  },
  urlLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apiPortals.components.ApiPortalEditDialog.url.label',
    defaultMessage: 'URL',
  },
});

const NAME_FIELD = 'apiPortalEditName';
const URL_FIELD = 'apiPortalEditUrl';
const DESCRIPTION_FIELD = 'apiPortalEditDescription';

export type ApiPortalEditDialogProps = {
  onClose: () => void;
  portal: ApiPortal;
};

/**
 * Edits a portal's name, URL and description. The handle is immutable and the
 * shared key has its own flow on the page, so neither appears here. Mount it
 * only while open, so each opening starts from the portal's current values.
 */
export function ApiPortalEditDialog({ onClose, portal }: ApiPortalEditDialogProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const updatePortal = useUpdateApiPortal();

  const [name, setName] = useState(portal.name);
  const [url, setUrl] = useState(portal.url);
  const [description, setDescription] = useState(portal.description ?? '');

  const nameError = name.trim() === '';
  const urlError = !isHttpsUrl(url.trim());

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (nameError || urlError) return;
    updatePortal.mutate(
      {
        apiPortalId: portal.id,
        body: { description: description.trim(), name: name.trim(), url: url.trim() },
      },
      {
        onSuccess: (updated) => {
          notify(intl.formatMessage(messages.saved, { name: updated.name }), 'success');
          onClose();
        },
      },
    );
  };

  return (
    <Dialog fullWidth maxWidth="sm" onClose={updatePortal.isPending ? undefined : onClose} open>
      <DialogTitle>
        <FormattedMessage {...messages.title} />
      </DialogTitle>
      <Box component="form" noValidate onSubmit={submit}>
        <DialogContent>
          <Form.Stack spacing={2} sx={{ mt: 1 }}>
            <FormControl error={nameError} fullWidth required>
              <FormLabel htmlFor={NAME_FIELD}>
                <FormattedMessage {...messages.nameLabel} />
              </FormLabel>
              <OutlinedInput
                id={NAME_FIELD}
                inputProps={{ maxLength: 255 }}
                onChange={(event) => setName(event.target.value)}
                size="small"
                value={name}
              />
              {nameError ? (
                <FormHelperText>
                  <FormattedMessage {...messages.nameErrorRequired} />
                </FormHelperText>
              ) : null}
            </FormControl>

            <FormControl error={urlError} fullWidth required>
              <FormLabel htmlFor={URL_FIELD}>
                <FormattedMessage {...messages.urlLabel} />
              </FormLabel>
              <OutlinedInput
                id={URL_FIELD}
                inputProps={{ maxLength: 500 }}
                onChange={(event) => setUrl(event.target.value)}
                size="small"
                value={url}
              />
              {urlError ? (
                <FormHelperText>
                  <FormattedMessage {...messages.urlErrorInvalid} />
                </FormHelperText>
              ) : null}
            </FormControl>

            <FormControl fullWidth>
              <FormLabel htmlFor={DESCRIPTION_FIELD}>
                <FormattedMessage {...messages.descriptionLabel} />
              </FormLabel>
              <OutlinedInput
                id={DESCRIPTION_FIELD}
                inputProps={{ maxLength: 1023 }}
                multiline
                onChange={(event) => setDescription(event.target.value)}
                rows={3}
                size="small"
                value={description}
              />
            </FormControl>
          </Form.Stack>
        </DialogContent>
        <DialogActions>
          <Button disabled={updatePortal.isPending} onClick={onClose} variant="outlined">
            <FormattedMessage {...messages.cancel} />
          </Button>
          <Button
            disabled={nameError || urlError || updatePortal.isPending}
            type="submit"
            variant="contained"
          >
            <FormattedMessage {...(updatePortal.isPending ? messages.saving : messages.save)} />
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}

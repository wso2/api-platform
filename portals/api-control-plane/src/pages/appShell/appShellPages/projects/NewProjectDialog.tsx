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
  FormLabel,
  OutlinedInput,
} from '@wso2/oxygen-ui';
import { useEffect, useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';

import { useCreateProject } from '../../../../api/resources/projects';
import { useNotifications } from '../../../../components/Notifications';
import { routes } from '../../../../routes/paths';

const NAME_MAX = 120;

/**
 * Field ids, shared by each control and the label/helper text that describe it.
 * Named constants rather than inline strings because a typo between `htmlFor`
 * and `id` silently breaks the label-to-input association — which looks fine and
 * is invisible to everything except a screen reader.
 */
const NAME_FIELD = 'new-project-name';
const DESCRIPTION_FIELD = 'new-project-description';

export type NewProjectDialogProps = {
  open: boolean;
  orgHandle: string;
  onClose: () => void;
};

/**
 * Create a project (platform-api `POST /api/v1/projects`). The backend persists
 * only name + description — the organization comes from the bearer token — so
 * the form is intentionally minimal. On success it navigates to the new
 * project's home.
 *
 * Each field is a `FormControl` owning its own `FormLabel`, input and
 * `FormHelperText`: `required` and `error` are declared once on the control and
 * reach all three through context, instead of being repeated on each part where
 * they could disagree.
 */
export function NewProjectDialog({
  open,
  orgHandle,
  onClose,
}: NewProjectDialogProps) {
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
    // A real `form` element, so Enter in the name field submits the way a form
    // is expected to — no key handler second-guessing the browser.
    event.preventDefault();
    if (!canSubmit) return;
    try {
      const project = await mutation.mutateAsync({
        displayName: trimmedName,
        description: description.trim() || undefined,
      });
      notify('Project created', 'success');
      onClose();
      navigate(routes.projectHome(orgHandle, project.id));
    } catch (error) {
      notify(
        error instanceof Error ? error.message : 'Failed to create project',
        'error'
      );
    }
  };

  return (
    <Dialog
      fullWidth
      maxWidth="sm"
      onClose={mutation.isPending ? undefined : onClose}
      open={open}
    >
      <DialogTitle>Create project</DialogTitle>
      <Box component="form" noValidate onSubmit={handleSubmit}>
        <DialogContent>
          <Form.Stack spacing={2.5} sx={{ mt: 1 }}>
            <FormControl error={isTooLong} fullWidth required>
              <FormLabel htmlFor={NAME_FIELD}>Name</FormLabel>
              <OutlinedInput
                autoFocus
                id={NAME_FIELD}
                aria-describedby={`${NAME_FIELD}-helper-text`}
                onChange={(event) => setName(event.target.value)}
                placeholder="e.g. Retail APIs"
                size="small"
                value={name}
              />
              <FormHelperText id={`${NAME_FIELD}-helper-text`}>
                {isTooLong
                  ? `Name must be ${NAME_MAX} characters or fewer.`
                  : 'A unique name for the project within this organization.'}
              </FormHelperText>
            </FormControl>

            <FormControl fullWidth>
              <FormLabel htmlFor={DESCRIPTION_FIELD}>Description</FormLabel>
              <OutlinedInput
                id={DESCRIPTION_FIELD}
                minRows={2}
                multiline
                onChange={(event) => setDescription(event.target.value)}
                placeholder="What this project is for (optional)"
                size="small"
                value={description}
              />
            </FormControl>
          </Form.Stack>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" disabled={mutation.isPending} onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={!canSubmit} type="submit" variant="contained">
            {mutation.isPending ? 'Creating…' : 'Create'}
          </Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}

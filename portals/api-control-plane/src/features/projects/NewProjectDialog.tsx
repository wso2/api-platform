import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useCreateProject } from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import { routes } from '../../routes/paths';

const NAME_MAX = 120;

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
 */
export function NewProjectDialog({
  open,
  orgHandle,
  onClose,
}: NewProjectDialogProps) {
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const mutation = useCreateProject(orgHandle);
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
  const canSubmit = trimmedName.length > 0 && !mutation.isPending;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    try {
      const project = await mutation.mutateAsync({
        name: trimmedName,
        description: description.trim() || undefined,
      });
      notify('Project created', 'success');
      onClose();
      navigate(routes.projectHome(orgHandle, project.handler));
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
      <DialogContent>
        <Stack spacing={2.5} sx={{ mt: 1 }}>
          <TextField
            autoFocus
            error={trimmedName.length > NAME_MAX}
            fullWidth
            helperText={
              trimmedName.length > NAME_MAX
                ? `Name must be ${NAME_MAX} characters or fewer.`
                : 'A unique name for the project within this organization.'
            }
            label="Name"
            onChange={(event) => setName(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && canSubmit) handleSubmit();
            }}
            placeholder="e.g. Retail APIs"
            required
            size="small"
            value={name}
          />
          <TextField
            fullWidth
            label="Description"
            minRows={2}
            multiline
            onChange={(event) => setDescription(event.target.value)}
            placeholder="What this project is for (optional)"
            size="small"
            value={description}
          />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button
          color="inherit"
          disabled={mutation.isPending}
          onClick={onClose}
        >
          Cancel
        </Button>
        <Button
          disabled={!canSubmit || trimmedName.length > NAME_MAX}
          onClick={handleSubmit}
          variant="contained"
        >
          {mutation.isPending ? 'Creating…' : 'Create'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

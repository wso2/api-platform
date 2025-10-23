import React from "react";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogActions,
  Typography,
  Stack,
  TextField,
  IconButton,
  Tooltip,
} from "@mui/material";
import InfoOutlined from "@mui/icons-material/InfoOutlined";
import EditOutlined from "@mui/icons-material/EditOutlined";
import { Button } from "./src/components/Button";
import { slugify } from "../utils/slug";

type CreateProjectDialogProps = {
  open: boolean;
  onClose: () => void;
  onCreate: (name: string, description: string) => Promise<void>;
  defaultName?: string;
};

const CreateProjectDialog: React.FC<CreateProjectDialogProps> = ({
  open,
  onClose,
  onCreate,
  defaultName = "",
}) => {
  const [displayName, setDisplayName] = React.useState(defaultName);
  const [identifier, setIdentifier] = React.useState(slugify(defaultName));
  const [description, setDescription] = React.useState("");
  const [errors, setErrors] = React.useState<{ name?: string; id?: string }>({});
  const [identifierLocked, setIdentifierLocked] = React.useState(true);
  const [submitting, setSubmitting] = React.useState(false);

  React.useEffect(() => {
    if (open) {
      setDisplayName(defaultName);
      const initialSlug = slugify(defaultName);
      setIdentifier(initialSlug);
      setDescription("");
      setErrors({});
      setIdentifierLocked(true);
      setSubmitting(false);
    }
  }, [open, defaultName]);

  React.useEffect(() => {
    if (identifierLocked) {
      setIdentifier(slugify(displayName));
    }
  }, [displayName, identifierLocked]);

  const validate = () => {
    const nextErrors: { name?: string; id?: string } = {};
    if (!displayName.trim()) {
      nextErrors.name = "Display name is required";
    }
    if (!identifier.trim()) {
      nextErrors.id = "Identifier is required";
    }
    setErrors(nextErrors);
    return Object.keys(nextErrors).length === 0;
  };

  const handleSubmit = async () => {
    if (!validate()) {
      return;
    }

    setSubmitting(true);
    try {
      await onCreate(displayName.trim(), description.trim());
      onClose();
    } catch (err) {
      // Surface top-level errors; retain local state
      const message =
        err instanceof Error ? err.message : "Failed to create project";
      setErrors((prev) => ({ ...prev, id: message }));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle sx={{ fontWeight: 700, fontSize: 26 }}>
        Create a Project
      </DialogTitle>

      <DialogContent sx={{ pt: 1, pb: 3 }}>
        <Typography variant="subtitle1" fontWeight={600} gutterBottom>
          Project Details
        </Typography>

        <Stack direction={{ xs: "column", md: "row" }} spacing={2} mt={2}>
          <TextField
            label="Display Name"
            placeholder="Enter Project Name"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            fullWidth
            error={Boolean(errors.name)}
            helperText={errors.name}
          />

          <TextField
            label="Identifier"
            placeholder="auto-generated"
            value={identifier}
            onChange={(event) => {
              if (identifierLocked) {
                return;
              }
              setIdentifier(slugify(event.target.value));
            }}
            fullWidth
            error={Boolean(errors.id)}
            helperText={errors.id || ""}
            InputProps={{
              readOnly: identifierLocked,
              endAdornment: (
                <Tooltip title={identifierLocked ? "Unlock to edit" : "Lock"}>
                  <IconButton
                    aria-label="Toggle identifier edit"
                    onClick={() => setIdentifierLocked((prev) => !prev)}
                    size="small"
                  >
                    <EditOutlined fontSize="small" />
                  </IconButton>
                </Tooltip>
              ),
            }}
          />

          <TextField
            label="Description"
            placeholder="Enter Description here"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            fullWidth
            multiline
            minRows={1}
            maxRows={4}
            helperText="Optional"
          />
        </Stack>

        <Stack direction="row" spacing={1} alignItems="center" mt={1.5}>
          <InfoOutlined sx={{ color: "text.secondary", fontSize: 18 }} />
          <Typography variant="caption" color="text.secondary">
            Identifiers are URL friendly and auto-generated from the display name. Unlock to provide a custom value.
          </Typography>
        </Stack>
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 3 }}>
        <Button variant="outlined" onClick={onClose} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={submitting || !displayName.trim() || !identifier.trim()}
        >
          Create
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default CreateProjectDialog;

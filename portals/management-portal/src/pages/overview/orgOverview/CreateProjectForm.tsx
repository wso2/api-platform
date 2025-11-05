import React from "react";
import {
  Box,
  IconButton,
  Stack,
  Tooltip,
  Typography,
  Button as MuiButton,
} from "@mui/material";
import EditOutlined from "@mui/icons-material/EditOutlined";
import ArrowBackIosNewRoundedIcon from "@mui/icons-material/ArrowBackIosNewRounded";
import { slugify } from "../utils";
import { TextInput } from "../../../components/src/components/TextInput";
import { Button } from "../../../components/src/components/Button";
import ArrowLeftLong from "../../../components/src/Icons/generated/ArrowLeftLong";

type Props = {
  onSubmit: (displayName: string, description: string) => Promise<void> | void;
  onBack: () => void;
};

const CreateProjectForm: React.FC<Props> = ({ onSubmit, onBack }) => {
  const [displayName, setDisplayName] = React.useState("");
  const [identifier, setIdentifier] = React.useState("");
  const [identifierLocked, setIdentifierLocked] = React.useState(true);
  const [description, setDescription] = React.useState("");
  const [errors, setErrors] = React.useState<{ name?: string; id?: string }>(
    {}
  );
  const [submitting, setSubmitting] = React.useState(false);

  React.useEffect(() => {
    if (identifierLocked) {
      setIdentifier(slugify(displayName));
    }
  }, [displayName, identifierLocked]);

  const validate = React.useCallback(() => {
    const next: { name?: string; id?: string } = {};
    if (!displayName.trim()) next.name = "Display name is required";
    if (!identifier.trim()) next.id = "Identifier is required";
    setErrors(next);
    return Object.keys(next).length === 0;
  }, [displayName, identifier]);

  const handleSubmit = async () => {
    if (!validate()) return;
    setSubmitting(true);
    try {
      await onSubmit(displayName.trim(), description.trim());
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Box sx={{ mb: 4 }}>
      {/* Back link */}
      <Box mb={2}>
        <Button
          onClick={onBack}
          variant="link"
          startIcon={<ArrowLeftLong fontSize="small" />}
        >
          Back to Home
        </Button>
      </Box>

      <Typography variant="h3" fontWeight={600} gutterBottom>
        Create a Project
      </Typography>

      {/* ROW 1 */}
      <Box
        sx={{ mt: 2, width: { xs: "100%", md: "50%" } }}
        display="flex"
        gap={2}
        flexDirection="row"
      >
        <TextInput
          label="Display Name"
          placeholder="Enter Project Name"
          value={displayName}
          onChange={setDisplayName}
          fullWidth
          error={Boolean(errors.name)}
          errorMessage={errors.name}
          testId="create-project-display-name"
          size="medium"
        />

        <TextInput
          label="Identifier"
          placeholder="auto-generated"
          value={identifier}
          onChange={(v) => {
            if (identifierLocked) return;
            setIdentifier(slugify(v));
          }}
          fullWidth
          error={Boolean(errors.id)}
          errorMessage={errors.id}
          testId="create-project-identifier"
          size="medium"
          readonly={identifierLocked}
          endAdornment={
            <Tooltip title={identifierLocked ? "Unlock to edit" : "Lock"}>
              <IconButton
                aria-label="Toggle identifier edit"
                onClick={() => setIdentifierLocked((p) => !p)}
                size="small"
              >
                <EditOutlined fontSize="small" />
              </IconButton>
            </Tooltip>
          }
        />
      </Box>

      {/* ROW 2 */}
      <Box sx={{ mt: 2, width: { xs: "100%", md: "50%" } }}>
        <TextInput
          label="Description"
          placeholder="Enter Description here"
          value={description}
          onChange={setDescription}
          fullWidth
          rows={4}
          optional
          testId="create-project-description"
          size="medium"
        />
      </Box>

      {/* Buttons */}
      <Stack direction="row" spacing={1} justifyContent="flex-start" mt={4}>
        <Button variant="outlined" onClick={onBack} disabled={submitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={submitting || !displayName.trim() || !identifier.trim()}
        >
          Create
        </Button>
      </Stack>
    </Box>
  );
};

export default CreateProjectForm;

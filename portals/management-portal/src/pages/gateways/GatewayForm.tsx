import React from "react";
import {
  Box,
  Checkbox,
  FormControlLabel,
  Grid,
  Stack,
  Typography,
} from "@mui/material";
import { Button } from "../../components/src/components/Button";
import { TextInput } from "../../components/src/components/TextInput";
import type { GatewayType } from "../../hooks/gateways";
import ArrowLeftLong from "../../components/src/Icons/generated/ArrowLeftLong";

type Props = {
  type: GatewayType;
  editingId: string | null;
  displayName: string;
  name: string;
  host: string;
  description: string;
  isCritical: boolean;
  isSubmitting: boolean;

  onChangeDisplayName: (v: string) => void;
  onChangeHost: (v: string) => void;
  onChangeDescription: (v: string) => void;
  onChangeIsCritical: (v: boolean) => void;

  onCancel: () => void;
  onSubmit: () => void;

  /** NEW: go-back handler for the header link */
  onBack: () => void;
};

const GatewayForm: React.FC<Props> = ({
  type,
  editingId,
  displayName,
  name,
  host,
  description,
  isCritical,
  isSubmitting,
  onChangeDisplayName,
  onChangeHost,
  onChangeDescription,
  onChangeIsCritical,
  onCancel,
  onSubmit,
  onBack,
}) => {
  return (
    <Box maxWidth={640}>
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

      {/* Title */}
      <Typography variant="h3" mb={2} fontWeight={600}>
        {editingId
          ? `Edit ${type === "hybrid" ? "On Premise" : "Cloud"} Gateway`
          : `Create ${type === "hybrid" ? "On Premise" : "Cloud"} Gateway`}
      </Typography>

      {/* Form grid */}
      <Grid container spacing={2}>
        {/* Row 1: Display name + Identifier */}
        <Grid size={{ xs: 12, md: 6}}>
          <TextInput
            label="Name"
            value={displayName}
            onChange={onChangeDisplayName}
            placeholder="e.g., My Production Gateway"
            fullWidth
            size="medium"
            testId="create-gw-name"
          />
        </Grid>
        <Grid size={{ xs: 12, md: 6}}>
          <TextInput
            label="Identifier"
            value={name}
            onChange={() => {}}
            fullWidth
            size="medium"
            readonly
            testId="create-gw-Identifire"
          />
        </Grid>

        {/* Row 2: Host */}
        <Grid  size={{ xs: 12}}>
          <TextInput
            label="Host"
            value={host}
            onChange={onChangeHost}
            placeholder="e.g., gateway.dev.local"
            fullWidth
            size="medium"
            testId="create-gw-host"
          />
        </Grid>

        {/* Row 3: Description */}
        <Grid size={{ xs: 12}}>
          <TextInput
            label="Description"
            value={description}
            onChange={onChangeDescription}
            placeholder="Optional description for your gateway"
            fullWidth
            rows={3}
            size="medium"
            testId="create-gw-description"
          />
        </Grid>

        {/* Checkbox row */}
        <Grid size={{ xs: 12}}>
          <FormControlLabel
            control={
              <Checkbox
                checked={isCritical}
                onChange={(e) => onChangeIsCritical(e.target.checked)}
              />
            }
            label="Mark this as a critical gateway"
          />
        </Grid>

        {/* Actions row */}
        <Grid size={{ xs: 12}}>
          <Stack direction="row" spacing={2} justifyContent="flex-start" mt={1}>
            <Button variant="outlined" onClick={onCancel}>
              Cancel
            </Button>
            <Button
              variant="contained"
              onClick={onSubmit}
              disabled={!displayName.trim() || isSubmitting}
              sx={{ textTransform: "none" }}
            >
              {editingId ? "Save" : isSubmitting ? "Adding..." : "Add"}
            </Button>
          </Stack>
        </Grid>
      </Grid>
    </Box>
  );
};

export default GatewayForm;

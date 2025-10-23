import React from "react";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Step,
  StepLabel,
  Stepper,
  TextField,
  Typography,
} from "@mui/material";

export type EndpointWizardStep = "endpoint" | "details";

export type EndpointCreationState = {
  endpointUrl: string;
  name: string;
  context: string;
  version: string;
  description?: string;
};

type EndpointCreationDialogProps = {
  open: boolean;
  step: EndpointWizardStep;
  state: EndpointCreationState;
  onChange: (patch: Partial<EndpointCreationState>) => void;
  onStepChange: (step: EndpointWizardStep) => void;
  onClose: () => void;
  onCreate: () => Promise<void> | void;
  creating?: boolean;
};

const steps: EndpointWizardStep[] = ["endpoint", "details"];

const EndpointCreationDialog: React.FC<EndpointCreationDialogProps> = ({
  open,
  step,
  state,
  onChange,
  onStepChange,
  onClose,
  onCreate,
  creating,
}) => {
  const stepIndex = steps.indexOf(step);

  return (
    <Dialog open={open} onClose={creating ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Create API from Endpoint</DialogTitle>
      <DialogContent>
        <Stepper activeStep={stepIndex} sx={{ pt: 1, pb: 3 }}>
          <Step key="endpoint">
            <StepLabel>Endpoint</StepLabel>
          </Step>
          <Step key="details">
            <StepLabel>API Details</StepLabel>
          </Step>
        </Stepper>

        {step === "endpoint" && (
          <Stack spacing={2}>
            <Typography color="text.secondary">
              Provide the backend service URL. We'll use it to configure the default
              endpoint of your API.
            </Typography>
            <TextField
              label="Endpoint URL"
              placeholder="https://api.example.com/v1"
              fullWidth
              value={state.endpointUrl}
              onChange={(event) => onChange({ endpointUrl: event.target.value })}
              autoFocus
            />
          </Stack>
        )}

        {step === "details" && (
          <Stack spacing={2}>
            <TextField
              label="Name"
              placeholder="Sample API"
              fullWidth
              value={state.name}
              onChange={(event) => onChange({ name: event.target.value })}
              autoFocus
            />
            <TextField
              label="Context"
              placeholder="/sample"
              fullWidth
              value={state.context}
              onChange={(event) => onChange({ context: event.target.value })}
            />
            <TextField
              label="Version"
              placeholder="1.0.0"
              fullWidth
              value={state.version}
              onChange={(event) => onChange({ version: event.target.value })}
            />
            <TextField
              label="Description"
              placeholder="Optional description"
              fullWidth
              multiline
              minRows={2}
              value={state.description ?? ""}
              onChange={(event) => onChange({ description: event.target.value })}
            />
            <Box sx={{ mt: 1 }}>
              <Typography variant="body2" color="text.secondary">
                The endpoint URL will be added as the default backend of this API.
              </Typography>
            </Box>
          </Stack>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose} disabled={creating} sx={{ textTransform: "none" }}>
          Cancel
        </Button>

        {step === "details" ? (
          <Button
            variant="contained"
            onClick={onCreate}
            disabled={creating || !state.name || !state.context || !state.version}
            sx={{ textTransform: "none" }}
          >
            {creating ? "Creating..." : "Create"}
          </Button>
        ) : (
          <Button
            variant="contained"
            onClick={() => onStepChange("details")}
            disabled={!state.endpointUrl.trim()}
            sx={{ textTransform: "none" }}
          >
            Next
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
};

export default EndpointCreationDialog;

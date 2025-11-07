import * as React from "react";
import {
  Box,
  Stack,
  Typography,
  Stepper,
  Step,
  StepLabel,
  Alert,
} from "@mui/material";
import { Button } from "../../../components/src/components/Button";
import { Card } from "../../../components/src/components/Card";
import { TextInput } from "../../../components/src/components/TextInput";

type EndpointWizardStep = "endpoint" | "details";

type EndpointCreationState = {
  endpointUrl: string;
  name: string;
  context: string;
  version: string;
  description?: string;
  contextEdited: boolean;
};

type CreateApiFn = (payload: {
  name: string;
  context: string;
  version: string;
  description?: string;
  projectId: string;
  backendServices: Array<{
    name: string;
    isDefault: boolean;
    endpoints: Array<{ url: string; description?: string }>;
    retries: number;
  }>;
}) => Promise<unknown>;

type Props = {
  open: boolean;
  selectedProjectId: string;
  createApi: CreateApiFn;
  onClose: () => void;
};

const EndPointCreationFlow: React.FC<Props> = ({
  open,
  selectedProjectId,
  createApi,
  onClose,
}) => {
  const [wizardStep, setWizardStep] =
    React.useState<EndpointWizardStep>("endpoint");
  const [wizardState, setWizardState] = React.useState<EndpointCreationState>({
    endpointUrl: "",
    name: "",
    context: "",
    version: "1.0.0",
    description: "",
    contextEdited: false,
  });
  const [wizardError, setWizardError] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);

  const reset = React.useCallback(() => {
    setWizardStep("endpoint");
    setWizardState({
      endpointUrl: "",
      name: "",
      context: "",
      version: "1.0.0",
      description: "",
      contextEdited: false,
    });
    setWizardError(null);
  }, []);

  const handleChange = React.useCallback(
    (patch: Partial<EndpointCreationState>) => {
      setWizardState((p) => ({ ...p, ...patch }));
    },
    []
  );

  const handleContextChange = React.useCallback(
    (value: string) => {
      handleChange({ context: value, contextEdited: true });
    },
    [handleChange]
  );

  const inferNameFromEndpoint = React.useCallback((url: string) => {
    try {
      const withoutQuery = url.split("?")[0];
      const segments = withoutQuery.split("/").filter(Boolean);
      const candidate = segments[segments.length - 1] ?? "api";
      const clean = candidate.replace(/[^a-zA-Z0-9]+/g, " ").trim();
      if (!clean) return "Sample API";
      return clean
        .split(" ")
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(" ");
    } catch {
      return "Sample API";
    }
  }, []);

  const handleNameChange = React.useCallback((value: string) => {
    setWizardState((prev) => {
      const next = { ...prev, name: value };
      if (!prev.contextEdited) {
        const slug = value
          .trim()
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, "-")
          .replace(/^-+|-+$/g, "");
        next.context = slug ? `/${slug}` : "";
      }
      return next;
    });
  }, []);

  const handleStepChange = React.useCallback(
    (next: EndpointWizardStep) => {
      if (next === "details") {
        const inferred = inferNameFromEndpoint(wizardState.endpointUrl);
        if (!wizardState.name.trim()) {
          handleNameChange(inferred);
        }
        if (!wizardState.contextEdited && !wizardState.context.trim()) {
          const slug = inferred
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, "-")
            .replace(/^-+|-+$/g, "");
          handleChange({
            context: slug ? `/${slug}` : "",
            contextEdited: false,
          });
        }
      }
      setWizardStep(next);
    },
    [handleChange, handleNameChange, inferNameFromEndpoint, wizardState]
  );

  const handleCreate = React.useCallback(async () => {
    const endpointUrl = wizardState.endpointUrl.trim();
    const name = wizardState.name.trim();
    const context = wizardState.context.trim();
    const version = wizardState.version.trim() || "1.0.0";

    if (!endpointUrl || !name || !context) {
      setWizardError("Please complete all required fields.");
      return;
    }

    try {
      setWizardError(null);
      setCreating(true);
      const uniqueBackendName = `default-backend-${Date.now().toString(
        36
      )}${Math.random().toString(36).slice(2, 8)}`;

      await createApi({
        name,
        context: context.startsWith("/") ? context : `/${context}`,
        version,
        description: wizardState.description?.trim() || undefined,
        projectId: selectedProjectId,
        backendServices: [
          {
            name: uniqueBackendName,
            isDefault: true,
            endpoints: [
              { url: endpointUrl, description: "Default backend endpoint" },
            ],
            retries: 0,
          },
        ],
      });

      reset();
      onClose();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to create API";
      setWizardError(message);
    } finally {
      setCreating(false);
    }
  }, [createApi, onClose, reset, selectedProjectId, wizardState]);

  if (!open) return null;

  return (
    <>
      <Typography variant="h3" fontWeight={600} sx={{ mb: 3 }}>
        Create API from Endpoint
      </Typography>

      <Card testId="" style={{ padding: 24, maxWidth: 800 }}>
        <Stepper
          activeStep={wizardStep === "endpoint" ? 0 : 1}
          sx={{ pt: 1, pb: 3, width: 500, maxWidth: "100%", mx: "auto" }}
        >
          <Step>
            <StepLabel>Endpoint</StepLabel>
          </Step>
          <Step>
            <StepLabel>API Details</StepLabel>
          </Step>
        </Stepper>

        {wizardStep === "endpoint" ? (
          <Stack spacing={2}>
            <Typography color="#848181ff">
              Provide the backend service URL. We'll use it to configure the
              default endpoint of your API.
            </Typography>
            <TextInput
              label="Endpoint URL"
              placeholder="https://api.example.com/v1"
              value={wizardState.endpointUrl}
              onChange={(v: string) => handleChange({ endpointUrl: v })}
              testId=""
              size="medium"
            />
          </Stack>
        ) : (
          <Stack spacing={2}>
            <TextInput
              label="Name"
              placeholder="Sample API"
              value={wizardState.name}
              onChange={(v: string) => handleNameChange(v)}
              testId=""
              size="medium"
            />
            <TextInput
              label="Context"
              placeholder="/sample"
              value={wizardState.context}
              onChange={(v: string) => handleContextChange(v)}
              testId=""
              size="medium"
            />
            <TextInput
              label="Version"
              placeholder="1.0.0"
              value={wizardState.version}
              onChange={(v: string) => handleChange({ version: v })}
              testId=""
              size="medium"
            />
            <TextInput
              label="Description"
              placeholder="Optional description"
              value={wizardState.description ?? ""}
              onChange={(v: string) => handleChange({ description: v })}
              multiline
              testId=""
            />
            <Box sx={{ mt: 1 }}>
              <Typography variant="body2" color="text.secondary">
                The endpoint URL will be added as the default backend of this
                API.
              </Typography>
            </Box>
          </Stack>
        )}

        <Stack
          direction="row"
          spacing={1}
          justifyContent="flex-end"
          sx={{ mt: 3 }}
        >
          <Button
            variant="outlined"
            onClick={() => {
              if (!creating) {
                reset();
                onClose();
              }
            }}
            disabled={creating}
            sx={{ textTransform: "none" }}
          >
            Cancel
          </Button>

          {wizardStep === "details" ? (
            <Button
              variant="contained"
              onClick={handleCreate}
              disabled={
                creating ||
                !wizardState.name.trim() ||
                !wizardState.context.trim() ||
                !wizardState.version.trim()
              }
              sx={{ textTransform: "none" }}
            >
              {creating ? "Creating..." : "Create"}
            </Button>
          ) : (
            <Button
              variant="contained"
              onClick={() => handleStepChange("details")}
              disabled={!wizardState.endpointUrl.trim()}
              sx={{ textTransform: "none" }}
            >
              Next
            </Button>
          )}
        </Stack>

        {wizardError && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {wizardError}
          </Alert>
        )}
      </Card>
    </>
  );
};

export default EndPointCreationFlow;

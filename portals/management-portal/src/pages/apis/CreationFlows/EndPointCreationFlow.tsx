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

import {
  useCreateComponentBuildpackContext,
} from "../../../context/CreateComponentBuildpackContext";
import { type CreateApiPayload } from "../../../hooks/apis";
import CreationMetaData from "./CreationMetaData";

type EndpointWizardStep = "endpoint" | "details";

type EndpointCreationState = {
  endpointUrl: string;
};

type CreateApiFn = (payload: CreateApiPayload) => Promise<unknown>;

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
  });

  const {
    endpointMeta,
    setEndpointMeta,
    resetEndpointMeta,
  } = useCreateComponentBuildpackContext();

  const [wizardError, setWizardError] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);

  // Reset this flow's slice when opened
  React.useEffect(() => {
    if (open) {
      resetEndpointMeta();
      setWizardStep("endpoint");
      setWizardState({ endpointUrl: "" });
      setWizardError(null);
    }
  }, [open, resetEndpointMeta]);

  const handleChange = React.useCallback(
    (patch: Partial<EndpointCreationState>) => {
      setWizardState((p) => ({ ...p, ...patch }));
    },
    []
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

  const handleStepChange = React.useCallback(
    (next: EndpointWizardStep) => {
      if (next === "details") {
        const inferred = inferNameFromEndpoint(wizardState.endpointUrl);
        const needsName = !(endpointMeta?.name || "").trim();
        const needsContext = !(endpointMeta?.context || "").trim();
        setEndpointMeta((prev: any) => {
          const base = prev || {};
          const nextMeta = { ...base };
          if (needsName) nextMeta.name = inferred;
          if (needsContext && !base?.contextEdited) {
            const slug = inferred
              .toLowerCase()
              .replace(/[^a-z0-9]+/g, "-")
              .replace(/^-+|-+$/g, "");
            nextMeta.context = slug ? `/${slug}` : "";
          }
          return nextMeta;
        });
      }
      setWizardStep(next);
    },
    [inferNameFromEndpoint, endpointMeta, setEndpointMeta, wizardState.endpointUrl]
  );

  const handleCreate = React.useCallback(async () => {
    const endpointUrl = wizardState.endpointUrl.trim();
    const displayName = (endpointMeta?.displayName || endpointMeta?.name || "").trim();
    const identifier = (endpointMeta?.identifier || endpointMeta?.name || "").trim();
    const context = (endpointMeta?.context || "").trim();
    const version = (endpointMeta?.version || "").trim() || "1.0.0";

    if (!endpointUrl || !displayName || !identifier || !context) {
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
        name: identifier,
        displayName,
        context: context.startsWith("/") ? context : `/${context}`,
        version,
        description: endpointMeta?.description?.trim() || undefined,
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

      // fresh state for the next time it's opened
      resetEndpointMeta();
      setWizardState({ endpointUrl: "" });
      onClose();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to create API";
      setWizardError(message);
    } finally {
      setCreating(false);
    }
  }, [createApi, endpointMeta, onClose, resetEndpointMeta, selectedProjectId, wizardState.endpointUrl]);

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
            <CreationMetaData scope="endpoint" title="API Details" />
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
                resetEndpointMeta();
                setWizardState({ endpointUrl: "" });
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
                !(endpointMeta?.name || "").trim() ||
                !(endpointMeta?.context || "").trim() ||
                !(endpointMeta?.version || "").trim()
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

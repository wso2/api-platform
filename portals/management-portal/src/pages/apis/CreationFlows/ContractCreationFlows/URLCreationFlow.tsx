import * as React from "react";
import { Box, Paper, Stack, Typography, Alert, Grid, CircularProgress } from "@mui/material";
import { Button } from "../../../../components/src/components/Button";
import { TextInput } from "../../../../components/src/components/TextInput";
import CreationMetaData from "../CreationMetaData";
import { useCreateComponentBuildpackContext } from "../../../../context/CreateComponentBuildpackContext";
import {
  useOpenApiValidation,
  type OpenApiValidationResponse,
} from "../../../../hooks/validation";
import type { ImportOpenApiRequest, ApiSummary } from "../../../../hooks/apis";
import {
  defaultServiceName,
  firstServerUrl,
  deriveContext,
  formatVersionToMajorMinor,
  isValidMajorMinorVersion,
} from "../../../../helpers/openApiHelpers";
import SwaggerPreviewWithSource from "../../../../components/src/components/Common/SwaggerPreviewWithSource";

type Props = {
  open: boolean;
  selectedProjectId: string;
  importOpenApi: (
    payload: ImportOpenApiRequest,
    opts?: { signal?: AbortSignal }
  ) => Promise<ApiSummary>;
  refreshApis: (projectId?: string) => Promise<ApiSummary[]>;
  onClose: () => void;
};

type Step = "url" | "details";

const URLCreationFlow: React.FC<Props> = ({
  open,
  selectedProjectId,
  importOpenApi,
  refreshApis,
  onClose,
}) => {
  const [step, setStep] = React.useState<Step>("url");
  const [specUrl, setSpecUrl] = React.useState<string>("");
  const [validationResult, setValidationResult] =
    React.useState<OpenApiValidationResponse | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [validating, setValidating] = React.useState(false);
  const [creating, setCreating] = React.useState(false);

  const { contractMeta, setContractMeta, resetContractMeta } =
    useCreateComponentBuildpackContext();
  const { validateOpenApiUrl } = useOpenApiValidation();
  const abortControllerRef = React.useRef<AbortController | null>(null);
  const debounceRef = React.useRef<number | null>(null);

  React.useEffect(() => {
    return () => {
      abortControllerRef.current?.abort();
    };
  }, []);

  React.useEffect(() => {
    if (open) {
      resetContractMeta();
      setStep("url");
      setSpecUrl("");
      setValidationResult(null);
      setError(null);
      setValidating(false);
    }
  }, [open, resetContractMeta]);

  const autoFill = React.useCallback(
    (api: any) => {
      const title = api?.name?.trim() || api?.displayName?.trim() || "";
      const version = formatVersionToMajorMinor(api?.version);
      const description = api?.description || "";
      const targetUrl = firstServerUrl(api);

      setContractMeta((prev: any) => ({
        ...prev,
        name: title || prev?.name || "Sample API",
        version,
        description,
        context: deriveContext(api),
        target: prev?.target || targetUrl || "",
      }));
    },
    [setContractMeta]
  );

  const handleFetchAndPreview = React.useCallback(async () => {
    if (!specUrl.trim()) return;

    abortControllerRef.current?.abort();
    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    try {
      setError(null);
      setValidating(true);
      setValidationResult(null);

      const result = await validateOpenApiUrl(specUrl.trim(), {
        signal: abortController.signal,
      });
      setValidationResult(result);

      if (result.isAPIDefinitionValid) {
        autoFill(result.api);
      } else {
        const errorMsg =
          result.errors?.join(", ") || "Invalid OpenAPI definition";
        setError(errorMsg);
      }
    } catch (e: any) {
      if (e.name === "AbortError") return;
      setError(e?.message || "Failed to validate OpenAPI from URL");
      setValidationResult(null);
    } finally {
      setValidating(false);
    }
  }, [specUrl, autoFill, validateOpenApiUrl]);

  React.useEffect(() => {
    const url = specUrl.trim();

    if (debounceRef.current) window.clearTimeout(debounceRef.current);
    abortControllerRef.current?.abort();

    if (!url) {
      setValidationResult(null);
      setError(null);
      setValidating(false);
      return;
    }

    debounceRef.current = window.setTimeout(() => {
      handleFetchAndPreview();
    }, 600);

    return () => {
      if (debounceRef.current) window.clearTimeout(debounceRef.current);
    };
  }, [specUrl, handleFetchAndPreview]);

  const finishAndClose = React.useCallback(() => {
    abortControllerRef.current?.abort();
    resetContractMeta();
    setStep("url");
    setSpecUrl("");
    setValidationResult(null);
    setError(null);
    setValidating(false);
    onClose();
  }, [onClose, resetContractMeta]);

  const isDefinitionValid = validationResult?.isAPIDefinitionValid ?? false;

  const onCreate = async () => {
    const name = (contractMeta?.name || "").trim();
    const context = (contractMeta?.context || "").trim();
    const version = (contractMeta?.version || "").trim();
    const description = (contractMeta?.description || "").trim() || undefined;
    const target = (contractMeta?.target || "").trim();

    if (!name || !context || !version) {
      setError("Please complete all required fields.");
      return;
    }
    if (target) {
      try {
        if (/^https?:\/\//i.test(target)) new URL(target);
      } catch {
        setError("Target must be a valid URL (or leave it empty).");
        return;
      }
    }

    if (!validationResult?.isAPIDefinitionValid) {
      setError("Please fetch and validate the OpenAPI definition first.");
      return;
    }

    setCreating(true);
    setError(null);

    const serviceName = defaultServiceName(name);
    const backendServices = target
      ? [
          {
            name: serviceName,
            isDefault: true,
            retries: 2,
            endpoints: [{ url: target, description: "Primary backend" }],
          },
        ]
      : [];

    try {
      await importOpenApi({
        api: {
          name,
          context,
          version,
          projectId: selectedProjectId,
          target,
          description,
          backendServices,
        },
        url: specUrl.trim(),
      });
    } catch (e: any) {
      setError(e?.message || "Failed to create API");
      setCreating(false);
      return;
    }

    try {
      await refreshApis(selectedProjectId);
    } catch (refreshError) {
      console.warn("Failed to refresh API list after creation:", refreshError);
    } finally {
      setCreating(false);
    }

    finishAndClose();
  };

  if (!open) return null;

  return (
    <Box>
      {step === "url" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Paper variant="outlined" sx={{ p: 3, borderRadius: 2 }}>
              <Typography variant="h5" sx={{ mb: 2 }}>
                URL for API Contract
              </Typography>
              <TextInput
                label=""
                placeholder="https://example.com/openapi.yaml"
                value={specUrl}
                onChange={(v: string) => setSpecUrl(v)}
                testId=""
                size="medium"
              />
              <Stack direction="row" spacing={1} sx={{ mt: 1 }} alignItems={"center"}>
                <Box>{validating ? <CircularProgress size={16} /> : null}</Box>
                <Button
                  variant="link"
                  onClick={() =>
                    setSpecUrl("https://petstore.swagger.io/v2/swagger.json")
                  }
                  disabled={validating}
                >
                  {validating ? "Validating..." : "Try with Sample URL"}
                </Button>
                <Box flex={1} />
                {/* <Button
                  variant="outlined"
                  onClick={handleFetchAndPreview}
                  disabled={!specUrl.trim() || validating}
                >
                  {validating ? "Validating..." : "Fetch & Preview"}
                </Button> */}
              </Stack>

              {error && (
                <Alert severity="error" sx={{ mt: 2 }}>
                  {error}
                </Alert>
              )}
            </Paper>

            <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
              <Button
                variant="outlined"
                onClick={finishAndClose}
                sx={{ textTransform: "none" }}
              >
                Cancel
              </Button>
              <Button
                variant="contained"
                disabled={!validationResult?.isAPIDefinitionValid || validating}
                onClick={() => setStep("details")}
                sx={{ textTransform: "none" }}
              >
                Next
              </Button>
            </Stack>
          </Grid>
          {isDefinitionValid && (
            <Grid size={{ xs: 12, md: 6 }}>
              <SwaggerPreviewWithSource
                title="Fetched OAS Definition"
                definitionUrl={specUrl}
                isValid={isDefinitionValid}
                isLoading={validating}
                apiEndpoint={(contractMeta?.target || "").trim() || undefined}
              />
            </Grid>
          )}
        </Grid>
      )}

      {step === "details" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            {/* <Paper variant="outlined" sx={{ p: 3, borderRadius: 2 }}> */}
            <CreationMetaData scope="contract" />
            <Stack
              direction="row"
              spacing={1}
              justifyContent="flex-start"
              sx={{ mt: 3 }}
            >
              <Button
                variant="outlined"
                onClick={() => setStep("url")}
                sx={{ textTransform: "none" }}
              >
                Back
              </Button>
              <Button
                variant="contained"
                disabled={
                  creating ||
                  !(contractMeta?.name || "").trim() ||
                  !(contractMeta?.context || "").trim() ||
                  !isValidMajorMinorVersion(
                    (contractMeta?.version || "").trim()
                  )
                }
                onClick={onCreate}
                sx={{ textTransform: "none" }}
              >
                {creating ? "Creating..." : "Create"}
              </Button>
            </Stack>

            {error && (
              <Alert severity="error" sx={{ mt: 2 }}>
                {error}
              </Alert>
            )}
            {/* </Paper> */}
          </Grid>
          {/* 
          <Grid size={{ xs: 12, md: 6 }}>
            <SwaggerPreviewWithSource
              title="Fetched OAS Definition"
              definitionUrl={specUrl}
              isValid={isDefinitionValid}
              isLoading={validating}
              apiEndpoint={(contractMeta?.target || "").trim() || undefined}
            />
          </Grid> */}
        </Grid>
      )}
    </Box>
  );
};

export default URLCreationFlow;

import * as React from "react";
import { Box, Paper, Stack, Typography, Alert, Grid } from "@mui/material";
import { Button } from "../../../../components/src/components/Button";
import { TextInput } from "../../../../components/src/components/TextInput";
import CreationMetaData from "../CreationMetaData";
import {
  useCreateComponentBuildpackContext,
} from "../../../../context/CreateComponentBuildpackContext";
import { useOpenApiValidation, type OpenApiValidationResponse } from "../../../../hooks/validation";
import { ApiOperationsList } from "../../../../components/src/components/Common/ApiOperationsList";

/* ---------- Types ---------- */
type Props = {
  open: boolean;
  selectedProjectId: string;
  importOpenApi: (payload: {
    api: {
      name: string;
      context: string;
      version: string;
      projectId: string;
      target?: string;
      description?: string;
      backendServices?: any[];
    };
    url?: string;
    definition?: string;
  }, opts?: { signal?: AbortSignal }) => Promise<void>;
  refreshApis: (projectId?: string) => Promise<any[]>;
  onClose: () => void;
};

type Step = "url" | "details";

/* ---------- helpers ---------- */

function slugify(val: string) {
  return val.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

function defaultServiceName(apiName: string) {
  const base = apiName?.trim() || "service";
  return `${slugify(base)}-service`;
}

function firstServerUrl(api: any) {
  const services = api?.["backend-services"] || [];
  const endpoint = services[0]?.endpoints?.[0]?.url;
  return endpoint?.trim() || "";
}

function deriveContext(api: any) {
  return api?.context || "/api";
}

function mapOperations(
  operations: any[],
  options?: { serviceName?: string; withFallbackName?: boolean }
) {
  if (!Array.isArray(operations)) return [];
  
  return operations.map((op: any) => ({
    name: options?.withFallbackName 
      ? (op.name || op.request?.path || "Unknown")
      : op.name,
    description: op.description,
    request: {
      method: op.request?.method || "GET",
      path: op.request?.path || "/",
      ...(options?.serviceName && { ["backend-services"]: [{ name: options.serviceName }] }),
    },
  }));
}

/* ---------- component ---------- */

const URLCreationFlow: React.FC<Props> = ({ open, selectedProjectId, importOpenApi, refreshApis, onClose }) => {
  const [step, setStep] = React.useState<Step>("url");
  const [specUrl, setSpecUrl] = React.useState<string>("");
  const [validationResult, setValidationResult] = React.useState<OpenApiValidationResponse | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [validating, setValidating] = React.useState(false);
  const [creating, setCreating] = React.useState(false);

  const { contractMeta, setContractMeta, resetContractMeta } = useCreateComponentBuildpackContext();
  const { validateOpenApiUrl } = useOpenApiValidation();

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

  const autoFill = React.useCallback((api: any) => {
    const title = api?.name?.trim() || api?.displayName?.trim() || "";
    const version = api?.version?.trim() || "1.0.0";
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
  }, [setContractMeta]);

  const handleFetchAndPreview = React.useCallback(async () => {
    if (!specUrl.trim()) return;

    try {
      setError(null);
      setValidating(true);
      setValidationResult(null);

      const result = await validateOpenApiUrl(specUrl.trim());
      setValidationResult(result);

      if (result.isAPIDefinitionValid) {
        autoFill(result.api);
        setStep("details");
      } else {
        const errorMsg = result.errors?.join(", ") || "Invalid OpenAPI definition";
        setError(errorMsg);
      }
    } catch (e: any) {
      setError(e?.message || "Failed to validate OpenAPI from URL");
      setValidationResult(null);
    } finally {
      setValidating(false);
    }
  }, [specUrl, autoFill, validateOpenApiUrl]);

  const finishAndClose = React.useCallback(() => {
    resetContractMeta();
    setStep("url");
    setSpecUrl("");
    setValidationResult(null);
    setError(null);
    setValidating(false);
    onClose();
  }, [onClose, resetContractMeta]);

  const previewOps = React.useMemo(() => {
    if (!validationResult?.isAPIDefinitionValid) return [];
    const api = validationResult.api as any;
    return mapOperations(api?.operations || [], { withFallbackName: true });
  }, [validationResult]);

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
    const backendServices =
      target
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
              <Typography variant="subtitle2" sx={{ mb: 1 }}>
                Public Specification URL
              </Typography>
              <TextInput
                label=""
                placeholder="https://example.com/openapi.yaml"
                value={specUrl}
                onChange={(v: string) => setSpecUrl(v)}
                testId=""
                size="medium"
              />

              <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
                <Button
                  variant="text"
                  onClick={() =>
                    setSpecUrl(
                      "https://petstore.swagger.io/v2/swagger.json"
                    )
                  }
                  disabled={validating}
                >
                  Try with Sample URL
                </Button>
                <Box flex={1} />
                <Button
                  variant="outlined"
                  onClick={handleFetchAndPreview}
                  disabled={!specUrl.trim() || validating}
                >
                  {validating ? "Validating..." : "Fetch & Preview"}
                </Button>
              </Stack>

              {error && (
                <Alert severity="error" sx={{ mt: 2 }}>
                  {error}
                </Alert>
              )}
            </Paper>

            <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
              <Button variant="outlined" onClick={finishAndClose} sx={{ textTransform: "none" }}>
                Cancel
              </Button>
            </Stack>
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            {validating ? (
              <Paper
                variant="outlined"
                sx={{ p: 3, borderRadius: 2, color: "text.secondary" }}
              >
                <Typography variant="body2">
                  Validating OpenAPI definition...
                </Typography>
              </Paper>
            ) : (
              <Paper
                variant="outlined"
                sx={{ p: 3, borderRadius: 2, color: "text.secondary" }}
              >
                <Typography variant="body2">
                  Enter a direct URL to an OpenAPI/Swagger document (YAML or JSON).
                  We'll fetch and preview it here.
                </Typography>
              </Paper>
            )}
          </Grid>
        </Grid>
      )}

      {step === "details" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Paper variant="outlined" sx={{ p: 3, borderRadius: 2 }}>
              <CreationMetaData scope="contract" title="API Details" />

              <Stack
                direction="row"
                spacing={1}
                justifyContent="flex-end"
                sx={{ mt: 3 }}
              >
                <Button variant="outlined" onClick={() => setStep("url")} sx={{ textTransform: "none" }}>
                  Back
                </Button>
                <Button
                  variant="contained"
                  disabled={
                    creating ||
                    !(contractMeta?.name || "").trim() ||
                    !(contractMeta?.context || "").trim() ||
                    !(contractMeta?.version || "").trim()
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
            </Paper>
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            <ApiOperationsList title="Fetched OAS Definition" operations={previewOps} />
          </Grid>
        </Grid>
      )}
    </Box>
  );
};

export default URLCreationFlow;

import * as React from "react";
import { Box, Paper, Stack, Typography, Alert, Grid } from "@mui/material";
import { Button } from "../../../../components/src/components/Button";
import { TextInput } from "../../../../components/src/components/TextInput";
import CreationMetaData from "../CreationMetaData";
import { useCreateComponentBuildpackContext } from "../../../../context/CreateComponentBuildpackContext";
import {
  useOpenApiValidation,
  type OpenApiValidationResponse,
} from "../../../../hooks/validation";
import { ApiOperationsList } from "../../../../components/src/components/Common/ApiOperationsList";
import type { ImportOpenApiRequest, ApiSummary } from "../../../../hooks/apis";
import {
  defaultServiceName,
  firstServerUrl,
  deriveContext,
  mapOperations,
  formatVersionToMajorMinor,
  isValidMajorMinorVersion,
} from "../../../../helpers/openApiHelpers";

const slugify = (val: string) =>
  (val || "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .trim();

const majorFromVersion = (v: string) => {
  const m = (v || "").trim().match(/\d+/);
  return m?.[0] ?? "";
};

const buildIdentifierFromNameAndVersion = (name: string, version: string) => {
  const base = slugify(name);
  const major = majorFromVersion(version);
  return major ? `${base}-v${major}` : base;
};

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
  const [metaHasErrors, setMetaHasErrors] = React.useState(false);

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
      setMetaHasErrors(false);
    }
  }, [open, resetContractMeta]);

  const autoFill = React.useCallback(
    (api: any) => {
      const title = api?.name?.trim() || api?.displayName?.trim() || "";
      const version = formatVersionToMajorMinor(api?.version);
      const description = api?.description || "";
      const targetUrl = firstServerUrl(api);

      const identifier = buildIdentifierFromNameAndVersion(title, version);

      const nextMeta = {
        name: title || "Sample API",
        displayName: title || "Sample API",
        version,
        description,
        context: deriveContext(api),
        target: targetUrl || "",
        identifier,
        identifierEdited: false,
      };

      setContractMeta((prev: any) => ({
        ...prev,
        ...nextMeta,
        target: prev?.target || nextMeta.target || "",
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
        setStep("details");
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

  const previewOps = React.useMemo(() => {
    if (!validationResult?.isAPIDefinitionValid) return [];
    const api = validationResult.api as any;
    return mapOperations(api?.operations || [], { withFallbackName: true });
  }, [validationResult]);

  const onCreate = async () => {
    const displayName = (
      contractMeta?.displayName ||
      contractMeta?.name ||
      ""
    ).trim();

    const context = (contractMeta?.context || "").trim();
    const version = (contractMeta?.version || "").trim();
    const description = (contractMeta?.description || "").trim() || undefined;
    const target = (contractMeta?.target || "").trim();

    const identifier =
      (contractMeta as any)?.identifier?.trim() ||
      buildIdentifierFromNameAndVersion(displayName, version);

    if (!displayName || !context || !version) {
      setError("Please complete all required fields.");
      return;
    }
    if (!identifier) {
      setError("Identifier is required.");
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

    const serviceName = defaultServiceName(displayName);
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

    const payload: ImportOpenApiRequest = {
      api: {
        name: identifier,
        displayName,
        context,
        version,
        projectId: selectedProjectId,
        target,
        description,
        backendServices,
      },
      url: specUrl.trim(),
    };

    try {
      await importOpenApi(payload);
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
                    setSpecUrl("https://petstore.swagger.io/v2/swagger.json")
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
              <Button
                variant="outlined"
                onClick={finishAndClose}
                sx={{ textTransform: "none" }}
              >
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
                  Enter a direct URL to an OpenAPI/Swagger document (YAML or
                  JSON). We'll fetch and preview it here.
                </Typography>
              </Paper>
            )}
          </Grid>
        </Grid>
      )}

      {step === "details" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            <CreationMetaData
              scope="contract"
              title="API Details"
              onValidationChange={({ hasError }) => setMetaHasErrors(hasError)}
            />

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
                disabled={creating}
              >
                Back
              </Button>

              <Button
                variant="contained"
                disabled={
                  creating ||
                  metaHasErrors ||
                  !(contractMeta?.displayName || contractMeta?.name || "").trim() ||
                  !(contractMeta as any)?.identifier?.trim() ||
                  !(contractMeta?.context || "").trim() ||
                  !isValidMajorMinorVersion((contractMeta?.version || "").trim())
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
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            <ApiOperationsList
              title="Fetched OAS Definition"
              operations={previewOps}
            />
          </Grid>
        </Grid>
      )}
    </Box>
  );
};

export default URLCreationFlow;

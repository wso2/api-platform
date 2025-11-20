import * as React from "react";
import { Alert, Box, Grid, Paper, Stack, Typography } from "@mui/material";
import UploadRoundedIcon from "@mui/icons-material/UploadRounded";
import { Button } from "../../../../components/src/components/Button";
import { IconButton } from "../../../../components/src/components/IconButton";
import Delete from "../../../../components/src/Icons/generated/Delete";
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

type Step = "upload" | "details";

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
const UploadCreationFlow: React.FC<Props> = ({ open, selectedProjectId, importOpenApi, refreshApis, onClose }) => {
  const [step, setStep] = React.useState<Step>("upload");
  const [rawSpec, setRawSpec] = React.useState<string>("");
  const [validationResult, setValidationResult] = React.useState<OpenApiValidationResponse | null>(null);
  const [fileName, setFileName] = React.useState<string>("");
  const [error, setError] = React.useState<string | null>(null);
  const [validating, setValidating] = React.useState(false);
  const [creating, setCreating] = React.useState(false);

  const { contractMeta, setContractMeta, resetContractMeta } = useCreateComponentBuildpackContext();
  const { validateOpenApiFile } = useOpenApiValidation();

  // Always-mounted input + stable id/label wiring
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const inputId = React.useId();
  const [fileKey, setFileKey] = React.useState(0);

  React.useEffect(() => {
    if (open) {
      resetContractMeta();
      setStep("upload");
      setRawSpec("");
      setValidationResult(null);
      setFileName("");
      setError(null);
      setValidating(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
      setFileKey((k) => k + 1);
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

  const handleFiles = React.useCallback(
    async (files: FileList | null) => {
      if (!files || !files[0]) return;
      const file = files[0];
      
      try {
        setError(null);
        setValidating(true);
        setValidationResult(null);
        
        const text = await file.text();
        setRawSpec(text);
        setFileName(file.name);

        const result = await validateOpenApiFile(file);
        setValidationResult(result);

        if (result.isAPIDefinitionValid) {
          autoFill(result.api);
        } else {
          const errorMsg = result.errors?.join(", ") || "Invalid OpenAPI definition";
          setError(errorMsg);
        }
      } catch (e: any) {
        setError(e?.message || "Failed to validate OpenAPI definition");
        setValidationResult(null);
      } finally {
        setValidating(false);
        if (fileInputRef.current) fileInputRef.current.value = "";
        setFileKey((k) => k + 1);
      }
    },
    [autoFill, validateOpenApiFile]
  );

  const onDrop = (e: React.DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    handleFiles(e.dataTransfer.files);
    if (fileInputRef.current) fileInputRef.current.value = "";
    setFileKey((k) => k + 1);
  };

  const finishAndClose = React.useCallback(() => {
    resetContractMeta();
    setStep("upload");
    setRawSpec("");
    setValidationResult(null);
    setFileName("");
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
      setError("Please upload a valid OpenAPI definition.");
      return;
    }

    try {
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
        definition: rawSpec,
      });

      await refreshApis(selectedProjectId);
      finishAndClose();
    } catch (e: any) {
      setError(e?.message || "Failed to create API");
    } finally {
      setCreating(false);
    }
  };

  return (
    <Box>
      {/* Hidden, always-mounted input controlled by labels */}
      <input
        id={inputId}
        ref={fileInputRef}
        key={fileKey}
        type="file"
        accept=".yaml,.yml,.json,.yamal"
        style={{ display: "none" }}
        onChange={(e) => {
          handleFiles(e.target.files);
          e.currentTarget.value = "";
        }}
      />

      {step === "upload" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Box
              component="label"
              htmlFor={inputId}
              onDragOver={(e) => e.preventDefault()}
              onDrop={onDrop}
              sx={{ display: "block", cursor: "pointer" }}
            >
              <Paper
                variant="outlined"
                sx={{
                  p: 4,
                  borderStyle: "dashed",
                  textAlign: "center",
                  minHeight: 280,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  position: "relative",
                  backgroundColor: "#f5f5f5ff",
                }}
              >
                {!rawSpec ? (
                  <Stack spacing={1} alignItems="center">
                    <Typography variant="h5" fontWeight={600}>
                      Upload API Contract
                    </Typography>
                    <Typography color="#aeacacff">
                      Drag &amp; Drop your files, click, or paste raw spec
                    </Typography>
                    <Button component="label" startIcon={<UploadRoundedIcon />} htmlFor={inputId}>
                      Upload
                    </Button>
                  </Stack>
                ) : (
                  <Stack spacing={2} alignItems="center">
                    <Typography variant="h6" fontWeight={700}>
                      {validating ? "Validating..." : "Uploaded file"}
                    </Typography>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Typography color="primary">{fileName}</Typography>
                      <IconButton
                        size="small"
                        color="error"
                        disabled={validating}
                        onClick={() => {
                          setRawSpec("");
                          setValidationResult(null);
                          setFileName("");
                          setError(null);
                          if (fileInputRef.current) fileInputRef.current.value = "";
                          setFileKey((k) => k + 1);
                        }}
                      >
                        <Delete fontSize="small" />
                      </IconButton>
                    </Stack>
                    {validating && (
                      <Typography variant="caption" color="text.secondary">
                        Validating OpenAPI definition...
                      </Typography>
                    )}
                  </Stack>
                )}
              </Paper>
            </Box>

            <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
              <Button variant="outlined" onClick={finishAndClose} sx={{ textTransform: "none" }}>
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

            {error && (
              <Alert severity="error" sx={{ mt: 2 }}>
                {error}
              </Alert>
            )}
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            <ApiOperationsList title="Fetched OAS Definition" operations={previewOps} />
          </Grid>
        </Grid>
      )}

      {step === "details" && (
        <Box>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, md: 6 }}>
              <Paper variant="outlined" sx={{ p: 3, borderRadius: 2 }}>
                <CreationMetaData scope="contract" title="API Details" />
                <Stack direction="row" spacing={1} justifyContent="flex-end" sx={{ mt: 3 }}>
                  <Button variant="outlined" onClick={() => setStep("upload")} sx={{ textTransform: "none" }}>
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
        </Box>
      )}
    </Box>
  );
};

export default UploadCreationFlow;

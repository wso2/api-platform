import * as React from "react";
import { Alert, Box, Grid, Paper, Stack, Typography } from "@mui/material";
import UploadRoundedIcon from "@mui/icons-material/UploadRounded";
import { Button } from "../../../../components/src/components/Button";
import { IconButton } from "../../../../components/src/components/IconButton";
import Delete from "../../../../components/src/Icons/generated/Delete";
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

type Step = "upload" | "details";

const UploadCreationFlow: React.FC<Props> = ({
  open,
  selectedProjectId,
  importOpenApi,
  refreshApis,
  onClose,
}) => {
  const [step, setStep] = React.useState<Step>("upload");
  const [uploadedFile, setUploadedFile] = React.useState<File | null>(null);
  const [validationResult, setValidationResult] =
    React.useState<OpenApiValidationResponse | null>(null);
  const [fileName, setFileName] = React.useState<string>("");
  const [error, setError] = React.useState<string | null>(null);
  const [validating, setValidating] = React.useState(false);
  const [creating, setCreating] = React.useState(false);

  const { contractMeta, setContractMeta, resetContractMeta } =
    useCreateComponentBuildpackContext();
  const { validateOpenApiFile } = useOpenApiValidation();
  const [metaHasErrors, setMetaHasErrors] = React.useState(false);

  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const inputId = React.useId();
  const [fileKey, setFileKey] = React.useState(0);
  const abortControllerRef = React.useRef<AbortController | null>(null);

  React.useEffect(() => {
    return () => {
      abortControllerRef.current?.abort();
    };
  }, []);

  React.useEffect(() => {
    if (open) {
      resetContractMeta();
      setStep("upload");
      setUploadedFile(null);
      setValidationResult(null);
      setFileName("");
      setError(null);
      setValidating(false);
      setMetaHasErrors(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
      setFileKey((k) => k + 1);
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

  const handleFiles = React.useCallback(
    async (files: FileList | null) => {
      if (!files || !files[0]) return;
      if (validating) return;

      const file = files[0];

      abortControllerRef.current?.abort();
      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      try {
        setError(null);
        setValidating(true);
        setValidationResult(null);

        setUploadedFile(file);
        setFileName(file.name);

        const result = await validateOpenApiFile(file, {
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
        setError(e?.message || "Failed to validate OpenAPI definition");
        setValidationResult(null);
      } finally {
        setValidating(false);
        if (fileInputRef.current) fileInputRef.current.value = "";
        setFileKey((k) => k + 1);
      }
    },
    [validating, autoFill, validateOpenApiFile]
  );

  const onDrop = (e: React.DragEvent<HTMLLabelElement>) => {
    e.preventDefault();
    handleFiles(e.dataTransfer.files);
  };

  const finishAndClose = React.useCallback(() => {
    abortControllerRef.current?.abort();
    resetContractMeta();
    setStep("upload");
    setUploadedFile(null);
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
      ((contractMeta as any)?.identifier || "").trim() ||
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
    if (!validationResult?.isAPIDefinitionValid || !uploadedFile) {
      setError("Please upload a valid OpenAPI definition.");
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
      definition: uploadedFile,
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

  return (
    <Box>
      <input
        id={inputId}
        ref={fileInputRef}
        key={fileKey}
        type="file"
        accept=".yaml,.yml,.json"
        style={{ display: "none" }}
        disabled={validating}
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
              sx={{
                display: "block",
                cursor: validating ? "not-allowed" : "pointer",
                opacity: validating ? 0.6 : 1,
              }}
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
                {!uploadedFile ? (
                  <Stack spacing={1} alignItems="center">
                    {validating ? (
                      <Typography variant="h5" fontWeight={600}>
                        Validating API Contract...
                      </Typography>
                    ) : (
                      <>
                        <Typography variant="h5" fontWeight={600}>
                          Upload API Contract
                        </Typography>
                        <Typography color="#aeacacff">
                          Drag &amp; drop your file or click to upload
                        </Typography>
                        <Button
                          component="label"
                          startIcon={<UploadRoundedIcon />}
                          htmlFor={inputId}
                        >
                          Upload
                        </Button>
                      </>
                    )}
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
                          setUploadedFile(null);
                          setValidationResult(null);
                          setFileName("");
                          setError(null);
                          if (fileInputRef.current)
                            fileInputRef.current.value = "";
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

      {step === "details" && (
        <Box>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, md: 6 }}>
              <CreationMetaData
                scope="contract"
                title="API Details"
                onValidationChange={({ hasError }) =>
                  setMetaHasErrors(hasError)
                }
              />

              <Stack
                direction="row"
                spacing={1}
                justifyContent="flex-start"
                sx={{ mt: 3 }}
              >
                <Button
                  variant="outlined"
                  onClick={() => setStep("upload")}
                  sx={{ textTransform: "none" }}
                >
                  Back
                </Button>
                <Button
                  variant="contained"
                  disabled={
                    creating ||
                    metaHasErrors ||
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
            </Grid>

            <Grid size={{ xs: 12, md: 6 }}>
              <ApiOperationsList
                title="Fetched OAS Definition"
                operations={previewOps}
              />
            </Grid>
          </Grid>
        </Box>
      )}
    </Box>
  );
};

export default UploadCreationFlow;

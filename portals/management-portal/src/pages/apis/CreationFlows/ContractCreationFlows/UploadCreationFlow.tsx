import * as React from "react";
import { Alert, Box, Grid, Paper, Stack, Typography } from "@mui/material";
import UploadRoundedIcon from "@mui/icons-material/UploadRounded";
import { Button } from "../../../../components/src/components/Button";
import yaml from "js-yaml";
import { IconButton } from "../../../../components/src/components/IconButton";
import Delete from "../../../../components/src/Icons/generated/Delete";
import CreationMetaData from "../CreationMetaData";
import {
  useCreateComponentBuildpackContext,
} from "../../../../context/CreateComponentBuildpackContext";

// NEW shared list & helper
import {
  ApiOperationsList,
  buildOperationsFromOpenAPI,
  type OpenAPI as SharedOpenAPI,
} from "../,,/../../../../components/src/components/Common/ApiOperationsList";

/* ---------- Types ---------- */
type Props = {
  open: boolean;
  selectedProjectId: string;
  createApi: (payload: {
    name: string;
    context: string;
    version: string;
    description?: string;
    projectId: string;
    contract?: string;
    backendServices?: Array<{
      name: string;
      isDefault?: boolean;
      endpoints: Array<{ url: string; description?: string }>;
      retries?: number;
    }>;
    operations?: Array<{
      name: string;
      description?: string;
      request: {
        method: string;
        path: string;
        ["backend-services"]?: Array<{ name: string }>;
      };
    }>;
  }) => Promise<any>;
  onClose: () => void;
};

type Step = "upload" | "details";

// reuse shared OpenAPI type
type OpenAPI = SharedOpenAPI;

/* ---------- helpers ---------- */

function slugify(val: string) {
  return val.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}
function defaultServiceName(apiName: string) {
  const base = apiName?.trim() || "service";
  return `${slugify(base)}-service`;
}
function firstServerUrl(doc?: OpenAPI) {
  const u = doc?.servers?.find((s) => !!s?.url)?.url?.trim();
  return u || "";
}
function deriveContext(doc: OpenAPI, fallbackTitle?: string) {
  const urlStr = doc?.servers?.[0]?.url;
  if (urlStr) {
    try {
      const u = new URL(urlStr, "https://placeholder.local");
      const p = u.pathname || "/";
      const segs = p.split("/").filter(Boolean);
      if (segs.length > 0) return `/${segs[0]}`;
      return "/";
    } catch {
      if (urlStr.startsWith("/")) return urlStr;
    }
  }
  const t = fallbackTitle ? slugify(fallbackTitle) : "api";
  return `/${t}`;
}

/* ---------- component ---------- */
const UploadCreationFlow: React.FC<Props> = ({ open, selectedProjectId, createApi, onClose }) => {
  const [step, setStep] = React.useState<Step>("upload");
  const [rawSpec, setRawSpec] = React.useState<string>("");
  const [doc, setDoc] = React.useState<OpenAPI | undefined>(undefined);
  const [fileName, setFileName] = React.useState<string>("");
  const [error, setError] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);

  const { contractMeta, setContractMeta, resetContractMeta } = useCreateComponentBuildpackContext();

  // Always-mounted input + stable id/label wiring
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const inputId = React.useId();
  const [fileKey, setFileKey] = React.useState(0);

  React.useEffect(() => {
    if (open) {
      resetContractMeta();
      setStep("upload");
      setRawSpec("");
      setDoc(undefined);
      setFileName("");
      setError(null);
      if (fileInputRef.current) fileInputRef.current.value = "";
      setFileKey((k) => k + 1);
    }
  }, [open, resetContractMeta]);

  const parseSpec = React.useCallback((text: string) => {
    try {
      const trimmed = text.trim();
      if (!trimmed) throw new Error("Empty definition");
      let parsed: any;
      if (trimmed.startsWith("{")) parsed = JSON.parse(trimmed);
      else parsed = yaml.load(text);
      if (!parsed || typeof parsed !== "object") throw new Error("Invalid OpenAPI definition");
      return parsed as OpenAPI;
    } catch (e: any) {
      throw new Error(e?.message || "Invalid OpenAPI definition");
    }
  }, []);

  const autoFill = React.useCallback((d: OpenAPI) => {
    const title = d?.info?.title?.trim() || "";
    const version = d?.info?.version?.trim() || "1.0.0";
    const description = d?.info?.description || "";
    const targetUrl = firstServerUrl(d);

    setContractMeta((prev: any) => ({
      ...prev,
      name: title || prev?.name || "Sample API",
      version,
      description,
      context: deriveContext(d, title),
      target: prev?.target || targetUrl || "",
    }));
  }, [setContractMeta]);

  const handleFiles = React.useCallback(
    async (files: FileList | null) => {
      if (!files || !files[0]) return;
      const f = files[0];
      try {
        setError(null);
        const text = await f.text();
        const parsed = parseSpec(text);
        setRawSpec(text);
        setDoc(parsed);
        setFileName(f.name);
        autoFill(parsed);
      } catch (e: any) {
        setError(e?.message || "OpenAPI Definition validation failed");
      } finally {
        if (fileInputRef.current) fileInputRef.current.value = "";
        setFileKey((k) => k + 1);
      }
    },
    [autoFill, parseSpec]
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
    setDoc(undefined);
    setFileName("");
    setError(null);
    onClose();
  }, [onClose, resetContractMeta]);

  // Preview operations built from OAS (no backend-service binding here)
  const previewOps = React.useMemo(() => buildOperationsFromOpenAPI(doc), [doc]);

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

      // Build operations and bind to backend-service if present
      const operations = buildOperationsFromOpenAPI(doc, serviceName);

      await createApi({
        name,
        context: context.startsWith("/") ? context : `/${context}`,
        version,
        description,
        projectId: selectedProjectId,
        contract: rawSpec,
        backendServices,
        operations,
      });

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
                      Uploaded file
                    </Typography>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Typography color="primary">{fileName}</Typography>
                      <IconButton
                        size="small"
                        color="error"
                        onClick={() => {
                          setRawSpec("");
                          setDoc(undefined);
                          setFileName("");
                          setError(null);
                          if (fileInputRef.current) fileInputRef.current.value = "";
                          setFileKey((k) => k + 1);
                        }}
                      >
                        <Delete fontSize="small" />
                      </IconButton>
                    </Stack>
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
                disabled={!rawSpec}
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
